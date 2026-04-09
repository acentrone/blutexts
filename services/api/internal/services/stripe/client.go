package stripe

import (
	"context"
	"fmt"

	stripeLib "github.com/stripe/stripe-go/v78"
	portalsession "github.com/stripe/stripe-go/v78/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v78/checkout/session"
	"github.com/stripe/stripe-go/v78/customer"
	"github.com/stripe/stripe-go/v78/subscription"
)

// Plans maps our internal plan names to Stripe price IDs.
type Plans struct {
	Setup   string // one-time $399
	Monthly string // $199/mo recurring
	Annual  string // $2600/yr recurring
}

// BillingService wraps Stripe SDK operations.
type BillingService struct {
	plans  Plans
	appURL string
}

func NewBillingService(secretKey string, plans Plans, appURL string) *BillingService {
	stripeLib.Key = secretKey
	return &BillingService{plans: plans, appURL: appURL}
}

// ============================================================
// Checkout
// ============================================================

type CreateCheckoutParams struct {
	Email     string
	FirstName string
	LastName  string
	Company   string
	Plan      string // "monthly" or "annual"
}

type CheckoutResult struct {
	ClientSecret string
	CustomerID   string
}

// CreateCheckoutSession creates a Stripe Checkout session that collects
// the one-time $399 setup fee plus the recurring subscription (monthly or annual).
func (b *BillingService) CreateCheckoutSession(ctx context.Context, params CreateCheckoutParams) (*CheckoutResult, error) {
	cust, err := customer.New(&stripeLib.CustomerParams{
		Email: stripeLib.String(params.Email),
		Name:  stripeLib.String(params.FirstName + " " + params.LastName),
		Metadata: map[string]string{
			"company":    params.Company,
			"first_name": params.FirstName,
			"last_name":  params.LastName,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create stripe customer: %w", err)
	}

	recurringPriceID := b.plans.Monthly
	if params.Plan == "annual" {
		recurringPriceID = b.plans.Annual
	}

	sessionParams := &stripeLib.CheckoutSessionParams{
		Customer:   stripeLib.String(cust.ID),
		Mode:       stripeLib.String("subscription"),
		SuccessURL: stripeLib.String(b.appURL + "/onboarding/success?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:  stripeLib.String(b.appURL + "/pricing?checkout=cancelled"),
		LineItems: []*stripeLib.CheckoutSessionLineItemParams{
			{
				Price:    stripeLib.String(b.plans.Setup),
				Quantity: stripeLib.Int64(1),
			},
			{
				Price:    stripeLib.String(recurringPriceID),
				Quantity: stripeLib.Int64(1),
			},
		},
		SubscriptionData: &stripeLib.CheckoutSessionSubscriptionDataParams{
			Metadata: map[string]string{
				"plan": params.Plan,
			},
		},
		PaymentMethodTypes:  stripeLib.StringSlice([]string{"card"}),
		AllowPromotionCodes: stripeLib.Bool(true),
	}

	sess, err := checkoutsession.New(sessionParams)
	if err != nil {
		return nil, fmt.Errorf("create checkout session: %w", err)
	}

	return &CheckoutResult{
		ClientSecret: sess.ClientSecret,
		CustomerID:   cust.ID,
	}, nil
}

// ============================================================
// Billing Portal
// ============================================================

func (b *BillingService) CreatePortalSession(ctx context.Context, customerID, returnURL string) (string, error) {
	sess, err := portalsession.New(&stripeLib.BillingPortalSessionParams{
		Customer:  stripeLib.String(customerID),
		ReturnURL: stripeLib.String(returnURL),
	})
	if err != nil {
		return "", fmt.Errorf("create portal session: %w", err)
	}
	return sess.URL, nil
}

// ============================================================
// Subscription Management
// ============================================================

func (b *BillingService) CancelSubscription(ctx context.Context, subscriptionID string) error {
	_, err := subscription.Update(subscriptionID, &stripeLib.SubscriptionParams{
		CancelAtPeriodEnd: stripeLib.Bool(true),
	})
	return err
}

func (b *BillingService) GetSubscription(ctx context.Context, subscriptionID string) (*stripeLib.Subscription, error) {
	return subscription.Get(subscriptionID, nil)
}
