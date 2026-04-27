package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bluesend/device-agent/pkg/imessage"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// httpAPIBase converts the WebSocket endpoint (wss:// or ws://) into the
// equivalent HTTP base URL so we can POST attachment uploads.
func (a *Agent) httpAPIBase() string {
	switch {
	case strings.HasPrefix(a.apiEndpoint, "wss://"):
		return "https://" + strings.TrimPrefix(a.apiEndpoint, "wss://")
	case strings.HasPrefix(a.apiEndpoint, "ws://"):
		return "http://" + strings.TrimPrefix(a.apiEndpoint, "ws://")
	}
	return a.apiEndpoint
}

// uploadAttachment POSTs a local attachment file to the API's device upload
// endpoint and returns the resulting public Attachment record. The API
// transcodes audio/x-caf voice messages to mp3 server-side so the URL we
// get back is web-playable everywhere.
func (a *Agent) uploadAttachment(local imessage.LocalAttachment) (*Attachment, error) {
	f, err := os.Open(local.FilePath)
	if err != nil {
		return nil, fmt.Errorf("open attachment: %w", err)
	}
	defer f.Close()

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, local.Filename))
	if local.MimeType != "" {
		header.Set("Content-Type", local.MimeType)
	}
	part, err := mw.CreatePart(header)
	if err != nil {
		return nil, fmt.Errorf("multipart: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, fmt.Errorf("copy attachment: %w", err)
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("multipart close: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, a.httpAPIBase()+"/api/devices/upload", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Device-Token", a.deviceToken)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upload failed %d: %s", resp.StatusCode, string(respBytes))
	}

	var att Attachment
	if err := json.Unmarshal(respBytes, &att); err != nil {
		return nil, fmt.Errorf("parse upload response: %w", err)
	}
	return &att, nil
}

// collectInboundAttachments queries chat.db for attachments on the given
// message ROWID and uploads each to the API. Failures on individual
// attachments are logged but do not block the message event — losing a
// photo is better than losing the whole conversation entry.
func (a *Agent) collectInboundAttachments(rowID int64) []Attachment {
	locals, err := a.imClient.GetAttachmentsForMessage(rowID)
	if err != nil {
		log.Printf("attachments lookup error (rowid=%d): %v", rowID, err)
		return nil
	}
	if len(locals) == 0 {
		return nil
	}
	var uploaded []Attachment
	for _, l := range locals {
		att, err := a.uploadAttachment(l)
		if err != nil {
			log.Printf("attachment upload failed (%s): %v", l.Filename, err)
			continue
		}
		uploaded = append(uploaded, *att)
	}
	return uploaded
}

// downloadToTemp fetches a URL and saves it to a temporary file, returning the path.
func downloadToTemp(fileURL, filename string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, fileURL, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download failed: %d", resp.StatusCode)
	}

	// Determine extension
	ext := ""
	if idx := strings.LastIndex(filename, "."); idx >= 0 {
		ext = strings.ToLower(filename[idx:])
	}

	tmpDir := filepath.Join(os.TempDir(), "bluetexts")
	os.MkdirAll(tmpDir, 0700)
	f, err := os.CreateTemp(tmpDir, "att-*"+ext)
	if err != nil {
		return "", err
	}
	defer f.Close()

	_, err = io.Copy(f, io.LimitReader(resp.Body, 100<<20))
	if err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// stageForVoiceHelper copies a file into the Messages.app sandbox container
// so the injected dylib can read it. Returns the staged path.
func stageForVoiceHelper(srcPath string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	// Stage in the REAL ~/Library/Messages/Attachments/ (not the sandbox container).
	// imagent runs outside the sandbox and needs files at the real path.
	stageDir := filepath.Join(home, "Library", "Messages", "Attachments", "BluTexts-staging")
	os.MkdirAll(stageDir, 0755)

	dst := filepath.Join(stageDir, filepath.Base(srcPath))
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("read source: %w", err)
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		return "", fmt.Errorf("write staged: %w", err)
	}
	return dst, nil
}

// Event types (must match API server models/models.go constants)
const (
	EventSendMessage     = "send_message"
	EventMessageStatus   = "message_status"
	EventInboundMessage  = "inbound_message"
	EventOutboundMessage = "outbound_message"
	EventHeartbeat       = "heartbeat"
	EventInitiateCall    = "initiate_call"
	EventCallControl     = "call_control"
	EventCallStatus      = "call_status"
)

type WSEvent struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type SendPayload struct {
	MessageID       string       `json:"message_id"`
	PhoneNumber     string       `json:"phone_number"`
	ToAddress       string       `json:"to_address"`
	Content         string       `json:"content"`
	IMessageAddress string       `json:"imessage_address"`
	Attachments     []Attachment `json:"attachments,omitempty"`
	Effect          string       `json:"effect,omitempty"`
}

// effectStyleIDs maps user-facing effect names (sent by the API) to Apple's
// internal expressiveSendStyleID strings used by IMCore.
var effectStyleIDs = map[string]string{
	// Bubble effects
	"slam":          "com.apple.MobileSMS.expressivesend.impact",
	"loud":          "com.apple.MobileSMS.expressivesend.loud",
	"gentle":        "com.apple.MobileSMS.expressivesend.gentle",
	"invisible_ink": "com.apple.MobileSMS.expressivesend.invisibleink",
	// Screen effects
	"echo":         "com.apple.messages.effect.CKEchoEffect",
	"spotlight":    "com.apple.messages.effect.CKSpotlightEffect",
	"balloons":     "com.apple.messages.effect.CKHappyBirthdayEffect",
	"confetti":     "com.apple.messages.effect.CKConfettiEffect",
	"love":         "com.apple.messages.effect.CKHeartEffect",
	"lasers":       "com.apple.messages.effect.CKLasersEffect",
	"fireworks":    "com.apple.messages.effect.CKFireworksEffect",
	"celebration":  "com.apple.messages.effect.CKSparklesEffect",
}

// CallPayload is sent by the API when a user initiates a FaceTime Audio call.
// The iMac joins the Agora channel to receive the agent's microphone audio
// (published by the Chrome extension) and to publish the FaceTime remote audio.
// Audio is bridged through BlackHole virtual audio devices on this Mac.
type CallPayload struct {
	CallID       string `json:"call_id"`
	To           string `json:"to"`           // contact phone/email
	FromNumber   string `json:"from_number"`  // caller ID
	AgoraChannel string `json:"agora_channel"`
	AgoraToken   string `json:"agora_token"`
	AgoraUID     uint32 `json:"agora_uid"`
	AgoraAppID   string `json:"agora_app_id"`
}

// CallControlPayload is sent by the API when the agent hangs up.
type CallControlPayload struct {
	CallID string `json:"call_id"`
	Action string `json:"action"` // "end"
}

// Attachment mirrors the server-side model for media files.
type Attachment struct {
	URL      string `json:"url"`
	Type     string `json:"type"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

type StatusPayload struct {
	MessageID    string `json:"message_id"`
	Status       string `json:"status"`
	IMessageGUID string `json:"imessage_guid,omitempty"`
	Error        string `json:"error,omitempty"`
}

type InboundPayload struct {
	IMessageGUID string       `json:"imessage_guid"`
	FromAddress  string       `json:"from_address"`
	ToAddress    string       `json:"to_address"`
	Content      string       `json:"content"`
	Attachments  []Attachment `json:"attachments,omitempty"`
	ReceivedAt   time.Time    `json:"received_at"`
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

// apiSendRecord tracks a message we recently sent via the API so we can
// filter it out when polling chat.db (prevents double-logging).
//
// Content is stored so dedup can match by content too. This matters because
// when iMessage attempts to send to a non-iMessage recipient and Messages.app
// auto-bounces to SMS via Continuity, BOTH attempts can produce chat.db rows
// with the same content. Address-only dedup would only skip one and the other
// would be re-ingested as a "new" outbound message (the duplicate-message bug
// users reported). Content matching catches both.
type apiSendRecord struct {
	ToAddress     string
	Content       string
	SentAt        time.Time
	HasAttachment bool
}

// SentMessage tracks a sent message for status polling.
type SentMessage struct {
	MessageID  string
	GUID       string
	SentAt     time.Time
	ToAddress  string
	LastStatus string
	Content    string // text content (empty for media-only messages)
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

	// Tracks recent API-initiated outbound sends so polling doesn't re-report them.
	apiSends   []apiSendRecord
	apiSendsMu sync.Mutex

	// Observable state for UI
	connected    bool
	connectedAt  time.Time
	statusMu     sync.RWMutex
	activityLog  []LogEntry
	logMu        sync.Mutex
	onStatus     func(connected bool) // callback for tray icon

	// Call bridge callbacks — the desktop wrapper wires these up so the
	// hidden WebView joins/leaves the Agora channel in response to server
	// commands. Both callbacks may be nil when the app runs headless.
	onCallStart func(CallPayload)
	onCallEnd   func(callID string)
}

// SetCallCallbacks registers the bridge hooks. The desktop wrapper uses
// runtime.EventsEmit to forward these to the hidden Svelte bridge view,
// where the Agora Web SDK actually runs.
func (a *Agent) SetCallCallbacks(onStart func(CallPayload), onEnd func(callID string)) {
	a.onCallStart = onStart
	a.onCallEnd = onEnd
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
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in Run: %v — restarting in 5s", r)
			time.Sleep(5 * time.Second)
			a.Run() // restart the whole loop
		}
	}()

	// Log Messages.app's available services on boot. Surfaces missing SMS
	// service (= Continuity / Text Message Forwarding not set up) loudly in
	// the agent log so we can debug delivery failures faster.
	a.imClient.LogAvailableServices()

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
		if r := recover(); r != nil {
			log.Printf("PANIC in runSession: %v", r)
		}
		a.connMu.Lock()
		if a.conn != nil {
			a.conn.Close()
			a.conn = nil
		}
		a.connMu.Unlock()
	}()

	go safeGoroutine("writePump", a.writePump)
	go safeGoroutine("pollMessages", a.pollMessages)
	go safeGoroutine("trackDeliveryStatus", a.trackDeliveryStatus)
	go safeGoroutine("heartbeat", a.heartbeat)

	a.readPump()
}

// safeGoroutine wraps a goroutine with panic recovery so a single failure
// doesn't crash the entire app.
func safeGoroutine(name string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in %s: %v", name, r)
		}
	}()
	fn()
}

func (a *Agent) readPump() {
	for {
		a.connMu.Lock()
		conn := a.conn
		a.connMu.Unlock()
		if conn == nil {
			return
		}

		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
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
			payload := event.Payload
			go safeGoroutine("handleSendMessage", func() { a.handleSendMessage(payload) })
		} else if event.Type == EventInitiateCall {
			payload := event.Payload
			go safeGoroutine("handleCall", func() { a.handleCall(payload) })
		} else if event.Type == EventCallControl {
			payload := event.Payload
			go safeGoroutine("handleCallControl", func() { a.handleCallControl(payload) })
		}
	}
}

func (a *Agent) writePump() {
	// Ping every 10 seconds to keep the WebSocket connection alive through
	// edge proxies (Railway closes idle connections after ~30s).
	ticker := time.NewTicker(10 * time.Second)
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

	log.Printf("Sending to %s from %s (msg_id: %s, attachments=%d)",
		req.ToAddress, fromHandle, req.MessageID, len(req.Attachments))
	a.addLog("outbound", fmt.Sprintf("→ %s", req.ToAddress))

	// ── Routing ──
	// iMessage-only. We always send as iMessage; if the recipient isn't on
	// iMessage, the chat.db status will reflect failure and we surface that
	// to the customer (no silent SMS retry). No pre-flight capability check.

	// Dedup: record once per expected chat.db row (1 for text, N for each
	// attachment plus 1 for caption if present). Content is matched too so
	// any auto-bounce duplicates get skipped alongside the primary row.
	numRecords := 1
	if len(req.Attachments) > 0 {
		numRecords = len(req.Attachments)
		if req.Content != "" {
			numRecords++
		}
	}
	for i := 0; i < numRecords; i++ {
		a.recordApiSend(req.ToAddress, req.Content, len(req.Attachments) > 0)
	}

	// Snapshot chat.db's max ROWID so we can unambiguously find the new
	// row post-send (content matching is unreliable on macOS 14+ where
	// attributedBody is populated but text is empty).
	preSendRowID, _ := a.imClient.GetLastRowID()

	// ── Attachments ──
	var attachmentPaths []string
	if len(req.Attachments) > 0 {
		for _, att := range req.Attachments {
			path, err := downloadToTemp(att.URL, att.Filename)
			if err != nil {
				log.Printf("download attachment error: %v", err)
				a.sendStatus(req.MessageID, "failed", "", "attachment download: "+err.Error())
				return
			}
			attachmentPaths = append(attachmentPaths, path)
		}
		defer func() {
			for _, p := range attachmentPaths {
				os.Remove(p)
			}
		}()

		attachService := imessage.ServiceIMessage
		for i, path := range attachmentPaths {
			isVoiceCAF := strings.HasSuffix(strings.ToLower(path), ".caf") &&
				i < len(req.Attachments) &&
				strings.HasPrefix(req.Attachments[i].Type, "audio/")

			if isVoiceCAF {
				stagedPath, stageErr := stageForVoiceHelper(path)
				if stageErr != nil {
					log.Printf("Voice staging failed: %v — sending as plain attachment", stageErr)
					if err := a.imClient.SendAttachment(req.ToAddress, path, attachService); err != nil {
						a.sendStatus(req.MessageID, "failed", "", err.Error())
						return
					}
					continue
				}
				log.Printf("Sending voice message via IMCore helper: %s", stagedPath)
				if err := a.imClient.SendVoiceMessage(req.ToAddress, stagedPath); err != nil {
					log.Printf("Voice helper failed: %v — sending as plain attachment", err)
					os.Remove(stagedPath)
					if err := a.imClient.SendAttachment(req.ToAddress, path, attachService); err != nil {
						a.sendStatus(req.MessageID, "failed", "", err.Error())
						return
					}
				} else {
					os.Remove(stagedPath)
				}
			} else {
				if err := a.imClient.SendAttachment(req.ToAddress, path, attachService); err != nil {
					log.Printf("Send attachment error: %v", err)
					a.sendStatus(req.MessageID, "failed", "", err.Error())
					return
				}
			}
		}
	}

	// ── Text ──
	if req.Content != "" {
		// iMessage-only. Effects need the IMCore dylib path; plain text
		// goes via AppleScript (simpler + faster). Failure is surfaced to
		// the customer rather than silently downgrading to SMS.
		var styleID string
		if req.Effect != "" {
			if id, ok := effectStyleIDs[req.Effect]; ok {
				styleID = id
			}
		}
		var sendErr error
		if styleID != "" {
			sendErr = a.imClient.SendIMessageWithEffect(req.ToAddress, req.Content, styleID)
			if sendErr != nil {
				log.Printf("IMCore send failed: %v — trying AppleScript iMessage", sendErr)
				sendErr = a.imClient.SendIMessage(req.ToAddress, req.Content)
			}
		} else {
			sendErr = a.imClient.SendIMessage(req.ToAddress, req.Content)
		}
		if sendErr != nil {
			a.sendStatus(req.MessageID, "failed", "", sendErr.Error())
			return
		}
	}

	// Find the chat.db row we just created so we can track delivery.
	guid := "pending-" + uuid.New().String()
	if req.Content != "" {
		lookup := a.imClient.FindRecentOutboundSince(req.ToAddress, preSendRowID, 8*time.Second)
		if lookup.Found {
			guid = lookup.GUID
			log.Printf("chat.db row found for msg=%s: guid=%s", req.MessageID, guid)
		} else {
			log.Printf("WARN: could not locate chat.db row for %s — delivery tracking disabled",
				req.ToAddress)
			a.addLog("status", fmt.Sprintf("⚠ delivery tracking unavailable for %s", req.ToAddress))
		}
	}

	a.sendStatus(req.MessageID, "sent", guid, "")

	a.sentMu.Lock()
	a.sentMessages[req.MessageID] = &SentMessage{
		MessageID:  req.MessageID,
		GUID:       guid,
		SentAt:     time.Now(),
		ToAddress:  req.ToAddress,
		LastStatus: "sent",
		Content:    req.Content,
	}
	a.sentMu.Unlock()
}

// handleCall places an outbound FaceTime Audio call on this Mac while bridging
// audio into an Agora channel. Flow:
//
//  1. Notify the hidden WebView (via onCallStart) to join the Agora channel
//     with the supplied token. The WebView picks up the agent's mic audio
//     from Agora's remote stream and writes it to BlackHole-In (which becomes
//     FaceTime.app's microphone). It captures BlackHole-Out (FaceTime output
//     = contact's voice) via getUserMedia and publishes it back into Agora
//     so the agent hears the contact.
//  2. Switch FaceTime.app's default input/output to BlackHole 2ch via
//     SwitchAudioSource so the next call routes correctly. This has to happen
//     before we open the URL so FaceTime picks up the settings at launch.
//  3. Open facetime-audio://<contact> — FaceTime shows the ringing UI.
//  4. Press Return to confirm the call (much more reliable than hunting for
//     a "Call" button across macOS versions).
//  5. Report "ringing" back to the server; the user hears audio through Agora.
func (a *Agent) handleCall(raw json.RawMessage) {
	var req CallPayload
	if err := json.Unmarshal(raw, &req); err != nil {
		log.Printf("invalid call payload: %v", err)
		return
	}

	log.Printf("Call request: to=%s call_id=%s agora_channel=%s", req.To, req.CallID, req.AgoraChannel)
	a.addLog("outbound", fmt.Sprintf("Calling %s via FaceTime Audio", req.To))

	// 1. Spin up the Agora bridge in the hidden WebView.
	if a.onCallStart != nil {
		a.onCallStart(req)
	} else {
		log.Println("onCallStart not wired — audio will not bridge to agent")
	}

	// 2. Route FaceTime.app's audio through the two BlackHole devices.
	//    - FaceTime mic (input)    = BlackHole 16ch — bridge writes agent voice here
	//    - FaceTime speaker (output) = BlackHole 2ch  — bridge captures contact voice here
	if err := setFaceTimeAudioDevices("BlackHole 16ch", "BlackHole 2ch"); err != nil {
		log.Printf("set audio device: %v (non-fatal; FaceTime may use wrong device)", err)
	}

	// 3. Open facetime-audio URL — never "facetime://" (video), never "tel://" (handoff).
	url := "facetime-audio://" + req.To
	log.Printf("Opening %s", url)
	if out, err := exec.Command("open", url).CombinedOutput(); err != nil {
		log.Printf("open URL failed: %v — %s", err, string(out))
		a.sendCallStatus(req.CallID, "failed", "could not open FaceTime: "+string(out))
		if a.onCallEnd != nil {
			a.onCallEnd(req.CallID)
		}
		return
	}

	// Give FaceTime time to open and show the confirmation sheet.
	time.Sleep(1500 * time.Millisecond)

	// 4. Press Return. FaceTime focuses the green call button by default when
	// opened via the URL scheme, so Return confirms the call. This avoids the
	// brittle button-name AppleScripting we used to do.
	pressReturn := `
tell application "FaceTime" to activate
delay 0.3
tell application "System Events"
    keystroke return
end tell
`
	if out, err := exec.Command("osascript", "-e", pressReturn).CombinedOutput(); err != nil {
		log.Printf("Return keystroke error: %v — %s", err, string(out))
		a.sendCallStatus(req.CallID, "ringing", "FaceTime opened, press Call manually")
		return
	}

	a.sendCallStatus(req.CallID, "ringing", "")
	a.addLog("outbound", fmt.Sprintf("Call ringing: %s", req.To))
}

// handleCallControl processes end/cancel from the server (agent hung up).
// We slam FaceTime.app closed — there's no clean "hang up" AppleScript that
// works reliably, and Cmd+W followed by Quit is the same effect.
func (a *Agent) handleCallControl(raw json.RawMessage) {
	var req CallControlPayload
	if err := json.Unmarshal(raw, &req); err != nil {
		log.Printf("invalid call_control payload: %v", err)
		return
	}

	log.Printf("Call control: call_id=%s action=%s", req.CallID, req.Action)

	// Tear down the Agora bridge first so the agent stops publishing immediately.
	if a.onCallEnd != nil {
		a.onCallEnd(req.CallID)
	}

	hangup := `
tell application "FaceTime"
    if it is running then
        quit
    end if
end tell
`
	_, _ = exec.Command("osascript", "-e", hangup).CombinedOutput()

	a.sendCallStatus(req.CallID, "ended", "")
	a.addLog("status", fmt.Sprintf("Call ended: %s", req.CallID))
}

// setFaceTimeAudioDevices sets FaceTime.app's input and output devices by
// writing the system default (FaceTime picks up system default at launch).
// Requires `brew install switchaudio-osx`.
func setFaceTimeAudioDevices(input, output string) error {
	if _, err := exec.LookPath("SwitchAudioSource"); err != nil {
		return fmt.Errorf("SwitchAudioSource not installed; relying on existing system default")
	}
	if out, err := exec.Command("SwitchAudioSource", "-t", "input", "-s", input).CombinedOutput(); err != nil {
		return fmt.Errorf("set input %s: %v — %s", input, err, string(out))
	}
	if out, err := exec.Command("SwitchAudioSource", "-t", "output", "-s", output).CombinedOutput(); err != nil {
		return fmt.Errorf("set output %s: %v — %s", output, err, string(out))
	}
	return nil
}

func (a *Agent) sendCallStatus(callID, status, errMsg string) {
	a.sendEvent(EventCallStatus, map[string]interface{}{
		"call_id": callID,
		"status":  status,
		"error":   errMsg,
	})
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
				// Skip outbound messages that were API-initiated — the server already
				// created the record (with its own R2-hosted attachments) and synced
				// to GHL. Reporting them would double-log. Content is passed for
				// content-aware dedup (defends against iMessage→SMS Continuity
				// bounce, which can produce >1 chat.db row per API send).
				if a.wasApiSend(msg.Handle, msg.Text) {
					log.Printf("Outbound [api-initiated] to %s — skipping event", msg.Handle)
					continue
				}
				// Direct-from-Messages.app outbound: upload any attachments so they
				// appear in the dashboard / GHL.
				attachments := a.collectInboundAttachments(msg.RowID)
				content := cleanMessageText(msg.Text, len(attachments) > 0)
				log.Printf("Outbound [%s] to %s: %.40s (attachments=%d)", svc, msg.Handle, msg.Text, len(attachments))
				a.addLog("outbound", fmt.Sprintf("→ %s [%s]: %.40s", msg.Handle, svc, msg.Text))
				a.sendEvent(EventOutboundMessage, InboundPayload{
					IMessageGUID: msg.GUID,
					FromAddress:  msg.Destination,
					ToAddress:    msg.Handle,
					Content:      content,
					Attachments:  attachments,
					ReceivedAt:   msg.Date,
				})
				continue
			}

			// Inbound: pull and upload any file attachments (voice messages,
			// photos, etc.) so the API can store them in R2 and surface them
			// to the web app + GHL.
			attachments := a.collectInboundAttachments(msg.RowID)
			content := cleanMessageText(msg.Text, len(attachments) > 0)
			log.Printf("Inbound [%s] from %s → %s: %.40s (attachments=%d)", svc, msg.Handle, msg.Destination, msg.Text, len(attachments))
			a.addLog("inbound", fmt.Sprintf("← %s → %s [%s]: %.40s", msg.Handle, msg.Destination, svc, msg.Text))
			a.sendEvent(EventInboundMessage, InboundPayload{
				IMessageGUID: msg.GUID,
				FromAddress:  msg.Handle,      // sender's phone/email
				ToAddress:    msg.Destination, // *our* local number
				Content:      content,
				Attachments:  attachments,
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
			ds, err := a.imClient.GetMessageStatus(sm.GUID)
			if err != nil {
				continue
			}
			if ds.Status == sm.LastStatus {
				continue
			}
			log.Printf("Status change for msg=%s guid=%s: %s → %s (errorCode=%d)",
				sm.MessageID, sm.GUID, sm.LastStatus, ds.Status, ds.ErrorCode)

			// iMessage-only — no SMS retry on failure. If iMessage couldn't
			// deliver (recipient on Android, blocked, etc.), surface the
			// error to the customer instead of silently swapping channels.
			if ds.Status == "failed" {
				reason := failureReason(ds.ErrorCode)
				a.addLog("status", fmt.Sprintf("✗ %s not delivered: %s", sm.ToAddress, reason))
				a.sendStatus(sm.MessageID, "failed", sm.GUID, reason)
			} else {
				a.addLog("status", fmt.Sprintf("✓ %s %s", sm.ToAddress, ds.Status))
				a.sendStatus(sm.MessageID, ds.Status, sm.GUID, "")
			}
			a.sentMu.Lock()
			if tracked, ok := a.sentMessages[sm.MessageID]; ok {
				tracked.LastStatus = ds.Status
			}
			a.sentMu.Unlock()
		}
	}
}

// failureReason converts a chat.db error code into something humans can act on.
// Codes are mostly stable across recent macOS versions; observed values:
func failureReason(code int) string {
	switch code {
	case -1:
		// Sentinel from GetMessageStatus when is_finished=1 but is_delivered=0
		// without an explicit error code — Continuity relay timeout.
		return "Not delivered — paired iPhone couldn't relay (check Text Message Forwarding is enabled for this Mac in iPhone Settings → Messages)"
	case 1, 3:
		return "Network or routing failure (check Continuity link)"
	case 22:
		return "Recipient number is invalid or unreachable"
	case 102:
		return "Relay timeout — paired iPhone may be offline or unreachable"
	case 0:
		return "Not delivered"
	default:
		return fmt.Sprintf("Not delivered (Messages.app error %d)", code)
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

// recordApiSend remembers that we just sent a message via the API
// so the poll loop doesn't re-report it as an outbound event.
func (a *Agent) recordApiSend(toAddress, content string, hasAttachment bool) {
	a.apiSendsMu.Lock()
	defer a.apiSendsMu.Unlock()
	a.apiSends = append(a.apiSends, apiSendRecord{
		ToAddress:     normalizePhone(toAddress),
		Content:       content,
		SentAt:        time.Now(),
		HasAttachment: hasAttachment,
	})
	// Prune entries older than 2 minutes
	cutoff := time.Now().Add(-2 * time.Minute)
	var fresh []apiSendRecord
	for _, r := range a.apiSends {
		if r.SentAt.After(cutoff) {
			fresh = append(fresh, r)
		}
	}
	a.apiSends = fresh
}

// wasApiSend returns true if a chat.db outbound row matches a recent API send.
//
// Matching strategy:
//   1. If the chat.db row has non-empty text AND we have a content match for
//      this address within the window: dedupe (don't consume — Messages.app
//      can produce multiple rows per send when iMessage auto-bounces to SMS,
//      and we want to skip ALL of them).
//   2. Otherwise (empty text, e.g. media-only): fall back to address-only
//      match and CONSUME the record so a single legitimate manual re-send
//      to the same contact later still gets through.
func (a *Agent) wasApiSend(toAddress, content string) bool {
	a.apiSendsMu.Lock()
	defer a.apiSendsMu.Unlock()
	normalized := normalizePhone(toAddress)
	cutoff := time.Now().Add(-2 * time.Minute)

	// Pass 1: content match (no consumption — covers the iMessage→SMS bounce)
	if content != "" {
		for _, r := range a.apiSends {
			if r.SentAt.Before(cutoff) {
				continue
			}
			if r.ToAddress == normalized && r.Content == content {
				return true
			}
		}
	}

	// Pass 2: address-only match, consume (legacy behavior, used when
	// content is empty or doesn't match anything we tracked)
	for i, r := range a.apiSends {
		if r.SentAt.Before(cutoff) {
			continue
		}
		if r.ToAddress == normalized {
			a.apiSends = append(a.apiSends[:i], a.apiSends[i+1:]...)
			return true
		}
	}
	return false
}

// cleanMessageText strips iMessage's Object Replacement Character (U+FFFC)
// placeholder that appears in the chat.db `text` column when the row is really
// an attachment. Returns "" for media-only messages so the bubble renders as
// audio/image only — otherwise the web app shows a stray "OBJ" box.
func cleanMessageText(text string, hasAttachments bool) string {
	cleaned := strings.ReplaceAll(text, "\ufffc", "")
	cleaned = strings.TrimSpace(cleaned)
	if hasAttachments && (cleaned == "" || cleaned == "[media/message]") {
		return ""
	}
	return cleaned
}

// normalizePhone strips formatting from a phone number or email for comparison.
func normalizePhone(s string) string {
	s = strings.TrimSpace(s)
	// Emails: lowercase
	if strings.Contains(s, "@") {
		return strings.ToLower(s)
	}
	// Phone: keep only digits and leading +
	var out strings.Builder
	for i, r := range s {
		if i == 0 && r == '+' {
			out.WriteRune(r)
			continue
		}
		if r >= '0' && r <= '9' {
			out.WriteRune(r)
		}
	}
	return out.String()
}
