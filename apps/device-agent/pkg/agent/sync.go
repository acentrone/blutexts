package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bluesend/device-agent/pkg/imessage"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Event types (must match API server models/models.go constants)
const (
	EventSendMessage     = "send_message"
	EventMessageStatus   = "message_status"
	EventInboundMessage  = "inbound_message"
	EventOutboundMessage = "outbound_message"
	EventHeartbeat      = "heartbeat"
)

type WSEvent struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type SendPayload struct {
	MessageID       string `json:"message_id"`
	PhoneNumber     string `json:"phone_number"`
	ToAddress       string `json:"to_address"`
	Content         string `json:"content"`
	IMessageAddress string `json:"imessage_address"`
}

type StatusPayload struct {
	MessageID    string `json:"message_id"`
	Status       string `json:"status"`
	IMessageGUID string `json:"imessage_guid,omitempty"`
	Error        string `json:"error,omitempty"`
}

type InboundPayload struct {
	IMessageGUID string    `json:"imessage_guid"`
	FromAddress  string    `json:"from_address"`
	ToAddress    string    `json:"to_address"`
	Content      string    `json:"content"`
	ReceivedAt   time.Time `json:"received_at"`
}

// LogEntry represents a recent activity event shown in the desktop UI.
type LogEntry struct {
	Time    string `json:"time"`
	Type    string `json:"type"` // "inbound", "outbound", "status", "connection"
	Message string `json:"message"`
}

// StatusInfo is returned to the desktop UI.
type StatusInfo struct {
	Connected   bool     `json:"connected"`
	Uptime      string   `json:"uptime"`
	Handles     []string `json:"handles"`
	DeviceName  string   `json:"device_name"`
}

// SentMessage tracks a sent message for status polling.
type SentMessage struct {
	MessageID  string
	GUID       string
	SentAt     time.Time
	ToAddress  string
	LastStatus string
}

// Agent manages the connection to the BlueSend API and orchestrates
// iMessage send/receive for all numbers hosted on this device.
type Agent struct {
	apiEndpoint string
	deviceToken string
	deviceName  string

	imClient *imessage.Client
	conn     *websocket.Conn
	connMu   sync.Mutex

	send chan []byte
	done chan struct{}

	lastRowID    int64
	localHandles []string // all iMessage handles (numbers/emails) on this device
	sentMessages map[string]*SentMessage
	sentMu       sync.Mutex

	// Observable state for UI
	connected    bool
	connectedAt  time.Time
	statusMu     sync.RWMutex
	activityLog  []LogEntry
	logMu        sync.Mutex
	onStatus     func(connected bool) // callback for tray icon
}

func NewAgent(endpoint, token, name string) (*Agent, error) {
	client, err := imessage.NewClient()
	if err != nil {
		return nil, fmt.Errorf("imessage client: %w", err)
	}

	// Get the current last row ID so we only poll new messages from this point
	lastRowID, _ := client.GetLastRowID()

	return &Agent{
		apiEndpoint:  endpoint,
		deviceToken:  token,
		deviceName:   name,
		imClient:     client,
		send:         make(chan []byte, 256),
		done:         make(chan struct{}),
		lastRowID:    lastRowID,
		sentMessages: make(map[string]*SentMessage),
	}, nil
}

// Run starts the agent's main loop with automatic reconnect.
func (a *Agent) Run() {
	defer a.imClient.Close()

	for {
		select {
		case <-a.done:
			return
		default:
		}

		log.Println("Connecting to BlueSend API...")
		if err := a.connect(); err != nil {
			log.Printf("Connection failed: %v — retrying in 5s", err)
			select {
			case <-a.done:
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}

		log.Println("Connected to BlueSend API")
		a.setConnected(true)
		a.addLog("connection", "Connected to BlueSend API")
		a.runSession()
		a.setConnected(false)
		a.addLog("connection", "Disconnected from BlueSend API")

		select {
		case <-a.done:
			return
		default:
			log.Println("Disconnected — reconnecting in 3s")
			time.Sleep(3 * time.Second)
		}
	}
}

func (a *Agent) connect() error {
	headers := http.Header{
		"X-Device-Token":  {a.deviceToken},
		"X-Agent-Version": {"1.0.0"},
		"X-Device-Name":   {a.deviceName},
	}

	conn, _, err := websocket.DefaultDialer.Dial(a.apiEndpoint+"/api/devices/connect", headers)
	if err != nil {
		return err
	}

	a.connMu.Lock()
	a.conn = conn
	a.connMu.Unlock()

	// Report all iMessage handles this device can send/receive from.
	// This includes the Mac Mini's own number plus any forwarded iPhone numbers.
	handles, err := a.imClient.GetHandles()
	if err != nil {
		log.Printf("Warning: could not list handles: %v", err)
	} else {
		a.localHandles = handles
		log.Printf("Device handles: %v", handles)
		a.sendEvent("register_handles", map[string]interface{}{
			"device_name": a.deviceName,
			"handles":     handles,
		})
	}

	return nil
}

func (a *Agent) runSession() {
	defer func() {
		a.connMu.Lock()
		if a.conn != nil {
			a.conn.Close()
			a.conn = nil
		}
		a.connMu.Unlock()
	}()

	go a.writePump()
	go a.pollMessages()
	go a.trackDeliveryStatus()
	go a.heartbeat()

	a.readPump()
}

func (a *Agent) readPump() {
	for {
		a.connMu.Lock()
		conn := a.conn
		a.connMu.Unlock()
		if conn == nil {
			return
		}

		conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(90 * time.Second))
			return nil
		})

		_, data, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("read error: %v", err)
			}
			return
		}

		var event WSEvent
		if err := json.Unmarshal(data, &event); err != nil {
			log.Printf("invalid event: %v", err)
			continue
		}

		if event.Type == EventSendMessage {
			go a.handleSendMessage(event.Payload)
		}
	}
}

func (a *Agent) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg := <-a.send:
			a.connMu.Lock()
			conn := a.conn
			a.connMu.Unlock()
			if conn == nil {
				return
			}
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			a.connMu.Lock()
			conn := a.conn
			a.connMu.Unlock()
			if conn == nil {
				return
			}
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			conn.WriteMessage(websocket.PingMessage, nil)
		case <-a.done:
			return
		}
	}
}

func (a *Agent) handleSendMessage(payload json.RawMessage) {
	var req SendPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("invalid send payload: %v", err)
		return
	}

	fromHandle := req.IMessageAddress
	if fromHandle == "" {
		fromHandle = req.PhoneNumber
	}

	log.Printf("Sending to %s from %s (msg_id: %s)", req.ToAddress, fromHandle, req.MessageID)
	a.addLog("outbound", fmt.Sprintf("→ %s from %s", req.ToAddress, fromHandle))

	if err := a.imClient.Send(req.ToAddress, fromHandle, req.Content); err != nil {
		log.Printf("Send error: %v", err)
		a.sendStatus(req.MessageID, "failed", "", err.Error())
		return
	}

	tempGUID := "pending-" + uuid.New().String()
	a.sendStatus(req.MessageID, "sent", tempGUID, "")

	a.sentMu.Lock()
	a.sentMessages[req.MessageID] = &SentMessage{
		MessageID:  req.MessageID,
		GUID:       tempGUID,
		SentAt:     time.Now(),
		ToAddress:  req.ToAddress,
		LastStatus: "sent",
	}
	a.sentMu.Unlock()
}

// pollMessages checks chat.db for new messages every 500ms using ROWID tracking.
func (a *Agent) pollMessages() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
		case <-a.done:
			return
		}

		messages, err := a.imClient.GetMessagesSinceRowID(a.lastRowID)
		if err != nil {
			log.Printf("poll error: %v", err)
			continue
		}

		if len(messages) > 0 {
			inCount, outCount := 0, 0
			for _, m := range messages {
				if m.IsFromMe {
					outCount++
				} else {
					inCount++
				}
			}
			a.addLog("status", fmt.Sprintf("Poll found %d new messages (%d in, %d out)", len(messages), inCount, outCount))
		}

		// Update the cursor to the highest ROWID we've seen
		for _, msg := range messages {
			if msg.RowID > a.lastRowID {
				a.lastRowID = msg.RowID
			}
		}

		for _, msg := range messages {
			svc := msg.ServiceName
			if svc == "" {
				svc = "?"
			}
			if msg.IsFromMe {
				log.Printf("Outbound [%s] to %s: %.40s", svc, msg.Handle, msg.Text)
				a.addLog("outbound", fmt.Sprintf("→ %s [%s]: %.40s", msg.Handle, svc, msg.Text))
				// Send outbound messages to API for web dashboard and GHL sync
				a.sendEvent(EventOutboundMessage, InboundPayload{
					IMessageGUID: msg.GUID,
					FromAddress:  msg.Destination, // the local account that sent it
					ToAddress:    msg.Handle,       // the recipient
					Content:      msg.Text,
					ReceivedAt:   msg.Date,
				})
				continue
			}

			log.Printf("Inbound [%s] from %s → %s: %.40s", svc, msg.Handle, msg.Destination, msg.Text)
			a.addLog("inbound", fmt.Sprintf("← %s → %s [%s]: %.40s", msg.Handle, msg.Destination, svc, msg.Text))
			a.sendEvent(EventInboundMessage, InboundPayload{
				IMessageGUID: msg.GUID,
				FromAddress:  msg.Handle,
				ToAddress:    msg.Destination,
				Content:      msg.Text,
				ReceivedAt:   msg.Date,
			})
		}
	}
}

// trackDeliveryStatus polls delivery status for recently sent messages.
func (a *Agent) trackDeliveryStatus() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
		case <-a.done:
			return
		}

		a.sentMu.Lock()
		toCheck := make(map[string]*SentMessage)
		for k, v := range a.sentMessages {
			if time.Since(v.SentAt) < 10*time.Minute {
				toCheck[k] = v
			} else {
				delete(a.sentMessages, k)
			}
		}
		a.sentMu.Unlock()

		for _, sm := range toCheck {
			if sm.GUID == "" || strings.HasPrefix(sm.GUID, "pending-") {
				continue
			}
			status, err := a.imClient.GetMessageStatus(sm.GUID)
			if err != nil {
				continue
			}
			if status != sm.LastStatus {
				a.sendStatus(sm.MessageID, status, sm.GUID, "")
				a.sentMu.Lock()
				if tracked, ok := a.sentMessages[sm.MessageID]; ok {
					tracked.LastStatus = status
				}
				a.sentMu.Unlock()
			}
		}
	}
}

func (a *Agent) heartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			a.sendEvent(EventHeartbeat, map[string]string{
				"device_name": a.deviceName,
				"timestamp":   time.Now().Format(time.RFC3339),
			})
		case <-a.done:
			return
		}
	}
}

func (a *Agent) sendStatus(messageID, status, guid, errMsg string) {
	a.sendEvent(EventMessageStatus, StatusPayload{
		MessageID:    messageID,
		Status:       status,
		IMessageGUID: guid,
		Error:        errMsg,
	})
}

func (a *Agent) sendEvent(eventType string, payload interface{}) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return
	}
	data, err := json.Marshal(WSEvent{Type: eventType, Payload: payloadBytes})
	if err != nil {
		return
	}
	select {
	case a.send <- data:
	default:
		log.Printf("send buffer full, dropping %s", eventType)
	}
}

// SetOnStatusChange sets a callback invoked when connection status changes.
func (a *Agent) SetOnStatusChange(fn func(connected bool)) {
	a.onStatus = fn
}

// GetStatus returns observable state for the desktop UI.
func (a *Agent) GetStatus() StatusInfo {
	a.statusMu.RLock()
	connected := a.connected
	connAt := a.connectedAt
	a.statusMu.RUnlock()

	uptime := ""
	if connected && !connAt.IsZero() {
		d := time.Since(connAt)
		if d < time.Minute {
			uptime = fmt.Sprintf("%ds", int(d.Seconds()))
		} else if d < time.Hour {
			uptime = fmt.Sprintf("%dm", int(d.Minutes()))
		} else {
			uptime = fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
		}
	}

	return StatusInfo{
		Connected:  connected,
		Uptime:     uptime,
		Handles:    a.localHandles,
		DeviceName: a.deviceName,
	}
}

// GetActivityLog returns recent activity entries for the UI.
func (a *Agent) GetActivityLog() []LogEntry {
	a.logMu.Lock()
	defer a.logMu.Unlock()
	out := make([]LogEntry, len(a.activityLog))
	copy(out, a.activityLog)
	return out
}

func (a *Agent) addLog(logType, message string) {
	a.logMu.Lock()
	defer a.logMu.Unlock()
	entry := LogEntry{
		Time:    time.Now().Format("15:04:05"),
		Type:    logType,
		Message: message,
	}
	a.activityLog = append(a.activityLog, entry)
	if len(a.activityLog) > 200 {
		a.activityLog = a.activityLog[len(a.activityLog)-200:]
	}
}

func (a *Agent) setConnected(c bool) {
	a.statusMu.Lock()
	a.connected = c
	if c {
		a.connectedAt = time.Now()
	}
	a.statusMu.Unlock()
	if a.onStatus != nil {
		a.onStatus(c)
	}
}

func (a *Agent) Stop() {
	close(a.done)
}
