package middleware

import (
	"net/http"
)

// AdminKeyAuth validates requests to internal admin endpoints using a static API key.
// This is used for the operator dashboard and device registration, in addition to
// the JWT-based auth for the regular admin user role.
type AdminKeyAuth struct {
	key string
}

func NewAdminKeyAuth(key string) *AdminKeyAuth {
	return &AdminKeyAuth{key: key}
}

func (a *AdminKeyAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := r.Header.Get("X-Admin-Key")
		if provided == "" || provided != a.key {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
