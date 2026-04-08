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
	"github.com/bluesend/api/internal/services/ghl"
	"github.com/bluesend/api/internal/services/messaging"
	stripeService "github.com/bluesend/api/internal/services/stripe"
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

	// ── Services ──────────────────────────────────────────────
	ghlClient := ghl.NewClient(cfg.GHLClientID, cfg.GHLClientSecret)
	ghlProvisioner := ghl.NewProvisioner(pool, ghlClient, cfg.AppURL)
	ghlSyncer := ghl.NewSyncer(pool, ghlClient)

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

		onAccountActivated := func(ctx context.Context, accountID uuid.UUID) {
			log.Printf("Account activated: %s — triggering provisioning", accountID)
			pool.Exec(ctx, `UPDATE accounts SET status = 'active', updated_at = NOW() WHERE id = $1`, accountID)
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
	adminKeyMw := middleware.NewAdminKeyAuth(cfg.AdminAPIKey)

	authHandler := handlers.NewAuthHandler(pool, cfg.JWTSecret, cfg.JWTRefreshSecret)
	messageHandler := handlers.NewMessageHandler(pool, msgRouter)
	billingHandler := handlers.NewBillingHandler(pool, billing, cfg.AppURL)
	adminHandler := handlers.NewAdminHandler(pool)
	deviceHandler := handlers.NewDeviceHandler(pool, deviceHub, clientHub, msgRouter)
	ghlHandler := handlers.NewGHLHandler(pool, ghlProvisioner, ghlSyncer, cfg.GHLWebhookSecret, cfg.AppURL)

	// ── Start Asynq worker ────────────────────────────────────
	go startWorker(asynqOpts, ghlSyncer)

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

	// ── Auth ──────────────────────────────────────────────────
	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/register", authHandler.Register)
		r.Post("/login", authHandler.Login)
		r.Post("/refresh", authHandler.Refresh)
		r.Post("/logout", authHandler.Logout)
		r.With(authMw.Authenticate).Get("/me", authHandler.Me)
	})

	// ── Authenticated user routes ─────────────────────────────
	r.Route("/api", func(r chi.Router) {
		r.Use(authMw.Authenticate)

		r.Post("/messages/send", messageHandler.Send)
		r.Get("/conversations", messageHandler.ListConversations)
		r.Get("/conversations/{conversationID}/messages", messageHandler.GetMessages)
		r.Get("/dashboard/stats", messageHandler.GetDashboardStats)
		r.Get("/messages/export", messageHandler.ExportCSV)

		r.Post("/billing/checkout", billingHandler.CreateCheckout)
		r.Post("/billing/portal", billingHandler.CreatePortalSession)
		r.Get("/billing/invoices", billingHandler.ListInvoices)
		r.Get("/billing/subscription", billingHandler.GetSubscription)

		r.Get("/oauth/connect", ghlHandler.InitiateOAuth)
		r.Get("/integration/status", ghlHandler.GetStatus)

		r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
			accountID, ok := middleware.GetAccountID(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			clientHub.ServeClient(w, r, accountID)
		})
	})

	// ── OAuth callbacks (no JWT — external redirects) ─────────
	r.Get("/api/oauth/callback", ghlHandler.OAuthCallback)

	// ── Webhooks (signature-verified, no JWT) ─────────────────
	if stripeWebhookHandler != nil {
		r.Post("/api/webhooks/stripe", stripeWebhookHandler.Handle)
	}
	r.Post("/api/webhooks/inbound", ghlHandler.HandleWebhook)

	// ── Admin (API key auth) ──────────────────────────────────
	r.Route("/api/admin", func(r chi.Router) {
		r.Use(adminKeyMw.Middleware)
		r.Get("/stats", adminHandler.GetSystemStats)
		r.Get("/accounts", adminHandler.ListAccounts)
		r.Patch("/accounts/{accountID}/status", adminHandler.UpdateAccountStatus)
		r.Post("/accounts/{accountID}/assign-number", adminHandler.AssignPhoneNumber)
		r.Get("/devices", adminHandler.ListDevices)
		r.Post("/devices/register", adminHandler.RegisterDevice)
		r.Patch("/devices/{deviceID}", adminHandler.UpdateDevice)
		r.Get("/audit-log", adminHandler.GetAuditLog)
	})

	// ── Device WebSocket (device token auth) ──────────────────
	r.Get("/api/devices/connect", deviceHandler.Connect)

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

func startWorker(redisOpts asynq.RedisClientOpt, syncer *ghl.Syncer) {
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
		return syncer.SyncMessageToGHL(ctx, msgID, accountID)
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

// Ensure pool is available in worker for future expansion
var _ *pgxpool.Pool
