package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/bluesend/api/internal/middleware"
	"github.com/bluesend/api/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SettingsHandler struct {
	db *pgxpool.Pool
}

func NewSettingsHandler(db *pgxpool.Pool) *SettingsHandler {
	return &SettingsHandler{db: db}
}

// GET /api/account/custom-fields — return the account's custom field schema
func (h *SettingsHandler) GetCustomFields(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())

	var schema json.RawMessage
	err := h.db.QueryRow(r.Context(),
		`SELECT custom_field_schema FROM accounts WHERE id = $1`, accountID,
	).Scan(&schema)
	if err != nil {
		writeJSON(w, map[string]interface{}{"fields": []interface{}{}}, http.StatusOK)
		return
	}

	writeJSON(w, map[string]interface{}{"fields": json.RawMessage(schema)}, http.StatusOK)
}

// PUT /api/account/custom-fields — replace the full custom field schema
func (h *SettingsHandler) UpdateCustomFields(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())
	role, _ := middleware.GetRole(r.Context())
	if role != string(models.UserRoleOwner) && role != string(models.UserRoleAdmin) {
		writeError(w, "only owners and admins can manage custom fields", http.StatusForbidden)
		return
	}

	var body struct {
		Fields []models.CustomFieldDefinition `json:"fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate fields
	seen := map[string]bool{}
	validTypes := map[string]bool{"text": true, "number": true, "select": true, "date": true, "url": true}
	for _, f := range body.Fields {
		if f.Key == "" || f.Label == "" {
			writeError(w, "each field must have a key and label", http.StatusBadRequest)
			return
		}
		if !validTypes[f.Type] {
			writeError(w, "invalid field type: "+f.Type+". Allowed: text, number, select, date, url", http.StatusBadRequest)
			return
		}
		if f.Type == "select" && len(f.Options) == 0 {
			writeError(w, "select fields must have at least one option", http.StatusBadRequest)
			return
		}
		if seen[f.Key] {
			writeError(w, "duplicate field key: "+f.Key, http.StatusBadRequest)
			return
		}
		seen[f.Key] = true
	}

	schemaJSON, _ := json.Marshal(body.Fields)
	_, err := h.db.Exec(r.Context(),
		`UPDATE accounts SET custom_field_schema = $1, updated_at = NOW() WHERE id = $2`,
		schemaJSON, accountID,
	)
	if err != nil {
		writeError(w, "update failed", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "updated"}, http.StatusOK)
}

// GET /api/account/settings — return account settings
func (h *SettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())

	var name, email, timezone string
	var schema json.RawMessage
	err := h.db.QueryRow(r.Context(), `
		SELECT name, email, timezone, custom_field_schema
		FROM accounts WHERE id = $1
	`, accountID).Scan(&name, &email, &timezone, &schema)
	if err != nil {
		writeError(w, "account not found", http.StatusNotFound)
		return
	}

	// Get GHL location ID for CRM link construction
	var ghlLocationID *string
	h.db.QueryRow(r.Context(),
		`SELECT location_id FROM ghl_connections WHERE account_id = $1 AND connected = true LIMIT 1`,
		accountID,
	).Scan(&ghlLocationID)

	writeJSON(w, map[string]interface{}{
		"name":                name,
		"email":               email,
		"timezone":            timezone,
		"custom_field_schema": json.RawMessage(schema),
		"ghl_location_id":     ghlLocationID,
	}, http.StatusOK)
}

// PATCH /api/account/settings — update account settings
func (h *SettingsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())
	role, _ := middleware.GetRole(r.Context())
	if role != string(models.UserRoleOwner) && role != string(models.UserRoleAdmin) {
		writeError(w, "only owners and admins can update settings", http.StatusForbidden)
		return
	}

	var body struct {
		Name     *string `json:"name"`
		Timezone *string `json:"timezone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if body.Name != nil {
		h.db.Exec(r.Context(), `UPDATE accounts SET name = $1, updated_at = NOW() WHERE id = $2`, *body.Name, accountID)
	}
	if body.Timezone != nil {
		h.db.Exec(r.Context(), `UPDATE accounts SET timezone = $1, updated_at = NOW() WHERE id = $2`, *body.Timezone, accountID)
	}

	writeJSON(w, map[string]string{"status": "updated"}, http.StatusOK)
}
