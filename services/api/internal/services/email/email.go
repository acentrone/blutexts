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
