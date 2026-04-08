package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/bluesend/api/internal/middleware"
	"github.com/bluesend/api/internal/models"
	"github.com/bluesend/api/internal/services/messaging"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MessageHandler struct {
	db     *pgxpool.Pool
	router *messaging.Router
}

func NewMessageHandler(db *pgxpool.Pool, router *messaging.Router) *MessageHandler {
	return &MessageHandler{db: db, router: router}
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
	if req.Content == "" || req.ToAddress == "" || req.PhoneNumberID == "" {
		writeError(w, "phone_number_id, to_address, and content are required", http.StatusBadRequest)
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
		       ct.imessage_address, ct.name,
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
		ContactAddress  string  `json:"contact_address"`
		ContactName     *string `json:"contact_name"`
		PhoneNumber     string  `json:"phone_number"`
		PhoneDisplayName *string `json:"phone_display_name"`
	}

	var conversations []ConversationRow
	for rows.Next() {
		var c ConversationRow
		if err := rows.Scan(
			&c.ID, &c.PhoneNumberID, &c.ContactID, &c.GHLConversationID,
			&c.LastMessageAt, &c.LastMessagePreview, &c.MessageCount, &c.UnreadCount, &c.Status,
			&c.ContactAddress, &c.ContactName,
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
		SELECT id, conversation_id, direction, content, imessage_guid,
		       status, sent_at, delivered_at, read_at, failed_at, error_message, created_at
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
		if err := rows.Scan(
			&m.ID, &m.ConversationID, &m.Direction, &m.Content, &m.IMessageGUID,
			&m.Status, &m.SentAt, &m.DeliveredAt, &m.ReadAt, &m.FailedAt, &m.ErrorMessage, &m.CreatedAt,
		); err != nil {
			continue
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

	var stats models.DashboardStats

	// Total sent (last 30 days)
	h.db.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM messages
		WHERE account_id = $1 AND direction = 'outbound' AND created_at >= NOW() - INTERVAL '30 days'
	`, accountID).Scan(&stats.TotalSent)

	// Delivered
	h.db.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM messages
		WHERE account_id = $1 AND direction = 'outbound' AND status IN ('delivered', 'read')
		AND created_at >= NOW() - INTERVAL '30 days'
	`, accountID).Scan(&stats.TotalDelivered)

	// Replied (unique contacts that sent inbound after outbound)
	h.db.QueryRow(r.Context(), `
		SELECT COUNT(DISTINCT contact_id) FROM messages
		WHERE account_id = $1 AND direction = 'inbound'
		AND created_at >= NOW() - INTERVAL '30 days'
	`, accountID).Scan(&stats.TotalReplied)

	// Response rate
	if stats.TotalSent > 0 {
		stats.ResponseRate = float64(stats.TotalReplied) / float64(stats.TotalSent) * 100
	}

	// Active conversations
	h.db.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM conversations WHERE account_id = $1 AND status = 'open'
	`, accountID).Scan(&stats.ActiveConvos)

	// Today's new contacts
	h.db.QueryRow(r.Context(), `
		SELECT COALESCE(SUM(message_count), 0) FROM rate_limit_daily
		JOIN phone_numbers pn ON pn.id = phone_number_id
		WHERE pn.account_id = $1 AND date = CURRENT_DATE AND is_new_contact = true
	`, accountID).Scan(&stats.TodayNewContacts)

	// Daily limit
	h.db.QueryRow(r.Context(), `
		SELECT COALESCE(SUM(daily_new_contact_limit), 50) FROM phone_numbers
		WHERE account_id = $1 AND status = 'active'
	`, accountID).Scan(&stats.DailyLimit)

	writeJSON(w, stats, http.StatusOK)
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
