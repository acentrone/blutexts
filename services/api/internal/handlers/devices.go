package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/bluesend/api/internal/models"
	"github.com/bluesend/api/internal/services/messaging"
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
}

func NewDeviceHandler(db *pgxpool.Pool, deviceHub *ws.DeviceHub, clientHub *ws.ClientHub, msgRouter *messaging.Router) *DeviceHandler {
	h := &DeviceHandler{
		db:        db,
		deviceHub: deviceHub,
		clientHub: clientHub,
		msgRouter: msgRouter,
	}
	go h.processDeviceEvents()
	return h
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
		case models.DeviceEventMessageStatus:
			h.handleMessageStatus(event)
		case models.DeviceEventHeartbeat:
			h.handleHeartbeat(event)
		}
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

	if err := h.msgRouter.HandleInbound(context.Background(), payload); err != nil {
		log.Printf("handle inbound error from device %s: %v", event.DeviceID, err)
		return
	}

	var accountID uuid.UUID
	err = h.db.QueryRow(context.Background(), `
		SELECT account_id FROM phone_numbers WHERE number = $1 OR imessage_address = $1
	`, payload.ToAddress).Scan(&accountID)
	if err == nil {
		h.clientHub.BroadcastToAccount(accountID, models.WSEvent{
			Type:    models.WSEventNewMessage,
			Payload: payload,
		})
	}
}

func (h *DeviceHandler) handleMessageStatus(event ws.InboundEvent) {
	payloadBytes, _ := json.Marshal(event.Event.Payload)
	var payload struct {
		MessageID string `json:"message_id"`
		Status    string `json:"status"`
		Error     string `json:"error"`
		GUID      string `json:"imessage_guid"`
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
		h.db.Exec(context.Background(), `
			UPDATE messages SET status = 'sent', sent_at = $1, imessage_guid = $2 WHERE id = $3
		`, now, payload.GUID, msgID)
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
				"message_id": payload.MessageID,
				"status":     payload.Status,
			},
		})
	}
}

func (h *DeviceHandler) handleHeartbeat(event ws.InboundEvent) {
	h.db.Exec(context.Background(), `
		UPDATE devices SET last_seen_at = NOW() WHERE id = $1
	`, event.DeviceID)
}
