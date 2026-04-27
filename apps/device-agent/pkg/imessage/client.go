// Package imessage provides iMessage send/receive functionality on macOS.
//
// This package uses two mechanisms:
// 1. SENDING: AppleScript via osascript to invoke Messages.app
// 2. RECEIVING: Direct SQLite read of ~/Library/Messages/chat.db
//    (requires "Full Disk Access" permission in System Settings → Privacy)
//
// The device must be signed into iMessage in the Messages app, and the
// running user must have Full Disk Access granted to the terminal / agent binary.
package imessage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// IPC directory is inside the Messages.app sandbox container
const voiceHelperIPCDir = "Library/Containers/com.apple.MobileSMS/Data/Library/Messages/BluTexts"

// Message represents an iMessage from chat.db
type Message struct {
	RowID       int64
	GUID        string
	Text        string
	IsFromMe    bool
	Handle      string // the phone number or email of the other party
	Destination string // which local handle (number) received this message
	ServiceName string // "iMessage" or "SMS"
	Date        time.Time
}

// LocalAttachment is an attachment record from chat.db pointing at a file
// on this Mac (typically under ~/Library/Messages/Attachments/...). The
// caller must read the file and upload it before sending the message event.
type LocalAttachment struct {
	FilePath       string // absolute, ~ expanded
	Filename       string // human-readable name (transfer_name)
	MimeType       string // e.g. audio/x-caf, image/jpeg
	TotalBytes     int64
	IsVoiceMessage bool // true when the .caf came in as a native voice message
}

// Client interfaces with the macOS Messages system.
type Client struct {
	db      *sql.DB
	chatDB  string
}

// NewClient opens the Messages SQLite database.
// Requires Full Disk Access permission.
func NewClient() (*Client, error) {
	u, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("get current user: %w", err)
	}

	chatDBPath := filepath.Join(u.HomeDir, "Library", "Messages", "chat.db")
	if _, err := os.Stat(chatDBPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("chat.db not found at %s — ensure Messages.app has been set up", chatDBPath)
	}

	// Open in read-only WAL mode to avoid interfering with Messages.app
	dsn := fmt.Sprintf("file:%s?mode=ro&_journal_mode=WAL", chatDBPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open chat.db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping chat.db (check Full Disk Access permission): %w", err)
	}

	return &Client{db: db, chatDB: chatDBPath}, nil
}

// Close closes the database connection.
func (c *Client) Close() {
	if c.db != nil {
		c.db.Close()
	}
}

// Service identifies which underlying transport delivered a message.
type Service string

const (
	ServiceIMessage Service = "imessage"
	ServiceSMS      Service = "sms"
)

// OutboundLookup carries the chat.db identity + delivery state of an
// outbound message we just sent via AppleScript / IMCore. Used both to
// verify routing (service column) AND to grab the real GUID so the
// background delivery poller can actually track this message — without
// the real GUID, polling skips API sends entirely.
type OutboundLookup struct {
	GUID    string
	Service string
	Found   bool
}

// FindRecentOutboundSince waits for a new outbound chat.db row to appear
// after `sinceRowID` matching the recipient handle, then returns its GUID +
// service. Critical for delivery tracking: polling keys on chat.db GUIDs,
// not our temp "pending-..." placeholder, so without resolving the real
// GUID we'd never detect Messages.app's "Not Delivered" state.
//
// We use ROWID-diff rather than content matching because Messages.app on
// recent macOS (14+) stores message text in `attributedBody` BLOB rather
// than the `text` column, leaving `text` empty/NULL. Content-based matching
// silently fails on those rows and we end up tracking nothing.
//
// The handle.id field can be stored with or without the "+" prefix
// depending on macOS version — we try both forms.
func (c *Client) FindRecentOutboundSince(toAddress string, sinceRowID int64, timeout time.Duration) OutboundLookup {
	res := OutboundLookup{}
	if c.db == nil {
		return res
	}
	deadline := time.Now().Add(timeout)
	normalized := normalizePhoneForChatDB(toAddress)

	candidates := []string{normalized}
	if strings.HasPrefix(normalized, "+") {
		candidates = append(candidates, normalized[1:])
	} else if !strings.Contains(normalized, "@") {
		candidates = append(candidates, "+"+normalized)
	}

	for time.Now().Before(deadline) {
		for _, candidate := range candidates {
			var guid, service string
			err := c.db.QueryRow(`
				SELECT m.guid, COALESCE(m.service, '')
				FROM message m
				JOIN handle h ON h.rowid = m.handle_id
				WHERE m.is_from_me = 1
				  AND m.ROWID > ?
				  AND h.id = ?
				ORDER BY m.ROWID DESC LIMIT 1
			`, sinceRowID, candidate).Scan(&guid, &service)
			if err == nil {
				res.GUID = guid
				res.Service = service
				res.Found = true
				return res
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	// chat.db lag — caller decides whether absence is a failure.
	return res
}

// normalizePhoneForChatDB mirrors what Messages.app stores in handle.id: a
// digits-only +E.164 number, or a lowercased email. Used by chat.db lookups.
func normalizePhoneForChatDB(s string) string {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "@") {
		return strings.ToLower(s)
	}
	var b strings.Builder
	for i, r := range s {
		if i == 0 && r == '+' {
			b.WriteRune(r)
			continue
		}
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// LogAvailableServices runs an AppleScript probe and logs the Messages
// services this Mac knows about. Helpful for debugging SMS-fallback failures
// — if no SMS service appears, Continuity isn't set up correctly.
func (c *Client) LogAvailableServices() {
	script := `
tell application "Messages"
	set info to ""
	repeat with svc in services
		set info to info & (service type of svc as text) & ":" & (name of svc) & ";"
	end repeat
	return info
end tell
`
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		log.Printf("Messages services probe failed: %v", err)
		return
	}
	listing := strings.TrimSpace(string(out))
	log.Printf("Messages services available: %s", listing)
	if !strings.Contains(strings.ToLower(listing), "sms") {
		log.Printf("WARNING: no SMS service found — Continuity SMS won't work. " +
			"Pair an iPhone signed into the same Apple ID and enable Settings → Messages → Text Message Forwarding for this Mac.")
	}
}

// SendIMessage sends a plain text iMessage via AppleScript. Used as the
// emergency fallback when the dylib helper isn't loaded; otherwise the
// dylib path (with availability checking + effect support) is preferred.
func (c *Client) SendIMessage(toAddress, content string) error {
	escaped := escapeAppleScript(content)
	return runSendScript(toAddress, escaped, "iMessage")
}

func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

func runSendScript(toAddress, escapedContent, serviceType string) error {
	script := fmt.Sprintf(`
tell application "Messages"
	set targetService to first service whose service type = %s
	set targetBuddy to buddy "%s" of targetService
	send "%s" to targetBuddy
end tell
`, serviceType, toAddress, escapedContent)
	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("osascript (%s): %w — output: %s", serviceType, err, string(output))
	}
	return nil
}

// SendAttachment sends a file attachment as an iMessage. The Service param
// is kept for source compatibility with existing callers — it's no longer
// honored (we only ship iMessage), but removing it would force a wider
// signature change for no behavioral gain.
func (c *Client) SendAttachment(toAddress, filePath string, _ Service) error {
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("attachment file not found: %w", err)
	}
	escapedPath := escapeAppleScript(filePath)
	return runAttachmentScript(toAddress, escapedPath, "iMessage")
}

func runAttachmentScript(toAddress, escapedPath, serviceType string) error {
	script := fmt.Sprintf(`
tell application "Messages"
	set targetService to first service whose service type = %s
	set targetBuddy to buddy "%s" of targetService
	set attachmentFile to POSIX file "%s"
	send attachmentFile to targetBuddy
end tell
`, serviceType, toAddress, escapedPath)
	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("osascript attachment (%s): %w — output: %s", serviceType, err, string(output))
	}
	return nil
}


// SendIMessageWithEffect sends a text via iMessage through the helper dylib
// with an optional expressive send effect (slam, confetti, balloons, ...).
// The dylib always attempts the send — failure detection happens later via
// chat.db polling (the source of truth) so the Go agent can auto-retry as
// SMS without forcing a brittle pre-flight availability check here.
func (c *Client) SendIMessageWithEffect(toAddress, content, effectStyleID string) error {
	_, err := callHelperJSON(map[string]interface{}{
		"action":  "send_text",
		"to":      toAddress,
		"content": content,
		"effect":  effectStyleID,
	}, 15*time.Second)
	return err
}

// callHelperJSON handles the file-based IPC with the BluTextsHelper.dylib.
// Accepts an arbitrary JSON-serializable payload and returns a parsed
// map[string]interface{} so callers can read both string and bool fields.
// Returns the parsed response (when available) ALONG WITH an error so callers
// can inspect helper-side error codes like "not_on_imessage".
func callHelperJSON(payload map[string]interface{}, timeout time.Duration) (map[string]interface{}, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("helper: %w", err)
	}
	ipcDir := filepath.Join(home, voiceHelperIPCDir)

	cmdUUID := fmt.Sprintf("%d", time.Now().UnixNano())
	cmdPath := filepath.Join(ipcDir, "cmd-"+cmdUUID+".json")
	respPath := filepath.Join(ipcDir, "resp-"+cmdUUID+".json")

	payload["uuid"] = cmdUUID
	body, _ := json.Marshal(payload)
	if err := os.WriteFile(cmdPath, body, 0644); err != nil {
		return nil, fmt.Errorf("helper write cmd: %w", err)
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(respPath)
		if err == nil {
			os.Remove(respPath)
			var resp map[string]interface{}
			if err := json.Unmarshal(data, &resp); err != nil {
				return nil, fmt.Errorf("helper response parse: %w", err)
			}
			if errMsg, ok := resp["error"].(string); ok && errMsg != "" {
				return resp, fmt.Errorf("helper: %s", errMsg)
			}
			return resp, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	os.Remove(cmdPath)
	return nil, fmt.Errorf("helper: timed out waiting for response")
}

// SendVoiceMessage sends an audio file as a NATIVE iMessage voice message
// via the BluTextsHelper.dylib injected into Messages.app. The helper listens
// on a Unix domain socket and uses private IMCore APIs to construct an
// IMMessage with the voice-message flag (0x300005), which produces the
// inline waveform + transcription UX on the recipient's iPhone.
//
// The audio file MUST be Opus-in-CAF at 24 kHz mono.
// Falls back to SendAttachment (regular file attachment) if the helper
// socket is not available.
// SendVoiceMessage sends an audio file as a NATIVE iMessage voice message
// via the dylib helper. Voice messages are iMessage-only — caller (the web
// UI) must already have determined the recipient is iMessage-capable.
func (c *Client) SendVoiceMessage(toAddress, filePath string) error {
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("voice file not found: %w", err)
	}
	_, err := callHelperJSON(map[string]interface{}{
		"action": "send_voice",
		"to":     toAddress,
		"file":   filePath,
	}, 15*time.Second)
	return err
}

// IsVoiceHelperAvailable returns true if the BluTextsHelper ready marker exists.
func IsVoiceHelperAvailable() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	readyPath := filepath.Join(home, voiceHelperIPCDir, ".ready")
	_, err = os.Stat(readyPath)
	return err == nil
}

// GetLastRowID returns the current maximum ROWID in the message table.
func (c *Client) GetLastRowID() (int64, error) {
	var rowID int64
	err := c.db.QueryRow(`SELECT COALESCE(MAX(ROWID), 0) FROM message`).Scan(&rowID)
	return rowID, err
}

// GetMessagesSinceRowID returns all messages with ROWID greater than the given value.
// This is more reliable than date-based polling as it catches all new messages
// regardless of timestamp format differences between macOS versions.
func (c *Client) GetMessagesSinceRowID(sinceRowID int64) ([]Message, error) {
	query := `
		SELECT
			m.ROWID,
			m.guid,
			COALESCE(m.text, ''),
			m.is_from_me,
			COALESCE(h.id, ''),
			COALESCE(m.account, ''),
			COALESCE(m.service, ''),
			m.date,
			m.attributedBody
		FROM message m
		LEFT JOIN handle h ON h.rowid = m.handle_id
		WHERE m.ROWID > ?
		ORDER BY m.ROWID ASC
		LIMIT 100
	`

	appleEpoch := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)

	rows, err := c.db.Query(query, sinceRowID)
	if err != nil {
		return nil, fmt.Errorf("query new messages: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		var dateNano int64
		var attrBody []byte
		if err := rows.Scan(&m.RowID, &m.GUID, &m.Text, &m.IsFromMe, &m.Handle, &m.Destination, &m.ServiceName, &dateNano, &attrBody); err != nil {
			continue
		}
		// Strip "p:" or "e:" or "E:" prefix from the account/destination field
		if len(m.Destination) > 2 && m.Destination[1] == ':' {
			m.Destination = m.Destination[2:]
		}
		// On newer macOS, text column is empty but attributedBody has content
		if m.Text == "" && len(attrBody) > 0 {
			extracted := extractTextFromAttributedBody(attrBody)
			if extracted != "" {
				m.Text = extracted
			}
		}
		// Never drop messages — use placeholder if we can't extract content
		if m.Text == "" {
			if len(attrBody) > 0 {
				m.Text = "[media/message]"
			} else {
				// No text and no attributedBody — skip system events (typing, read receipts)
				continue
			}
		}
		m.Date = appleEpoch.Add(time.Duration(dateNano))
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

// extractTextFromAttributedBody extracts plain text from the attributedBody blob.
// Pure Go — no external tools. Handles both typedstream and bplist formats.
func extractTextFromAttributedBody(data []byte) (result string) {
	// Never let a bad blob crash the agent
	defer func() {
		if r := recover(); r != nil {
			result = ""
		}
	}()

	if len(data) == 0 {
		return ""
	}

	// Split blob on null bytes and find text segments
	segments := splitOnNulls(data)

	for _, seg := range segments {
		text := strings.TrimSpace(seg)
		if len(text) < 2 {
			continue
		}
		if isSystemString(text) {
			continue
		}
		// Check that it's mostly printable
		printable := 0
		for _, r := range text {
			if r >= 0x20 && r != 0xfffd {
				printable++
			}
		}
		if printable > len(text)/2 {
			return text
		}
	}

	return ""
}

// splitOnNulls splits binary data and returns runs of ASCII printable text.
// Only matches 0x20-0x7e (standard ASCII) to avoid binary garbage.
func splitOnNulls(data []byte) []string {
	var results []string
	start := -1

	for i := 0; i < len(data); i++ {
		b := data[i]
		isAsciiPrintable := (b >= 0x20 && b <= 0x7e) || b == 0x0a || b == 0x0d
		if isAsciiPrintable {
			if start < 0 {
				start = i
			}
		} else {
			if start >= 0 && i-start >= 2 {
				results = append(results, string(data[start:i]))
			}
			start = -1
		}
	}
	if start >= 0 && len(data)-start >= 2 {
		results = append(results, string(data[start:]))
	}

	return results
}

// isSystemString returns true if the string is a class name, key, or framework identifier.
func isSystemString(s string) bool {
	prefixes := []string{
		"NS", "__", "$", "stream", "typed", "ITMS", "com.apple",
		"Apple", "iM", "kIM", "Attributed", "Mutable", "Object",
		"String", "Dictionary", "Value", "Number", "Data",
		"Array", "Set", "bplist", "WebKit", "aps-", "MSMessage",
		"pluginPayload", "sI", "fI", "cI",
	}
	exact := map[string]bool{
		"+": true, "-": true, "null": true, "YES": true, "NO": true,
		"true": true, "false": true, "i": true, "I": true, "iI": true, "f": true,
		"c": true, "s": true, "S": true, "C": true, "q": true, "Q": true,
		"d": true, "B": true,
	}

	if exact[s] {
		return true
	}
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// GetAttachmentsForMessage returns all file attachments associated with the
// given message ROWID. File paths are resolved (leading ~ expanded). Audio
// attachments delivered as native iMessage voice messages have the
// is_audio_message bit set on the joining row, which we surface so callers
// can flag them for waveform UI.
func (c *Client) GetAttachmentsForMessage(rowID int64) ([]LocalAttachment, error) {
	// is_audio_message lives on the message row (not the attachment), but it
	// applies to every attachment in that message — voice messages always
	// have a single .caf attachment. We pass it through for tagging.
	rows, err := c.db.Query(`
		SELECT
			COALESCE(a.filename, ''),
			COALESCE(a.transfer_name, ''),
			COALESCE(a.mime_type, ''),
			COALESCE(a.total_bytes, 0),
			COALESCE(m.is_audio_message, 0)
		FROM message_attachment_join maj
		JOIN attachment a ON a.ROWID = maj.attachment_id
		JOIN message m ON m.ROWID = maj.message_id
		WHERE maj.message_id = ?
	`, rowID)
	if err != nil {
		return nil, fmt.Errorf("query attachments: %w", err)
	}
	defer rows.Close()

	home, _ := os.UserHomeDir()
	var attachments []LocalAttachment
	for rows.Next() {
		var path, name, mime string
		var bytes int64
		var isAudioMsg int
		if err := rows.Scan(&path, &name, &mime, &bytes, &isAudioMsg); err != nil {
			continue
		}
		if path == "" {
			continue
		}
		// chat.db stores paths with a literal "~" — expand it.
		if strings.HasPrefix(path, "~") && home != "" {
			path = filepath.Join(home, path[1:])
		}
		// Skip files that haven't finished downloading yet.
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if name == "" {
			name = filepath.Base(path)
		}
		if mime == "" {
			mime = guessMimeFromExt(path)
		}
		attachments = append(attachments, LocalAttachment{
			FilePath:       path,
			Filename:       name,
			MimeType:       mime,
			TotalBytes:     bytes,
			IsVoiceMessage: isAudioMsg == 1,
		})
	}
	return attachments, rows.Err()
}

// guessMimeFromExt is a tiny fallback for attachment rows where chat.db
// didn't populate mime_type (older macOS versions occasionally drop it).
func guessMimeFromExt(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".caf":
		return "audio/x-caf"
	case ".m4a":
		return "audio/m4a"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".heic":
		return "image/heic"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".pdf":
		return "application/pdf"
	}
	return "application/octet-stream"
}

// MessageDeliveryStatus is the result of polling chat.db for a sent message.
// ErrorCode is non-zero when Messages.app marked the message as failed
// (e.g. "Not Delivered" red exclamation in the UI). Common values:
//   1, 3   = network / iMessage routing failure
//   22     = bad / unrouteable recipient (often SMS to invalid number)
//   102    = relay timeout (paired iPhone unreachable / cellular dropped)
type MessageDeliveryStatus struct {
	Status    string // "pending" | "sent" | "delivered" | "read" | "failed"
	ErrorCode int
}

// GetMessageStatus checks delivery/read/error state of a sent message by GUID.
// Returns Status="failed" when Messages.app marked the message as Not
// Delivered. Failure can show up two ways depending on macOS version + send
// path:
//
//   1. error column is non-zero (most common — explicit failure code from
//      iMessage / SMS infrastructure)
//   2. is_finished=1 AND is_delivered=0 (Messages.app considered the send
//      complete but never got delivery confirmation — this is what we see
//      for SMS Continuity timeouts where the iPhone can't relay)
//
// Without both checks we'd happily report "sent" forever for messages
// the user can clearly see as Not Delivered.
func (c *Client) GetMessageStatus(guid string) (MessageDeliveryStatus, error) {
	var isDelivered, isRead, isFinished bool
	var errorCode int
	err := c.db.QueryRow(`
		SELECT is_delivered, is_read, COALESCE(is_finished, 0), COALESCE(error, 0)
		FROM message WHERE guid = ?
	`, guid).Scan(&isDelivered, &isRead, &isFinished, &errorCode)
	if err == sql.ErrNoRows {
		return MessageDeliveryStatus{Status: "pending"}, nil
	}
	if err != nil {
		return MessageDeliveryStatus{}, err
	}

	if errorCode != 0 {
		return MessageDeliveryStatus{Status: "failed", ErrorCode: errorCode}, nil
	}
	if isRead {
		return MessageDeliveryStatus{Status: "read"}, nil
	}
	if isDelivered {
		return MessageDeliveryStatus{Status: "delivered"}, nil
	}
	if isFinished {
		// Messages.app marked the message complete but never got a delivery
		// confirmation — Continuity relay/carrier timeout.
		return MessageDeliveryStatus{Status: "failed", ErrorCode: -1}, nil
	}
	return MessageDeliveryStatus{Status: "sent"}, nil
}

// GetHandles returns all iMessage identities on this device.
// Combines multiple sources: chat.db accounts, system preferences, and
// the destination_caller_id field to find phone numbers.
func (c *Client) GetHandles() ([]string, error) {
	seen := make(map[string]bool)
	var handles []string

	add := func(h string) {
		if h != "" && !seen[h] {
			seen[h] = true
			handles = append(handles, h)
		}
	}

	// Source 1: message.account field (sent messages)
	rows, err := c.db.Query(`
		SELECT DISTINCT account FROM message
		WHERE is_from_me = 1
		  AND account IS NOT NULL
		  AND account != ''
		  AND service = 'iMessage'
	`)
	if err == nil {
		for rows.Next() {
			var h string
			rows.Scan(&h)
			if len(h) > 2 && h[1] == ':' {
				add(h[2:])
			}
		}
		rows.Close()
	}

	// Source 2: message.destination_caller_id (sometimes has the phone number)
	rows2, err := c.db.Query(`
		SELECT DISTINCT destination_caller_id FROM message
		WHERE is_from_me = 1
		  AND destination_caller_id IS NOT NULL
		  AND destination_caller_id != ''
		  AND service = 'iMessage'
	`)
	if err == nil {
		for rows2.Next() {
			var h string
			rows2.Scan(&h)
			add(h)
		}
		rows2.Close()
	}

	// Source 3: Try reading iMessage aliases from system preferences
	sysHandles := c.getSystemIMHandles()
	for _, h := range sysHandles {
		add(h)
	}

	return handles, nil
}

// getSystemIMHandles reads iMessage registered addresses from macOS preferences.
func (c *Client) getSystemIMHandles() []string {
	var handles []string

	// Try com.apple.iChat IMAccounts
	out, err := exec.Command("defaults", "read", "com.apple.iChat", "Accounts").Output()
	if err == nil {
		// Parse the plist-style output for phone numbers and emails
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			// Look for phone number patterns
			if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "\"+" ) {
				num := strings.Trim(line, "\" ,;")
				if len(num) >= 10 {
					handles = append(handles, num)
				}
			}
		}
	}

	// Try to get the phone number via AppleScript from Messages.app
	script := `tell application "Messages" to get the name of every account whose service type is iMessage`
	out2, err := exec.Command("osascript", "-e", script).Output()
	if err == nil {
		for _, name := range strings.Split(strings.TrimSpace(string(out2)), ", ") {
			name = strings.TrimSpace(name)
			if name != "" {
				handles = append(handles, name)
			}
		}
	}

	return handles
}
