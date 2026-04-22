package handlers

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bluesend/api/internal/models"
	"github.com/bluesend/api/internal/services/audit"
	"github.com/bluesend/api/internal/services/messaging"
	"github.com/bluesend/api/internal/services/storage"
	ws "github.com/bluesend/api/internal/websocket"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DeviceHandler manages WebSocket connections from physical device agents
// and processes inbound events from them.
type DeviceHandler struct {
	db        *pgxpool.Pool
	deviceHub *ws.DeviceHub
	clientHub *ws.ClientHub
	msgRouter *messaging.Router
	storage   *storage.R2Client
}

func NewDeviceHandler(db *pgxpool.Pool, deviceHub *ws.DeviceHub, clientHub *ws.ClientHub, msgRouter *messaging.Router, r2 *storage.R2Client) *DeviceHandler {
	h := &DeviceHandler{
		db:        db,
		deviceHub: deviceHub,
		clientHub: clientHub,
		msgRouter: msgRouter,
		storage:   r2,
	}
	go h.processDeviceEvents()
	return h
}

// POST /api/devices/upload — device agents upload inbound attachment files
// (images, voice messages, etc.) so the API can store them in R2 and surface
// them to the web app and GHL. Authenticated by X-Device-Token, NOT JWT.
//
// CAF voice messages are transcoded to mp3 here so browsers (which cannot
// decode CAF) and GHL (whose media whitelist excludes CAF) both accept them.
func (h *DeviceHandler) UploadAttachment(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		writeError(w, "media uploads are not configured on this server", http.StatusServiceUnavailable)
		return
	}

	token := r.Header.Get("X-Device-Token")
	if token == "" {
		writeError(w, "X-Device-Token required", http.StatusUnauthorized)
		return
	}
	var deviceID uuid.UUID
	if err := h.db.QueryRow(r.Context(), `SELECT id FROM devices WHERE device_token = $1`, token).Scan(&deviceID); err != nil {
		writeError(w, "invalid device token", http.StatusUnauthorized)
		return
	}

	const maxSize = 100 << 20 // 100MB
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)
	if err := r.ParseMultipartForm(maxSize); err != nil {
		writeError(w, "file too large or invalid multipart", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, "file field required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, "could not read file", http.StatusBadRequest)
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	filename := header.Filename
	size := header.Size

	// Inbound voice memos arrive as Opus-in-CAF. Browsers can't play CAF
	// and GHL won't accept it, so transcode to mp3 before storing.
	if strings.HasPrefix(strings.ToLower(contentType), "audio/x-caf") || strings.HasSuffix(strings.ToLower(filename), ".caf") {
		if mp3Data, convErr := ConvertCAFToMP3(data); convErr != nil {
			log.Printf("device upload: caf→mp3 failed: %v — storing original", convErr)
		} else {
			data = mp3Data
			contentType = "audio/mpeg"
			base := filename
			for _, ext := range []string{".caf", ".CAF"} {
				base = strings.TrimSuffix(base, ext)
			}
			if base == "" {
				base = "voice-message"
			}
			filename = base + ".mp3"
			size = int64(len(data))
		}
	}

	url, err := h.storage.Upload(r.Context(), data, contentType, filename)
	if err != nil {
		writeError(w, "upload failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, models.Attachment{
		URL:      url,
		Type:     contentType,
		Filename: filename,
		Size:     size,
	}, http.StatusOK)
}

// GET /api/devices/connect — WebSocket endpoint for device agents
func (h *DeviceHandler) Connect(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-Device-Token")
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	if token == "" {
		http.Error(w, "X-Device-Token required", http.StatusUnauthorized)
		return
	}

	var deviceID uuid.UUID
	err := h.db.QueryRow(r.Context(), `
		SELECT id FROM devices WHERE device_token = $1
	`, token).Scan(&deviceID)
	if err != nil {
		http.Error(w, "invalid device token", http.StatusUnauthorized)
		return
	}

	ip := r.RemoteAddr
	agentVersion := r.Header.Get("X-Agent-Version")
	h.db.Exec(r.Context(), `
		UPDATE devices SET status = 'online', last_seen_at = NOW(), ip_address = $1, agent_version = $2, updated_at = NOW()
		WHERE id = $3
	`, ip, agentVersion, deviceID)

	defer func() {
		h.db.Exec(context.Background(), `
			UPDATE devices SET status = 'offline', updated_at = NOW() WHERE id = $1
		`, deviceID)
	}()

	h.deviceHub.ServeDevice(w, r, deviceID)
}

// processDeviceEvents reads from the device hub's inbound channel and handles events.
func (h *DeviceHandler) processDeviceEvents() {
	for event := range h.deviceHub.InboundEvents() {
		switch event.Event.Type {
		case models.DeviceEventInboundMessage:
			h.handleInboundMessage(event)
		case models.DeviceEventOutboundMessage:
			h.handleOutboundMessage(event)
		case models.DeviceEventMessageStatus:
			h.handleMessageStatus(event)
		case models.DeviceEventCallStatus:
			h.handleCallStatus(event)
		case models.DeviceEventHeartbeat:
			h.handleHeartbeat(event)
		}
	}
}

// handleCallStatus updates the call_logs row as the device reports progress
// (ringing → connected → ended/failed) and mirrors the event to the agent's
// browser over the client WebSocket.
func (h *DeviceHandler) handleCallStatus(event ws.InboundEvent) {
	payloadBytes, _ := json.Marshal(event.Event.Payload)
	var p models.DeviceCallStatusPayload
	if err := json.Unmarshal(payloadBytes, &p); err != nil {
		return
	}
	callID, err := uuid.Parse(p.CallID)
	if err != nil {
		return
	}

	ctx := context.Background()
	now := time.Now()
	var accountID uuid.UUID
	_ = h.db.QueryRow(ctx, `SELECT account_id FROM call_logs WHERE id=$1 AND device_id=$2`,
		callID, event.DeviceID).Scan(&accountID)

	switch p.Status {
	case "ringing":
		h.db.Exec(ctx, `UPDATE call_logs SET status='ringing', updated_at=$2 WHERE id=$1`, callID, now)
	case "connected":
		h.db.Exec(ctx, `UPDATE call_logs SET status='connected', connected_at=$2, updated_at=$2 WHERE id=$1`, callID, now)
	case "ended":
		h.db.Exec(ctx, `
			UPDATE call_logs
			SET status = CASE WHEN connected_at IS NULL THEN 'missed' ELSE 'completed' END,
			    duration_seconds=$2, ended_at=$3, updated_at=$3
			WHERE id=$1
		`, callID, p.Duration, now)
	case "failed":
		reason := p.Error
		h.db.Exec(ctx, `
			UPDATE call_logs SET status='failed', failure_reason=$2, ended_at=$3, updated_at=$3 WHERE id=$1
		`, callID, reason, now)
	}

	if accountID != uuid.Nil {
		h.clientHub.BroadcastToAccount(accountID, models.WSEvent{
			Type:    "call_status",
			Payload: p,
		})
	}
}

func (h *DeviceHandler) handleInboundMessage(event ws.InboundEvent) {
	payloadBytes, err := json.Marshal(event.Event.Payload)
	if err != nil {
		return
	}
	var payload models.DeviceInboundPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		log.Printf("invalid inbound message payload from device %s: %v", event.DeviceID, err)
		return
	}

	msg, err := h.msgRouter.HandleInbound(context.Background(), payload)
	if err != nil {
		log.Printf("handle inbound error from device %s: %v", event.DeviceID, err)
		return
	}
	if msg == nil {
		// Duplicate — already processed, no need to re-broadcast.
		return
	}

	// Broadcast the full Message (with conversation_id, direction, attachments,
	// real id) so the web app can append it to the open thread immediately
	// without polling.
	h.clientHub.BroadcastToAccount(msg.AccountID, models.WSEvent{
		Type:    models.WSEventNewMessage,
		Payload: msg,
	})
	audit.Log(context.Background(), h.db, msg.AccountID, uuid.Nil, "message.received", "message", msg.ID.String(),
		map[string]interface{}{
			"from":    payload.FromAddress,
			"to":      payload.ToAddress,
			"preview": truncateAudit(payload.Content, 60),
		}, "")
}

func truncateAudit(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func (h *DeviceHandler) handleOutboundMessage(event ws.InboundEvent) {
	payloadBytes, err := json.Marshal(event.Event.Payload)
	if err != nil {
		return
	}
	var payload models.DeviceInboundPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		log.Printf("invalid outbound message payload from device %s: %v", event.DeviceID, err)
		return
	}

	msg, err := h.msgRouter.HandleOutbound(context.Background(), payload)
	if err != nil {
		log.Printf("handle outbound error from device %s: %v", event.DeviceID, err)
		return
	}
	if msg == nil {
		return
	}

	h.clientHub.BroadcastToAccount(msg.AccountID, models.WSEvent{
		Type:    models.WSEventNewMessage,
		Payload: msg,
	})
}

func (h *DeviceHandler) handleMessageStatus(event ws.InboundEvent) {
	payloadBytes, _ := json.Marshal(event.Event.Payload)
	var payload struct {
		MessageID string `json:"message_id"`
		Status    string `json:"status"`
		Error     string `json:"error"`
		GUID      string `json:"imessage_guid"`
		Service   string `json:"service"` // "imessage" or "sms" — empty falls back to current value
	}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return
	}

	msgID, err := uuid.Parse(payload.MessageID)
	if err != nil {
		return
	}

	now := time.Now()
	switch payload.Status {
	case "sent":
		// Persist the underlying transport (imessage/sms) when the device reports
		// it. Older agents that don't send `service` leave the existing value
		// alone (COALESCE on NULLIF empty-string).
		h.db.Exec(context.Background(), `
			UPDATE messages SET status = 'sent', sent_at = $1, imessage_guid = $2,
			  service = COALESCE(NULLIF($3, ''), service)
			WHERE id = $4
		`, now, payload.GUID, payload.Service, msgID)

		// Update the contact's iMessage capability cache from the service the
		// device actually used. Saves the next send the cost of an availability
		// lookup and lets the web UI conditionally disable effects / voice
		// recording for SMS-only contacts.
		if payload.Service == "imessage" || payload.Service == "sms" {
			capable := payload.Service == "imessage"
			h.db.Exec(context.Background(), `
				UPDATE contacts SET imessage_capable = $1, imessage_checked_at = NOW()
				WHERE id = (SELECT contact_id FROM messages WHERE id = $2)
			`, capable, msgID)
		}
	case "delivered":
		h.db.Exec(context.Background(), `
			UPDATE messages SET status = 'delivered', delivered_at = $1 WHERE id = $2
		`, now, msgID)
	case "read":
		h.db.Exec(context.Background(), `
			UPDATE messages SET status = 'read', read_at = $1 WHERE id = $2
		`, now, msgID)
	case "failed":
		h.db.Exec(context.Background(), `
			UPDATE messages SET status = 'failed', failed_at = $1, error_message = $2 WHERE id = $3
		`, now, payload.Error, msgID)
	}

	var accountID uuid.UUID
	h.db.QueryRow(context.Background(), `SELECT account_id FROM messages WHERE id = $1`, msgID).Scan(&accountID)
	if accountID != uuid.Nil {
		eventType := models.WSEventMessageDelivered
		if payload.Status == "failed" {
			eventType = models.WSEventMessageFailed
		}
		h.clientHub.BroadcastToAccount(accountID, models.WSEvent{
			Type: eventType,
			Payload: map[string]string{
				"message_id":    payload.MessageID,
				"status":        payload.Status,
				"service":       payload.Service,
				"error_message": payload.Error,
			},
		})
	}
}

func (h *DeviceHandler) handleHeartbeat(event ws.InboundEvent) {
	h.db.Exec(context.Background(), `
		UPDATE devices SET last_seen_at = NOW() WHERE id = $1
	`, event.DeviceID)
}
