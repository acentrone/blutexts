package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type contextKey string

const (
	ContextKeyUserID    contextKey = "user_id"
	ContextKeyAccountID contextKey = "account_id"
	ContextKeyUserRole  contextKey = "user_role"
)

type Claims struct {
	UserID    uuid.UUID `json:"user_id"`
	AccountID uuid.UUID `json:"account_id"`
	Role      string    `json:"role"`
	jwt.RegisteredClaims
}

type AuthMiddleware struct {
	jwtSecret string
}

func NewAuthMiddleware(jwtSecret string) *AuthMiddleware {
	return &AuthMiddleware{jwtSecret: jwtSecret}
}

// Authenticate validates the JWT and injects claims into context.
func (a *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		if token == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		claims, err := a.validateToken(token)
		if err != nil {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), ContextKeyUserID, claims.UserID)
		ctx = context.WithValue(ctx, ContextKeyAccountID, claims.AccountID)
		ctx = context.WithValue(ctx, ContextKeyUserRole, claims.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin ensures the authenticated user has admin role.
func (a *AuthMiddleware) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, ok := r.Context().Value(ContextKeyUserRole).(string)
		if !ok || role != "admin" {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *AuthMiddleware) validateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(a.jwtSecret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		// Also check cookie for dashboard (httpOnly)
		if c, err := r.Cookie("access_token"); err == nil {
			return c.Value
		}
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}

// IssueToken creates a signed JWT access token.
func IssueToken(userID, accountID uuid.UUID, role, secret string) (string, error) {
	claims := &Claims{
		UserID:    userID,
		AccountID: accountID,
		Role:      role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "bluesend",
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// IssueRefreshToken creates a longer-lived refresh token.
func IssueRefreshToken(userID, accountID uuid.UUID, role, secret string) (string, error) {
	claims := &Claims{
		UserID:    userID,
		AccountID: accountID,
		Role:      role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "bluesend-refresh",
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// GetUserID extracts user ID from context.
func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ContextKeyUserID).(uuid.UUID)
	return id, ok
}

// GetAccountID extracts account ID from context.
func GetAccountID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ContextKeyAccountID).(uuid.UUID)
	return id, ok
}
