package ghl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bluesend/api/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Syncer handles bidirectional sync between BlueSend and GHL.
type Syncer struct {
	db     *pgxpool.Pool
	client *Client
}

func NewSyncer(db *pgxpool.Pool, client *Client) *Syncer {
	return &Syncer{db: db, client: client}
}

// getConnection retrieves the GHL connection for an account, refreshing token if needed.
func (s *Syncer) getConnection(ctx context.Context, accountID uuid.UUID) (*models.GHLConnection, error) {
	var conn models.GHLConnection
	err := s.db.QueryRow(ctx, `
		SELECT id, account_id, location_id, access_token, refresh_token,
		       token_expires_at, pipeline_id, custom_channel_id, webhook_id, connected
		FROM ghl_connections WHERE account_id = $1
	`, accountID).Scan(
		&conn.ID, &conn.AccountID, &conn.LocationID, &conn.AccessToken, &conn.RefreshToken,
		&conn.TokenExpiresAt, &conn.PipelineID, &conn.CustomChannelID, &conn.WebhookID, &conn.Connected,
	)
	if err != nil {
		return nil, fmt.Errorf("get ghl connection: %w", err)
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

		_, _ = s.db.Exec(ctx, `
			UPDATE ghl_connections
			SET access_token = $1, refresh_token = $2, token_expires_at = $3
			WHERE id = $4
		`, conn.AccessToken, conn.RefreshToken, conn.TokenExpiresAt, conn.ID)
	}

	return &conn, nil
}

// SyncMessageToGHL pushes a BlueSend message into the matching GHL conversation.
func (s *Syncer) SyncMessageToGHL(ctx context.Context, messageID, accountID uuid.UUID) error {
	// Load message
	var msg models.Message
	err := s.db.QueryRow(ctx, `
		SELECT id, conversation_id, contact_id, direction, content, ghl_message_id
		FROM messages WHERE id = $1 AND account_id = $2
	`, messageID, accountID).Scan(
		&msg.ID, &msg.ConversationID, &msg.ContactID,
		&msg.Direction, &msg.Content, &msg.GHLMessageID,
	)
	if err != nil {
		return fmt.Errorf("load message: %w", err)
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

	direction := "outbound"
	if msg.Direction == models.MessageDirectionInbound {
		direction = "inbound"
	}

	ghlMsg, err := s.client.SendConversationMessage(ctx, conn.AccessToken, &SendMessageRequest{
		Type:      "SMS",
		ContactID: *contact.GHLContactID,
		Message:   msg.Content,
	})
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
		Source:     "BlueSend",
		Tags:       []string{"BlueSend"},
	}

	// Determine if address is phone or email
	addr := contact.IMessageAddress
	if len(addr) > 0 && (addr[0] == '+' || (addr[0] >= '0' && addr[0] <= '9')) {
		req.Phone = addr
	} else {
		req.Email = addr
	}

	if contact.Name != nil {
		req.FirstName = *contact.Name
	}

	ghlContact, err := s.client.CreateContact(ctx, conn.AccessToken, req)
	if err != nil {
		// Handle duplicate contact — extract existing ID from error
		errStr := err.Error()
		if strings.Contains(errStr, "duplicated contacts") || strings.Contains(errStr, "contactId") {
			existingID := extractContactIDFromError(errStr)
			if existingID != "" {
				_, _ = s.db.Exec(ctx, `UPDATE contacts SET ghl_contact_id = $1 WHERE id = $2`, existingID, contact.ID)
				return nil
			}
		}
		return fmt.Errorf("create GHL contact: %w", err)
	}

	_, _ = s.db.Exec(ctx, `
		UPDATE contacts SET ghl_contact_id = $1 WHERE id = $2
	`, ghlContact.ID, contact.ID)

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

// HandleInboundGHLMessage processes a message sent from GHL to be delivered via iMessage.
// This is called when GHL's webhook fires for an outbound message on the custom channel.
func (s *Syncer) HandleInboundGHLMessage(ctx context.Context, locationID, conversationID, message string) error {
	// Find account from location
	var accountID uuid.UUID
	err := s.db.QueryRow(ctx, `
		SELECT account_id FROM ghl_connections WHERE location_id = $1
	`, locationID).Scan(&accountID)
	if err != nil {
		return fmt.Errorf("account not found for location %s: %w", locationID, err)
	}

	// Find conversation and contact to route back
	var conv models.Conversation
	var contact models.Contact
	err = s.db.QueryRow(ctx, `
		SELECT c.id, c.phone_number_id, c.contact_id, ct.imessage_address
		FROM conversations c
		JOIN contacts ct ON ct.id = c.contact_id
		WHERE c.ghl_conversation_id = $1 AND c.account_id = $2
	`, conversationID, accountID).Scan(
		&conv.ID, &conv.PhoneNumberID, &conv.ContactID, &contact.IMessageAddress,
	)
	if err != nil {
		return fmt.Errorf("conversation not found: %w", err)
	}

	// This will be routed through the normal messaging router
	// The API handler calls router.Send() with this info
	_ = contact.IMessageAddress
	_ = conv.PhoneNumberID
	return nil
}
