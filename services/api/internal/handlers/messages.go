package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/bluesend/api/internal/middleware"
	"github.com/bluesend/api/internal/models"
	"github.com/bluesend/api/internal/services/messaging"
	"github.com/bluesend/api/internal/services/storage"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MessageHandler struct {
	db      *pgxpool.Pool
	router  *messaging.Router
	storage *storage.R2Client
}

func NewMessageHandler(db *pgxpool.Pool, router *messaging.Router, r2 *storage.R2Client) *MessageHandler {
	return &MessageHandler{db: db, router: router, storage: r2}
}

// POST /api/messages/upload — accepts multipart file upload, stores in R2, returns URL
func (h *MessageHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		writeError(w, "media uploads are not configured on this server", http.StatusServiceUnavailable)
		return
	}

	// 100MB hard cap (videos). Per-type limits enforced after we read content type below.
	const maxImageSize = 25 << 20  // 25 MB
	const maxVideoSize = 100 << 20 // 100 MB
	const maxAudioSize = 25 << 20  // 25 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxVideoSize)
	if err := r.ParseMultipartForm(maxVideoSize); err != nil {
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

	// Validate supported types
	if !isSupportedMediaType(contentType) {
		writeError(w, "unsupported file type: "+contentType, http.StatusBadRequest)
		return
	}

	// Per-type size validation
	switch {
	case strings.HasPrefix(contentType, "image/"):
		if int64(len(data)) > maxImageSize {
			writeError(w, "image exceeds 25MB limit", http.StatusBadRequest)
			return
		}
	case strings.HasPrefix(contentType, "video/"):
		if int64(len(data)) > maxVideoSize {
			writeError(w, "video exceeds 100MB limit", http.StatusBadRequest)
			return
		}
	case strings.HasPrefix(contentType, "audio/"):
		if int64(len(data)) > maxAudioSize {
			writeError(w, "audio exceeds 25MB limit", http.StatusBadRequest)
			return
		}
	}

	filename := header.Filename
	size := header.Size
	originalData := data
	originalContentType := contentType
	originalFilename := filename
	webURL := "" // Browser-playable URL (set for audio conversions)

	// Voice memos: convert browser-recorded webm/ogg → Opus-in-CAF at 24kHz mono.
	// CAF is the format iMessage uses for native voice messages. Browsers can't
	// play CAF, so we upload BOTH the original webm (for web playback) and the
	// converted CAF (for iMessage delivery).
	if strings.HasPrefix(contentType, "audio/webm") || strings.HasPrefix(contentType, "audio/ogg") {
		// Upload the original webm first (for browser playback)
		log.Printf("upload: storing original %s (%d bytes) for web playback", contentType, len(data))
		origURL, err := h.storage.Upload(r.Context(), originalData, originalContentType, originalFilename)
		if err != nil {
			log.Printf("upload: original webm upload failed: %v", err)
		} else {
			webURL = origURL
		}

		// Convert to CAF for iMessage
		log.Printf("upload: converting %s (%d bytes) to Opus@24k CAF", contentType, len(data))
		converted, err := convertToOpusCAF(data)
		if err != nil {
			log.Printf("upload: Opus CAF conversion FAILED: %v — using original", err)
		} else {
			data = converted
			contentType = "audio/x-caf"
			base := filename
			for _, ext := range []string{".webm", ".ogg", ".opus"} {
				base = strings.TrimSuffix(base, ext)
			}
			filename = base + ".caf"
			size = int64(len(data))
			log.Printf("upload: converted to Opus CAF (%d bytes) filename=%s", size, filename)
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
		WebURL:   webURL,
	}, http.StatusOK)
}

// convertToOpusCAF converts arbitrary audio input (webm/opus/ogg) to the
// exact format iOS voice memos use: Opus codec in a CAF container at
// 24000 Hz mono. This matches what Sendblue and LoopMessage require, and
// what iPhones produce when you tap-and-hold the mic in Messages.app.
// Captures ffmpeg stderr so conversion failures surface in logs.
func convertToOpusCAF(input []byte) ([]byte, error) {
	inFile, err := os.CreateTemp("", "audio-in-*.webm")
	if err != nil {
		return nil, err
	}
	defer os.Remove(inFile.Name())
	if _, err := inFile.Write(input); err != nil {
		inFile.Close()
		return nil, err
	}
	inFile.Close()

	outPath := inFile.Name() + ".caf"
	defer os.Remove(outPath)

	cmd := exec.Command("ffmpeg",
		"-y",
		"-i", inFile.Name(),
		"-c:a", "libopus",
		"-b:a", "24k",
		"-ar", "24000",
		"-ac", "1",
		"-f", "caf",
		outPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w — %s", err, stderr.String())
	}

	return os.ReadFile(outPath)
}

func isSupportedMediaType(ct string) bool {
	ct = strings.ToLower(ct)
	allowed := []string{
		"image/jpeg", "image/jpg", "image/png", "image/gif", "image/webp", "image/heic",
		"video/mp4", "video/quicktime", "video/mov",
		"audio/m4a", "audio/mp4", "audio/mpeg", "audio/webm", "audio/ogg", "audio/x-m4a",
	}
	for _, a := range allowed {
		if strings.HasPrefix(ct, a) {
			return true
		}
	}
	return false
}

// ConvertCAFToMP3 transcodes a native iMessage voice-message .caf (Opus@24k
// in a CAF container) into mp3 so browsers and GHL can play / accept it.
// Both Apple Mail and the Chrome <audio> element refuse to decode CAF, and
// GHL's media uploads only accept a small whitelist (mp3 is the safest
// audio choice). Exposed for use by the device upload handler on inbound
// voice messages.
func ConvertCAFToMP3(input []byte) ([]byte, error) {
	inFile, err := os.CreateTemp("", "voice-in-*.caf")
	if err != nil {
		return nil, err
	}
	defer os.Remove(inFile.Name())
	if _, err := inFile.Write(input); err != nil {
		inFile.Close()
		return nil, err
	}
	inFile.Close()

	outPath := inFile.Name() + ".mp3"
	defer os.Remove(outPath)

	cmd := exec.Command("ffmpeg",
		"-y",
		"-i", inFile.Name(),
		"-c:a", "libmp3lame",
		"-q:a", "5", // ~130 kbps VBR — speech-quality
		"-ac", "1",
		"-ar", "24000",
		outPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg caf→mp3: %w — %s", err, stderr.String())
	}

	return os.ReadFile(outPath)
}

// POST /api/messages/send
func (h *MessageHandler) Send(w http.ResponseWriter, r *http.Request) {
	accountID, ok := middleware.GetAccountID(r.Context())
	if !ok {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req models.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.ToAddress == "" || req.PhoneNumberID == "" {
		writeError(w, "phone_number_id and to_address are required", http.StatusBadRequest)
		return
	}
	if req.Content == "" && len(req.Attachments) == 0 {
		writeError(w, "content or attachments required", http.StatusBadRequest)
		return
	}
	if len(req.Content) > 5000 {
		writeError(w, "content exceeds 5000 character limit", http.StatusBadRequest)
		return
	}

	resp, err := h.router.Send(r.Context(), &req, accountID)
	if err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	status := http.StatusCreated
	if resp.RateLimited {
		status = http.StatusTooManyRequests
	}
	writeJSON(w, resp, status)
}

// GET /api/conversations
func (h *MessageHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())

	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
		if limit > 100 {
			limit = 100
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		offset, _ = strconv.Atoi(o)
	}

	rows, err := h.db.Query(r.Context(), `
		SELECT c.id, c.phone_number_id, c.contact_id, c.ghl_conversation_id,
		       c.last_message_at, c.last_message_preview, c.message_count, c.unread_count, c.status,
		       ct.imessage_address, ct.name, ct.imessage_capable,
		       pn.number, pn.display_name
		FROM conversations c
		JOIN contacts ct ON ct.id = c.contact_id
		JOIN phone_numbers pn ON pn.id = c.phone_number_id
		WHERE c.account_id = $1
		ORDER BY c.last_message_at DESC NULLS LAST
		LIMIT $2 OFFSET $3
	`, accountID, limit, offset)
	if err != nil {
		writeError(w, "database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type ConversationRow struct {
		models.Conversation
		ContactAddress         string  `json:"contact_address"`
		ContactName            *string `json:"contact_name"`
		ContactIMessageCapable *bool   `json:"contact_imessage_capable"`
		PhoneNumber            string  `json:"phone_number"`
		PhoneDisplayName       *string `json:"phone_display_name"`
	}

	var conversations []ConversationRow
	for rows.Next() {
		var c ConversationRow
		if err := rows.Scan(
			&c.ID, &c.PhoneNumberID, &c.ContactID, &c.GHLConversationID,
			&c.LastMessageAt, &c.LastMessagePreview, &c.MessageCount, &c.UnreadCount, &c.Status,
			&c.ContactAddress, &c.ContactName, &c.ContactIMessageCapable,
			&c.PhoneNumber, &c.PhoneDisplayName,
		); err != nil {
			continue
		}
		conversations = append(conversations, c)
	}

	if conversations == nil {
		conversations = []ConversationRow{}
	}

	writeJSON(w, map[string]interface{}{
		"conversations": conversations,
		"limit":         limit,
		"offset":        offset,
	}, http.StatusOK)
}

// GET /api/conversations/{conversationID}/messages
func (h *MessageHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())
	conversationID := chi.URLParam(r, "conversationID")

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
		if limit > 200 {
			limit = 200
		}
	}
	before := r.URL.Query().Get("before") // cursor-based pagination

	query := `
		SELECT id, conversation_id, direction, content, attachments, imessage_guid,
		       status, sent_at, delivered_at, read_at, failed_at, error_message, service, created_at
		FROM messages
		WHERE conversation_id = $1 AND account_id = $2
	`
	args := []interface{}{conversationID, accountID}

	if before != "" {
		query += ` AND created_at < (SELECT created_at FROM messages WHERE id = $3)`
		args = append(args, before)
	}
	query += ` ORDER BY created_at DESC LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, limit)

	rows, err := h.db.Query(r.Context(), query, args...)
	if err != nil {
		writeError(w, "database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var m models.Message
		var attachmentsJSON []byte
		if err := rows.Scan(
			&m.ID, &m.ConversationID, &m.Direction, &m.Content, &attachmentsJSON, &m.IMessageGUID,
			&m.Status, &m.SentAt, &m.DeliveredAt, &m.ReadAt, &m.FailedAt, &m.ErrorMessage, &m.Service, &m.CreatedAt,
		); err != nil {
			continue
		}
		if len(attachmentsJSON) > 0 {
			_ = json.Unmarshal(attachmentsJSON, &m.Attachments)
		}
		messages = append(messages, m)
	}

	// Mark conversation as read
	h.db.Exec(r.Context(), `UPDATE conversations SET unread_count = 0 WHERE id = $1 AND account_id = $2`,
		conversationID, accountID)

	if messages == nil {
		messages = []models.Message{}
	}

	writeJSON(w, map[string]interface{}{
		"messages": messages,
		"limit":    limit,
	}, http.StatusOK)
}

// GET /api/dashboard/stats
func (h *MessageHandler) GetDashboardStats(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())

	// Resolve date window from ?from / ?to / ?range. Default: last 30d.
	from, to := resolveDateRange(r)

	var stats models.DashboardStats
	stats.From = from
	stats.To = to

	// ── Top-level aggregates ──────────────────────────────────────────────
	// Counts span both services. The per-service breakdown is computed
	// below in serviceStats(). Keeping the aggregate query separate keeps
	// response_rate meaningful even if a customer has only one service.
	h.db.QueryRow(r.Context(), `
		SELECT
		  COUNT(*) FILTER (WHERE direction = 'outbound'),
		  COUNT(*) FILTER (WHERE direction = 'outbound' AND status IN ('delivered','read')),
		  COUNT(DISTINCT contact_id) FILTER (WHERE direction = 'inbound')
		FROM messages
		WHERE account_id = $1 AND created_at >= $2 AND created_at < $3
	`, accountID, from, to).Scan(&stats.TotalSent, &stats.TotalDelivered, &stats.TotalReplied)

	// Reply rate uses "% of contacts you reached out to that wrote back" —
	// denominator is distinct contacts messaged, NOT total send volume.
	// (The previous formula divided replies by send count, which made any
	// list with multiple sends per contact look broken.)
	var totalContactsMessaged int
	h.db.QueryRow(r.Context(), `
		SELECT COUNT(DISTINCT contact_id) FROM messages
		WHERE account_id = $1 AND direction = 'outbound'
		  AND created_at >= $2 AND created_at < $3
	`, accountID, from, to).Scan(&totalContactsMessaged)
	if totalContactsMessaged > 0 {
		stats.ResponseRate = float64(stats.TotalReplied) / float64(totalContactsMessaged) * 100
	}

	// Per-service breakdown — direct apples-to-apples comparison in the
	// same window. A contact who got both iMessage AND SMS sends and then
	// replied counts for both services (we're measuring channel
	// performance, not deduping the customer).
	stats.Breakdown.IMessage = h.serviceStats(r, accountID, "imessage", from, to)
	stats.Breakdown.SMS = h.serviceStats(r, accountID, "sms", from, to)

	// "Today" counters are independent of the selected window — they
	// power the daily-cap progress bar (always today, regardless of the
	// range the user picked for the response-rate panel).
	h.db.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM conversations WHERE account_id = $1 AND status = 'open'
	`, accountID).Scan(&stats.ActiveConvos)

	h.db.QueryRow(r.Context(), `
		SELECT COALESCE(SUM(message_count), 0) FROM rate_limit_daily
		JOIN phone_numbers pn ON pn.id = phone_number_id
		WHERE pn.account_id = $1 AND date = CURRENT_DATE AND is_new_contact = true
	`, accountID).Scan(&stats.TodayNewContacts)

	h.db.QueryRow(r.Context(), `
		SELECT COALESCE(SUM(daily_new_contact_limit), 50) FROM phone_numbers
		WHERE account_id = $1 AND status = 'active'
	`, accountID).Scan(&stats.DailyLimit)

	writeJSON(w, stats, http.StatusOK)
}

// serviceStats computes the per-service slice of the dashboard breakdown.
// See models.ServiceStats for the reply-rate definition (and why we use it).
func (h *MessageHandler) serviceStats(r *http.Request, accountID interface{}, service string, from, to time.Time) models.ServiceStats {
	var s models.ServiceStats

	// Sent / delivered / contacts-messaged for this service in the window.
	h.db.QueryRow(r.Context(), `
		SELECT
		  COUNT(*),
		  COUNT(*) FILTER (WHERE status IN ('delivered','read')),
		  COUNT(DISTINCT contact_id)
		FROM messages
		WHERE account_id = $1 AND direction = 'outbound' AND service = $2
		  AND created_at >= $3 AND created_at < $4
	`, accountID, service, from, to).Scan(&s.Sent, &s.Delivered, &s.ContactsMessaged)

	// Replies attributed to this service: distinct inbound senders whose
	// contact also received an outbound on this service in the same window.
	// EXISTS keeps it efficient — no big join to materialize.
	h.db.QueryRow(r.Context(), `
		SELECT COUNT(DISTINCT m.contact_id)
		FROM messages m
		WHERE m.account_id = $1 AND m.direction = 'inbound'
		  AND m.created_at >= $2 AND m.created_at < $3
		  AND EXISTS (
		    SELECT 1 FROM messages o
		    WHERE o.account_id = m.account_id
		      AND o.contact_id = m.contact_id
		      AND o.direction = 'outbound'
		      AND o.service = $4
		      AND o.created_at >= $2 AND o.created_at < $3
		  )
	`, accountID, from, to, service).Scan(&s.ContactsReplied)

	if s.ContactsMessaged > 0 {
		s.ReplyRate = float64(s.ContactsReplied) / float64(s.ContactsMessaged) * 100
	}
	return s
}

// resolveDateRange normalizes ?from / ?to / ?range into a [from, to) window.
// Accepts:
//   ?range=7d|30d|90d              — preset shortcut
//   ?from=YYYY-MM-DD&to=YYYY-MM-DD — explicit (inclusive of both days)
// Falls back to last 30d on any invalid input — read-only dashboard, no
// reason to 400 a button click.
func resolveDateRange(r *http.Request) (time.Time, time.Time) {
	now := time.Now().UTC()
	to := now

	if rng := r.URL.Query().Get("range"); rng != "" {
		switch rng {
		case "7d":
			return now.Add(-7 * 24 * time.Hour), to
		case "30d":
			return now.Add(-30 * 24 * time.Hour), to
		case "90d":
			return now.Add(-90 * 24 * time.Hour), to
		}
	}

	defaultFrom := now.Add(-30 * 24 * time.Hour)
	from := defaultFrom

	if s := r.URL.Query().Get("from"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			from = t.UTC()
		}
	}
	if s := r.URL.Query().Get("to"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			to = t.UTC().Add(24 * time.Hour) // inclusive of the chosen day
			if to.After(now) {
				to = now
			}
		}
	}
	if !from.Before(to) {
		return defaultFrom, now
	}
	return from, to
}

// GET /api/account/info — returns phone number, device status, and setup info for the user's dashboard
func (h *MessageHandler) GetAccountInfo(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())

	type PhoneInfo struct {
		ID              string  `json:"id"`
		Number          string  `json:"number"`
		IMessageAddress *string `json:"imessage_address"`
		DisplayName     *string `json:"display_name"`
		Status          string  `json:"status"`
		DeviceName      *string `json:"device_name"`
		DeviceStatus    *string `json:"device_status"`
		DeviceLastSeen  *string `json:"device_last_seen"`
	}

	var info PhoneInfo
	var voiceEnabled bool
	err := h.db.QueryRow(r.Context(), `
		SELECT pn.id::text, pn.number, pn.imessage_address, pn.display_name, pn.status,
		       d.name, d.status, d.last_seen_at::text, pn.voice_enabled
		FROM phone_numbers pn
		LEFT JOIN devices d ON d.id = pn.device_id
		WHERE pn.account_id = $1 AND pn.status = 'active'
		LIMIT 1
	`, accountID).Scan(
		&info.ID, &info.Number, &info.IMessageAddress, &info.DisplayName, &info.Status,
		&info.DeviceName, &info.DeviceStatus, &info.DeviceLastSeen, &voiceEnabled,
	)

	var callingEnabled bool
	h.db.QueryRow(r.Context(),
		`SELECT calling_enabled FROM accounts WHERE id = $1`, accountID,
	).Scan(&callingEnabled)

	if err != nil {
		// No phone number assigned yet
		writeJSON(w, map[string]interface{}{
			"has_number":      false,
			"calling_enabled": callingEnabled,
		}, http.StatusOK)
		return
	}

	writeJSON(w, map[string]interface{}{
		"has_number":      true,
		"phone":           info,
		"calling_enabled": callingEnabled && voiceEnabled,
	}, http.StatusOK)
}

// GET /api/messages/export — CSV export
func (h *MessageHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())

	rows, err := h.db.Query(r.Context(), `
		SELECT m.created_at, m.direction, ct.imessage_address, ct.name,
		       pn.number, m.content, m.status
		FROM messages m
		JOIN contacts ct ON ct.id = m.contact_id
		JOIN phone_numbers pn ON pn.id = m.phone_number_id
		WHERE m.account_id = $1
		ORDER BY m.created_at DESC
		LIMIT 10000
	`, accountID)
	if err != nil {
		writeError(w, "export error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="bluesend-messages.csv"`)

	w.Write([]byte("Timestamp,Direction,Contact Address,Contact Name,Phone Number,Content,Status\n"))
	for rows.Next() {
		var ts, direction, address, pn, content, status string
		var name *string
		rows.Scan(&ts, &direction, &address, &name, &pn, &content, &status)
		n := ""
		if name != nil {
			n = *name
		}
		w.Write([]byte(csvEscape(ts) + "," + csvEscape(direction) + "," + csvEscape(address) + "," +
			csvEscape(n) + "," + csvEscape(pn) + "," + csvEscape(content) + "," + csvEscape(status) + "\n"))
	}
}

func csvEscape(s string) string {
	// Wrap in quotes if contains comma, newline, or quote
	for _, c := range s {
		if c == ',' || c == '\n' || c == '"' {
			return `"` + replaceAll(s, `"`, `""`) + `"`
		}
	}
	return s
}

func replaceAll(s, old, new string) string {
	result := ""
	for i := 0; i < len(s); i++ {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			result += new
			i += len(old) - 1
		} else {
			result += string(s[i])
		}
	}
	return result
}
