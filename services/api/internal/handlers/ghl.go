package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/bluesend/api/internal/services/ghl"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type GHLHandler struct {
	db          *pgxpool.Pool
	provisioner *ghl.Provisioner
	syncer      *ghl.Syncer
	webhookSecret string
	appURL        string
}

func NewGHLHandler(db *pgxpool.Pool, provisioner *ghl.Provisioner, syncer *ghl.Syncer, webhookSecret, appURL string) *GHLHandler {
	return &GHLHandler{
		db:            db,
		provisioner:   provisioner,
		syncer:        syncer,
		webhookSecret: webhookSecret,
		appURL:        appURL,
	}
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
	http.Redirect(w, r, h.appURL+"/dashboard?ghl=connected", http.StatusTemporaryRedirect)
}

// POST /api/webhooks/inbound — GHL sends events here
func (h *GHLHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	// Verify webhook signature
	if h.webhookSecret != "" {
		sig := r.Header.Get("X-GHL-Signature")
		if !verifyGHLSignature(body, sig, h.webhookSecret) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	var event struct {
		Type       string                 `json:"type"`
		LocationID string                 `json:"locationId"`
		Data       map[string]interface{} `json:"data"`
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

	// Handle relevant events
	switch event.Type {
	case "ConversationProviderOutboundMessage":
		// GHL operator sent a message via our custom channel — route to device
		go h.handleOutboundFromGHL(event.LocationID, event.Data)
	case "InboundMessage":
		// External message arrived in GHL — likely already synced from our side
		// No action needed unless GHL is the source of truth for some channels
	}

	w.WriteHeader(http.StatusOK)
}

func (h *GHLHandler) handleOutboundFromGHL(locationID string, data map[string]interface{}) {
	conversationID, _ := data["conversationId"].(string)
	message, _ := data["message"].(string)
	if conversationID == "" || message == "" {
		return
	}
	if err := h.syncer.HandleInboundGHLMessage(nil, locationID, conversationID, message); err != nil {
		log.Printf("GHL outbound handler error: %v", err)
	}
}

// DELETE /api/integration/disconnect — disconnect GHL for current account
func (h *GHLHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	accountIDStr := r.URL.Query().Get("account_id")
	if accountIDStr == "" {
		writeError(w, "account_id required", http.StatusBadRequest)
		return
	}

	h.db.Exec(r.Context(), `DELETE FROM ghl_connections WHERE account_id = $1`, accountIDStr)
	h.db.Exec(r.Context(), `UPDATE accounts SET ghl_location_id = NULL WHERE id = $1`, accountIDStr)

	writeJSON(w, map[string]string{"status": "disconnected"}, http.StatusOK)
}

// GET /api/integration/status — check connection status for current account
func (h *GHLHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	accountIDStr := r.URL.Query().Get("account_id")
	if accountIDStr == "" {
		writeError(w, "account_id required", http.StatusBadRequest)
		return
	}

	var locationID string
	var connected bool
	err := h.db.QueryRow(r.Context(), `
		SELECT location_id, connected FROM ghl_connections WHERE account_id = $1
	`, accountIDStr).Scan(&locationID, &connected)

	if err != nil {
		writeJSON(w, map[string]interface{}{"connected": false}, http.StatusOK)
		return
	}

	writeJSON(w, map[string]interface{}{
		"connected":   connected,
		"location_id": locationID,
	}, http.StatusOK)
}

func verifyGHLSignature(body []byte, signature, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
