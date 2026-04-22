package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/bluesend/api/internal/middleware"
	"github.com/bluesend/api/internal/models"
	"github.com/bluesend/api/internal/services/email"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type TeamHandler struct {
	db               *pgxpool.Pool
	email            *email.Service
	jwtSecret        string
	jwtRefreshSecret string
}

func NewTeamHandler(db *pgxpool.Pool, emailSvc *email.Service, jwtSecret, jwtRefreshSecret string) *TeamHandler {
	return &TeamHandler{db: db, email: emailSvc, jwtSecret: jwtSecret, jwtRefreshSecret: jwtRefreshSecret}
}

// GET /api/team/members — list team members
func (h *TeamHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())

	rows, err := h.db.Query(r.Context(), `
		SELECT id::text, email, first_name, last_name, role, last_login_at, created_at
		FROM users
		WHERE account_id = $1
		ORDER BY created_at ASC
	`, accountID)
	if err != nil {
		writeError(w, "database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Member struct {
		ID          string  `json:"id"`
		Email       string  `json:"email"`
		FirstName   *string `json:"first_name"`
		LastName    *string `json:"last_name"`
		Role        string  `json:"role"`
		LastLoginAt *string `json:"last_login_at"`
		CreatedAt   string  `json:"created_at"`
	}

	var members []Member
	for rows.Next() {
		var m Member
		var lastLogin *time.Time
		var createdAt time.Time
		if err := rows.Scan(&m.ID, &m.Email, &m.FirstName, &m.LastName, &m.Role, &lastLogin, &createdAt); err != nil {
			continue
		}
		m.CreatedAt = createdAt.Format(time.RFC3339)
		if lastLogin != nil {
			s := lastLogin.Format(time.RFC3339)
			m.LastLoginAt = &s
		}
		members = append(members, m)
	}
	if members == nil {
		members = []Member{}
	}

	writeJSON(w, map[string]interface{}{"members": members}, http.StatusOK)
}

// POST /api/team/invite — invite a team member
func (h *TeamHandler) Invite(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())
	userID, _ := middleware.GetUserID(r.Context())
	role, _ := middleware.GetRole(r.Context())

	if role != string(models.UserRoleOwner) && role != string(models.UserRoleAdmin) {
		writeError(w, "only owners and admins can invite team members", http.StatusForbidden)
		return
	}

	var body models.InviteRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.Email == "" {
		writeError(w, "email is required", http.StatusBadRequest)
		return
	}
	if body.Role != "member" && body.Role != "admin" {
		body.Role = "member"
	}

	// Check if email already exists as a user in this account
	var exists bool
	h.db.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND account_id = $2)`,
		body.Email, accountID,
	).Scan(&exists)
	if exists {
		writeError(w, "this email is already a member of your team", http.StatusConflict)
		return
	}

	// Check for existing pending invitation
	h.db.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM invitations WHERE email = $1 AND account_id = $2 AND accepted_at IS NULL AND expires_at > NOW())`,
		body.Email, accountID,
	).Scan(&exists)
	if exists {
		writeError(w, "a pending invitation already exists for this email", http.StatusConflict)
		return
	}

	// Generate secure token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	token := hex.EncodeToString(tokenBytes)

	_, err := h.db.Exec(r.Context(), `
		INSERT INTO invitations (account_id, email, role, token, invited_by)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (account_id, email) DO UPDATE
		SET role = $3, token = $4, invited_by = $5, expires_at = NOW() + INTERVAL '7 days', accepted_at = NULL
	`, accountID, body.Email, body.Role, token, userID)
	if err != nil {
		writeError(w, "failed to create invitation: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Send invitation email
	if h.email != nil && h.email.Enabled() {
		h.email.SendTeamInvite(body.Email, token)
	}

	writeJSON(w, map[string]string{"status": "invited"}, http.StatusCreated)
}

// GET /api/team/invitations — list pending invitations
func (h *TeamHandler) ListInvitations(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())

	rows, err := h.db.Query(r.Context(), `
		SELECT i.id::text, i.email, i.role, i.expires_at, i.created_at,
		       u.email as invited_by_email
		FROM invitations i
		JOIN users u ON u.id = i.invited_by
		WHERE i.account_id = $1 AND i.accepted_at IS NULL AND i.expires_at > NOW()
		ORDER BY i.created_at DESC
	`, accountID)
	if err != nil {
		writeJSON(w, map[string]interface{}{"invitations": []interface{}{}}, http.StatusOK)
		return
	}
	defer rows.Close()

	type InviteRow struct {
		ID             string `json:"id"`
		Email          string `json:"email"`
		Role           string `json:"role"`
		ExpiresAt      string `json:"expires_at"`
		CreatedAt      string `json:"created_at"`
		InvitedByEmail string `json:"invited_by_email"`
	}

	var invitations []InviteRow
	for rows.Next() {
		var inv InviteRow
		var expiresAt, createdAt time.Time
		if err := rows.Scan(&inv.ID, &inv.Email, &inv.Role, &expiresAt, &createdAt, &inv.InvitedByEmail); err != nil {
			continue
		}
		inv.ExpiresAt = expiresAt.Format(time.RFC3339)
		inv.CreatedAt = createdAt.Format(time.RFC3339)
		invitations = append(invitations, inv)
	}
	if invitations == nil {
		invitations = []InviteRow{}
	}

	writeJSON(w, map[string]interface{}{"invitations": invitations}, http.StatusOK)
}

// DELETE /api/team/invitations/{invitationID} — revoke an invitation
func (h *TeamHandler) RevokeInvitation(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())
	role, _ := middleware.GetRole(r.Context())
	if role != string(models.UserRoleOwner) && role != string(models.UserRoleAdmin) {
		writeError(w, "only owners and admins can revoke invitations", http.StatusForbidden)
		return
	}

	invitationID := chi.URLParam(r, "invitationID")
	res, err := h.db.Exec(r.Context(),
		`DELETE FROM invitations WHERE id = $1 AND account_id = $2 AND accepted_at IS NULL`,
		invitationID, accountID,
	)
	if err != nil {
		writeError(w, "revoke failed", http.StatusInternalServerError)
		return
	}
	if res.RowsAffected() == 0 {
		writeError(w, "invitation not found", http.StatusNotFound)
		return
	}

	writeJSON(w, map[string]string{"status": "revoked"}, http.StatusOK)
}

// DELETE /api/team/members/{userID} — remove a team member
func (h *TeamHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())
	callerRole, _ := middleware.GetRole(r.Context())
	callerID, _ := middleware.GetUserID(r.Context())

	if callerRole != string(models.UserRoleOwner) {
		writeError(w, "only the account owner can remove team members", http.StatusForbidden)
		return
	}

	targetID := chi.URLParam(r, "userID")
	if targetID == callerID.String() {
		writeError(w, "you cannot remove yourself", http.StatusBadRequest)
		return
	}

	res, err := h.db.Exec(r.Context(),
		`DELETE FROM users WHERE id = $1 AND account_id = $2 AND role != 'owner'`,
		targetID, accountID,
	)
	if err != nil {
		writeError(w, "remove failed", http.StatusInternalServerError)
		return
	}
	if res.RowsAffected() == 0 {
		writeError(w, "member not found or cannot be removed", http.StatusNotFound)
		return
	}

	// Also clean up their refresh tokens
	h.db.Exec(r.Context(), `DELETE FROM refresh_tokens WHERE user_id = $1`, targetID)

	writeJSON(w, map[string]string{"status": "removed"}, http.StatusOK)
}

// PATCH /api/team/members/{userID}/role — change a member's role
func (h *TeamHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())
	callerRole, _ := middleware.GetRole(r.Context())

	if callerRole != string(models.UserRoleOwner) {
		writeError(w, "only the account owner can change roles", http.StatusForbidden)
		return
	}

	targetID := chi.URLParam(r, "userID")
	var body struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.Role != "member" && body.Role != "admin" {
		writeError(w, "role must be 'member' or 'admin'", http.StatusBadRequest)
		return
	}

	res, err := h.db.Exec(r.Context(),
		`UPDATE users SET role = $1, updated_at = NOW() WHERE id = $2 AND account_id = $3 AND role != 'owner'`,
		body.Role, targetID, accountID,
	)
	if err != nil {
		writeError(w, "update failed", http.StatusInternalServerError)
		return
	}
	if res.RowsAffected() == 0 {
		writeError(w, "member not found or cannot change owner role", http.StatusNotFound)
		return
	}

	writeJSON(w, map[string]string{"status": "updated"}, http.StatusOK)
}

// POST /api/auth/accept-invite — accept an invitation and create account
func (h *TeamHandler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token     string `json:"token"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Password  string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.Token == "" || body.Password == "" || len(body.Password) < 8 {
		writeError(w, "token and password (min 8 chars) required", http.StatusBadRequest)
		return
	}

	// Find invitation
	var invitationID, accountID uuid.UUID
	var invEmail, invRole string
	var expiresAt time.Time
	var acceptedAt *time.Time
	err := h.db.QueryRow(r.Context(), `
		SELECT id, account_id, email, role, expires_at, accepted_at
		FROM invitations WHERE token = $1
	`, body.Token).Scan(&invitationID, &accountID, &invEmail, &invRole, &expiresAt, &acceptedAt)
	if err != nil {
		writeError(w, "invalid or expired invitation", http.StatusBadRequest)
		return
	}
	if acceptedAt != nil {
		writeError(w, "this invitation has already been accepted", http.StatusBadRequest)
		return
	}
	if time.Now().After(expiresAt) {
		writeError(w, "this invitation has expired", http.StatusBadRequest)
		return
	}

	// Check if user already exists with this email
	var existingID *uuid.UUID
	h.db.QueryRow(r.Context(), `SELECT id FROM users WHERE email = $1`, invEmail).Scan(&existingID)
	if existingID != nil {
		writeError(w, "an account with this email already exists — try logging in", http.StatusConflict)
		return
	}

	// Create user
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	userID := uuid.New()
	_, err = h.db.Exec(r.Context(), `
		INSERT INTO users (id, account_id, email, password_hash, first_name, last_name, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
	`, userID, accountID, invEmail, string(hash), body.FirstName, body.LastName, invRole)
	if err != nil {
		writeError(w, "failed to create user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Mark invitation as accepted
	h.db.Exec(r.Context(), `UPDATE invitations SET accepted_at = NOW() WHERE id = $1`, invitationID)

	// Issue tokens
	accessToken, err := middleware.IssueToken(userID, accountID, invRole, h.jwtSecret)
	if err != nil {
		writeError(w, "token issue error", http.StatusInternalServerError)
		return
	}
	refreshToken, err := middleware.IssueRefreshToken(userID, accountID, invRole, h.jwtRefreshSecret)
	if err != nil {
		writeError(w, "token issue error", http.StatusInternalServerError)
		return
	}

	// Store refresh token
	tokenHash := hashToken(refreshToken)
	h.db.Exec(r.Context(), `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES (uuid_generate_v4(), $1, $2, $3, NOW())
	`, userID, tokenHash, time.Now().Add(30*24*time.Hour))

	writeJSON(w, map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user": map[string]interface{}{
			"id":         userID.String(),
			"email":      invEmail,
			"role":       invRole,
			"first_name": body.FirstName,
			"last_name":  body.LastName,
		},
	}, http.StatusCreated)
}
