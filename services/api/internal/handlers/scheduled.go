package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/bluesend/api/internal/middleware"
	"github.com/bluesend/api/internal/services/messaging"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ScheduledHandler struct {
	db    *pgxpool.Pool
	asynq *asynq.Client
}

func NewScheduledHandler(db *pgxpool.Pool, asynqClient *asynq.Client) *ScheduledHandler {
	return &ScheduledHandler{db: db, asynq: asynqClient}
}

// POST /api/messages/schedule — schedule a message for later
func (h *ScheduledHandler) Create(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())
	userID, _ := middleware.GetUserID(r.Context())

	var body struct {
		PhoneNumberID string `json:"phone_number_id"`
		ToAddress     string `json:"to_address"`
		Content       string `json:"content"`
		Attachments   []struct {
			URL      string `json:"url"`
			Type     string `json:"type"`
			Filename string `json:"filename"`
			Size     int64  `json:"size"`
			WebURL   string `json:"web_url,omitempty"`
		} `json:"attachments,omitempty"`
		Effect      string `json:"effect,omitempty"`
		ScheduledAt string `json:"scheduled_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if body.PhoneNumberID == "" || body.ToAddress == "" {
		writeError(w, "phone_number_id and to_address required", http.StatusBadRequest)
		return
	}
	if body.Content == "" && len(body.Attachments) == 0 {
		writeError(w, "content or attachments required", http.StatusBadRequest)
		return
	}
	if body.ScheduledAt == "" {
		writeError(w, "scheduled_at required", http.StatusBadRequest)
		return
	}

	scheduledAt, err := time.Parse(time.RFC3339, body.ScheduledAt)
	if err != nil {
		writeError(w, "scheduled_at must be RFC3339 format", http.StatusBadRequest)
		return
	}
	if scheduledAt.Before(time.Now().Add(1 * time.Minute)) {
		writeError(w, "scheduled_at must be at least 1 minute in the future", http.StatusBadRequest)
		return
	}

	phoneNumberID, err := uuid.Parse(body.PhoneNumberID)
	if err != nil {
		writeError(w, "invalid phone_number_id", http.StatusBadRequest)
		return
	}

	// Verify phone number belongs to account
	var exists bool
	h.db.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM phone_numbers WHERE id = $1 AND account_id = $2 AND status = 'active')`,
		phoneNumberID, accountID,
	).Scan(&exists)
	if !exists {
		writeError(w, "phone number not found or not active", http.StatusBadRequest)
		return
	}

	attachJSON, _ := json.Marshal(body.Attachments)
	if len(body.Attachments) == 0 {
		attachJSON = []byte("[]")
	}

	id := uuid.New()
	var effect *string
	if body.Effect != "" {
		effect = &body.Effect
	}

	_, err = h.db.Exec(r.Context(), `
		INSERT INTO scheduled_messages (id, account_id, phone_number_id, to_address, content, attachments, effect, scheduled_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, id, accountID, phoneNumberID, body.ToAddress, body.Content, attachJSON, effect, scheduledAt, userID)
	if err != nil {
		writeError(w, "failed to schedule message: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Enqueue Asynq task to fire at scheduled time
	payload, _ := json.Marshal(map[string]string{
		"scheduled_message_id": id.String(),
		"account_id":           accountID.String(),
	})
	task := asynq.NewTask(messaging.TaskSendScheduled, payload,
		asynq.ProcessAt(scheduledAt),
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Second),
	)
	if _, err := h.asynq.Enqueue(task); err != nil {
		// Log but don't fail — the message is stored and can be retried
		_ = err
	}

	writeJSON(w, map[string]interface{}{
		"id":           id.String(),
		"scheduled_at": scheduledAt.Format(time.RFC3339),
		"status":       "pending",
	}, http.StatusCreated)
}

// GET /api/messages/scheduled — list scheduled messages
func (h *ScheduledHandler) List(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())

	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending"
	}

	rows, err := h.db.Query(r.Context(), `
		SELECT id::text, phone_number_id::text, to_address, content, attachments, effect,
		       scheduled_at, status, sent_at, error_message, created_at
		FROM scheduled_messages
		WHERE account_id = $1 AND status = $2
		ORDER BY scheduled_at ASC
		LIMIT 100
	`, accountID, status)
	if err != nil {
		writeError(w, "database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Row struct {
		ID            string      `json:"id"`
		PhoneNumberID string      `json:"phone_number_id"`
		ToAddress     string      `json:"to_address"`
		Content       string      `json:"content"`
		Attachments   interface{} `json:"attachments"`
		Effect        *string     `json:"effect,omitempty"`
		ScheduledAt   string      `json:"scheduled_at"`
		Status        string      `json:"status"`
		SentAt        *string     `json:"sent_at,omitempty"`
		ErrorMessage  *string     `json:"error_message,omitempty"`
		CreatedAt     string      `json:"created_at"`
	}

	var items []Row
	for rows.Next() {
		var row Row
		var attsJSON []byte
		var scheduledAt, createdAt time.Time
		var sentAt *time.Time

		if err := rows.Scan(
			&row.ID, &row.PhoneNumberID, &row.ToAddress, &row.Content, &attsJSON, &row.Effect,
			&scheduledAt, &row.Status, &sentAt, &row.ErrorMessage, &createdAt,
		); err != nil {
			continue
		}
		row.ScheduledAt = scheduledAt.Format(time.RFC3339)
		row.CreatedAt = createdAt.Format(time.RFC3339)
		if sentAt != nil {
			s := sentAt.Format(time.RFC3339)
			row.SentAt = &s
		}

		var atts interface{}
		json.Unmarshal(attsJSON, &atts)
		if atts == nil {
			atts = []interface{}{}
		}
		row.Attachments = atts

		items = append(items, row)
	}
	if items == nil {
		items = []Row{}
	}

	writeJSON(w, map[string]interface{}{"scheduled_messages": items}, http.StatusOK)
}

// DELETE /api/messages/scheduled/{id} — cancel a scheduled message
func (h *ScheduledHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())
	msgID := chi.URLParam(r, "scheduledID")

	res, err := h.db.Exec(r.Context(), `
		UPDATE scheduled_messages SET status = 'cancelled', updated_at = NOW()
		WHERE id = $1 AND account_id = $2 AND status = 'pending'
	`, msgID, accountID)
	if err != nil {
		writeError(w, "cancel failed", http.StatusInternalServerError)
		return
	}
	if res.RowsAffected() == 0 {
		writeError(w, "scheduled message not found or already sent", http.StatusNotFound)
		return
	}

	writeJSON(w, map[string]string{"status": "cancelled"}, http.StatusOK)
}
