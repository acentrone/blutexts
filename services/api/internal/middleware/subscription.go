package middleware

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RequireActiveSubscription rejects requests from accounts that are not in an
// active (or setting_up) state. Accounts that are pending checkout, cancelled,
// past-due, or suspended get a 402 with a JSON body the frontend can key on to
// show an appropriate upgrade/reactivation prompt.
//
// Routes that should remain accessible without an active subscription (billing
// endpoints, /me, WebSocket, etc.) should be mounted OUTSIDE the group that
// uses this middleware.
type SubscriptionGate struct {
	db *pgxpool.Pool
}

func NewSubscriptionGate(db *pgxpool.Pool) *SubscriptionGate {
	return &SubscriptionGate{db: db}
}

func (s *SubscriptionGate) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := GetAccountID(r.Context())
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		var status string
		err := s.db.QueryRow(context.Background(),
			`SELECT status FROM accounts WHERE id = $1`, accountID,
		).Scan(&status)
		if err != nil {
			http.Error(w, `{"error":"account not found"}`, http.StatusUnauthorized)
			return
		}

		switch status {
		case "active", "setting_up":
			// Good to go — account has a valid subscription.
			next.ServeHTTP(w, r)
		case "past_due":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusPaymentRequired)
			w.Write([]byte(`{"error":"subscription_past_due","message":"Your payment is past due. Please update your billing information to continue using BluTexts."}`))
		case "cancelled":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusPaymentRequired)
			w.Write([]byte(`{"error":"subscription_cancelled","message":"Your subscription has been cancelled. Reactivate to regain access."}`))
		case "suspended":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusPaymentRequired)
			w.Write([]byte(`{"error":"account_suspended","message":"Your account has been suspended. Please contact support."}`))
		default:
			// pending, or any unknown status — they haven't paid yet
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusPaymentRequired)
			w.Write([]byte(`{"error":"subscription_required","message":"A subscription is required to use BluTexts. Please complete checkout to get started."}`))
		}
	})
}
