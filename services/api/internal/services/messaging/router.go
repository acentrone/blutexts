package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bluesend/api/internal/models"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const (
	TaskSendMessage  = "message:send"
	TaskSyncToGHL    = "ghl:sync_message"
	TaskSyncContact  = "ghl:sync_contact"
)

// Router handles message send requests: validates rate limits, persists to DB,
// queues delivery to device, and queues GHL sync.
type Router struct {
	db          *pgxpool.Pool
	redis       *redis.Client
	rateLimiter *RateLimiter
	asynq       *asynq.Client
	hub         DeviceHub // interface to push to device WebSocket
}

// DeviceHub is the interface for pushing send jobs to physical devices.
type DeviceHub interface {
	SendToDevice(deviceID uuid.UUID, event models.DeviceWSEvent) error
}

func NewRouter(db *pgxpool.Pool, rdb *redis.Client, hub DeviceHub, asynqClient *asynq.Client) *Router {
	return &Router{
		db:          db,
		redis:       rdb,
		rateLimiter: NewRateLimiter(db, rdb),
		asynq:       asynqClient,
		hub:         hub,
	}
}

// Send validates and enqueues an outbound message.
func (r *Router) Send(ctx context.Context, req *models.SendMessageRequest, accountID uuid.UUID) (*models.SendMessageResponse, error) {
	phoneNumberID, err := uuid.Parse(req.PhoneNumberID)
	if err != nil {
		return nil, fmt.Errorf("invalid phone_number_id: %w", err)
	}

	// Verify phone number belongs to this account and is active
	var pn models.PhoneNumber
	err = r.db.QueryRow(ctx, `
		SELECT id, account_id, device_id, number, imessage_address, status, daily_new_contact_limit
		FROM phone_numbers
		WHERE id = $1 AND account_id = $2
	`, phoneNumberID, accountID).Scan(
		&pn.ID, &pn.AccountID, &pn.DeviceID, &pn.Number, &pn.IMessageAddress,
		&pn.Status, &pn.DailyNewContactLimit,
	)
	if err != nil {
		return nil, fmt.Errorf("phone number not found or unauthorized")
	}
	if pn.Status != models.PhoneNumberStatusActive {
		return nil, fmt.Errorf("phone number is not active (status: %s)", pn.Status)
	}
	if pn.DeviceID == nil {
		return nil, fmt.Errorf("phone number has no assigned device")
	}

	// Rate limit check
	check, err := r.rateLimiter.Check(ctx, phoneNumberID, req.ToAddress)
	if err != nil {
		return nil, fmt.Errorf("rate limit check: %w", err)
	}
	if !check.Allowed {
		return &models.SendMessageResponse{
			RateLimited: true,
			RateLimit: &models.RateLimitInfo{
				DailyNewContactsUsed:  check.Used,
				DailyNewContactsLimit: check.Limit,
				IsNewContact:          true,
				ResetsAt:              check.ResetsAt.Format(time.RFC3339),
			},
		}, nil
	}

	// Upsert contact
	contact, err := r.upsertContact(ctx, accountID, phoneNumberID, req.ToAddress)
	if err != nil {
		return nil, fmt.Errorf("upsert contact: %w", err)
	}

	// Upsert conversation
	conv, err := r.upsertConversation(ctx, accountID, phoneNumberID, contact.ID)
	if err != nil {
		return nil, fmt.Errorf("upsert conversation: %w", err)
	}

	// Create message record
	msg := &models.Message{
		ID:             uuid.New(),
		ConversationID: conv.ID,
		AccountID:      accountID,
		PhoneNumberID:  phoneNumberID,
		ContactID:      contact.ID,
		Direction:      models.MessageDirectionOutbound,
		Content:        req.Content,
		Status:         models.MessageStatusPending,
		CreatedAt:      time.Now(),
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO messages (id, conversation_id, account_id, phone_number_id, contact_id, direction, content, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, msg.ID, msg.ConversationID, msg.AccountID, msg.PhoneNumberID, msg.ContactID,
		msg.Direction, msg.Content, msg.Status, msg.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert message: %w", err)
	}

	// Update conversation preview
	_, _ = r.db.Exec(ctx, `
		UPDATE conversations
		SET last_message_at = $1, last_message_preview = $2, message_count = message_count + 1
		WHERE id = $3
	`, time.Now(), truncate(req.Content, 100), conv.ID)

	// Push send job to device via WebSocket hub
	sendPayload := models.DeviceSendPayload{
		MessageID:       msg.ID.String(),
		PhoneNumber:     pn.Number,
		ToAddress:       req.ToAddress,
		Content:         req.Content,
		IMessageAddress: deref(pn.IMessageAddress),
	}
	_ = r.hub.SendToDevice(*pn.DeviceID, models.DeviceWSEvent{
		Type:    models.DeviceEventSendMessage,
		Payload: sendPayload,
	})

	// Queue GHL sync (async, non-blocking)
	payload, _ := json.Marshal(map[string]string{
		"message_id": msg.ID.String(),
		"account_id": accountID.String(),
	})
	task := asynq.NewTask(TaskSyncToGHL, payload, asynq.MaxRetry(3), asynq.Timeout(30*time.Second))
	_, _ = r.asynq.Enqueue(task)

	// Record rate limit usage
	_ = r.rateLimiter.Record(ctx, phoneNumberID, req.ToAddress, check.IsNewContact)

	// Queue GHL contact sync if new
	if check.IsNewContact {
		contactPayload, _ := json.Marshal(map[string]string{
			"contact_id": contact.ID.String(),
			"account_id": accountID.String(),
		})
		contactTask := asynq.NewTask(TaskSyncContact, contactPayload, asynq.MaxRetry(3))
		_, _ = r.asynq.Enqueue(contactTask)
	}

	return &models.SendMessageResponse{
		Message:     msg,
		RateLimited: false,
		RateLimit: &models.RateLimitInfo{
			DailyNewContactsUsed:  check.Used + boolInt(check.IsNewContact),
			DailyNewContactsLimit: check.Limit,
			IsNewContact:          check.IsNewContact,
			ResetsAt:              midnight().Format(time.RFC3339),
		},
	}, nil
}

// HandleInbound processes a message received from a device agent.
func (r *Router) HandleInbound(ctx context.Context, payload models.DeviceInboundPayload) error {
	// Find the phone number that received this message
	var pn models.PhoneNumber
	err := r.db.QueryRow(ctx, `
		SELECT id, account_id FROM phone_numbers
		WHERE number = $1 OR imessage_address = $1
	`, payload.ToAddress).Scan(&pn.ID, &pn.AccountID)
	if err != nil {
		return fmt.Errorf("phone number not found for address %s: %w", payload.ToAddress, err)
	}

	// Upsert contact
	contact, err := r.upsertContact(ctx, pn.AccountID, pn.ID, payload.FromAddress)
	if err != nil {
		return fmt.Errorf("upsert contact: %w", err)
	}

	// Upsert conversation
	conv, err := r.upsertConversation(ctx, pn.AccountID, pn.ID, contact.ID)
	if err != nil {
		return fmt.Errorf("upsert conversation: %w", err)
	}

	// Deduplicate by imessage_guid
	var exists bool
	_ = r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages WHERE imessage_guid = $1)`,
		payload.IMessageGUID).Scan(&exists)
	if exists {
		return nil // already processed
	}

	guid := payload.IMessageGUID
	msg := &models.Message{
		ID:             uuid.New(),
		ConversationID: conv.ID,
		AccountID:      pn.AccountID,
		PhoneNumberID:  pn.ID,
		ContactID:      contact.ID,
		Direction:      models.MessageDirectionInbound,
		Content:        payload.Content,
		IMessageGUID:   &guid,
		Status:         models.MessageStatusDelivered,
		SentAt:         &payload.ReceivedAt,
		DeliveredAt:    &payload.ReceivedAt,
		CreatedAt:      time.Now(),
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO messages (id, conversation_id, account_id, phone_number_id, contact_id,
		                      direction, content, imessage_guid, status, sent_at, delivered_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, msg.ID, msg.ConversationID, msg.AccountID, msg.PhoneNumberID, msg.ContactID,
		msg.Direction, msg.Content, msg.IMessageGUID, msg.Status,
		msg.SentAt, msg.DeliveredAt, msg.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert inbound message: %w", err)
	}

	// Update conversation
	_, _ = r.db.Exec(ctx, `
		UPDATE conversations
		SET last_message_at = $1, last_message_preview = $2,
		    message_count = message_count + 1, unread_count = unread_count + 1
		WHERE id = $3
	`, time.Now(), truncate(payload.Content, 100), conv.ID)

	// Update contact stats
	_, _ = r.db.Exec(ctx, `
		UPDATE contacts SET last_message_at = $1, message_count = message_count + 1 WHERE id = $2
	`, time.Now(), contact.ID)

	// Queue GHL sync
	syncPayload, _ := json.Marshal(map[string]string{
		"message_id": msg.ID.String(),
		"account_id": pn.AccountID.String(),
	})
	task := asynq.NewTask(TaskSyncToGHL, syncPayload, asynq.MaxRetry(3))
	_, _ = r.asynq.Enqueue(task)

	return nil
}

// HandleOutbound processes outbound messages detected by the device agent
// (messages sent directly from Messages.app, not through the API).
func (r *Router) HandleOutbound(ctx context.Context, payload models.DeviceInboundPayload) error {
	// FromAddress = the local account that sent it, ToAddress = the recipient
	var pn models.PhoneNumber
	err := r.db.QueryRow(ctx, `
		SELECT id, account_id FROM phone_numbers
		WHERE number = $1 OR imessage_address = $1
	`, payload.FromAddress).Scan(&pn.ID, &pn.AccountID)
	if err != nil {
		return fmt.Errorf("phone number not found for sender %s: %w", payload.FromAddress, err)
	}

	contact, err := r.upsertContact(ctx, pn.AccountID, pn.ID, payload.ToAddress)
	if err != nil {
		return fmt.Errorf("upsert contact: %w", err)
	}

	conv, err := r.upsertConversation(ctx, pn.AccountID, pn.ID, contact.ID)
	if err != nil {
		return fmt.Errorf("upsert conversation: %w", err)
	}

	// Deduplicate
	var exists bool
	_ = r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages WHERE imessage_guid = $1)`,
		payload.IMessageGUID).Scan(&exists)
	if exists {
		return nil
	}

	guid := payload.IMessageGUID
	now := time.Now()
	msg := &models.Message{
		ID:             uuid.New(),
		ConversationID: conv.ID,
		AccountID:      pn.AccountID,
		PhoneNumberID:  pn.ID,
		ContactID:      contact.ID,
		Direction:      models.MessageDirectionOutbound,
		Content:        payload.Content,
		IMessageGUID:   &guid,
		Status:         models.MessageStatusSent,
		SentAt:         &now,
		CreatedAt:      now,
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO messages (id, conversation_id, account_id, phone_number_id, contact_id,
		                      direction, content, imessage_guid, status, sent_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, msg.ID, msg.ConversationID, msg.AccountID, msg.PhoneNumberID, msg.ContactID,
		msg.Direction, msg.Content, msg.IMessageGUID, msg.Status,
		msg.SentAt, msg.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert outbound message: %w", err)
	}

	// Update conversation
	_, _ = r.db.Exec(ctx, `
		UPDATE conversations
		SET last_message_at = $1, last_message_preview = $2, message_count = message_count + 1
		WHERE id = $3
	`, now, truncate(payload.Content, 100), conv.ID)

	// Update contact stats
	_, _ = r.db.Exec(ctx, `
		UPDATE contacts SET last_message_at = $1, message_count = message_count + 1 WHERE id = $2
	`, now, contact.ID)

	// Queue GHL sync
	syncPayload, _ := json.Marshal(map[string]string{
		"message_id": msg.ID.String(),
		"account_id": pn.AccountID.String(),
	})
	task := asynq.NewTask(TaskSyncToGHL, syncPayload, asynq.MaxRetry(3))
	_, _ = r.asynq.Enqueue(task)

	return nil
}

func (r *Router) upsertContact(ctx context.Context, accountID, phoneNumberID uuid.UUID, address string) (*models.Contact, error) {
	var c models.Contact
	err := r.db.QueryRow(ctx, `
		INSERT INTO contacts (id, account_id, phone_number_id, imessage_address, first_message_at, last_message_at, created_at, updated_at)
		VALUES (uuid_generate_v4(), $1, $2, $3, NOW(), NOW(), NOW(), NOW())
		ON CONFLICT (account_id, imessage_address) DO UPDATE
		SET last_message_at = NOW(), phone_number_id = EXCLUDED.phone_number_id
		RETURNING id, account_id, phone_number_id, imessage_address, name, ghl_contact_id, message_count
	`, accountID, phoneNumberID, address).Scan(
		&c.ID, &c.AccountID, &c.PhoneNumberID, &c.IMessageAddress, &c.Name, &c.GHLContactID, &c.MessageCount,
	)
	return &c, err
}

func (r *Router) upsertConversation(ctx context.Context, accountID, phoneNumberID, contactID uuid.UUID) (*models.Conversation, error) {
	var c models.Conversation
	err := r.db.QueryRow(ctx, `
		INSERT INTO conversations (id, account_id, phone_number_id, contact_id, created_at, updated_at)
		VALUES (uuid_generate_v4(), $1, $2, $3, NOW(), NOW())
		ON CONFLICT (phone_number_id, contact_id) DO UPDATE
		SET updated_at = NOW()
		RETURNING id, account_id, phone_number_id, contact_id, ghl_conversation_id, message_count
	`, accountID, phoneNumberID, contactID).Scan(
		&c.ID, &c.AccountID, &c.PhoneNumberID, &c.ContactID, &c.GHLConversationID, &c.MessageCount,
	)
	return &c, err
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
