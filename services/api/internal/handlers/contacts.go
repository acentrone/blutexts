package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/bluesend/api/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ContactHandler struct {
	db *pgxpool.Pool
}

func NewContactHandler(db *pgxpool.Pool) *ContactHandler {
	return &ContactHandler{db: db}
}

// GET /api/contacts — list all contacts with search, tag filter, pagination
func (h *ContactHandler) List(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())

	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
		if limit > 200 {
			limit = 200
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		offset, _ = strconv.Atoi(o)
	}

	search := strings.TrimSpace(r.URL.Query().Get("search"))
	tag := strings.TrimSpace(r.URL.Query().Get("tag"))

	query := `
		SELECT c.id, c.imessage_address, c.name, c.email, c.company, c.notes,
		       c.tags, c.custom_fields, c.ghl_contact_id, c.imessage_capable,
		       c.first_message_at, c.last_message_at, c.message_count,
		       c.created_at, c.updated_at,
		       (SELECT COUNT(*) FROM conversations WHERE contact_id = c.id AND account_id = c.account_id) as conversation_count
		FROM contacts c
		WHERE c.account_id = $1
	`
	args := []interface{}{accountID}
	idx := 2

	if search != "" {
		query += ` AND (
			c.imessage_address ILIKE $` + strconv.Itoa(idx) + `
			OR c.name ILIKE $` + strconv.Itoa(idx) + `
			OR c.email ILIKE $` + strconv.Itoa(idx) + `
			OR c.company ILIKE $` + strconv.Itoa(idx) + `
		)`
		args = append(args, "%"+search+"%")
		idx++
	}

	if tag != "" {
		query += ` AND $` + strconv.Itoa(idx) + ` = ANY(c.tags)`
		args = append(args, tag)
		idx++
	}

	// Get total count for pagination
	countQuery := `SELECT COUNT(*) FROM contacts c WHERE c.account_id = $1`
	countArgs := []interface{}{accountID}
	if search != "" {
		countQuery += ` AND (c.imessage_address ILIKE $2 OR c.name ILIKE $2 OR c.email ILIKE $2 OR c.company ILIKE $2)`
		countArgs = append(countArgs, "%"+search+"%")
	}
	var total int
	h.db.QueryRow(r.Context(), countQuery, countArgs...).Scan(&total)

	query += ` ORDER BY c.last_message_at DESC NULLS LAST LIMIT $` + strconv.Itoa(idx) + ` OFFSET $` + strconv.Itoa(idx+1)
	args = append(args, limit, offset)

	rows, err := h.db.Query(r.Context(), query, args...)
	if err != nil {
		writeError(w, "database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type ContactRow struct {
		ID                string                 `json:"id"`
		IMessageAddress   string                 `json:"imessage_address"`
		Name              *string                `json:"name"`
		Email             *string                `json:"email"`
		Company           *string                `json:"company"`
		Notes             string                 `json:"notes"`
		Tags              []string               `json:"tags"`
		CustomFields      map[string]interface{} `json:"custom_fields"`
		GHLContactID      *string                `json:"ghl_contact_id"`
		IMessageCapable   *bool                  `json:"imessage_capable"`
		FirstMessageAt    *string                `json:"first_message_at"`
		LastMessageAt     *string                `json:"last_message_at"`
		MessageCount      int                    `json:"message_count"`
		CreatedAt         string                 `json:"created_at"`
		UpdatedAt         string                 `json:"updated_at"`
		ConversationCount int                    `json:"conversation_count"`
	}

	var contacts []ContactRow
	for rows.Next() {
		var c ContactRow
		var tagsArr []string
		var cfJSON []byte
		var firstMsg, lastMsg, createdAt, updatedAt interface{}

		if err := rows.Scan(
			&c.ID, &c.IMessageAddress, &c.Name, &c.Email, &c.Company, &c.Notes,
			&tagsArr, &cfJSON, &c.GHLContactID, &c.IMessageCapable,
			&firstMsg, &lastMsg, &c.MessageCount,
			&createdAt, &updatedAt, &c.ConversationCount,
		); err != nil {
			continue
		}
		c.Tags = tagsArr
		if c.Tags == nil {
			c.Tags = []string{}
		}
		if len(cfJSON) > 0 {
			json.Unmarshal(cfJSON, &c.CustomFields)
		}
		if c.CustomFields == nil {
			c.CustomFields = map[string]interface{}{}
		}
		// Format timestamps
		if firstMsg != nil {
			s := firstMsg.(interface{ String() string }).String()
			c.FirstMessageAt = &s
		}
		if lastMsg != nil {
			s := lastMsg.(interface{ String() string }).String()
			c.LastMessageAt = &s
		}
		if createdAt != nil {
			s := createdAt.(interface{ String() string }).String()
			c.CreatedAt = s
		}
		if updatedAt != nil {
			s := updatedAt.(interface{ String() string }).String()
			c.UpdatedAt = s
		}

		contacts = append(contacts, c)
	}
	if contacts == nil {
		contacts = []ContactRow{}
	}

	writeJSON(w, map[string]interface{}{
		"contacts": contacts,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	}, http.StatusOK)
}

// POST /api/contacts — explicitly create a contact (without sending a message).
// Returns the created (or upserted, if the address already exists) contact.
//
// Address normalization mirrors the messaging router so new contacts created
// here resolve to the same row when a message later flows in or out.
func (h *ContactHandler) Create(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())

	var body struct {
		IMessageAddress string                 `json:"imessage_address"`
		PhoneNumberID   string                 `json:"phone_number_id"` // optional: which of our numbers "owns" this contact
		Name            string                 `json:"name"`
		Email           string                 `json:"email"`
		Company         string                 `json:"company"`
		Notes           string                 `json:"notes"`
		Tags            []string               `json:"tags"`
		CustomFields    map[string]interface{} `json:"custom_fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	addr := normalizeContactAddress(body.IMessageAddress)
	if addr == "" {
		writeError(w, "imessage_address is required (phone number or email)", http.StatusBadRequest)
		return
	}
	if !looksLikeAddress(addr) {
		writeError(w, "imessage_address must be a valid phone number or email", http.StatusBadRequest)
		return
	}

	// Resolve phone_number_id: if not provided, pick the first active one on the account.
	var phoneNumberID string
	if body.PhoneNumberID != "" {
		err := h.db.QueryRow(r.Context(),
			`SELECT id::text FROM phone_numbers WHERE id = $1 AND account_id = $2`,
			body.PhoneNumberID, accountID,
		).Scan(&phoneNumberID)
		if err != nil {
			writeError(w, "phone_number_id not found on this account", http.StatusBadRequest)
			return
		}
	} else {
		err := h.db.QueryRow(r.Context(),
			`SELECT id::text FROM phone_numbers WHERE account_id = $1 AND status = 'active' ORDER BY created_at LIMIT 1`,
			accountID,
		).Scan(&phoneNumberID)
		if err != nil {
			writeError(w, "no active phone number on this account — cannot create contact", http.StatusBadRequest)
			return
		}
	}

	tags := body.Tags
	if tags == nil {
		tags = []string{}
	}
	cf := body.CustomFields
	if cf == nil {
		cf = map[string]interface{}{}
	}
	cfJSON, _ := json.Marshal(cf)

	var contactID string
	err := h.db.QueryRow(r.Context(), `
		INSERT INTO contacts (id, account_id, phone_number_id, imessage_address,
		                      name, email, company, notes, tags, custom_fields,
		                      created_at, updated_at)
		VALUES (uuid_generate_v4(), $1, $2, $3,
		        $4, $5, $6, COALESCE($7, ''), $8, $9::jsonb,
		        NOW(), NOW())
		ON CONFLICT (account_id, imessage_address) DO UPDATE
		  SET name          = COALESCE(EXCLUDED.name, contacts.name),
		      email         = COALESCE(EXCLUDED.email, contacts.email),
		      company       = COALESCE(EXCLUDED.company, contacts.company),
		      notes         = CASE WHEN EXCLUDED.notes <> '' THEN EXCLUDED.notes ELSE contacts.notes END,
		      tags          = CASE WHEN array_length(EXCLUDED.tags, 1) > 0 THEN EXCLUDED.tags ELSE contacts.tags END,
		      custom_fields = CASE WHEN EXCLUDED.custom_fields <> '{}'::jsonb THEN EXCLUDED.custom_fields ELSE contacts.custom_fields END,
		      updated_at    = NOW()
		RETURNING id::text
	`,
		accountID, phoneNumberID, addr,
		nilIfEmpty(body.Name), nilIfEmpty(body.Email), nilIfEmpty(body.Company),
		body.Notes, tags, cfJSON,
	).Scan(&contactID)
	if err != nil {
		writeError(w, "create contact: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{
		"id":               contactID,
		"imessage_address": addr,
		"status":           "created",
	}, http.StatusCreated)
}

// normalizeContactAddress mirrors normalizeAddress in messaging/router.go but
// is duplicated here to avoid an import cycle. Keep them in sync.
func normalizeContactAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if strings.Contains(addr, "@") {
		return strings.ToLower(addr)
	}
	var sb strings.Builder
	hasPlus := false
	for i, c := range addr {
		if i == 0 && c == '+' {
			sb.WriteRune(c)
			hasPlus = true
			continue
		}
		if c >= '0' && c <= '9' {
			sb.WriteRune(c)
		}
	}
	result := sb.String()
	if !hasPlus {
		digits := result
		if len(digits) == 10 {
			result = "+1" + digits
		} else if len(digits) == 11 && digits[0] == '1' {
			result = "+" + digits
		} else if len(digits) > 0 {
			result = "+" + digits
		}
	}
	return result
}

func looksLikeAddress(addr string) bool {
	if strings.Contains(addr, "@") {
		// minimal email check: has @ with non-empty parts
		parts := strings.Split(addr, "@")
		return len(parts) == 2 && len(parts[0]) > 0 && strings.Contains(parts[1], ".")
	}
	// phone: must start with +, have at least 8 digits total
	if !strings.HasPrefix(addr, "+") {
		return false
	}
	digits := 0
	for _, c := range addr[1:] {
		if c >= '0' && c <= '9' {
			digits++
		}
	}
	return digits >= 8
}

// GET /api/contacts/{contactID} — get a single contact with recent messages
func (h *ContactHandler) Get(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())
	contactID := chi.URLParam(r, "contactID")

	var c struct {
		ID              string                 `json:"id"`
		IMessageAddress string                 `json:"imessage_address"`
		Name            *string                `json:"name"`
		Email           *string                `json:"email"`
		Company         *string                `json:"company"`
		Notes           string                 `json:"notes"`
		Tags            []string               `json:"tags"`
		CustomFields    map[string]interface{} `json:"custom_fields"`
		GHLContactID    *string                `json:"ghl_contact_id"`
		IMessageCapable *bool                  `json:"imessage_capable"`
		FirstMessageAt  *string                `json:"first_message_at"`
		LastMessageAt   *string                `json:"last_message_at"`
		MessageCount    int                    `json:"message_count"`
		CreatedAt       string                 `json:"created_at"`
		UpdatedAt       string                 `json:"updated_at"`
	}

	var tagsArr []string
	var cfJSON []byte

	err := h.db.QueryRow(r.Context(), `
		SELECT id::text, imessage_address, name, email, company, notes,
		       tags, custom_fields, ghl_contact_id, imessage_capable,
		       first_message_at::text, last_message_at::text, message_count,
		       created_at::text, updated_at::text
		FROM contacts
		WHERE id = $1 AND account_id = $2
	`, contactID, accountID).Scan(
		&c.ID, &c.IMessageAddress, &c.Name, &c.Email, &c.Company, &c.Notes,
		&tagsArr, &cfJSON, &c.GHLContactID, &c.IMessageCapable,
		&c.FirstMessageAt, &c.LastMessageAt, &c.MessageCount,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		writeError(w, "contact not found", http.StatusNotFound)
		return
	}
	c.Tags = tagsArr
	if c.Tags == nil {
		c.Tags = []string{}
	}
	if len(cfJSON) > 0 {
		json.Unmarshal(cfJSON, &c.CustomFields)
	}
	if c.CustomFields == nil {
		c.CustomFields = map[string]interface{}{}
	}

	writeJSON(w, c, http.StatusOK)
}

// PATCH /api/contacts/{contactID} — update contact fields
func (h *ContactHandler) Update(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())
	contactID := chi.URLParam(r, "contactID")

	var body struct {
		Name         *string                `json:"name"`
		Email        *string                `json:"email"`
		Company      *string                `json:"company"`
		Notes        *string                `json:"notes"`
		Tags         []string               `json:"tags"`
		CustomFields map[string]interface{} `json:"custom_fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Build dynamic update
	sets := []string{}
	args := []interface{}{}
	idx := 1

	if body.Name != nil {
		sets = append(sets, "name = $"+strconv.Itoa(idx))
		args = append(args, nilIfEmpty(*body.Name))
		idx++
	}
	if body.Email != nil {
		sets = append(sets, "email = $"+strconv.Itoa(idx))
		args = append(args, nilIfEmpty(*body.Email))
		idx++
	}
	if body.Company != nil {
		sets = append(sets, "company = $"+strconv.Itoa(idx))
		args = append(args, nilIfEmpty(*body.Company))
		idx++
	}
	if body.Notes != nil {
		sets = append(sets, "notes = $"+strconv.Itoa(idx))
		args = append(args, *body.Notes)
		idx++
	}
	if body.Tags != nil {
		sets = append(sets, "tags = $"+strconv.Itoa(idx))
		args = append(args, body.Tags)
		idx++
	}
	if body.CustomFields != nil {
		cfJSON, _ := json.Marshal(body.CustomFields)
		sets = append(sets, "custom_fields = $"+strconv.Itoa(idx))
		args = append(args, cfJSON)
		idx++
	}

	if len(sets) == 0 {
		writeError(w, "no fields to update", http.StatusBadRequest)
		return
	}

	sets = append(sets, "updated_at = NOW()")
	query := "UPDATE contacts SET " + strings.Join(sets, ", ") +
		" WHERE id = $" + strconv.Itoa(idx) + " AND account_id = $" + strconv.Itoa(idx+1)
	args = append(args, contactID, accountID)

	res, err := h.db.Exec(r.Context(), query, args...)
	if err != nil {
		writeError(w, "update failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if res.RowsAffected() == 0 {
		writeError(w, "contact not found", http.StatusNotFound)
		return
	}

	writeJSON(w, map[string]string{"status": "updated"}, http.StatusOK)
}

// DELETE /api/contacts/{contactID} — delete a contact and its conversations/messages
func (h *ContactHandler) Delete(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())
	contactID := chi.URLParam(r, "contactID")

	// Delete conversations first (CASCADE will handle messages)
	h.db.Exec(r.Context(), `DELETE FROM conversations WHERE contact_id = $1 AND account_id = $2`, contactID, accountID)

	res, err := h.db.Exec(r.Context(), `DELETE FROM contacts WHERE id = $1 AND account_id = $2`, contactID, accountID)
	if err != nil {
		writeError(w, "delete failed", http.StatusInternalServerError)
		return
	}
	if res.RowsAffected() == 0 {
		writeError(w, "contact not found", http.StatusNotFound)
		return
	}

	writeJSON(w, map[string]string{"status": "deleted"}, http.StatusOK)
}

// GET /api/contacts/tags — list all unique tags across the account's contacts
func (h *ContactHandler) ListTags(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())

	rows, err := h.db.Query(r.Context(), `
		SELECT DISTINCT unnest(tags) as tag
		FROM contacts
		WHERE account_id = $1 AND tags IS NOT NULL AND array_length(tags, 1) > 0
		ORDER BY tag
	`, accountID)
	if err != nil {
		writeJSON(w, map[string]interface{}{"tags": []string{}}, http.StatusOK)
		return
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var t string
		rows.Scan(&t)
		tags = append(tags, t)
	}
	if tags == nil {
		tags = []string{}
	}

	writeJSON(w, map[string]interface{}{"tags": tags}, http.StatusOK)
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// GET /api/phone-numbers — list the account's iMessage numbers (used by the
// compose dialog to pick which number to send a new message from).
func (h *ContactHandler) ListPhoneNumbers(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())

	rows, err := h.db.Query(r.Context(), `
		SELECT id::text, number, COALESCE(imessage_address, ''), status, created_at::text
		FROM phone_numbers
		WHERE account_id = $1
		ORDER BY status = 'active' DESC, created_at ASC
	`, accountID)
	if err != nil {
		writeError(w, "database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type pnRow struct {
		ID              string `json:"id"`
		Number          string `json:"number"`
		IMessageAddress string `json:"imessage_address"`
		Status          string `json:"status"`
		CreatedAt       string `json:"created_at"`
	}
	var out []pnRow
	for rows.Next() {
		var p pnRow
		if err := rows.Scan(&p.ID, &p.Number, &p.IMessageAddress, &p.Status, &p.CreatedAt); err != nil {
			continue
		}
		out = append(out, p)
	}
	if out == nil {
		out = []pnRow{}
	}
	writeJSON(w, map[string]interface{}{"phone_numbers": out}, http.StatusOK)
}
