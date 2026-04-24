package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/bluesend/api/internal/middleware"
	"github.com/bluesend/api/internal/models"
	"github.com/bluesend/api/internal/services/audit"
	"github.com/bluesend/api/internal/services/email"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	db               *pgxpool.Pool
	jwtSecret        string
	jwtRefreshSecret string
	email            *email.Service
}

func NewAuthHandler(db *pgxpool.Pool, jwtSecret, jwtRefreshSecret string, emailSvc *email.Service) *AuthHandler {
	return &AuthHandler{db: db, jwtSecret: jwtSecret, jwtRefreshSecret: jwtRefreshSecret, email: emailSvc}
}

// POST /api/auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Email == "" || req.Password == "" || len(req.Password) < 8 {
		writeError(w, "email and password (min 8 chars) required", http.StatusBadRequest)
		return
	}

	// Check email uniqueness
	var exists bool
	h.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, req.Email).Scan(&exists)
	if exists {
		writeError(w, "email already registered", http.StatusConflict)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Create account in 'pending' state — the subscription gate blocks every
	// gated route (messages/send, contacts, calls, ws-broadcast, etc.) until
	// the Stripe webhook flips status → 'setting_up' on checkout.completed.
	// Anything that lands here without paying is harmless: they have a JWT
	// and a /me payload, but every product surface returns 402.
	accountID := uuid.New()
	_, err = h.db.Exec(r.Context(), `
		INSERT INTO accounts (id, name, email, status, plan, preferred_area_code, created_at, updated_at)
		VALUES ($1, $2, $3, 'pending', 'free', $4, NOW(), NOW())
	`, accountID, req.Company, req.Email, req.PreferredAreaCode)
	if err != nil {
		writeError(w, "could not create account", http.StatusInternalServerError)
		return
	}

	// Create user
	userID := uuid.New()
	_, err = h.db.Exec(r.Context(), `
		INSERT INTO users (id, account_id, email, password_hash, first_name, last_name, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'owner', NOW(), NOW())
	`, userID, accountID, req.Email, string(hash), req.FirstName, req.LastName)
	if err != nil {
		writeError(w, "could not create user", http.StatusInternalServerError)
		return
	}

	resp, err := h.issueTokens(r, accountID, userID, "owner")
	if err != nil {
		writeError(w, "token issue error", http.StatusInternalServerError)
		return
	}

	audit.Log(r.Context(), h.db, accountID, userID, "account.created", "account", accountID.String(),
		map[string]interface{}{
			"email":   req.Email,
			"company": req.Company,
		}, clientIP(r))

	writeJSON(w, resp, http.StatusCreated)
}

// POST /api/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var user models.User
	var account models.Account
	err := h.db.QueryRow(r.Context(), `
		SELECT u.id, u.account_id, u.email, u.password_hash, u.first_name, u.last_name, u.role,
		       a.id, a.name, a.email, a.status, a.plan, a.setup_complete
		FROM users u
		JOIN accounts a ON a.id = u.account_id
		WHERE u.email = $1
	`, req.Email).Scan(
		&user.ID, &user.AccountID, &user.Email, &user.PasswordHash, &user.FirstName, &user.LastName, &user.Role,
		&account.ID, &account.Name, &account.Email, &account.Status, &account.Plan, &account.SetupComplete,
	)
	if err != nil {
		writeError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		writeError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	h.db.Exec(r.Context(), `UPDATE users SET last_login_at = NOW() WHERE id = $1`, user.ID)

	resp, err := h.issueTokens(r, account.ID, user.ID, string(user.Role))
	if err != nil {
		writeError(w, "token issue error", http.StatusInternalServerError)
		return
	}
	resp.User = &user
	resp.Account = &account

	audit.Log(r.Context(), h.db, account.ID, user.ID, "user.login", "user", user.ID.String(),
		map[string]interface{}{"email": user.Email}, clientIP(r))

	writeJSON(w, resp, http.StatusOK)
}

// clientIP extracts the client's IP address from the request, respecting X-Forwarded-For.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return fwd
	}
	return r.RemoteAddr
}

// POST /api/auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.RefreshToken == "" {
		writeError(w, "refresh_token required", http.StatusBadRequest)
		return
	}

	tokenHash := hashToken(body.RefreshToken)
	var userID, accountID uuid.UUID
	var role string
	var expiresAt time.Time

	err := h.db.QueryRow(r.Context(), `
		SELECT rt.user_id, u.account_id, u.role, rt.expires_at
		FROM refresh_tokens rt
		JOIN users u ON u.id = rt.user_id
		WHERE rt.token_hash = $1
	`, tokenHash).Scan(&userID, &accountID, &role, &expiresAt)
	if err != nil || time.Now().After(expiresAt) {
		writeError(w, "invalid or expired refresh token", http.StatusUnauthorized)
		return
	}

	// Rotate token
	h.db.Exec(r.Context(), `DELETE FROM refresh_tokens WHERE token_hash = $1`, tokenHash)

	resp, err := h.issueTokens(r, accountID, userID, role)
	if err != nil {
		writeError(w, "token issue error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, resp, http.StatusOK)
}

// POST /api/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.RefreshToken != "" {
		tokenHash := hashToken(body.RefreshToken)
		h.db.Exec(r.Context(), `DELETE FROM refresh_tokens WHERE token_hash = $1`, tokenHash)
	}
	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:    "access_token",
		Value:   "",
		MaxAge:  -1,
		Path:    "/",
		Secure:  true,
		HttpOnly: true,
	})
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/auth/me
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r.Context())
	accountID, _ := middleware.GetAccountID(r.Context())

	var user models.User
	var account models.Account
	err := h.db.QueryRow(r.Context(), `
		SELECT u.id, u.account_id, u.email, u.first_name, u.last_name, u.role,
		       a.id, a.name, a.email, a.status, a.plan, a.setup_complete, a.timezone,
		       a.custom_field_schema
		FROM users u
		JOIN accounts a ON a.id = u.account_id
		WHERE u.id = $1 AND a.id = $2
	`, userID, accountID).Scan(
		&user.ID, &user.AccountID, &user.Email, &user.FirstName, &user.LastName, &user.Role,
		&account.ID, &account.Name, &account.Email, &account.Status, &account.Plan,
		&account.SetupComplete, &account.Timezone,
		&account.CustomFieldSchema,
	)
	if err != nil {
		writeError(w, "user not found", http.StatusNotFound)
		return
	}

	// Get GHL location ID for CRM link construction
	var ghlLocationID *string
	h.db.QueryRow(r.Context(),
		`SELECT location_id FROM ghl_connections WHERE account_id = $1 AND connected = true LIMIT 1`,
		accountID,
	).Scan(&ghlLocationID)

	writeJSON(w, map[string]interface{}{
		"user":            user,
		"account":         account,
		"ghl_location_id": ghlLocationID,
	}, http.StatusOK)
}

// POST /api/auth/forgot-password
// Sends a password-reset email. Always returns 200 to avoid leaking whether
// the email exists — the actual email is only sent if the account is real.
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" {
		writeError(w, "email required", http.StatusBadRequest)
		return
	}

	// Look up user — fail silently if not found (no account enumeration).
	var userID uuid.UUID
	err := h.db.QueryRow(r.Context(), `SELECT id FROM users WHERE email = $1`, body.Email).Scan(&userID)
	if err != nil {
		writeJSON(w, map[string]string{"status": "ok"}, http.StatusOK)
		return
	}

	// Invalidate any previous reset tokens for this user.
	h.db.Exec(r.Context(), `DELETE FROM password_reset_tokens WHERE user_id = $1 AND used_at IS NULL`, userID)

	// Generate a cryptographically random token.
	rawToken, err := generateSecureToken(32)
	if err != nil {
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	tokenHash := hashToken(rawToken)
	expiresAt := time.Now().Add(1 * time.Hour)

	_, err = h.db.Exec(r.Context(), `
		INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES (uuid_generate_v4(), $1, $2, $3, NOW())
	`, userID, tokenHash, expiresAt)
	if err != nil {
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Send the email (or log in dev mode).
	if err := h.email.SendPasswordReset(body.Email, rawToken); err != nil {
		fmt.Printf("password reset email error: %v\n", err)
		// Still return 200 so we don't leak info.
	}

	writeJSON(w, map[string]string{"status": "ok"}, http.StatusOK)
}

// POST /api/auth/reset-password
// Validates the token and sets a new password.
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.Token == "" || len(body.Password) < 8 {
		writeError(w, "token and password (min 8 chars) required", http.StatusBadRequest)
		return
	}

	tokenHash := hashToken(body.Token)

	var tokenID, userID uuid.UUID
	var expiresAt time.Time
	var usedAt *time.Time
	err := h.db.QueryRow(r.Context(), `
		SELECT id, user_id, expires_at, used_at
		FROM password_reset_tokens
		WHERE token_hash = $1
	`, tokenHash).Scan(&tokenID, &userID, &expiresAt, &usedAt)
	if err != nil {
		writeError(w, "invalid or expired reset link", http.StatusBadRequest)
		return
	}
	if usedAt != nil {
		writeError(w, "this reset link has already been used", http.StatusBadRequest)
		return
	}
	if time.Now().After(expiresAt) {
		writeError(w, "this reset link has expired — request a new one", http.StatusBadRequest)
		return
	}

	// Hash new password.
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Update the user's password and mark the token as used.
	h.db.Exec(r.Context(), `UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2`, string(hash), userID)
	h.db.Exec(r.Context(), `UPDATE password_reset_tokens SET used_at = NOW() WHERE id = $1`, tokenID)

	// Revoke all refresh tokens so existing sessions are logged out.
	h.db.Exec(r.Context(), `DELETE FROM refresh_tokens WHERE user_id = $1`, userID)

	var accountID uuid.UUID
	h.db.QueryRow(r.Context(), `SELECT account_id FROM users WHERE id = $1`, userID).Scan(&accountID)
	audit.Log(r.Context(), h.db, accountID, userID, "user.password_reset", "user", userID.String(), nil, clientIP(r))

	writeJSON(w, map[string]string{"status": "ok"}, http.StatusOK)
}

// generateSecureToken returns a hex-encoded cryptographically random string.
func generateSecureToken(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (h *AuthHandler) issueTokens(r *http.Request, accountID, userID uuid.UUID, role string) (*models.AuthResponse, error) {
	accessToken, err := middleware.IssueToken(userID, accountID, role, h.jwtSecret)
	if err != nil {
		return nil, err
	}

	refreshToken, err := middleware.IssueRefreshToken(userID, accountID, role, h.jwtRefreshSecret)
	if err != nil {
		return nil, err
	}

	// Store refresh token hash
	tokenHash := hashToken(refreshToken)
	h.db.Exec(r.Context(), `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES (uuid_generate_v4(), $1, $2, $3, NOW())
	`, userID, tokenHash, time.Now().Add(30*24*time.Hour))

	return &models.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
