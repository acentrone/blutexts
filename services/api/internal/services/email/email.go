package email

import (
	"fmt"
	"log"

	"github.com/resend/resend-go/v2"
)

// Service wraps the Resend API for transactional emails.
// When the API key is empty the service logs emails to stdout instead of
// sending them — useful for local development without a Resend account.
type Service struct {
	client  *resend.Client
	from    string
	appURL  string
	enabled bool
}

func New(apiKey, fromAddress, appURL string) *Service {
	if apiKey == "" {
		return &Service{enabled: false, from: fromAddress, appURL: appURL}
	}
	return &Service{
		client:  resend.NewClient(apiKey),
		from:    fromAddress,
		appURL:  appURL,
		enabled: true,
	}
}

func (s *Service) Enabled() bool { return s.enabled }

// SendPasswordReset sends a password-reset email with a tokenized link.
func (s *Service) SendPasswordReset(toEmail, token string) error {
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.appURL, token)

	subject := "Reset your BluTexts password"
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; max-width: 480px; margin: 0 auto; padding: 40px 20px; color: #1a1a1a;">
  <div style="text-align: center; margin-bottom: 32px;">
    <div style="display: inline-block; width: 40px; height: 40px; background: #007AFF; border-radius: 12px; line-height: 40px; color: white; font-size: 20px;">&#x1F4AC;</div>
    <h2 style="margin: 12px 0 0; font-size: 20px;">BluTexts</h2>
  </div>

  <p style="font-size: 15px; line-height: 1.6;">
    We received a request to reset the password for your BluTexts account.
    Click the button below to choose a new password. This link expires in 1 hour.
  </p>

  <div style="text-align: center; margin: 32px 0;">
    <a href="%s" style="display: inline-block; background: #007AFF; color: white; font-weight: 600; padding: 12px 32px; border-radius: 10px; text-decoration: none; font-size: 15px;">
      Reset password
    </a>
  </div>

  <p style="font-size: 13px; color: #888; line-height: 1.5;">
    If you didn't request this, you can safely ignore this email — your password won't change.
  </p>

  <hr style="border: none; border-top: 1px solid #eee; margin: 32px 0;" />
  <p style="font-size: 12px; color: #aaa; text-align: center;">
    BluTexts &mdash; iMessage CRM for Go High Level<br />
    <a href="%s" style="color: #aaa;">%s</a>
  </p>
</body>
</html>`, resetURL, s.appURL, s.appURL)

	if !s.enabled {
		log.Printf("[email-dev] Password reset for %s → %s", toEmail, resetURL)
		return nil
	}

	params := &resend.SendEmailRequest{
		From:    s.from,
		To:      []string{toEmail},
		Subject: subject,
		Html:    html,
	}

	sent, err := s.client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("resend send: %w", err)
	}
	log.Printf("Password reset email sent to %s (resend_id=%s)", toEmail, sent.Id)
	return nil
}

// SendWelcome is the post-payment "you're in, here's how to install" email.
// Sent from the Stripe webhook's onAccountActivated callback once
// checkout.session.completed lands. Includes the Mac DMG download link
// and a one-paragraph "what to expect" so the customer isn't dropped into
// an empty dashboard with no instructions.
//
// dmgURL is fetched at call-site from agent_releases (active=true) so the
// email always points at the latest signed build.
func (s *Service) SendWelcome(toEmail, firstName, dmgURL string) error {
	subject := "Welcome to BluTexts — your dedicated number is being provisioned"

	greeting := "there"
	if firstName != "" {
		greeting = firstName
	}

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; max-width: 520px; margin: 0 auto; padding: 40px 20px; color: #1a1a1a;">
  <div style="text-align: center; margin-bottom: 32px;">
    <div style="display: inline-block; width: 40px; height: 40px; background: #2E6FE0; border-radius: 12px; line-height: 40px; color: white; font-size: 20px;">&#x1F4AC;</div>
    <h2 style="margin: 12px 0 0; font-size: 20px;">BluTexts</h2>
  </div>

  <h1 style="font-size: 22px; line-height: 1.3; margin: 0 0 16px;">
    Hey %s — you&rsquo;re in.
  </h1>

  <p style="font-size: 15px; line-height: 1.6; margin: 0 0 16px;">
    Payment received. Here&rsquo;s what happens next:
  </p>

  <ol style="font-size: 15px; line-height: 1.7; padding-left: 22px; margin: 0 0 24px;">
    <li><strong>We&rsquo;re provisioning your dedicated iMessage number.</strong>
      Apple requires a manual identity step on our end — typically done within
      a few business hours. You&rsquo;ll get a follow-up email when your number
      is live.</li>
    <li><strong>Install the BluTexts Mac app</strong> on the machine you want
      to send from (download below). You only need this on one Mac per
      number — your team uses the web app from anywhere.</li>
    <li><strong>Sign in to the dashboard</strong> at
      <a href="%s" style="color: #2E6FE0;">%s</a> to invite teammates and
      connect Go High Level.</li>
  </ol>

  <div style="text-align: center; margin: 28px 0;">
    <a href="%s" style="display: inline-block; background: #2E6FE0; color: white; font-weight: 600; padding: 14px 32px; border-radius: 10px; text-decoration: none; font-size: 15px;">
      Download the Mac app
    </a>
  </div>

  <p style="font-size: 13px; color: #666; line-height: 1.5; margin: 0 0 8px;">
    <strong>About the 50-conversations-per-day limit:</strong> Apple caps every
    iMessage line at 50 NEW conversations per day to keep numbers in good
    standing. Replies and ongoing threads are unlimited. We&rsquo;ll surface
    your daily count in the dashboard.
  </p>

  <p style="font-size: 14px; line-height: 1.6; margin: 24px 0 0;">
    Questions? Just reply to this email — it goes straight to the founders.
  </p>

  <hr style="border: none; border-top: 1px solid #eee; margin: 32px 0 16px;" />
  <p style="font-size: 12px; color: #aaa; text-align: center;">
    BluTexts &mdash; iMessage for business<br />
    <a href="%s" style="color: #aaa;">%s</a>
  </p>
</body>
</html>`, greeting, s.appURL, s.appURL, dmgURL, s.appURL, s.appURL)

	if !s.enabled {
		log.Printf("[email-dev] Welcome for %s → DMG: %s", toEmail, dmgURL)
		return nil
	}

	params := &resend.SendEmailRequest{
		From:    s.from,
		To:      []string{toEmail},
		Subject: subject,
		Html:    html,
	}

	sent, err := s.client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("resend send: %w", err)
	}
	log.Printf("Welcome email sent to %s (resend_id=%s)", toEmail, sent.Id)
	return nil
}

// SendProvisioningAlert pages the founder when a new paid customer needs a
// number assigned. Number provisioning is a manual Apple-identity step today
// — automation is later — so until then, every paid checkout fires this so
// nobody is left waiting in the dashboard.
//
// Plain-text format on purpose: ops emails should be skimmable in a list view.
func (s *Service) SendProvisioningAlert(toEmail, customerEmail, customerName, company, areaCode string, accountID string) error {
	subject := fmt.Sprintf("[BluTexts] New paid customer needs a number: %s (%s)", company, customerEmail)

	body := fmt.Sprintf(`A new paid customer just completed checkout and needs a dedicated iMessage number assigned.

Customer:    %s
Email:       %s
Company:     %s
Area code:   %s
Account ID:  %s

To assign a number from the pool:
  1. Open the admin console: %s/admin/customers
  2. Find the account by email
  3. Click "Assign number" → pick from available

Until a number is assigned, the customer sees the dashboard but cannot send.
SLA target: assign within 4 business hours.
`, customerName, customerEmail, company, areaCode, accountID, s.appURL)

	if !s.enabled {
		log.Printf("[email-dev] Provisioning alert for %s (%s)", customerEmail, accountID)
		return nil
	}

	params := &resend.SendEmailRequest{
		From:    s.from,
		To:      []string{toEmail},
		Subject: subject,
		Text:    body,
	}

	sent, err := s.client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("resend send: %w", err)
	}
	log.Printf("Provisioning alert sent to %s (resend_id=%s)", toEmail, sent.Id)
	return nil
}

// SendTeamInvite sends an invitation email to join a BluTexts account.
func (s *Service) SendTeamInvite(toEmail, token string) error {
	inviteURL := fmt.Sprintf("%s/accept-invite?token=%s", s.appURL, token)

	subject := "You've been invited to BluTexts"
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; max-width: 480px; margin: 0 auto; padding: 40px 20px; color: #1a1a1a;">
  <div style="text-align: center; margin-bottom: 32px;">
    <div style="display: inline-block; width: 40px; height: 40px; background: #007AFF; border-radius: 12px; line-height: 40px; color: white; font-size: 20px;">&#x1F4AC;</div>
    <h2 style="margin: 12px 0 0; font-size: 20px;">BluTexts</h2>
  </div>

  <p style="font-size: 15px; line-height: 1.6;">
    You've been invited to join a team on BluTexts — the iMessage CRM platform.
    Click the button below to create your account and start collaborating.
    This link expires in 7 days.
  </p>

  <div style="text-align: center; margin: 32px 0;">
    <a href="%s" style="display: inline-block; background: #007AFF; color: white; font-weight: 600; padding: 12px 32px; border-radius: 10px; text-decoration: none; font-size: 15px;">
      Accept invitation
    </a>
  </div>

  <p style="font-size: 13px; color: #888; line-height: 1.5;">
    If you weren't expecting this invitation, you can safely ignore this email.
  </p>

  <hr style="border: none; border-top: 1px solid #eee; margin: 32px 0;" />
  <p style="font-size: 12px; color: #aaa; text-align: center;">
    BluTexts &mdash; iMessage CRM for Go High Level<br />
    <a href="%s" style="color: #aaa;">%s</a>
  </p>
</body>
</html>`, inviteURL, s.appURL, s.appURL)

	if !s.enabled {
		log.Printf("[email-dev] Team invite for %s → %s", toEmail, inviteURL)
		return nil
	}

	params := &resend.SendEmailRequest{
		From:    s.from,
		To:      []string{toEmail},
		Subject: subject,
		Html:    html,
	}

	sent, err := s.client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("resend send: %w", err)
	}
	log.Printf("Team invite email sent to %s (resend_id=%s)", toEmail, sent.Id)
	return nil
}
