// Package voice generates Agora RTC tokens for the FaceTime Audio call bridge.
//
// Each call creates a uniquely named Agora channel with two participants:
//   - the agent's browser (Chrome extension, real mic/speakers)
//   - the hosted Mac's hidden WebView (BlackHole virtual audio ↔ FaceTime.app)
//
// Tokens are short-lived (1h) and role-scoped: publisher for both participants
// since each must publish their own audio track.
package voice

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/AgoraIO-Community/go-tokenbuilder/rtctokenbuilder2"
)

const (
	// TokenTTLSeconds is how long an issued Agora RTC token remains valid.
	TokenTTLSeconds = 3600

	// PrivilegeTTLSeconds matches TokenTTLSeconds; once the token expires,
	// the user is kicked from the channel.
	PrivilegeTTLSeconds = 3600
)

// Service builds Agora tokens for the two participants of a call.
type Service struct {
	AppID          string
	AppCertificate string
}

// New returns nil when Agora credentials are not configured (calling feature disabled).
func New(appID, appCertificate string) *Service {
	if appID == "" || appCertificate == "" {
		return nil
	}
	return &Service{AppID: appID, AppCertificate: appCertificate}
}

// Enabled reports whether calling is configured.
func (s *Service) Enabled() bool {
	return s != nil && s.AppID != "" && s.AppCertificate != ""
}

// NewChannelName returns a cryptographically random channel name.
// Agora channel names are limited to 64 ASCII chars.
func NewChannelName() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "bt-" + hex.EncodeToString(b[:]), nil
}

// BuildToken issues an RTC token granting publish+subscribe in the given channel
// for the given uid. Each call participant (agent, bridge) gets a distinct uid.
func (s *Service) BuildToken(channel string, uid uint32) (string, error) {
	if !s.Enabled() {
		return "", errors.New("voice service not configured")
	}
	tok, err := rtctokenbuilder2.BuildTokenWithUid(
		s.AppID, s.AppCertificate, channel, uid,
		rtctokenbuilder2.RolePublisher,
		TokenTTLSeconds, PrivilegeTTLSeconds,
	)
	if err != nil {
		return "", fmt.Errorf("build agora token: %w", err)
	}
	return tok, nil
}
