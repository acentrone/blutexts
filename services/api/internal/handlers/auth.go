package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/bluesend/api/internal/middleware"
	"github.com/bluesend/api/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	db               *pgxpool.Pool
	jwtSecret        string
	jwtRefreshSecret string
}

func NewAuthHandler(db *pgxpool.Pool, jwtSecret, jwtRefreshSecret string) *AuthHandler {
	return &AuthHandler{db: db, jwtSecret: jwtSecret, jwtRefreshSecret: jwtRefreshSecret}
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

	// Create account
	accountID := uuid.New()
	_, err = h.db.Exec(r.Context(), `
		INSERT INTO accounts (id, name, email, status, plan, created_at, updated_at)
		VALUES ($1, $2, $3, 'active', 'free', NOW(), NOW())
	`, accountID, req.Company, req.Email)
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

	writeJSON(w, resp, http.StatusOK)
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
		       a.id, a.name, a.email, a.status, a.plan, a.setup_complete, a.timezone
		FROM users u
		JOIN accounts a ON a.id = u.account_id
		WHERE u.id = $1 AND a.id = $2
	`, userID, accountID).Scan(
		&user.ID, &user.AccountID, &user.Email, &user.FirstName, &user.LastName, &user.Role,
		&account.ID, &account.Name, &account.Email, &account.Status, &account.Plan,
		&account.SetupComplete, &account.Timezone,
	)
	if err != nil {
		writeError(w, "user not found", http.StatusNotFound)
		return
	}

	writeJSON(w, map[string]interface{}{
		"user":    user,
		"account": account,
	}, http.StatusOK)
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
