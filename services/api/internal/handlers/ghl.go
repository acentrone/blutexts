package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/bluesend/api/internal/middleware"
	"github.com/bluesend/api/internal/models"
	"github.com/bluesend/api/internal/services/audit"
	"github.com/bluesend/api/internal/services/ghl"
	"github.com/bluesend/api/internal/services/messaging"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type GHLHandler struct {
	db            *pgxpool.Pool
	provisioner   *ghl.Provisioner
	msgRouter     *messaging.Router
	appURL        string
	webhookSecret string
}

func NewGHLHandler(db *pgxpool.Pool, provisioner *ghl.Provisioner, msgRouter *messaging.Router, appURL, webhookSecret string) *GHLHandler {
	return &GHLHandler{
		db:            db,
		provisioner:   provisioner,
		msgRouter:     msgRouter,
		appURL:        appURL,
		webhookSecret: webhookSecret,
	}
}

// verifyWebhookSignature checks the GHL HMAC-SHA256 signature on a webhook
// body. GHL signs request bodies with the shared secret and sends the hex
// digest in either `x-wh-signature` or `x-ghl-signature`. Returns true when
// at least one of those headers matches; false otherwise.
//
// Constant-time compare to prevent timing attacks. We accept either header
// to be defensive against GHL changing the canonical name (it has shifted
// historically). Empty signature → reject.
func (h *GHLHandler) verifyWebhookSignature(body []byte, r *http.Request) bool {
	if h.webhookSecret == "" {
		// Misconfiguration — fail closed rather than open.
		return false
	}
	provided := r.Header.Get("x-wh-signature")
	if provided == "" {
		provided = r.Header.Get("x-ghl-signature")
	}
	if provided == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(h.webhookSecret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(provided))
}

// GET /api/oauth/connect — redirect to GHL OAuth (called from dashboard after signup)
func (h *GHLHandler) InitiateOAuth(w http.ResponseWriter, r *http.Request) {
	accountIDStr := r.URL.Query().Get("account_id")
	if accountIDStr == "" {
		writeError(w, "account_id required", http.StatusBadRequest)
		return
	}
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		writeError(w, "invalid account_id", http.StatusBadRequest)
		return
	}

	oauthURL := h.provisioner.GenerateOAuthURL(accountID)
	writeJSON(w, map[string]string{"url": oauthURL}, http.StatusOK)
}

// GET /api/oauth/callback — GHL redirects here after user authorizes
func (h *GHLHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state") // accountID
	if code == "" || state == "" {
		http.Redirect(w, r, h.appURL+"/dashboard?ghl=error", http.StatusTemporaryRedirect)
		return
	}

	accountID, err := uuid.Parse(state)
	if err != nil {
		http.Redirect(w, r, h.appURL+"/dashboard?ghl=error", http.StatusTemporaryRedirect)
		return
	}

	redirectURI := h.appURL + "/api/oauth/callback"
	conn, err := h.provisioner.CompleteOAuth(r.Context(), accountID, code, redirectURI)
	if err != nil {
		log.Printf("GHL OAuth error for account %s: %v", accountID, err)
		http.Redirect(w, r, h.appURL+"/dashboard?ghl=error", http.StatusTemporaryRedirect)
		return
	}

	log.Printf("GHL connected for account %s, location %s", accountID, conn.LocationID)
	audit.Log(r.Context(), h.db, accountID, uuid.Nil, "ghl.connected", "account", accountID.String(),
		map[string]interface{}{"location_id": conn.LocationID}, r.RemoteAddr)

	http.Redirect(w, r, h.appURL+"/dashboard?ghl=connected", http.StatusTemporaryRedirect)
}

// POST /api/webhooks/inbound — GHL conversation provider delivery webhook.
// Verifies the HMAC signature first; without this anyone with the URL
// could forge events and trigger arbitrary outbound iMessage sends from
// any connected customer's number.
func (h *GHLHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	if !h.verifyWebhookSignature(body, r) {
		log.Printf("GHL webhook: signature verification failed (ip=%s)", r.RemoteAddr)
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	// GHL conversation provider webhook has the fields at top level
	var event struct {
		Type                   string `json:"type"`
		Source                 string `json:"source"`
		LocationID             string `json:"locationId"`
		ContactID              string `json:"contactId"`
		ConversationID         string `json:"conversationId"`
		MessageID              string `json:"messageId"`
		Message                string `json:"message"`
		Body                   string `json:"body"`
		Phone                  string `json:"phone"`
		ConversationProviderID string `json:"conversationProviderId"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	// Log the event
	h.db.Exec(r.Context(), `
		INSERT INTO ghl_webhook_events (id, location_id, event_type, payload, created_at)
		VALUES (uuid_generate_v4(), $1, $2, $3, NOW())
	`, event.LocationID, event.Type, body)

	// Ignore echoes from our own API sync (prevents infinite loops)
	if event.Source == "api" || event.Type == "OutboundMessage" || event.Type == "InboundMessage" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Dedupe by GHL message ID
	if event.MessageID != "" {
		var exists bool
		_ = h.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM messages WHERE ghl_message_id = $1)`,
			event.MessageID).Scan(&exists)
		if exists {
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	// Determine the message content (some payloads use "message", others use "body")
	content := event.Message
	if content == "" {
		content = event.Body
	}

	// Route outbound messages from GHL to the device agent for iMessage delivery.
	if event.Phone != "" && content != "" && event.ConversationProviderID != "" {
		log.Printf("GHL outbound: to=%s msg=%.60s", event.Phone, content)
		go func() {
			bgCtx := context.Background()

			// Find the account and its active phone number linked to this GHL location
			var accountID uuid.UUID
			var phoneNumberID string
			err := h.db.QueryRow(bgCtx, `
				SELECT a.id, pn.id::text
				FROM accounts a
				JOIN ghl_connections gc ON gc.account_id = a.id
				JOIN phone_numbers pn ON pn.account_id = a.id AND pn.status = 'active'
				WHERE gc.location_id = $1
				LIMIT 1
			`, event.LocationID).Scan(&accountID, &phoneNumberID)
			if err != nil {
				log.Printf("GHL outbound: account/number not found for location %s: %v", event.LocationID, err)
				return
			}

			_, err = h.msgRouter.Send(bgCtx, &models.SendMessageRequest{
				PhoneNumberID: phoneNumberID,
				ToAddress:     event.Phone,
				Content:       content,
				GHLMessageID:  event.MessageID,
			}, accountID)
			if err != nil {
				log.Printf("GHL outbound send error: %v", err)
				audit.Log(bgCtx, h.db, accountID, uuid.Nil, "message.send_failed", "message", "",
					map[string]interface{}{
						"to":    event.Phone,
						"error": err.Error(),
					}, "")
			} else {
				audit.Log(bgCtx, h.db, accountID, uuid.Nil, "message.sent_from_ghl", "message", "",
					map[string]interface{}{
						"to":      event.Phone,
						"preview": truncate60(content),
					}, "")
			}
		}()
	}

	w.WriteHeader(http.StatusOK)
}

// DELETE /api/integration/disconnect — disconnect GHL for the authenticated
// caller's account. Account ID is taken from the JWT, NOT from URL params,
// because the previous version let any authenticated user disconnect any
// other customer's GHL integration by guessing UUIDs.
func (h *GHLHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	accountID, ok := middleware.GetAccountID(r.Context())
	if !ok {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	h.db.Exec(r.Context(), `DELETE FROM ghl_connections WHERE account_id = $1`, accountID)
	h.db.Exec(r.Context(), `UPDATE accounts SET ghl_location_id = NULL WHERE id = $1`, accountID)

	userID, _ := middleware.GetUserID(r.Context())
	audit.Log(r.Context(), h.db, accountID, userID, "ghl.disconnected_by_user", "account", accountID.String(), nil, r.RemoteAddr)

	writeJSON(w, map[string]string{"status": "disconnected"}, http.StatusOK)
}

// GET /api/integration/status — check connection status for the authenticated
// caller's account (JWT-scoped, not URL-scoped — see Disconnect comment).
func (h *GHLHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	accountID, ok := middleware.GetAccountID(r.Context())
	if !ok {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var locationID string
	var connected bool
	err := h.db.QueryRow(r.Context(), `
		SELECT location_id, connected FROM ghl_connections WHERE account_id = $1
	`, accountID).Scan(&locationID, &connected)

	if err != nil {
		writeJSON(w, map[string]interface{}{"connected": false}, http.StatusOK)
		return
	}

	writeJSON(w, map[string]interface{}{
		"connected":   connected,
		"location_id": locationID,
	}, http.StatusOK)
}

func truncate60(s string) string {
	if len(s) <= 60 {
		return s
	}
	return s[:60] + "…"
}
