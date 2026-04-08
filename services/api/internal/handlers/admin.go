package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/bluesend/api/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminHandler struct {
	db *pgxpool.Pool
}

func NewAdminHandler(db *pgxpool.Pool) *AdminHandler {
	return &AdminHandler{db: db}
}

// GET /api/admin/accounts
func (h *AdminHandler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		offset, _ = strconv.Atoi(o)
	}

	statusFilter := r.URL.Query().Get("status")
	query := `
		SELECT a.id, a.name, a.email, a.status, a.plan, a.setup_complete,
		       a.stripe_customer_id, a.stripe_subscription_id, a.created_at,
		       (SELECT COUNT(*) FROM messages WHERE account_id = a.id AND created_at >= NOW() - INTERVAL '30 days') as msg_count_30d,
		       (SELECT COUNT(*) FROM phone_numbers WHERE account_id = a.id AND status = 'active') as active_numbers
		FROM accounts a
	`
	args := []interface{}{}
	if statusFilter != "" {
		query += " WHERE a.status = $1"
		args = append(args, statusFilter)
	}
	query += " ORDER BY a.created_at DESC LIMIT $" + strconv.Itoa(len(args)+1) + " OFFSET $" + strconv.Itoa(len(args)+2)
	args = append(args, limit, offset)

	rows, err := h.db.Query(r.Context(), query, args...)
	if err != nil {
		writeError(w, "database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type AccountRow struct {
		models.Account
		Msg30d        int `json:"messages_30d"`
		ActiveNumbers int `json:"active_numbers"`
	}

	var accounts []AccountRow
	for rows.Next() {
		var a AccountRow
		rows.Scan(&a.ID, &a.Name, &a.Email, &a.Status, &a.Plan, &a.SetupComplete,
			&a.StripeCustomerID, &a.StripeSubscriptionID, &a.CreatedAt,
			&a.Msg30d, &a.ActiveNumbers)
		accounts = append(accounts, a)
	}
	if accounts == nil {
		accounts = []AccountRow{}
	}

	writeJSON(w, map[string]interface{}{"accounts": accounts, "limit": limit, "offset": offset}, http.StatusOK)
}

// PATCH /api/admin/accounts/{accountID}/status
func (h *AdminHandler) UpdateAccountStatus(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")

	var body struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	validStatuses := map[string]bool{
		"active": true, "suspended": true, "past_due": true, "cancelled": true, "setting_up": true,
	}
	if !validStatuses[body.Status] {
		writeError(w, "invalid status", http.StatusBadRequest)
		return
	}

	_, err := h.db.Exec(r.Context(), `
		UPDATE accounts SET status = $1, updated_at = NOW() WHERE id = $2
	`, body.Status, accountID)
	if err != nil {
		writeError(w, "update failed", http.StatusInternalServerError)
		return
	}

	h.db.Exec(r.Context(), `
		INSERT INTO audit_log (id, action, entity_type, entity_id, details, created_at)
		VALUES (uuid_generate_v4(), 'admin_status_change', 'account', $1, $2, NOW())
	`, accountID, map[string]string{"status": body.Status, "reason": body.Reason})

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/admin/devices
func (h *AdminHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(r.Context(), `
		SELECT id, name, type, serial_number, status, last_seen_at, ip_address,
		       agent_version, os_version, capacity, assigned_count, error_message, created_at
		FROM devices
		ORDER BY status DESC, last_seen_at DESC
	`)
	if err != nil {
		writeError(w, "database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var devices []models.Device
	for rows.Next() {
		var d models.Device
		rows.Scan(&d.ID, &d.Name, &d.Type, &d.SerialNumber, &d.Status, &d.LastSeenAt, &d.IPAddress,
			&d.AgentVersion, &d.OSVersion, &d.Capacity, &d.AssignedCount, &d.ErrorMessage, &d.CreatedAt)
		devices = append(devices, d)
	}
	if devices == nil {
		devices = []models.Device{}
	}

	writeJSON(w, map[string]interface{}{"devices": devices}, http.StatusOK)
}

// POST /api/admin/devices/register — called by device setup script to onboard a new physical device
func (h *AdminHandler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string `json:"name"`
		Type   string `json:"type"`
		Serial string `json:"serial_number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, "name and type required", http.StatusBadRequest)
		return
	}
	if req.Type != "mac_mini" && req.Type != "iphone" {
		writeError(w, "type must be mac_mini or iphone", http.StatusBadRequest)
		return
	}

	deviceID := uuid.New()
	token := uuid.New().String() // device uses this to authenticate WebSocket

	var serialArg interface{}
	if req.Serial != "" {
		serialArg = req.Serial
	}

	_, err := h.db.Exec(r.Context(), `
		INSERT INTO devices (id, name, type, serial_number, device_token, status, capacity, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'offline', 5, NOW(), NOW())
	`, deviceID, req.Name, req.Type, serialArg, token)
	if err != nil {
		writeError(w, "could not register device", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{
		"device_id":    deviceID.String(),
		"device_token": token,
		"message":      "Device registered. Use device_token in the agent config.",
	}, http.StatusCreated)
}

// PATCH /api/admin/devices/{deviceID}
func (h *AdminHandler) UpdateDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceID")

	var body struct {
		Status   string `json:"status"`
		Capacity int    `json:"capacity"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	if body.Status != "" {
		h.db.Exec(r.Context(), `UPDATE devices SET status = $1, updated_at = NOW() WHERE id = $2`,
			body.Status, deviceID)
	}
	if body.Capacity > 0 {
		h.db.Exec(r.Context(), `UPDATE devices SET capacity = $1, updated_at = NOW() WHERE id = $2`,
			body.Capacity, deviceID)
	}

	w.WriteHeader(http.StatusNoContent)
}

// POST /api/admin/accounts/{accountID}/assign-number — manually assign a phone number
func (h *AdminHandler) AssignPhoneNumber(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")

	var req struct {
		Number      string `json:"number"`
		DeviceID    string `json:"device_id"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Number == "" {
		writeError(w, "number required", http.StatusBadRequest)
		return
	}

	numberID := uuid.New()
	_, err := h.db.Exec(r.Context(), `
		INSERT INTO phone_numbers (id, account_id, device_id, number, display_name, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'provisioning', NOW(), NOW())
	`, numberID, accountID, req.DeviceID, req.Number, req.DisplayName)
	if err != nil {
		writeError(w, "could not assign number", http.StatusInternalServerError)
		return
	}

	// Update device assigned count
	h.db.Exec(r.Context(), `
		UPDATE devices SET assigned_count = assigned_count + 1 WHERE id = $1
	`, req.DeviceID)

	writeJSON(w, map[string]string{
		"phone_number_id": numberID.String(),
		"status":          "provisioning",
	}, http.StatusCreated)
}

// GET /api/admin/stats — operator overview
func (h *AdminHandler) GetSystemStats(w http.ResponseWriter, r *http.Request) {
	stats := map[string]interface{}{}

	h.db.QueryRow(r.Context(), `SELECT COUNT(*) FROM accounts WHERE status = 'active'`).
		Scan(func(n *int) { stats["active_accounts"] = n }(&stats["active_accounts"]))

	// Simpler direct scans
	var activeAccounts, settingUpAccounts, pastDueAccounts, totalMessages30d, onlineDevices int
	var totalMRR float64

	h.db.QueryRow(r.Context(), `SELECT COUNT(*) FROM accounts WHERE status = 'active'`).Scan(&activeAccounts)
	h.db.QueryRow(r.Context(), `SELECT COUNT(*) FROM accounts WHERE status = 'setting_up'`).Scan(&settingUpAccounts)
	h.db.QueryRow(r.Context(), `SELECT COUNT(*) FROM accounts WHERE status = 'past_due'`).Scan(&pastDueAccounts)
	h.db.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM messages WHERE created_at >= NOW() - INTERVAL '30 days'
	`).Scan(&totalMessages30d)
	h.db.QueryRow(r.Context(), `SELECT COUNT(*) FROM devices WHERE status = 'online'`).Scan(&onlineDevices)
	h.db.QueryRow(r.Context(), `
		SELECT COALESCE(SUM(CASE WHEN plan = 'monthly' THEN 199 WHEN plan = 'annual' THEN 2600/12.0 ELSE 0 END), 0)
		FROM accounts WHERE status = 'active'
	`).Scan(&totalMRR)

	writeJSON(w, map[string]interface{}{
		"active_accounts":    activeAccounts,
		"setting_up":         settingUpAccounts,
		"past_due":           pastDueAccounts,
		"total_messages_30d": totalMessages30d,
		"online_devices":     onlineDevices,
		"estimated_mrr":      totalMRR,
		"as_of":              time.Now().Format(time.RFC3339),
	}, http.StatusOK)
}

// GET /api/admin/audit-log
func (h *AdminHandler) GetAuditLog(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(r.Context(), `
		SELECT al.id, al.account_id, al.action, al.entity_type, al.entity_id,
		       al.details, al.created_at, a.name as account_name
		FROM audit_log al
		LEFT JOIN accounts a ON a.id = al.account_id
		ORDER BY al.created_at DESC
		LIMIT 100
	`)
	if err != nil {
		writeError(w, "database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type AuditEntry struct {
		ID          string                 `json:"id"`
		AccountID   *string                `json:"account_id"`
		AccountName *string                `json:"account_name"`
		Action      string                 `json:"action"`
		EntityType  *string                `json:"entity_type"`
		EntityID    *string                `json:"entity_id"`
		Details     map[string]interface{} `json:"details"`
		CreatedAt   time.Time              `json:"created_at"`
	}

	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var detailsJSON []byte
		rows.Scan(&e.ID, &e.AccountID, &e.Action, &e.EntityType, &e.EntityID,
			&detailsJSON, &e.CreatedAt, &e.AccountName)
		json.Unmarshal(detailsJSON, &e.Details)
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []AuditEntry{}
	}

	writeJSON(w, map[string]interface{}{"entries": entries}, http.StatusOK)
}
