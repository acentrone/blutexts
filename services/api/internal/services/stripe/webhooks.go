package stripe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bluesend/api/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	stripeLib "github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/webhook"
)

// WebhookHandler processes Stripe webhook events.
type WebhookHandler struct {
	db            *pgxpool.Pool
	webhookSecret string
	billing       *BillingService
	// onAccountActivated is called after successful payment to trigger GHL provisioning
	onAccountActivated func(ctx context.Context, accountID uuid.UUID)
}

func NewWebhookHandler(db *pgxpool.Pool, webhookSecret string, billing *BillingService, onActivated func(context.Context, uuid.UUID)) *WebhookHandler {
	return &WebhookHandler{
		db:                 db,
		webhookSecret:      webhookSecret,
		billing:            billing,
		onAccountActivated: onActivated,
	}
}

// Handle is the HTTP handler for POST /api/webhooks/stripe
func (h *WebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	const maxBodyBytes = int64(65536)
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "body read error", http.StatusBadRequest)
		return
	}

	event, err := webhook.ConstructEvent(body, r.Header.Get("Stripe-Signature"), h.webhookSecret)
	if err != nil {
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	// Idempotency check
	var alreadyProcessed bool
	_ = h.db.QueryRow(r.Context(), `
		SELECT EXISTS(SELECT 1 FROM billing_events WHERE stripe_event_id = $1 AND processed = true)
	`, event.ID).Scan(&alreadyProcessed)
	if alreadyProcessed {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Log the raw event
	payloadJSON, _ := json.Marshal(event)
	h.db.Exec(r.Context(), `
		INSERT INTO billing_events (stripe_event_id, event_type, payload, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (stripe_event_id) DO NOTHING
	`, event.ID, event.Type, payloadJSON)

	var processErr error

	switch event.Type {
	case "checkout.session.completed":
		processErr = h.handleCheckoutCompleted(r.Context(), event)
	case "invoice.paid":
		processErr = h.handleInvoicePaid(r.Context(), event)
	case "invoice.payment_failed":
		processErr = h.handleInvoicePaymentFailed(r.Context(), event)
	case "customer.subscription.deleted":
		processErr = h.handleSubscriptionDeleted(r.Context(), event)
	case "customer.subscription.updated":
		processErr = h.handleSubscriptionUpdated(r.Context(), event)
	}

	if processErr != nil {
		h.db.Exec(r.Context(), `
			UPDATE billing_events SET error = $1 WHERE stripe_event_id = $2
		`, processErr.Error(), event.ID)
		// Return 200 to prevent Stripe retrying for application errors
		// (infra errors should return 5xx)
		fmt.Printf("Stripe webhook processing error [%s]: %v\n", event.Type, processErr)
	} else {
		h.db.Exec(r.Context(), `
			UPDATE billing_events SET processed = true, processed_at = NOW() WHERE stripe_event_id = $1
		`, event.ID)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) handleCheckoutCompleted(ctx context.Context, event stripeLib.Event) error {
	var sess stripeLib.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		return fmt.Errorf("unmarshal checkout session: %w", err)
	}

	// Find account by stripe_customer_id
	var accountID uuid.UUID
	err := h.db.QueryRow(ctx, `
		SELECT id FROM accounts WHERE stripe_customer_id = $1
	`, sess.Customer.ID).Scan(&accountID)
	if err != nil {
		return fmt.Errorf("account not found for customer %s: %w", sess.Customer.ID, err)
	}

	plan := "monthly"
	if sess.Subscription != nil && sess.Subscription.Metadata != nil {
		if p, ok := sess.Subscription.Metadata["plan"]; ok {
			plan = p
		}
	}

	_, err = h.db.Exec(ctx, `
		UPDATE accounts
		SET status = 'setting_up', plan = $1, setup_fee_paid = true,
		    stripe_subscription_id = $2, updated_at = NOW()
		WHERE id = $3
	`, plan, sess.Subscription.ID, accountID)
	if err != nil {
		return err
	}

	// Trigger GHL provisioning + phone number assignment
	if h.onAccountActivated != nil {
		go h.onAccountActivated(context.Background(), accountID)
	}

	return nil
}

func (h *WebhookHandler) handleInvoicePaid(ctx context.Context, event stripeLib.Event) error {
	var inv stripeLib.Invoice
	if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
		return err
	}
	if inv.Customer == nil {
		return nil
	}

	// Store invoice record
	var accountID uuid.UUID
	err := h.db.QueryRow(ctx, `SELECT id FROM accounts WHERE stripe_customer_id = $1`, inv.Customer.ID).Scan(&accountID)
	if err != nil {
		return err
	}

	_, _ = h.db.Exec(ctx, `
		INSERT INTO invoices (id, account_id, stripe_invoice_id, amount_due, amount_paid, currency, status, invoice_pdf_url, paid_at, created_at)
		VALUES (uuid_generate_v4(), $1, $2, $3, $4, $5, 'paid', $6, NOW(), NOW())
		ON CONFLICT (stripe_invoice_id) DO UPDATE
		SET status = 'paid', amount_paid = EXCLUDED.amount_paid, paid_at = NOW()
	`, accountID, inv.ID, inv.AmountDue, inv.AmountPaid, string(inv.Currency), inv.InvoicePDF)

	// Re-activate if past_due
	_, _ = h.db.Exec(ctx, `
		UPDATE accounts SET status = 'active', updated_at = NOW()
		WHERE id = $1 AND status = 'past_due'
	`, accountID)

	return nil
}

func (h *WebhookHandler) handleInvoicePaymentFailed(ctx context.Context, event stripeLib.Event) error {
	var inv stripeLib.Invoice
	if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
		return err
	}
	if inv.Customer == nil {
		return nil
	}

	_, err := h.db.Exec(ctx, `
		UPDATE accounts SET status = 'past_due', updated_at = NOW()
		WHERE stripe_customer_id = $1
	`, inv.Customer.ID)
	return err
}

func (h *WebhookHandler) handleSubscriptionDeleted(ctx context.Context, event stripeLib.Event) error {
	var sub stripeLib.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return err
	}

	_, err := h.db.Exec(ctx, `
		UPDATE accounts SET status = 'cancelled', updated_at = $1
		WHERE stripe_subscription_id = $2
	`, time.Now(), sub.ID)
	return err
}

func (h *WebhookHandler) handleSubscriptionUpdated(ctx context.Context, event stripeLib.Event) error {
	var sub stripeLib.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return err
	}

	plan := "monthly"
	if p, ok := sub.Metadata["plan"]; ok {
		plan = p
	}

	_, err := h.db.Exec(ctx, `
		UPDATE accounts SET plan = $1, updated_at = NOW()
		WHERE stripe_subscription_id = $2
	`, plan, sub.ID)
	return err
}
