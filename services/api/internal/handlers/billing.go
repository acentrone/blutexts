package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/bluesend/api/internal/middleware"
	"github.com/bluesend/api/internal/models"
	stripeService "github.com/bluesend/api/internal/services/stripe"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BillingHandler struct {
	db      *pgxpool.Pool
	billing *stripeService.BillingService
	appURL  string
}

func NewBillingHandler(db *pgxpool.Pool, billing *stripeService.BillingService, appURL string) *BillingHandler {
	return &BillingHandler{db: db, billing: billing, appURL: appURL}
}

// POST /api/billing/checkout — create Stripe Checkout session (pre-auth, called during onboarding)
func (h *BillingHandler) CreateCheckout(w http.ResponseWriter, r *http.Request) {
	if h.billing == nil {
		writeError(w, "billing is not enabled", http.StatusServiceUnavailable)
		return
	}
	var req models.CreateCheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Plan != "monthly" && req.Plan != "annual" {
		writeError(w, "plan must be monthly or annual", http.StatusBadRequest)
		return
	}

	result, err := h.billing.CreateCheckoutSession(r.Context(), stripeService.CreateCheckoutParams{
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Company:   req.Company,
		Plan:      req.Plan,
	})
	if err != nil {
		writeError(w, "could not create checkout session", http.StatusInternalServerError)
		return
	}

	// Persist stripe_customer_id to account if user is logged in
	if accountID, ok := middleware.GetAccountID(r.Context()); ok {
		h.db.Exec(r.Context(), `
			UPDATE accounts SET stripe_customer_id = $1 WHERE id = $2
		`, result.CustomerID, accountID)
	}

	writeJSON(w, models.CreateCheckoutResponse{
		URL:          result.URL,
		SessionID:    result.SessionID,
		ClientSecret: result.ClientSecret,
		CustomerID:   result.CustomerID,
	}, http.StatusOK)
}

// POST /api/billing/portal — redirect to Stripe customer portal
func (h *BillingHandler) CreatePortalSession(w http.ResponseWriter, r *http.Request) {
	if h.billing == nil {
		writeError(w, "billing is not enabled", http.StatusServiceUnavailable)
		return
	}
	accountID, _ := middleware.GetAccountID(r.Context())

	var customerID string
	err := h.db.QueryRow(r.Context(), `
		SELECT stripe_customer_id FROM accounts WHERE id = $1
	`, accountID).Scan(&customerID)
	if err != nil || customerID == "" {
		writeError(w, "no billing account found", http.StatusNotFound)
		return
	}

	portalURL, err := h.billing.CreatePortalSession(r.Context(), customerID, h.appURL+"/dashboard/billing")
	if err != nil {
		writeError(w, "could not create portal session", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"url": portalURL}, http.StatusOK)
}

// GET /api/billing/invoices
func (h *BillingHandler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())

	rows, err := h.db.Query(r.Context(), `
		SELECT id, stripe_invoice_id, amount_due, amount_paid, currency, status,
		       invoice_pdf_url, period_start, period_end, paid_at, created_at
		FROM invoices
		WHERE account_id = $1
		ORDER BY created_at DESC
		LIMIT 24
	`, accountID)
	if err != nil {
		writeError(w, "database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Invoice struct {
		ID             string  `json:"id"`
		StripeID       string  `json:"stripe_invoice_id"`
		AmountDue      int     `json:"amount_due"`
		AmountPaid     int     `json:"amount_paid"`
		Currency       string  `json:"currency"`
		Status         string  `json:"status"`
		PDFUrl         *string `json:"invoice_pdf_url"`
		PeriodStart    *string `json:"period_start"`
		PeriodEnd      *string `json:"period_end"`
		PaidAt         *string `json:"paid_at"`
		CreatedAt      string  `json:"created_at"`
	}

	var invoices []Invoice
	for rows.Next() {
		var inv Invoice
		rows.Scan(&inv.ID, &inv.StripeID, &inv.AmountDue, &inv.AmountPaid, &inv.Currency,
			&inv.Status, &inv.PDFUrl, &inv.PeriodStart, &inv.PeriodEnd, &inv.PaidAt, &inv.CreatedAt)
		invoices = append(invoices, inv)
	}
	if invoices == nil {
		invoices = []Invoice{}
	}

	writeJSON(w, map[string]interface{}{"invoices": invoices}, http.StatusOK)
}

// GET /api/billing/subscription
func (h *BillingHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	accountID, _ := middleware.GetAccountID(r.Context())

	var account models.Account
	err := h.db.QueryRow(r.Context(), `
		SELECT id, status, plan, stripe_customer_id, stripe_subscription_id, setup_fee_paid
		FROM accounts WHERE id = $1
	`, accountID).Scan(
		&account.ID, &account.Status, &account.Plan,
		&account.StripeCustomerID, &account.StripeSubscriptionID, &account.SetupFeePaid,
	)
	if err != nil {
		writeError(w, "account not found", http.StatusNotFound)
		return
	}

	writeJSON(w, map[string]interface{}{
		"status":            account.Status,
		"plan":              account.Plan,
		"setup_fee_paid":    account.SetupFeePaid,
		"has_subscription":  account.StripeSubscriptionID != nil,
	}, http.StatusOK)
}
