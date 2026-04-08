package ghl

import (
	"context"
	"fmt"
	"time"

	"github.com/bluesend/api/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Provisioner handles creating a full GHL sub-account integration after signup.
type Provisioner struct {
	db         *pgxpool.Pool
	client     *Client
	appURL     string
	webhookURL string
}

func NewProvisioner(db *pgxpool.Pool, client *Client, appURL string) *Provisioner {
	return &Provisioner{
		db:         db,
		client:     client,
		appURL:     appURL,
		webhookURL: appURL + "/api/webhooks/inbound",
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
		"contacts.readonly contacts.write conversations.readonly conversations.write locations.readonly",
		accountID.String(),
	)
}

// CompleteOAuth exchanges the authorization code and persists the connection.
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
		Connected:      false, // will be set to true after webhook registration
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	_, err = p.db.Exec(ctx, `
		INSERT INTO ghl_connections (id, account_id, location_id, access_token, refresh_token,
		                              token_expires_at, connected, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, false, $7, $8)
		ON CONFLICT (account_id) DO UPDATE
		SET location_id = EXCLUDED.location_id,
		    access_token = EXCLUDED.access_token,
		    refresh_token = EXCLUDED.refresh_token,
		    token_expires_at = EXCLUDED.token_expires_at,
		    updated_at = NOW()
	`, conn.ID, conn.AccountID, conn.LocationID, conn.AccessToken, conn.RefreshToken,
		conn.TokenExpiresAt, conn.CreatedAt, conn.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("save ghl connection: %w", err)
	}

	// Also update accounts table
	_, _ = p.db.Exec(ctx, `
		UPDATE accounts SET ghl_location_id = $1 WHERE id = $2
	`, conn.LocationID, accountID)

	// Register webhook asynchronously (non-blocking for OAuth callback)
	go func() {
		bgCtx := context.Background()
		if err := p.RegisterWebhook(bgCtx, accountID); err != nil {
			fmt.Printf("ERROR registering GHL webhook for account %s: %v\n", accountID, err)
		}
	}()

	return conn, nil
}

// RegisterWebhook registers our webhook with GHL for this location.
func (p *Provisioner) RegisterWebhook(ctx context.Context, accountID uuid.UUID) error {
	syncer := NewSyncer(p.db, p.client)
	conn, err := syncer.getConnection(ctx, accountID)
	if err != nil {
		return err
	}

	webhook, err := p.client.RegisterWebhook(ctx, conn.AccessToken, conn.LocationID, p.webhookURL)
	if err != nil {
		return fmt.Errorf("register webhook: %w", err)
	}

	_, err = p.db.Exec(ctx, `
		UPDATE ghl_connections
		SET webhook_id = $1, connected = true, updated_at = NOW()
		WHERE id = $2
	`, webhook.ID, conn.ID)
	return err
}
