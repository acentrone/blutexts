package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bluesend/api/internal/config"
	"github.com/bluesend/api/internal/db"
	"github.com/bluesend/api/internal/handlers"
	"github.com/bluesend/api/internal/middleware"
	"github.com/bluesend/api/internal/models"
	"github.com/bluesend/api/internal/services/crypto"
	emailService "github.com/bluesend/api/internal/services/email"
	"github.com/bluesend/api/internal/services/ghl"
	"github.com/bluesend/api/internal/services/messaging"
	"github.com/bluesend/api/internal/services/storage"
	stripeService "github.com/bluesend/api/internal/services/stripe"
	"github.com/bluesend/api/internal/services/voice"
	ws "github.com/bluesend/api/internal/websocket"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	// ── Database ──────────────────────────────────────────────
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection error: %v", err)
	}
	defer pool.Close()
	log.Println("Connected to PostgreSQL")

	// ── Redis ─────────────────────────────────────────────────
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis URL parse error: %v", err)
	}
	rdb := redis.NewClient(redisOpts)
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("redis connection error: %v", err)
	}
	defer rdb.Close()
	log.Println("Connected to Redis")

	// ── Asynq (job queue) ─────────────────────────────────────
	asynqOpts := asynq.RedisClientOpt{
		Addr:     redisOpts.Addr,
		Password: redisOpts.Password,
		DB:       redisOpts.DB,
	}
	asynqClient := asynq.NewClient(asynqOpts)
	defer asynqClient.Close()

	// ── WebSocket Hubs ────────────────────────────────────────
	deviceHub := ws.NewDeviceHub()
	clientHub := ws.NewClientHub()

	// Browser-side WS upgrades come from the dashboard origin only. Native
	// device-agent connections set no Origin and bypass this list (auth is
	// the bearer token on /api/devices/connect).
	ws.SetAllowedOrigins([]string{cfg.AppURL, "http://localhost:3000"})

	// ── Services ──────────────────────────────────────────────
	encryptor, err := crypto.New(cfg.EncryptionKey)
	if err != nil {
		log.Fatalf("invalid ENCRYPTION_KEY: %v", err)
	}
	if encryptor == nil {
		log.Println("WARNING: ENCRYPTION_KEY not set — GHL OAuth tokens will be stored in plaintext at rest")
	} else {
		log.Println("At-rest encryption enabled (AES-256-GCM)")
	}

	ghlClient := ghl.NewClient(cfg.GHLClientID, cfg.GHLClientSecret)
	ghlProvisioner := ghl.NewProvisioner(pool, ghlClient, cfg.AppURL, encryptor)
	ghlSyncer := ghl.NewSyncer(pool, ghlClient, encryptor)

	// Email service is initialized BEFORE Stripe so the onAccountActivated
	// closure (defined below) can capture it. Optional — when RESEND_API_KEY
	// is empty the service logs to stdout instead of sending.
	emailSvc := emailService.New(cfg.ResendAPIKey, cfg.FromEmail, cfg.AppURL)
	if emailSvc.Enabled() {
		log.Println("Resend email enabled")
	} else {
		log.Println("Resend email disabled — logging emails to stdout")
	}

	// Stripe is optional — skip in testing/free mode
	var billing *stripeService.BillingService
	var stripeWebhookHandler *stripeService.WebhookHandler
	if cfg.StripeSecretKey != "" {
		stripePlans := stripeService.Plans{
			Setup:   cfg.StripePriceSetup,
			Monthly: cfg.StripePriceMonthly,
			Annual:  cfg.StripePriceAnnual,
		}
		billing = stripeService.NewBillingService(cfg.StripeSecretKey, stripePlans, cfg.AppURL)

		// Fired by stripe webhook on checkout.session.completed AFTER it has
		// already flipped the account to status='setting_up'. We do the
		// "tell the customer + page the founder" side-effects here.
		//
		// Number provisioning + iPhone assignment is a manual ops step
		// (BluTexts hosts the iPhone fleet that backs every customer line)
		// — the email to centroneaj@gmail.com is the page so nobody is
		// left waiting in an empty dashboard.
		onAccountActivated := func(ctx context.Context, accountID uuid.UUID) {
			log.Printf("Account activated: %s — sending welcome + provisioning alert", accountID)

			var customerEmail, firstName, company, areaCode string
			err := pool.QueryRow(ctx, `
				SELECT
				  u.email,
				  u.first_name,
				  a.name,
				  COALESCE(a.preferred_area_code, '')
				FROM accounts a
				JOIN users u ON u.account_id = a.id AND u.role = 'owner'
				WHERE a.id = $1
				LIMIT 1
			`, accountID).Scan(&customerEmail, &firstName, &company, &areaCode)
			if err != nil {
				log.Printf("onAccountActivated lookup failed for %s: %v", accountID, err)
				return
			}

			// Welcome the customer (best-effort — failure is logged, not fatal)
			if err := emailSvc.SendWelcome(customerEmail, firstName); err != nil {
				log.Printf("welcome email failed for %s: %v", customerEmail, err)
			}

			// Page the founder so the manual number-assignment step happens
			fullName := firstName
			if fullName == "" {
				fullName = customerEmail
			}
			if err := emailSvc.SendProvisioningAlert(
				cfg.OpsAlertEmail,
				customerEmail, fullName, company, areaCode,
				accountID.String(),
			); err != nil {
				log.Printf("provisioning alert failed for %s: %v", accountID, err)
			}
		}
		stripeWebhookHandler = stripeService.NewWebhookHandler(
			pool, cfg.StripeWebhookSecret, billing, onAccountActivated,
		)
		log.Println("Stripe billing enabled")
	} else {
		log.Println("Stripe billing disabled — free mode")
	}

	msgRouter := messaging.NewRouter(pool, rdb, deviceHub, asynqClient)

	// ── HTTP Handlers ─────────────────────────────────────────
	authMw := middleware.NewAuthMiddleware(cfg.JWTSecret)
	adminKeyMw := middleware.NewAdminAuth(cfg.AdminAPIKey, authMw)
	subGate := middleware.NewSubscriptionGate(pool)
	rateLimit := middleware.NewRateLimiter(rdb)

	// Storage (R2 — optional; media features are disabled if not configured)
	r2Client := storage.NewR2Client()
	if r2Client != nil {
		log.Println("R2 storage configured — media uploads enabled")
	} else {
		log.Println("R2 storage not configured — media uploads disabled")
	}

	// Voice (Agora token service + FaceTime Audio bridge). nil when unconfigured.
	voiceSvc := voice.New(cfg.AgoraAppID, cfg.AgoraAppCertificate)
	if voiceSvc.Enabled() {
		log.Println("Agora voice configured — FaceTime Audio calling enabled")
	} else {
		log.Println("Agora voice not configured — calling disabled")
	}

	authHandler := handlers.NewAuthHandler(pool, cfg.JWTSecret, cfg.JWTRefreshSecret, emailSvc)
	messageHandler := handlers.NewMessageHandler(pool, msgRouter, r2Client)
	billingHandler := handlers.NewBillingHandler(pool, billing, cfg.AppURL)
	adminHandler := handlers.NewAdminHandler(pool)
	deviceHandler := handlers.NewDeviceHandler(pool, deviceHub, clientHub, msgRouter, r2Client)
	ghlHandler := handlers.NewGHLHandler(pool, ghlProvisioner, msgRouter, cfg.AppURL, cfg.GHLWebhookSecret)
	contactHandler := handlers.NewContactHandler(pool)
	callHandler := handlers.NewCallHandler(pool, voiceSvc, deviceHub)
	settingsHandler := handlers.NewSettingsHandler(pool)
	scheduledHandler := handlers.NewScheduledHandler(pool, asynqClient)
	teamHandler := handlers.NewTeamHandler(pool, emailSvc, cfg.JWTSecret, cfg.JWTRefreshSecret)

	// ── Start Asynq worker ────────────────────────────────────
	go startWorker(asynqOpts, ghlSyncer, pool, msgRouter)

	// ── HTTP Router ───────────────────────────────────────────
	r := chi.NewRouter()

	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.AppURL, "http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Admin-Key"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// ── Agent version check (public, no auth) ────────────────
	r.Get("/api/agent/version", func(w http.ResponseWriter, r *http.Request) {
		current := r.URL.Query().Get("current")

		// Read latest version from DB (or hardcode for now)
		var version, downloadURL, notes string
		var required bool
		err := pool.QueryRow(r.Context(), `
			SELECT version, download_url, notes, required
			FROM agent_releases
			WHERE active = true
			ORDER BY created_at DESC LIMIT 1
		`).Scan(&version, &downloadURL, &notes, &required)
		if err != nil || version == "" || version == current {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"version":      version,
			"download_url": downloadURL,
			"notes":        notes,
			"required":     required,
		})
	})

	// ── Auth ──────────────────────────────────────────────────
	// Rate limits below are intentionally generous for legit users + tight
	// enough that abuse hits a wall fast. Numbers picked from observed
	// Auth0/Clerk defaults + adjusted for our self-serve volume.
	//
	//   register   : 5/IP/15min   — typical user signs up once, ever
	//   login      : 10/IP/15min  — leaves room for typo-fest, blocks brute force
	//   forgot-pwd : 5/IP/hour    + 3/email/hour (per-email blocks reset spam)
	//   webhook    : 60/IP/min    — GHL bursts on bulk operations; unauth path
	r.Route("/api/auth", func(r chi.Router) {
		r.With(rateLimit.Limit(5, 15*time.Minute, middleware.IPKey)).
			Post("/register", authHandler.Register)
		r.With(rateLimit.Limit(10, 15*time.Minute, middleware.IPKey)).
			Post("/login", authHandler.Login)
		r.Post("/refresh", authHandler.Refresh)
		r.Post("/logout", authHandler.Logout)
		r.With(
			rateLimit.Limit(5, time.Hour, middleware.IPKey),
			rateLimit.LimitByEmail(3, time.Hour),
		).Post("/forgot-password", authHandler.ForgotPassword)
		r.Post("/reset-password", authHandler.ResetPassword)
		r.Post("/accept-invite", teamHandler.AcceptInvite)
		r.With(authMw.Authenticate).Get("/me", authHandler.Me)
	})

	// ── Authenticated user routes ─────────────────────────────
	r.Route("/api", func(r chi.Router) {
		r.Use(authMw.Authenticate)

		// ── Always accessible (no subscription check) ─────────
		// Billing must remain open so users can reactivate / manage their plan.
		// /me and /ws are needed for the dashboard shell to render.
		r.Post("/billing/checkout", billingHandler.CreateCheckout)
		r.Post("/billing/portal", billingHandler.CreatePortalSession)
		r.Get("/billing/invoices", billingHandler.ListInvoices)
		r.Get("/billing/subscription", billingHandler.GetSubscription)

		r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
			accountID, ok := middleware.GetAccountID(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			clientHub.ServeClient(w, r, accountID)
		})

		// ── Subscription-gated routes ─────────────────────────
		// Everything below requires an active (or setting_up) account.
		// Cancelled, past-due, or pending accounts get a 402 with a
		// machine-readable error so the frontend can show a paywall.
		r.Group(func(r chi.Router) {
			r.Use(subGate.Require)

			r.Post("/messages/send", messageHandler.Send)
			r.Post("/messages/upload", messageHandler.Upload)

			// FaceTime Audio calling via Agora bridge
			r.Post("/calls/start", callHandler.Start)
			r.Post("/calls/{callID}/end", callHandler.End)
			r.Get("/calls", callHandler.List)

			r.Get("/contacts", contactHandler.List)
			r.Post("/contacts", contactHandler.Create)
			r.Get("/contacts/tags", contactHandler.ListTags)
			r.Get("/contacts/{contactID}", contactHandler.Get)
			r.Patch("/contacts/{contactID}", contactHandler.Update)
			r.Delete("/contacts/{contactID}", contactHandler.Delete)

			r.Get("/phone-numbers", contactHandler.ListPhoneNumbers)

			r.Get("/conversations", messageHandler.ListConversations)
			r.Get("/conversations/{conversationID}/messages", messageHandler.GetMessages)
			r.Get("/dashboard/stats", messageHandler.GetDashboardStats)
			r.Get("/account/info", messageHandler.GetAccountInfo)
			r.Get("/messages/export", messageHandler.ExportCSV)

			// Scheduled messages
			r.Post("/messages/schedule", scheduledHandler.Create)
			r.Get("/messages/scheduled", scheduledHandler.List)
			r.Delete("/messages/scheduled/{scheduledID}", scheduledHandler.Cancel)

			// Account settings & custom fields
			r.Get("/account/settings", settingsHandler.GetSettings)
			r.Patch("/account/settings", settingsHandler.UpdateSettings)
			r.Get("/account/custom-fields", settingsHandler.GetCustomFields)
			r.Put("/account/custom-fields", settingsHandler.UpdateCustomFields)

			// Team members
			r.Get("/team/members", teamHandler.ListMembers)
			r.Post("/team/invite", teamHandler.Invite)
			r.Get("/team/invitations", teamHandler.ListInvitations)
			r.Delete("/team/invitations/{invitationID}", teamHandler.RevokeInvitation)
			r.Delete("/team/members/{userID}", teamHandler.RemoveMember)
			r.Patch("/team/members/{userID}/role", teamHandler.UpdateRole)

			r.Get("/oauth/connect", ghlHandler.InitiateOAuth)
			r.Get("/integration/status", ghlHandler.GetStatus)
			r.Delete("/integration/disconnect", ghlHandler.Disconnect)
		})
	})

	// ── OAuth callbacks (no JWT — external redirects) ─────────
	r.Get("/api/oauth/callback", ghlHandler.OAuthCallback)

	// ── Webhooks (signature-verified, no JWT) ─────────────────
	if stripeWebhookHandler != nil {
		r.Post("/api/webhooks/stripe", stripeWebhookHandler.Handle)
	}
	// Inbound is unauthenticated (GHL signs with HMAC, verified inside the
	// handler) — rate-limit per-IP so a misbehaving sender can't flood.
	// Stripe webhooks aren't limited because Stripe's IPs are well-known
	// and they retry aggressively on 429.
	r.With(rateLimit.Limit(60, time.Minute, middleware.IPKey)).
		Post("/api/webhooks/inbound", ghlHandler.HandleWebhook)

	// ── Admin (API key auth) ──────────────────────────────────
	r.Route("/api/admin", func(r chi.Router) {
		r.Use(adminKeyMw.Middleware)
		r.Get("/stats", adminHandler.GetSystemStats)
		r.Get("/accounts", adminHandler.ListAccounts)
		r.Patch("/accounts/{accountID}/status", adminHandler.UpdateAccountStatus)
		r.Post("/accounts/{accountID}/cancel", adminHandler.CancelAccount)
		r.Post("/accounts/{accountID}/reinstate", adminHandler.ReinstateAccount)
		r.Patch("/accounts/{accountID}/auto-reply", adminHandler.UpdateAutoReply)
		r.Patch("/accounts/{accountID}/calling", adminHandler.UpdateAccountCalling)
		r.Patch("/phone-numbers/{numberID}/voice", adminHandler.UpdateNumberVoice)
		r.Get("/accounts/{accountID}/numbers", adminHandler.GetAccountNumbers)
		r.Get("/accounts/{accountID}/ghl", adminHandler.GetAccountGHL)
		r.Get("/accounts/{accountID}/audit-log", adminHandler.GetAccountAuditLog)
		r.Delete("/accounts/{accountID}/ghl", adminHandler.DisconnectAccountGHL)
		r.Post("/accounts/{accountID}/assign-number", adminHandler.AssignPhoneNumber)
		r.Delete("/phone-numbers/{numberID}", adminHandler.DeletePhoneNumber)
		r.Get("/number-health", adminHandler.GetNumberHealth)
		r.Patch("/phone-numbers/{numberID}/health", adminHandler.UpdateNumberHealth)
		r.Get("/audit-log", adminHandler.GetAuditLog)
		r.Get("/devices", adminHandler.ListDevices)
		r.Post("/devices/register", adminHandler.RegisterDevice)
		r.Patch("/devices/{deviceID}", adminHandler.UpdateDevice)
		r.Delete("/devices/{deviceID}", adminHandler.DeleteDevice)
		r.Post("/devices/{deviceID}/rotate-token", adminHandler.RotateDeviceToken)
	})

	// ── Device WebSocket + uploads (device token auth) ────────
	r.Get("/api/devices/connect", deviceHandler.Connect)
	r.Post("/api/devices/upload", deviceHandler.UploadAttachment)

	// ── Start server ──────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("BlueSend API listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-quit
	log.Println("Shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
	log.Println("Stopped")
}

func startWorker(redisOpts asynq.RedisClientOpt, syncer *ghl.Syncer, pool *pgxpool.Pool, msgRouter *messaging.Router) {
	srv := asynq.NewServer(redisOpts, asynq.Config{Concurrency: 10})
	mux := asynq.NewServeMux()

	mux.HandleFunc(messaging.TaskSyncToGHL, func(ctx context.Context, task *asynq.Task) error {
		var payload struct {
			MessageID string `json:"message_id"`
			AccountID string `json:"account_id"`
		}
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return err
		}
		msgID, err := uuid.Parse(payload.MessageID)
		if err != nil {
			return fmt.Errorf("invalid message_id: %w", err)
		}
		accountID, err := uuid.Parse(payload.AccountID)
		if err != nil {
			return fmt.Errorf("invalid account_id: %w", err)
		}
		if err := syncer.SyncMessageToGHL(ctx, msgID, accountID); err != nil {
			log.Printf("GHL sync error for message %s: %v", msgID, err)
			return err
		}
		log.Printf("GHL sync success for message %s", msgID)
		return nil
	})

	mux.HandleFunc(messaging.TaskSendScheduled, func(ctx context.Context, task *asynq.Task) error {
		var payload struct {
			ScheduledMessageID string `json:"scheduled_message_id"`
			AccountID          string `json:"account_id"`
		}
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return err
		}

		// Read the scheduled message
		var phoneNumberID, toAddress, content, effect string
		var attachmentsJSON []byte
		var status string
		err := pool.QueryRow(ctx, `
			SELECT phone_number_id::text, to_address, content, attachments, COALESCE(effect, ''), status
			FROM scheduled_messages WHERE id = $1
		`, payload.ScheduledMessageID).Scan(&phoneNumberID, &toAddress, &content, &attachmentsJSON, &effect, &status)
		if err != nil {
			return fmt.Errorf("scheduled message not found: %w", err)
		}
		if status != "pending" {
			log.Printf("Scheduled message %s already %s, skipping", payload.ScheduledMessageID, status)
			return nil
		}

		accountID, _ := uuid.Parse(payload.AccountID)
		req := &models.SendMessageRequest{
			PhoneNumberID: phoneNumberID,
			ToAddress:     toAddress,
			Content:       content,
			Effect:        effect,
		}
		json.Unmarshal(attachmentsJSON, &req.Attachments)

		resp, err := msgRouter.Send(ctx, req, accountID)
		if err != nil {
			pool.Exec(ctx, `UPDATE scheduled_messages SET status = 'failed', error_message = $1, updated_at = NOW() WHERE id = $2`,
				err.Error(), payload.ScheduledMessageID)
			return fmt.Errorf("scheduled send failed: %w", err)
		}
		if resp.RateLimited {
			pool.Exec(ctx, `UPDATE scheduled_messages SET status = 'failed', error_message = 'rate limited', updated_at = NOW() WHERE id = $1`,
				payload.ScheduledMessageID)
			return nil
		}

		pool.Exec(ctx, `UPDATE scheduled_messages SET status = 'sent', sent_at = NOW(), updated_at = NOW() WHERE id = $1`,
			payload.ScheduledMessageID)
		log.Printf("Scheduled message %s sent successfully", payload.ScheduledMessageID)
		return nil
	})

	mux.HandleFunc(messaging.TaskSyncContact, func(ctx context.Context, task *asynq.Task) error {
		var payload struct {
			ContactID string `json:"contact_id"`
			AccountID string `json:"account_id"`
		}
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return err
		}
		contactID, err := uuid.Parse(payload.ContactID)
		if err != nil {
			return fmt.Errorf("invalid contact_id: %w", err)
		}
		accountID, err := uuid.Parse(payload.AccountID)
		if err != nil {
			return fmt.Errorf("invalid account_id: %w", err)
		}
		return syncer.SyncContactToGHL(ctx, contactID, accountID)
	})

	if err := srv.Run(mux); err != nil {
		fmt.Fprintf(os.Stderr, "asynq worker error: %v\n", err)
	}
}

// Ensure types are available for compilation.
var _ *pgxpool.Pool
