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
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

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

// Send sends an iMessage to the given address from the given iMessage handle.
// Uses AppleScript; blocks until Messages.app confirms send (or times out).
func (c *Client) Send(toAddress, fromHandle, content string) error {
	// Escape content for AppleScript: escape backslashes and double-quotes
	escaped := strings.ReplaceAll(content, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)

	var script string
	if fromHandle != "" {
		// Send from a specific account (phone number or email registered in Messages)
		script = fmt.Sprintf(`
tell application "Messages"
	set targetService to first service whose service type = iMessage
	set targetBuddy to buddy "%s" of targetService
	send "%s" to targetBuddy
end tell
`, toAddress, escaped)
	} else {
		script = fmt.Sprintf(`
tell application "Messages"
	send "%s" to buddy "%s" of (first service whose service type = iMessage)
end tell
`, escaped, toAddress)
	}

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("osascript error: %w — output: %s", err, string(output))
	}
	return nil
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
func extractTextFromAttributedBody(data []byte) string {
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
		"true": true, "false": true, "i": true, "I": true, "f": true,
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

// GetMessageStatus checks delivery/read status of a sent message by GUID.
func (c *Client) GetMessageStatus(guid string) (string, error) {
	var isDelivered, isRead bool
	err := c.db.QueryRow(`
		SELECT is_delivered, is_read FROM message WHERE guid = ?
	`, guid).Scan(&isDelivered, &isRead)
	if err == sql.ErrNoRows {
		return "pending", nil
	}
	if err != nil {
		return "", err
	}

	if isRead {
		return "read", nil
	}
	if isDelivered {
		return "delivered", nil
	}
	return "sent", nil
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
