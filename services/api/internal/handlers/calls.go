package handlers

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"time"

	"github.com/bluesend/api/internal/middleware"
	"github.com/bluesend/api/internal/models"
	"github.com/bluesend/api/internal/services/voice"
	ws "github.com/bluesend/api/internal/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CallHandler implements the FaceTime Audio calling flow.
//
// Flow for an outbound call:
//  1. Chrome extension calls POST /api/calls/start.
//  2. Server checks accounts.calling_enabled + phone_numbers.voice_enabled.
//  3. Server mints two Agora tokens (one for the agent browser, one for the
//     hosted Mac bridge) on a freshly generated channel name.
//  4. Server inserts a call_logs row with status=initiated.
//  5. Server pushes a device WS event with the bridge token so the iMac joins
//     the channel and opens facetime-audio://+E164.
//  6. Server returns the agent token so the browser joins the channel.
//  7. Agent browser audio flows through Agora into the iMac's hidden WebView,
//     which writes it to BlackHole-In — that's what FaceTime.app hears.
//     FaceTime.app output (contact voice) comes out BlackHole-Out, is captured
//     by the WebView via getUserMedia, and published to Agora for the agent.
type CallHandler struct {
	db    *pgxpool.Pool
	voice *voice.Service
	hub   *ws.DeviceHub
}

func NewCallHandler(db *pgxpool.Pool, v *voice.Service, hub *ws.DeviceHub) *CallHandler {
	return &CallHandler{db: db, voice: v, hub: hub}
}

// Agora UID assignments: using fixed values keeps the bridge's subscribe logic
// simple (it only needs to pipe remote uid=agentUID into BlackHole-In).
const (
	agoraUIDBridge uint32 = 1
	agoraUIDAgent  uint32 = 2
)

// POST /api/calls/start
//
//	{
//	  "phone_number_id": "<uuid>",
//	  "to": "+15551234567"    // phone number or iMessage/FaceTime address
//	}
//
// Response:
//
//	{
//	  "call_id": "<uuid>",
//	  "agora": { "app_id": "...", "channel": "...", "token": "...", "uid": 2 }
//	}
func (h *CallHandler) Start(w http.ResponseWriter, r *http.Request) {
	accountID, ok := middleware.GetAccountID(r.Context())
	if !ok {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if !h.voice.Enabled() {
		writeError(w, "calling is not configured on this server", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		PhoneNumberID string `json:"phone_number_id"`
		To            string `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.PhoneNumberID == "" || req.To == "" {
		writeError(w, "phone_number_id and to are required", http.StatusBadRequest)
		return
	}

	// Upsell gate: both the account AND the specific number must be voice-enabled.
	var (
		callingEnabled bool
		voiceEnabled   bool
		fromNumber     string
		deviceID       uuid.UUID
		phoneNumberID  uuid.UUID
	)
	err := h.db.QueryRow(r.Context(), `
		SELECT a.calling_enabled, pn.voice_enabled, pn.number, pn.device_id, pn.id
		FROM phone_numbers pn
		JOIN accounts a ON a.id = pn.account_id
		WHERE pn.id = $1 AND pn.account_id = $2 AND pn.status = 'active'
	`, req.PhoneNumberID, accountID).Scan(&callingEnabled, &voiceEnabled, &fromNumber, &deviceID, &phoneNumberID)
	if err != nil {
		writeError(w, "phone number not found or inactive", http.StatusBadRequest)
		return
	}
	if !callingEnabled {
		writeError(w, "calling is not enabled on this account — contact support to upgrade", http.StatusPaymentRequired)
		return
	}
	if !voiceEnabled {
		writeError(w, "calling is not enabled on this phone number", http.StatusForbidden)
		return
	}

	// Fresh Agora channel per call
	channel, err := voice.NewChannelName()
	if err != nil {
		writeError(w, "failed to allocate channel", http.StatusInternalServerError)
		return
	}

	bridgeToken, err := h.voice.BuildToken(channel, agoraUIDBridge)
	if err != nil {
		writeError(w, "failed to mint bridge token: "+err.Error(), http.StatusInternalServerError)
		return
	}
	agentToken, err := h.voice.BuildToken(channel, agoraUIDAgent)
	if err != nil {
		writeError(w, "failed to mint agent token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	now := time.Now()
	callID := uuid.New()

	_, err = h.db.Exec(r.Context(), `
		INSERT INTO call_logs (
			id, account_id, phone_number_id, device_id, direction,
			from_number, to_number, agora_channel, status, started_at
		) VALUES ($1, $2, $3, $4, 'outbound', $5, $6, $7, 'initiated', $8)
	`, callID, accountID, phoneNumberID, deviceID, fromNumber, req.To, channel, now)
	if err != nil {
		writeError(w, "failed to log call: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Push to device. If the device isn't connected, mark failed and bail.
	if err := h.hub.SendToDevice(deviceID, models.DeviceWSEvent{
		Type: models.DeviceEventInitiateCall,
		Payload: models.DeviceCallPayload{
			CallID:       callID.String(),
			To:           req.To,
			FromNumber:   fromNumber,
			AgoraChannel: channel,
			AgoraToken:   bridgeToken,
			AgoraUID:     agoraUIDBridge,
			AgoraAppID:   h.voice.AppID,
		},
	}); err != nil {
		_, _ = h.db.Exec(r.Context(), `
			UPDATE call_logs SET status='failed', failure_reason=$2, ended_at=NOW(), updated_at=NOW()
			WHERE id=$1
		`, callID, "device offline")
		writeError(w, "device not connected: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	writeJSON(w, map[string]any{
		"call_id": callID.String(),
		"agora": map[string]any{
			"app_id":  h.voice.AppID,
			"channel": channel,
			"token":   agentToken,
			"uid":     agoraUIDAgent,
		},
	}, http.StatusOK)
}

// POST /api/calls/{callID}/end — agent hangs up.
func (h *CallHandler) End(w http.ResponseWriter, r *http.Request) {
	accountID, ok := middleware.GetAccountID(r.Context())
	if !ok {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	callID, err := uuid.Parse(chi.URLParam(r, "callID"))
	if err != nil {
		writeError(w, "invalid call id", http.StatusBadRequest)
		return
	}

	var (
		deviceID uuid.UUID
		status   string
	)
	err = h.db.QueryRow(r.Context(), `
		SELECT device_id, status FROM call_logs
		WHERE id=$1 AND account_id=$2
	`, callID, accountID).Scan(&deviceID, &status)
	if err != nil {
		writeError(w, "call not found", http.StatusNotFound)
		return
	}

	// Idempotent: if already terminal, just respond ok.
	if status == string(models.CallStatusCompleted) ||
		status == string(models.CallStatusFailed) ||
		status == string(models.CallStatusMissed) ||
		status == string(models.CallStatusCancelled) {
		writeJSON(w, map[string]string{"status": status}, http.StatusOK)
		return
	}

	_ = h.hub.SendToDevice(deviceID, models.DeviceWSEvent{
		Type: models.DeviceEventCallControl,
		Payload: models.DeviceCallControl{
			CallID: callID.String(),
			Action: "end",
		},
	})

	writeJSON(w, map[string]string{"status": "ending"}, http.StatusOK)
}

// GET /api/calls — recent call log for the authenticated user.
func (h *CallHandler) List(w http.ResponseWriter, r *http.Request) {
	accountID, ok := middleware.GetAccountID(r.Context())
	if !ok {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	rows, err := h.db.Query(r.Context(), `
		SELECT id, direction, from_number, to_number, status,
		       duration_seconds, started_at, connected_at, ended_at, created_at
		FROM call_logs
		WHERE account_id=$1
		ORDER BY created_at DESC
		LIMIT 100
	`, accountID)
	if err != nil {
		writeError(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	out := make([]map[string]any, 0, 32)
	for rows.Next() {
		var (
			id                                      uuid.UUID
			direction, from, to, status             string
			duration                                *int
			startedAt, connectedAt, endedAt, created *time.Time
		)
		if err := rows.Scan(&id, &direction, &from, &to, &status, &duration, &startedAt, &connectedAt, &endedAt, &created); err != nil {
			continue
		}
		out = append(out, map[string]any{
			"id":               id,
			"direction":        direction,
			"from":             from,
			"to":               to,
			"status":           status,
			"duration_seconds": duration,
			"started_at":       startedAt,
			"connected_at":     connectedAt,
			"ended_at":         endedAt,
			"created_at":       created,
		})
	}
	writeJSON(w, out, http.StatusOK)
}

// randUID generates a 32-bit unsigned int suitable for Agora uid. Unused today
// (we use fixed UIDs) but kept for future per-agent uid assignment.
func randUID() uint32 {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return binary.LittleEndian.Uint32(b[:])
}
