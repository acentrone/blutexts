package ghl

import (
	"context"
	"fmt"
	"time"

	"github.com/bluesend/api/internal/models"
	"github.com/bluesend/api/internal/services/crypto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Provisioner handles GHL OAuth and connection persistence.
type Provisioner struct {
	db     *pgxpool.Pool
	client *Client
	appURL string
	enc    *crypto.Encryptor
}

func NewProvisioner(db *pgxpool.Pool, client *Client, appURL string, enc *crypto.Encryptor) *Provisioner {
	return &Provisioner{
		db:     db,
		client: client,
		appURL: appURL,
		enc:    enc,
	}
}

// GenerateOAuthURL returns the GHL OAuth authorization URL for a given account.
func (p *Provisioner) GenerateOAuthURL(accountID uuid.UUID) string {
	redirectURI := p.appURL + "/api/oauth/callback"
	return fmt.Sprintf(
		"%s/oauth/chooselocation?response_type=code&redirect_uri=%s&client_id=%s&scope=%s&state=%s",
		AuthURL,
		redirectURI,
		p.client.clientID,
		"contacts.readonly contacts.write conversations.readonly conversations.write conversations/message.readonly conversations/message.write locations.readonly",
		accountID.String(),
	)
}

// CompleteOAuth exchanges the authorization code and persists the connection.
// Inbound messages flow through the conversation provider's delivery URL
// (configured in the GHL marketplace app settings), not through registered webhooks.
func (p *Provisioner) CompleteOAuth(ctx context.Context, accountID uuid.UUID, code, redirectURI string) (*models.GHLConnection, error) {
	tr, err := p.client.ExchangeCode(ctx, code, redirectURI)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	conn := &models.GHLConnection{
		ID:             uuid.New(),
		AccountID:      accountID,
		LocationID:     tr.LocationID,
		AccessToken:    tr.AccessToken,
		RefreshToken:   tr.RefreshToken,
		TokenExpiresAt: time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second),
		Connected:      true,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Encrypt OAuth tokens at rest. A read-only DB compromise should not
	// hand an attacker control of every customer's GHL location.
	encAccess, err := p.enc.Encrypt(conn.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("encrypt access_token: %w", err)
	}
	encRefresh, err := p.enc.Encrypt(conn.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("encrypt refresh_token: %w", err)
	}

	_, err = p.db.Exec(ctx, `
		INSERT INTO ghl_connections (id, account_id, location_id, access_token, refresh_token,
		                              token_expires_at, connected, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, true, $7, $8)
		ON CONFLICT (account_id) DO UPDATE
		SET location_id = EXCLUDED.location_id,
		    access_token = EXCLUDED.access_token,
		    refresh_token = EXCLUDED.refresh_token,
		    token_expires_at = EXCLUDED.token_expires_at,
		    connected = true,
		    updated_at = NOW()
	`, conn.ID, conn.AccountID, conn.LocationID, encAccess, encRefresh,
		conn.TokenExpiresAt, conn.CreatedAt, conn.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("save ghl connection: %w", err)
	}

	// Update account's GHL location reference
	_, _ = p.db.Exec(ctx, `
		UPDATE accounts SET ghl_location_id = $1 WHERE id = $2
	`, conn.LocationID, accountID)

	return conn, nil
}
