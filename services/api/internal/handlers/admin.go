package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/bluesend/api/internal/middleware"
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

// logAudit records an admin action to the audit_log table.
// accountID and userID may be nil for system-wide actions.
func (h *AdminHandler) logAudit(r *http.Request, action string, entityType, entityID string, details map[string]interface{}) {
	actorUserID, _ := middleware.GetUserID(r.Context())
	accountUUID, _ := middleware.GetAccountID(r.Context())

	detailsJSON, _ := json.Marshal(details)
	ip := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ip = fwd
	}

	var entityUUID interface{}
	if entityID != "" {
		if parsed, err := uuid.Parse(entityID); err == nil {
			entityUUID = parsed
		}
	}

	var userIDArg interface{}
	if actorUserID != uuid.Nil {
		userIDArg = actorUserID
	}
	var accountIDArg interface{}
	if accountUUID != uuid.Nil {
		accountIDArg = accountUUID
	}

	_, err := h.db.Exec(r.Context(), `
		INSERT INTO audit_log (id, account_id, user_id, action, entity_type, entity_id, details, ip_address, created_at)
		VALUES (uuid_generate_v4(), $1, $2, $3, NULLIF($4, ''), $5, $6, $7::inet, NOW())
	`, accountIDArg, userIDArg, action, entityType, entityUUID, detailsJSON, ip)
	if err != nil {
		// Don't fail the request if audit logging fails
		_ = err
	}
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
		       COALESCE(a.preferred_area_code, '') as preferred_area_code,
		       a.calling_enabled,
		       a.cancelled_at, a.grace_period_ends_at, a.auto_reply_starts_at,
		       a.auto_reply_enabled, a.auto_reply_message,
		       (SELECT COUNT(*) FROM messages WHERE account_id = a.id AND created_at >= NOW() - INTERVAL '30 days') as msg_count_30d,
		       (SELECT COUNT(*) FROM phone_numbers WHERE account_id = a.id AND status IN ('active', 'suspended')) as active_numbers
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
		PreferredAreaCode string `json:"preferred_area_code"`
		Msg30d            int    `json:"messages_30d"`
		ActiveNumbers     int    `json:"active_numbers"`
	}

	var accounts []AccountRow
	for rows.Next() {
		var a AccountRow
		rows.Scan(&a.ID, &a.Name, &a.Email, &a.Status, &a.Plan, &a.SetupComplete,
			&a.StripeCustomerID, &a.StripeSubscriptionID, &a.CreatedAt,
			&a.PreferredAreaCode, &a.CallingEnabled,
			&a.CancelledAt, &a.GracePeriodEndsAt, &a.AutoReplyStartsAt,
			&a.AutoReplyEnabled, &a.AutoReplyMessage,
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

	h.logAudit(r, "account.status_changed", "account", accountID, map[string]interface{}{
		"status": body.Status,
		"reason": body.Reason,
	})

	w.WriteHeader(http.StatusNoContent)
}

// PATCH /api/admin/accounts/{accountID}/calling — enable/disable FaceTime Audio
// calling for a customer (upsell gate).
func (h *AdminHandler) UpdateAccountCalling(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	_, err := h.db.Exec(r.Context(), `
		UPDATE accounts SET calling_enabled = $1, updated_at = NOW() WHERE id = $2
	`, body.Enabled, accountID)
	if err != nil {
		writeError(w, "update failed", http.StatusInternalServerError)
		return
	}

	// If disabling at the account level, also disable on every number so state is consistent.
	if !body.Enabled {
		h.db.Exec(r.Context(), `UPDATE phone_numbers SET voice_enabled = false, updated_at = NOW() WHERE account_id = $1`, accountID)
	}

	h.logAudit(r, "account.calling_toggled", "account", accountID, map[string]interface{}{
		"enabled": body.Enabled,
	})
	w.WriteHeader(http.StatusNoContent)
}

// PATCH /api/admin/phone-numbers/{numberID}/voice — toggle voice on a single number.
func (h *AdminHandler) UpdateNumberVoice(w http.ResponseWriter, r *http.Request) {
	numberID := chi.URLParam(r, "numberID")

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Only allow enabling if the owning account has calling_enabled.
	if body.Enabled {
		var accountCalling bool
		err := h.db.QueryRow(r.Context(), `
			SELECT a.calling_enabled FROM phone_numbers pn
			JOIN accounts a ON a.id = pn.account_id
			WHERE pn.id = $1
		`, numberID).Scan(&accountCalling)
		if err != nil {
			writeError(w, "number not found", http.StatusNotFound)
			return
		}
		if !accountCalling {
			writeError(w, "account does not have calling enabled", http.StatusBadRequest)
			return
		}
	}

	_, err := h.db.Exec(r.Context(), `
		UPDATE phone_numbers SET voice_enabled = $1, updated_at = NOW() WHERE id = $2
	`, body.Enabled, numberID)
	if err != nil {
		writeError(w, "update failed", http.StatusInternalServerError)
		return
	}
	h.logAudit(r, "phone_number.voice_toggled", "phone_number", numberID, map[string]interface{}{
		"enabled": body.Enabled,
	})
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

	// Prevent duplicate device names
	var exists bool
	_ = h.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM devices WHERE name = $1)`, req.Name).Scan(&exists)
	if exists {
		writeError(w, "a device with this name already exists — delete it first or choose a different name", http.StatusConflict)
		return
	}

	deviceID := uuid.New()
	token := uuid.New().String()

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

	h.logAudit(r, "device.registered", "device", deviceID.String(), map[string]interface{}{
		"name": req.Name,
		"type": req.Type,
	})

	writeJSON(w, map[string]string{
		"device_id":    deviceID.String(),
		"device_token": token,
		"message":      "Device registered. Use device_token in the agent config.",
	}, http.StatusCreated)
}

// DELETE /api/admin/devices/{deviceID} — remove a device
func (h *AdminHandler) DeleteDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceID")

	// Don't allow deletion if phone numbers are assigned
	var count int
	_ = h.db.QueryRow(r.Context(), `SELECT COUNT(*) FROM phone_numbers WHERE device_id = $1`, deviceID).Scan(&count)
	if count > 0 {
		writeError(w, "cannot delete device with assigned phone numbers — reassign them first", http.StatusConflict)
		return
	}

	res, err := h.db.Exec(r.Context(), `DELETE FROM devices WHERE id = $1`, deviceID)
	if err != nil {
		writeError(w, "could not delete device", http.StatusInternalServerError)
		return
	}
	if res.RowsAffected() == 0 {
		writeError(w, "device not found", http.StatusNotFound)
		return
	}

	h.logAudit(r, "device.deleted", "device", deviceID, nil)

	writeJSON(w, map[string]string{"status": "deleted"}, http.StatusOK)
}

// POST /api/admin/devices/{deviceID}/rotate-token — generate a new device token
func (h *AdminHandler) RotateDeviceToken(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceID")
	newToken := uuid.New().String()

	res, err := h.db.Exec(r.Context(), `
		UPDATE devices SET device_token = $1, updated_at = NOW() WHERE id = $2
	`, newToken, deviceID)
	if err != nil {
		writeError(w, "could not rotate token", http.StatusInternalServerError)
		return
	}
	if res.RowsAffected() == 0 {
		writeError(w, "device not found", http.StatusNotFound)
		return
	}

	h.logAudit(r, "device.token_rotated", "device", deviceID, nil)

	writeJSON(w, map[string]string{
		"device_id":    deviceID,
		"device_token": newToken,
		"message":      "Token rotated. Update the device agent config with the new token.",
	}, http.StatusOK)
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
		Number          string `json:"number"`
		DeviceID        string `json:"device_id"`
		DisplayName     string `json:"display_name"`
		IMessageAddress string `json:"imessage_address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Number == "" {
		writeError(w, "number required", http.StatusBadRequest)
		return
	}

	// Enforce one active number per account
	var activeCount int
	_ = h.db.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM phone_numbers
		WHERE account_id = $1 AND status = 'active'
	`, accountID).Scan(&activeCount)
	if activeCount > 0 {
		writeError(w, "account already has an active phone number — remove it first or create a new account", http.StatusConflict)
		return
	}

	numberID := uuid.New()
	_, err := h.db.Exec(r.Context(), `
		INSERT INTO phone_numbers (id, account_id, device_id, number, imessage_address, display_name, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'active', NOW(), NOW())
	`, numberID, accountID, req.DeviceID, req.Number, req.IMessageAddress, req.DisplayName)
	if err != nil {
		writeError(w, "could not assign number", http.StatusInternalServerError)
		return
	}

	// Update device assigned count
	if req.DeviceID != "" {
		h.db.Exec(r.Context(), `
			UPDATE devices SET assigned_count = assigned_count + 1 WHERE id = $1
		`, req.DeviceID)
	}

	h.logAudit(r, "phone_number.assigned", "phone_number", numberID.String(), map[string]interface{}{
		"number":     req.Number,
		"account_id": accountID,
		"device_id":  req.DeviceID,
	})

	writeJSON(w, map[string]string{
		"phone_number_id": numberID.String(),
		"status":          "active",
	}, http.StatusCreated)
}

// GET /api/admin/accounts/{accountID}/audit-log — account-scoped audit log
func (h *AdminHandler) GetAccountAuditLog(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
		if limit > 500 {
			limit = 500
		}
	}

	rows, err := h.db.Query(r.Context(), `
		SELECT al.id, al.user_id, al.action, al.entity_type, al.entity_id,
		       al.details, al.ip_address::text, al.created_at,
		       u.email as user_email
		FROM audit_log al
		LEFT JOIN users u ON u.id = al.user_id
		WHERE al.account_id = $1
		ORDER BY al.created_at DESC
		LIMIT $2
	`, accountID, limit)
	if err != nil {
		writeError(w, "database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type AuditEntry struct {
		ID         string                 `json:"id"`
		UserID     *string                `json:"user_id"`
		UserEmail  *string                `json:"user_email"`
		Action     string                 `json:"action"`
		EntityType *string                `json:"entity_type"`
		EntityID   *string                `json:"entity_id"`
		Details    map[string]interface{} `json:"details"`
		IPAddress  *string                `json:"ip_address"`
		CreatedAt  time.Time              `json:"created_at"`
	}

	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var detailsJSON []byte
		rows.Scan(&e.ID, &e.UserID, &e.Action, &e.EntityType, &e.EntityID,
			&detailsJSON, &e.IPAddress, &e.CreatedAt, &e.UserEmail)
		json.Unmarshal(detailsJSON, &e.Details)
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []AuditEntry{}
	}

	writeJSON(w, map[string]interface{}{"entries": entries}, http.StatusOK)
}

// GET /api/admin/accounts/{accountID}/numbers — list phone numbers for an account
func (h *AdminHandler) GetAccountNumbers(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")

	rows, err := h.db.Query(r.Context(), `
		SELECT pn.id, pn.number, pn.imessage_address, pn.display_name, pn.status,
		       pn.device_id, COALESCE(d.name, '') as device_name, pn.voice_enabled, pn.created_at
		FROM phone_numbers pn
		LEFT JOIN devices d ON d.id = pn.device_id
		WHERE pn.account_id = $1
		ORDER BY pn.created_at DESC
	`, accountID)
	if err != nil {
		writeError(w, "database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type PhoneNumberInfo struct {
		ID              string  `json:"id"`
		Number          string  `json:"number"`
		IMessageAddress *string `json:"imessage_address"`
		DisplayName     *string `json:"display_name"`
		Status          string  `json:"status"`
		DeviceID        *string `json:"device_id"`
		DeviceName      string  `json:"device_name"`
		VoiceEnabled    bool    `json:"voice_enabled"`
		CreatedAt       string  `json:"created_at"`
	}

	var numbers []PhoneNumberInfo
	for rows.Next() {
		var n PhoneNumberInfo
		rows.Scan(&n.ID, &n.Number, &n.IMessageAddress, &n.DisplayName, &n.Status,
			&n.DeviceID, &n.DeviceName, &n.VoiceEnabled, &n.CreatedAt)
		numbers = append(numbers, n)
	}
	if numbers == nil {
		numbers = []PhoneNumberInfo{}
	}

	writeJSON(w, map[string]interface{}{"numbers": numbers}, http.StatusOK)
}

// GET /api/admin/accounts/{accountID}/ghl — get GHL connection status for an account
func (h *AdminHandler) GetAccountGHL(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")

	var locationID string
	var connected bool
	var createdAt string
	err := h.db.QueryRow(r.Context(), `
		SELECT location_id, connected, created_at::text FROM ghl_connections WHERE account_id = $1
	`, accountID).Scan(&locationID, &connected, &createdAt)

	if err != nil {
		writeJSON(w, map[string]interface{}{
			"connected":   false,
			"location_id": "",
		}, http.StatusOK)
		return
	}

	writeJSON(w, map[string]interface{}{
		"connected":    connected,
		"location_id":  locationID,
		"connected_at": createdAt,
	}, http.StatusOK)
}

// DELETE /api/admin/phone-numbers/{numberID} — remove a phone number
func (h *AdminHandler) DeletePhoneNumber(w http.ResponseWriter, r *http.Request) {
	numberID := chi.URLParam(r, "numberID")

	// Get the device_id before deletion so we can decrement assigned_count
	var deviceID *string
	_ = h.db.QueryRow(r.Context(), `SELECT device_id::text FROM phone_numbers WHERE id = $1`, numberID).Scan(&deviceID)

	res, err := h.db.Exec(r.Context(), `DELETE FROM phone_numbers WHERE id = $1`, numberID)
	if err != nil {
		writeError(w, "could not delete phone number", http.StatusInternalServerError)
		return
	}
	if res.RowsAffected() == 0 {
		writeError(w, "phone number not found", http.StatusNotFound)
		return
	}

	if deviceID != nil {
		h.db.Exec(r.Context(), `
			UPDATE devices SET assigned_count = GREATEST(assigned_count - 1, 0) WHERE id = $1
		`, *deviceID)
	}

	h.logAudit(r, "phone_number.deleted", "phone_number", numberID, nil)

	writeJSON(w, map[string]string{"status": "deleted"}, http.StatusOK)
}

// DELETE /api/admin/accounts/{accountID}/ghl — disconnect GHL for an account
func (h *AdminHandler) DisconnectAccountGHL(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	h.db.Exec(r.Context(), `DELETE FROM ghl_connections WHERE account_id = $1`, accountID)
	h.db.Exec(r.Context(), `UPDATE accounts SET ghl_location_id = NULL WHERE id = $1`, accountID)
	h.logAudit(r, "ghl.disconnected", "account", accountID, nil)
	writeJSON(w, map[string]string{"status": "disconnected"}, http.StatusOK)
}

// GET /api/admin/number-health — overview of all phone numbers with send stats
func (h *AdminHandler) GetNumberHealth(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(r.Context(), `
		SELECT
			pn.id,
			pn.number,
			pn.display_name,
			pn.status,
			pn.health_status,
			pn.health_notes,
			pn.daily_new_contact_limit,
			pn.consecutive_failures,
			pn.last_send_success_at::text,
			pn.last_send_failure_at::text,
			a.id AS account_id,
			a.name AS account_name,
			a.email AS account_email,
			COALESCE(d.name, '') AS device_name,
			COALESCE(d.status, 'none') AS device_status,
			(SELECT COUNT(*) FROM messages m WHERE m.phone_number_id = pn.id AND m.direction = 'outbound' AND m.created_at::date = CURRENT_DATE) AS messages_today,
			(SELECT COUNT(*) FROM messages m WHERE m.phone_number_id = pn.id AND m.direction = 'outbound' AND m.status = 'failed' AND m.created_at::date = CURRENT_DATE) AS failures_today,
			(SELECT COALESCE(SUM(message_count), 0) FROM rate_limit_daily WHERE phone_number_id = pn.id AND date = CURRENT_DATE AND is_new_contact = true) AS new_contacts_today,
			(SELECT COUNT(*) FROM messages m WHERE m.phone_number_id = pn.id AND m.direction = 'outbound' AND m.created_at >= NOW() - INTERVAL '7 days') AS messages_week
		FROM phone_numbers pn
		JOIN accounts a ON a.id = pn.account_id
		LEFT JOIN devices d ON d.id = pn.device_id
		ORDER BY pn.created_at DESC
	`)
	if err != nil {
		writeError(w, "database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type NumberHealth struct {
		ID                   string  `json:"id"`
		Number               string  `json:"number"`
		DisplayName          *string `json:"display_name"`
		Status               string  `json:"status"`
		HealthStatus         string  `json:"health_status"`
		HealthNotes          *string `json:"health_notes"`
		DailyNewContactLimit int     `json:"daily_new_contact_limit"`
		ConsecutiveFailures  int     `json:"consecutive_failures"`
		LastSuccessAt        *string `json:"last_send_success_at"`
		LastFailureAt        *string `json:"last_send_failure_at"`
		AccountID            string  `json:"account_id"`
		AccountName          string  `json:"account_name"`
		AccountEmail         string  `json:"account_email"`
		DeviceName           string  `json:"device_name"`
		DeviceStatus         string  `json:"device_status"`
		MessagesToday        int     `json:"messages_today"`
		FailuresToday        int     `json:"failures_today"`
		NewContactsToday     int     `json:"new_contacts_today"`
		MessagesWeek         int     `json:"messages_week"`
	}

	var numbers []NumberHealth
	for rows.Next() {
		var n NumberHealth
		rows.Scan(
			&n.ID, &n.Number, &n.DisplayName, &n.Status, &n.HealthStatus, &n.HealthNotes,
			&n.DailyNewContactLimit, &n.ConsecutiveFailures, &n.LastSuccessAt, &n.LastFailureAt,
			&n.AccountID, &n.AccountName, &n.AccountEmail,
			&n.DeviceName, &n.DeviceStatus,
			&n.MessagesToday, &n.FailuresToday, &n.NewContactsToday, &n.MessagesWeek,
		)
		numbers = append(numbers, n)
	}
	if numbers == nil {
		numbers = []NumberHealth{}
	}

	writeJSON(w, map[string]interface{}{"numbers": numbers}, http.StatusOK)
}

// PATCH /api/admin/phone-numbers/{numberID}/health — update health status
func (h *AdminHandler) UpdateNumberHealth(w http.ResponseWriter, r *http.Request) {
	numberID := chi.URLParam(r, "numberID")
	var req struct {
		HealthStatus string `json:"health_status"`
		HealthNotes  string `json:"health_notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request", http.StatusBadRequest)
		return
	}

	_, err := h.db.Exec(r.Context(), `
		UPDATE phone_numbers
		SET health_status = $1, health_notes = NULLIF($2, ''), updated_at = NOW()
		WHERE id = $3
	`, req.HealthStatus, req.HealthNotes, numberID)
	if err != nil {
		writeError(w, "could not update health", http.StatusInternalServerError)
		return
	}

	h.logAudit(r, "number.health_updated", "phone_number", numberID, map[string]interface{}{
		"health_status": req.HealthStatus,
		"notes":         req.HealthNotes,
	})

	w.WriteHeader(http.StatusNoContent)
}

// POST /api/admin/accounts/{accountID}/cancel — initiates the cancellation lifecycle.
// Sets status=cancelled, cancelled_at=now, grace_period_ends_at=+30d,
// auto_reply_starts_at=+7d. Suspends phone numbers (outbound blocked, inbound
// still works for auto-reply).
func (h *AdminHandler) CancelAccount(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")

	var body struct {
		Reason string `json:"reason"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	now := time.Now()
	gracePeriodEnds := now.Add(30 * 24 * time.Hour)  // 30 days
	autoReplyStarts := now.Add(7 * 24 * time.Hour)   // 7 days

	res, err := h.db.Exec(r.Context(), `
		UPDATE accounts
		SET status = 'cancelled',
		    cancelled_at = $1,
		    grace_period_ends_at = $2,
		    auto_reply_starts_at = $3,
		    updated_at = NOW()
		WHERE id = $4 AND status != 'cancelled'
	`, now, gracePeriodEnds, autoReplyStarts, accountID)
	if err != nil {
		writeError(w, "update failed", http.StatusInternalServerError)
		return
	}
	if res.RowsAffected() == 0 {
		writeError(w, "account not found or already cancelled", http.StatusBadRequest)
		return
	}

	// Suspend phone numbers so no outbound sending is possible,
	// but the device still receives inbound messages for auto-reply.
	h.db.Exec(r.Context(), `
		UPDATE phone_numbers SET status = 'suspended', updated_at = NOW()
		WHERE account_id = $1 AND status = 'active'
	`, accountID)

	h.logAudit(r, "account.cancelled", "account", accountID, map[string]interface{}{
		"reason":              body.Reason,
		"grace_period_ends":   gracePeriodEnds.Format(time.RFC3339),
		"auto_reply_starts":   autoReplyStarts.Format(time.RFC3339),
	})

	writeJSON(w, map[string]interface{}{
		"status":              "cancelled",
		"cancelled_at":        now.Format(time.RFC3339),
		"grace_period_ends_at": gracePeriodEnds.Format(time.RFC3339),
		"auto_reply_starts_at": autoReplyStarts.Format(time.RFC3339),
	}, http.StatusOK)
}

// POST /api/admin/accounts/{accountID}/reinstate — reverses cancellation.
// Clears all cancellation lifecycle fields, re-activates phone numbers,
// and sets account status back to active.
func (h *AdminHandler) ReinstateAccount(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")

	var body struct {
		Reason string `json:"reason"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	res, err := h.db.Exec(r.Context(), `
		UPDATE accounts
		SET status = 'active',
		    cancelled_at = NULL,
		    grace_period_ends_at = NULL,
		    auto_reply_starts_at = NULL,
		    updated_at = NOW()
		WHERE id = $1 AND status IN ('cancelled', 'suspended')
	`, accountID)
	if err != nil {
		writeError(w, "update failed", http.StatusInternalServerError)
		return
	}
	if res.RowsAffected() == 0 {
		writeError(w, "account not found or not in cancelled/suspended state", http.StatusBadRequest)
		return
	}

	// Re-activate phone numbers that were suspended during cancellation
	h.db.Exec(r.Context(), `
		UPDATE phone_numbers SET status = 'active', updated_at = NOW()
		WHERE account_id = $1 AND status = 'suspended'
	`, accountID)

	h.logAudit(r, "account.reinstated", "account", accountID, map[string]interface{}{
		"reason": body.Reason,
	})

	writeJSON(w, map[string]interface{}{
		"status": "active",
	}, http.StatusOK)
}

// PATCH /api/admin/accounts/{accountID}/auto-reply — toggle auto-reply and
// optionally update the message text.
func (h *AdminHandler) UpdateAutoReply(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")

	var body struct {
		Enabled *bool  `json:"enabled"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if body.Enabled != nil {
		h.db.Exec(r.Context(), `
			UPDATE accounts SET auto_reply_enabled = $1, updated_at = NOW() WHERE id = $2
		`, *body.Enabled, accountID)
	}
	if body.Message != "" {
		h.db.Exec(r.Context(), `
			UPDATE accounts SET auto_reply_message = $1, updated_at = NOW() WHERE id = $2
		`, body.Message, accountID)
	}

	h.logAudit(r, "account.auto_reply_updated", "account", accountID, map[string]interface{}{
		"enabled": body.Enabled,
		"message": body.Message,
	})

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/admin/stats — operator overview
func (h *AdminHandler) GetSystemStats(w http.ResponseWriter, r *http.Request) {
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

	actionFilter := r.URL.Query().Get("action")
	accountFilter := r.URL.Query().Get("account_id")

	query := `
		SELECT al.id, al.account_id, al.user_id, al.action, al.entity_type, al.entity_id,
		       al.details, al.ip_address::text, al.created_at,
		       a.name as account_name, a.email as account_email,
		       u.email as user_email
		FROM audit_log al
		LEFT JOIN accounts a ON a.id = al.account_id
		LEFT JOIN users u ON u.id = al.user_id
		WHERE 1=1
	`
	args := []interface{}{}
	idx := 1
	if actionFilter != "" {
		query += " AND al.action = $" + strconv.Itoa(idx)
		args = append(args, actionFilter)
		idx++
	}
	if accountFilter != "" {
		query += " AND al.account_id = $" + strconv.Itoa(idx)
		args = append(args, accountFilter)
		idx++
	}
	query += " ORDER BY al.created_at DESC LIMIT $" + strconv.Itoa(idx) + " OFFSET $" + strconv.Itoa(idx+1)
	args = append(args, limit, offset)

	rows, err := h.db.Query(r.Context(), query, args...)
	if err != nil {
		writeError(w, "database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type AuditEntry struct {
		ID           string                 `json:"id"`
		AccountID    *string                `json:"account_id"`
		AccountName  *string                `json:"account_name"`
		AccountEmail *string                `json:"account_email"`
		UserID       *string                `json:"user_id"`
		UserEmail    *string                `json:"user_email"`
		Action       string                 `json:"action"`
		EntityType   *string                `json:"entity_type"`
		EntityID     *string                `json:"entity_id"`
		Details      map[string]interface{} `json:"details"`
		IPAddress    *string                `json:"ip_address"`
		CreatedAt    time.Time              `json:"created_at"`
	}

	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var detailsJSON []byte
		rows.Scan(&e.ID, &e.AccountID, &e.UserID, &e.Action, &e.EntityType, &e.EntityID,
			&detailsJSON, &e.IPAddress, &e.CreatedAt, &e.AccountName, &e.AccountEmail, &e.UserEmail)
		json.Unmarshal(detailsJSON, &e.Details)
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []AuditEntry{}
	}

	// Also return the distinct action types for the filter dropdown
	actionRows, _ := h.db.Query(r.Context(), `SELECT DISTINCT action FROM audit_log ORDER BY action`)
	var actions []string
	if actionRows != nil {
		defer actionRows.Close()
		for actionRows.Next() {
			var a string
			actionRows.Scan(&a)
			actions = append(actions, a)
		}
	}

	writeJSON(w, map[string]interface{}{
		"entries": entries,
		"actions": actions,
		"limit":   limit,
		"offset":  offset,
	}, http.StatusOK)
}
