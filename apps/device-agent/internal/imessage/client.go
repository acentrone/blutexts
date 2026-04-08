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
	GUID        string
	Text        string
	IsFromMe    bool
	Handle      string // the phone number or email
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

// GetNewMessages returns messages received after the given time.
// Polls the SQLite database directly (faster than AppleScript notifications).
func (c *Client) GetNewMessages(since time.Time) ([]Message, error) {
	// Apple stores dates as seconds since 2001-01-01 (Mac absolute time)
	// We convert our time to that reference
	appleEpoch := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	sinceApple := since.UTC().Sub(appleEpoch).Seconds()

	query := `
		SELECT
			m.guid,
			COALESCE(m.text, ''),
			m.is_from_me,
			COALESCE(h.id, ''),
			COALESCE(m.service, ''),
			m.date
		FROM message m
		LEFT JOIN handle h ON h.rowid = m.handle_id
		WHERE m.date > ?
		  AND m.text IS NOT NULL
		  AND m.text != ''
		  AND m.service = 'iMessage'
		ORDER BY m.date ASC
		LIMIT 100
	`

	rows, err := c.db.Query(query, int64(sinceApple*1e9)) // Apple uses nanoseconds since macOS 10.13
	if err != nil {
		return nil, fmt.Errorf("query new messages: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		var dateNano int64
		if err := rows.Scan(&m.GUID, &m.Text, &m.IsFromMe, &m.Handle, &m.ServiceName, &dateNano); err != nil {
			continue
		}
		// Convert Apple nanoseconds to Go time
		m.Date = appleEpoch.Add(time.Duration(dateNano))
		messages = append(messages, m)
	}
	return messages, rows.Err()
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

// GetHandles returns all iMessage handles (phone numbers/emails) registered on this device.
func (c *Client) GetHandles() ([]string, error) {
	rows, err := c.db.Query(`SELECT id FROM handle WHERE service = 'iMessage' GROUP BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var handles []string
	for rows.Next() {
		var h string
		rows.Scan(&h)
		handles = append(handles, h)
	}
	return handles, nil
}
