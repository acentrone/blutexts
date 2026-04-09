package middleware

import (
	"context"
	"net/http"
)

// AdminAuth validates requests to admin endpoints.
// Accepts either a static API key (X-Admin-Key header) for programmatic access,
// or a JWT Bearer token from a user with role='admin'.
type AdminAuth struct {
	key     string
	authMw  *AuthMiddleware
}

func NewAdminAuth(key string, authMw *AuthMiddleware) *AdminAuth {
	return &AdminAuth{key: key, authMw: authMw}
}

func (a *AdminAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try API key first
		if apiKey := r.Header.Get("X-Admin-Key"); apiKey != "" && apiKey == a.key {
			next.ServeHTTP(w, r)
			return
		}

		// Try JWT with admin role
		token := extractBearerToken(r)
		if token != "" {
			claims, err := a.authMw.validateToken(token)
			if err == nil && claims.Role == "admin" {
				ctx := context.WithValue(r.Context(), ContextKeyUserID, claims.UserID)
				ctx = context.WithValue(ctx, ContextKeyAccountID, claims.AccountID)
				ctx = context.WithValue(ctx, ContextKeyUserRole, claims.Role)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
	})
}
