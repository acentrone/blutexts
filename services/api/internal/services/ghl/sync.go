package ghl

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bluesend/api/internal/models"
	"github.com/bluesend/api/internal/services/crypto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Syncer handles bidirectional sync between BlueSend and GHL.
type Syncer struct {
	db     *pgxpool.Pool
	client *Client
	enc    *crypto.Encryptor
}

func NewSyncer(db *pgxpool.Pool, client *Client, enc *crypto.Encryptor) *Syncer {
	return &Syncer{db: db, client: client, enc: enc}
}

// getConnection retrieves the GHL connection for an account, refreshing token if needed.
// OAuth tokens are stored encrypted at rest via crypto.Encryptor (envelope-encrypted
// AES-256-GCM); we decrypt on read and re-encrypt before any UPDATE.
func (s *Syncer) getConnection(ctx context.Context, accountID uuid.UUID) (*models.GHLConnection, error) {
	var conn models.GHLConnection
	var storedAccess, storedRefresh string
	err := s.db.QueryRow(ctx, `
		SELECT id, account_id, location_id, access_token, refresh_token,
		       token_expires_at, pipeline_id, custom_channel_id, webhook_id, connected
		FROM ghl_connections WHERE account_id = $1
	`, accountID).Scan(
		&conn.ID, &conn.AccountID, &conn.LocationID, &storedAccess, &storedRefresh,
		&conn.TokenExpiresAt, &conn.PipelineID, &conn.CustomChannelID, &conn.WebhookID, &conn.Connected,
	)
	if err != nil {
		return nil, fmt.Errorf("get ghl connection: %w", err)
	}
	if conn.AccessToken, err = s.enc.Decrypt(storedAccess); err != nil {
		return nil, fmt.Errorf("decrypt access_token: %w", err)
	}
	if conn.RefreshToken, err = s.enc.Decrypt(storedRefresh); err != nil {
		return nil, fmt.Errorf("decrypt refresh_token: %w", err)
	}

	// Refresh token if expiring within 5 minutes
	if time.Until(conn.TokenExpiresAt) < 5*time.Minute {
		tr, err := s.client.RefreshAccessToken(ctx, conn.RefreshToken)
		if err != nil {
			return nil, fmt.Errorf("refresh GHL token: %w", err)
		}
		conn.AccessToken = tr.AccessToken
		conn.RefreshToken = tr.RefreshToken
		conn.TokenExpiresAt = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)

		encAccess, _ := s.enc.Encrypt(conn.AccessToken)
		encRefresh, _ := s.enc.Encrypt(conn.RefreshToken)
		_, _ = s.db.Exec(ctx, `
			UPDATE ghl_connections
			SET access_token = $1, refresh_token = $2, token_expires_at = $3
			WHERE id = $4
		`, encAccess, encRefresh, conn.TokenExpiresAt, conn.ID)
	}

	return &conn, nil
}

// SyncMessageToGHL pushes a BlueSend message into the matching GHL conversation.
func (s *Syncer) SyncMessageToGHL(ctx context.Context, messageID, accountID uuid.UUID) error {
	// Load message
	var msg models.Message
	var attachmentsJSON []byte
	err := s.db.QueryRow(ctx, `
		SELECT id, conversation_id, contact_id, direction, content, attachments, ghl_message_id
		FROM messages WHERE id = $1 AND account_id = $2
	`, messageID, accountID).Scan(
		&msg.ID, &msg.ConversationID, &msg.ContactID,
		&msg.Direction, &msg.Content, &attachmentsJSON, &msg.GHLMessageID,
	)
	if err != nil {
		return fmt.Errorf("load message: %w", err)
	}
	if len(attachmentsJSON) > 0 {
		_ = json.Unmarshal(attachmentsJSON, &msg.Attachments)
	}

	// Already synced
	if msg.GHLMessageID != nil {
		return nil
	}

	conn, err := s.getConnection(ctx, accountID)
	if err != nil {
		return err
	}

	// Ensure contact is synced first
	var contact models.Contact
	err = s.db.QueryRow(ctx, `
		SELECT id, imessage_address, name, ghl_contact_id FROM contacts WHERE id = $1
	`, msg.ContactID).Scan(&contact.ID, &contact.IMessageAddress, &contact.Name, &contact.GHLContactID)
	if err != nil {
		return fmt.Errorf("load contact: %w", err)
	}

	if contact.GHLContactID == nil {
		if err := s.SyncContactToGHL(ctx, contact.ID, accountID); err != nil {
			return fmt.Errorf("sync contact: %w", err)
		}
		// Reload
		_ = s.db.QueryRow(ctx, `SELECT ghl_contact_id FROM contacts WHERE id = $1`, contact.ID).Scan(&contact.GHLContactID)
	}

	// Get/create GHL conversation
	var conv models.Conversation
	_ = s.db.QueryRow(ctx, `SELECT ghl_conversation_id FROM conversations WHERE id = $1`, msg.ConversationID).
		Scan(&conv.GHLConversationID)

	if conv.GHLConversationID == nil {
		ghlConv, err := s.client.GetOrCreateConversation(ctx, conn.AccessToken, conn.LocationID, *contact.GHLContactID)
		if err != nil {
			return fmt.Errorf("get/create conversation: %w", err)
		}
		_, _ = s.db.Exec(ctx, `UPDATE conversations SET ghl_conversation_id = $1 WHERE id = $2`,
			ghlConv.ID, msg.ConversationID)
		conv.GHLConversationID = &ghlConv.ID
	}

	providerID := os.Getenv("GHL_CONVERSATION_PROVIDER_ID")
	if providerID == "" {
		providerID = "69d8f7bc2b4bdc470dd45f5b"
	}

	var attachmentURLs []string
	for _, a := range msg.Attachments {
		attachmentURLs = append(attachmentURLs, a.URL)
	}

	msgReq := &SendMessageRequest{
		Type:                   "Custom",
		ContactID:              *contact.GHLContactID,
		Message:                msg.Content,
		ConversationProviderId: providerID,
		Attachments:            attachmentURLs,
	}

	// Inbound messages use the inbound endpoint (no delivery trigger).
	// Outbound messages use the regular endpoint — GHL fires a delivery webhook back,
	// but our webhook handler filters `source=api` events to prevent the loop.
	var ghlMsg *SendMessageResponse
	if msg.Direction == models.MessageDirectionInbound {
		ghlMsg, err = s.client.LogInboundMessage(ctx, conn.AccessToken, msgReq)
	} else {
		ghlMsg, err = s.client.SendConversationMessage(ctx, conn.AccessToken, msgReq)
	}
	if err != nil {
		return fmt.Errorf("send to GHL: %w", err)
	}

	now := time.Now()
	_, _ = s.db.Exec(ctx, `
		UPDATE messages SET ghl_message_id = $1, ghl_synced_at = $2 WHERE id = $3
	`, ghlMsg.MessageID, now, msg.ID)

	_, _ = s.db.Exec(ctx, `
		UPDATE ghl_connections SET last_synced_at = $1 WHERE account_id = $2
	`, now, accountID)

	return nil
}

// SyncContactToGHL creates or updates a contact in GHL.
func (s *Syncer) SyncContactToGHL(ctx context.Context, contactID, accountID uuid.UUID) error {
	var contact models.Contact
	err := s.db.QueryRow(ctx, `
		SELECT id, imessage_address, name, ghl_contact_id FROM contacts WHERE id = $1 AND account_id = $2
	`, contactID, accountID).Scan(&contact.ID, &contact.IMessageAddress, &contact.Name, &contact.GHLContactID)
	if err != nil {
		return fmt.Errorf("load contact: %w", err)
	}

	if contact.GHLContactID != nil {
		return nil // already synced
	}

	conn, err := s.getConnection(ctx, accountID)
	if err != nil {
		return err
	}

	req := &CreateContactRequest{
		LocationID: conn.LocationID,
		Source:     "BluTexts iMessage",
		Tags:       []string{"BluTexts", "iMessage"},
	}

	// Determine if address is phone or email
	addr := contact.IMessageAddress
	if len(addr) > 0 && (addr[0] == '+' || (addr[0] >= '0' && addr[0] <= '9')) {
		req.Phone = addr
	} else {
		req.Email = addr
	}

	if contact.Name != nil && *contact.Name != "" {
		req.FirstName = *contact.Name
	}

	ghlContact, err := s.client.CreateContact(ctx, conn.AccessToken, req)
	if err != nil {
		// Handle duplicate contact — GHL returns the existing contact ID in the error
		errStr := err.Error()
		if strings.Contains(errStr, "duplicated contacts") || strings.Contains(errStr, "contactId") {
			existingID := extractContactIDFromError(errStr)
			if existingID != "" {
				// Fetch the existing contact to pull its name back into BluTexts
				existing, fetchErr := s.client.GetContact(ctx, conn.AccessToken, existingID)
				if fetchErr == nil && existing != nil {
					name := strings.TrimSpace(existing.FirstName + " " + existing.LastName)
					if name != "" {
						_, _ = s.db.Exec(ctx, `
							UPDATE contacts SET ghl_contact_id = $1, name = $2 WHERE id = $3
						`, existingID, name, contact.ID)
						return nil
					}
				}
				_, _ = s.db.Exec(ctx, `UPDATE contacts SET ghl_contact_id = $1 WHERE id = $2`, existingID, contact.ID)
				return nil
			}
		}
		return fmt.Errorf("create GHL contact: %w", err)
	}

	// Save the GHL contact ID and name (if GHL generated one)
	name := strings.TrimSpace(ghlContact.FirstName + " " + ghlContact.LastName)
	if name != "" {
		_, _ = s.db.Exec(ctx, `
			UPDATE contacts SET ghl_contact_id = $1, name = $2 WHERE id = $3
		`, ghlContact.ID, name, contact.ID)
	} else {
		_, _ = s.db.Exec(ctx, `
			UPDATE contacts SET ghl_contact_id = $1 WHERE id = $2
		`, ghlContact.ID, contact.ID)
	}

	return nil
}

// extractContactIDFromError parses the existing contact ID from a GHL duplicate error.
func extractContactIDFromError(errStr string) string {
	// Error format: ...\"contactId\":\"arbYr9z6c29sSWEYMyH2\"...
	marker := "\"contactId\":\""
	idx := strings.Index(errStr, marker)
	if idx < 0 {
		return ""
	}
	start := idx + len(marker)
	end := strings.Index(errStr[start:], "\"")
	if end < 0 {
		return ""
	}
	return errStr[start : start+end]
}

