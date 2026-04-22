package websocket

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bluesend/api/internal/models"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// allowedOrigins is the set of Origins permitted on WebSocket upgrades.
// Populated from the APP_URL + DEVICE_WS_URL env vars at startup; populated
// to nil means "allow all" (only used in dev when ALLOW_ANY_WS_ORIGIN=1).
var allowedOrigins map[string]bool

// SetAllowedOrigins is called from main on boot to seed the allowlist.
// Keys are normalized to lowercase scheme://host[:port].
func SetAllowedOrigins(origins []string) {
	if len(origins) == 0 {
		allowedOrigins = nil
		return
	}
	allowedOrigins = make(map[string]bool, len(origins))
	for _, o := range origins {
		allowedOrigins[strings.ToLower(o)] = true
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow same-origin (no Origin header — typical for native clients
		// like the Go device agent which doesn't set Origin) AND the
		// configured allowlist of browser-side origins.
		origin := r.Header.Get("Origin")
		if origin == "" {
			// Non-browser client (device agent, curl, websocat). The agent
			// authenticates separately via Bearer token, so no-origin is
			// safe here.
			return true
		}
		if allowedOrigins == nil {
			// Boot didn't populate the allowlist — fail closed.
			return false
		}
		return allowedOrigins[strings.ToLower(origin)]
	},
}

// ============================================================
// Device Hub — manages WebSocket connections from physical devices
// ============================================================

// DeviceHub manages connected device agents and routes messages to them.
type DeviceHub struct {
	mu      sync.RWMutex
	devices map[uuid.UUID]*DeviceConn // deviceID → connection
	inbound chan InboundEvent
	db      interface {
		HandleInbound(models.DeviceInboundPayload) error
	} // injected
}

type DeviceConn struct {
	DeviceID uuid.UUID
	conn     *websocket.Conn
	send     chan []byte
	hub      *DeviceHub
}

type InboundEvent struct {
	DeviceID uuid.UUID
	Event    models.DeviceWSEvent
}

func NewDeviceHub() *DeviceHub {
	return &DeviceHub{
		devices: make(map[uuid.UUID]*DeviceConn),
		inbound: make(chan InboundEvent, 256),
	}
}

// ServeDevice upgrades an HTTP connection to WebSocket for a device agent.
// The device must pass its token via the Authorization header.
func (h *DeviceHub) ServeDevice(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("device WebSocket upgrade error: %v", err)
		return
	}

	dc := &DeviceConn{
		DeviceID: deviceID,
		conn:     conn,
		send:     make(chan []byte, 256),
		hub:      h,
	}

	// If this device is already registered (reconnect or duplicate), forcibly
	// close the old connection before swapping in the new one. Without this,
	// the stale readPump eventually errors out and its disconnect() call
	// removes the NEW connection from the map, leaving a zombie where the
	// device thinks it's connected but the server has no route to it.
	h.mu.Lock()
	if old, ok := h.devices[deviceID]; ok {
		log.Printf("Device %s reconnecting — closing stale connection", deviceID)
		_ = old.conn.Close()
	}
	h.devices[deviceID] = dc
	h.mu.Unlock()

	log.Printf("Device connected: %s", deviceID)

	go dc.writePump()
	dc.readPump()
}

// SendToDevice pushes an event to a connected device agent.
func (h *DeviceHub) SendToDevice(deviceID uuid.UUID, event models.DeviceWSEvent) error {
	h.mu.RLock()
	dc, ok := h.devices[deviceID]
	h.mu.RUnlock()

	if !ok {
		return fmt.Errorf("device %s not connected", deviceID)
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	select {
	case dc.send <- data:
		return nil
	default:
		return fmt.Errorf("device %s send buffer full", deviceID)
	}
}

// GetConnectedDevices returns IDs of all currently connected devices.
func (h *DeviceHub) GetConnectedDevices() []uuid.UUID {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]uuid.UUID, 0, len(h.devices))
	for id := range h.devices {
		ids = append(ids, id)
	}
	return ids
}

// disconnect removes a device from the hub, but only if the map entry still
// points at the same DeviceConn. This prevents a stale readPump (from a prior
// session) from evicting a newer live connection after the client reconnected.
func (h *DeviceHub) disconnect(dc *DeviceConn) {
	h.mu.Lock()
	if current, ok := h.devices[dc.DeviceID]; ok && current == dc {
		delete(h.devices, dc.DeviceID)
		log.Printf("Device disconnected: %s", dc.DeviceID)
	} else {
		log.Printf("Stale readPump finished for %s — ignoring (newer session is live)", dc.DeviceID)
	}
	h.mu.Unlock()
}

func (dc *DeviceConn) readPump() {
	defer func() {
		dc.hub.disconnect(dc)
		dc.conn.Close()
	}()

	dc.conn.SetReadLimit(512 * 1024)
	dc.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	dc.conn.SetPongHandler(func(string) error {
		dc.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, data, err := dc.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("device %s unexpected close: %v", dc.DeviceID, err)
			}
			break
		}

		var event models.DeviceWSEvent
		if err := json.Unmarshal(data, &event); err != nil {
			log.Printf("device %s invalid event: %v", dc.DeviceID, err)
			continue
		}

		dc.hub.inbound <- InboundEvent{DeviceID: dc.DeviceID, Event: event}
	}
}

func (dc *DeviceConn) writePump() {
	// Ping every 10 seconds to keep the connection alive through edge proxies.
	ticker := time.NewTicker(10 * time.Second)
	defer func() {
		ticker.Stop()
		dc.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-dc.send:
			dc.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				dc.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := dc.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			dc.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := dc.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// InboundEvents returns the channel of events received from devices.
func (h *DeviceHub) InboundEvents() <-chan InboundEvent {
	return h.inbound
}

// ============================================================
// Client Hub — manages WebSocket connections from browser clients (dashboard)
// ============================================================

type ClientHub struct {
	mu      sync.RWMutex
	clients map[uuid.UUID]map[*ClientConn]bool // accountID → set of connections
}

type ClientConn struct {
	AccountID uuid.UUID
	conn      *websocket.Conn
	send      chan []byte
	hub       *ClientHub
}

func NewClientHub() *ClientHub {
	return &ClientHub{
		clients: make(map[uuid.UUID]map[*ClientConn]bool),
	}
}

// ServeClient upgrades an authenticated user browser connection.
func (h *ClientHub) ServeClient(w http.ResponseWriter, r *http.Request, accountID uuid.UUID) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	cc := &ClientConn{
		AccountID: accountID,
		conn:      conn,
		send:      make(chan []byte, 64),
		hub:       h,
	}

	h.mu.Lock()
	if h.clients[accountID] == nil {
		h.clients[accountID] = make(map[*ClientConn]bool)
	}
	h.clients[accountID][cc] = true
	h.mu.Unlock()

	go cc.writePump()
	cc.readPump()
}

// BroadcastToAccount sends an event to all browser clients connected for an account.
func (h *ClientHub) BroadcastToAccount(accountID uuid.UUID, event models.WSEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	h.mu.RLock()
	conns := h.clients[accountID]
	h.mu.RUnlock()

	for cc := range conns {
		select {
		case cc.send <- data:
		default:
			// Slow client, drop message
		}
	}
}

func (h *ClientHub) disconnect(cc *ClientConn) {
	h.mu.Lock()
	if conns, ok := h.clients[cc.AccountID]; ok {
		delete(conns, cc)
		if len(conns) == 0 {
			delete(h.clients, cc.AccountID)
		}
	}
	h.mu.Unlock()
}

func (cc *ClientConn) readPump() {
	defer func() {
		cc.hub.disconnect(cc)
		cc.conn.Close()
	}()
	cc.conn.SetReadLimit(1024)
	cc.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	cc.conn.SetPongHandler(func(string) error {
		cc.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		if _, _, err := cc.conn.ReadMessage(); err != nil {
			break
		}
	}
}

func (cc *ClientConn) writePump() {
	ticker := time.NewTicker(10 * time.Second)
	defer func() {
		ticker.Stop()
		cc.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-cc.send:
			cc.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				cc.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := cc.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			cc.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := cc.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
