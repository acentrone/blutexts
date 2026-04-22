package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bluesend/api/internal/models"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const (
	TaskSendMessage    = "message:send"
	TaskSyncToGHL      = "ghl:sync_message"
	TaskSyncContact    = "ghl:sync_contact"
	TaskSendScheduled  = "message:send_scheduled"
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

// Hub returns the device hub for direct device communication (e.g., calls).
func (r *Router) Hub() DeviceHub {
	return r.hub
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
		Attachments:    req.Attachments,
		Status:         models.MessageStatusPending,
		CreatedAt:      time.Now(),
	}

	attachmentsJSON, _ := json.Marshal(req.Attachments)
	if len(req.Attachments) == 0 {
		attachmentsJSON = []byte("[]")
	}

	// If this send was initiated by a GHL webhook, mark it as already-synced
	// so the GHL syncer skips re-pushing it (preventing double logging).
	if req.GHLMessageID != "" {
		ghlID := req.GHLMessageID
		msg.GHLMessageID = &ghlID
		now := time.Now()
		_, err = r.db.Exec(ctx, `
			INSERT INTO messages (id, conversation_id, account_id, phone_number_id, contact_id, direction, content, attachments, status, created_at, ghl_message_id, ghl_synced_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`, msg.ID, msg.ConversationID, msg.AccountID, msg.PhoneNumberID, msg.ContactID,
			msg.Direction, msg.Content, attachmentsJSON, msg.Status, msg.CreatedAt, ghlID, now)
	} else {
		_, err = r.db.Exec(ctx, `
			INSERT INTO messages (id, conversation_id, account_id, phone_number_id, contact_id, direction, content, attachments, status, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, msg.ID, msg.ConversationID, msg.AccountID, msg.PhoneNumberID, msg.ContactID,
			msg.Direction, msg.Content, attachmentsJSON, msg.Status, msg.CreatedAt)
	}
	if err != nil {
		return nil, fmt.Errorf("insert message: %w", err)
	}

	// Update conversation preview
	preview := req.Content
	if preview == "" && len(req.Attachments) > 0 {
		preview = fmt.Sprintf("[%d attachment%s]", len(req.Attachments), plural(len(req.Attachments)))
	}
	_, _ = r.db.Exec(ctx, `
		UPDATE conversations
		SET last_message_at = $1, last_message_preview = $2, message_count = message_count + 1
		WHERE id = $3
	`, time.Now(), truncate(preview, 100), conv.ID)

	// Look up cached iMessage capability so the device can skip the
	// availability lookup. nil means we haven't checked yet — the device
	// will query Apple's identity servers and report back, and we'll
	// persist the result on the contact for next time.
	var imessageCapable *bool
	_ = r.db.QueryRow(ctx,
		`SELECT imessage_capable FROM contacts WHERE id = $1`, contact.ID,
	).Scan(&imessageCapable)

	// Push send job to device via WebSocket hub
	sendPayload := models.DeviceSendPayload{
		MessageID:       msg.ID.String(),
		PhoneNumber:     pn.Number,
		ToAddress:       req.ToAddress,
		Content:         req.Content,
		IMessageAddress: deref(pn.IMessageAddress),
		Attachments:     req.Attachments,
		Effect:          req.Effect,
		IMessageCapable: imessageCapable,
	}
	if err := r.hub.SendToDevice(*pn.DeviceID, models.DeviceWSEvent{
		Type:    models.DeviceEventSendMessage,
		Payload: sendPayload,
	}); err != nil {
		log.Printf("SendToDevice failed (device=%s): %v", *pn.DeviceID, err)
	} else {
		log.Printf("Send dispatched to device=%s msg=%s to=%s", *pn.DeviceID, msg.ID, req.ToAddress)
	}

	// Queue GHL sync (async, non-blocking) — but skip when this send was
	// initiated by a GHL webhook, since the message already exists in GHL.
	if req.GHLMessageID == "" {
		payload, _ := json.Marshal(map[string]string{
			"message_id": msg.ID.String(),
			"account_id": accountID.String(),
		})
		task := asynq.NewTask(TaskSyncToGHL, payload, asynq.MaxRetry(3), asynq.Timeout(30*time.Second))
		_, _ = r.asynq.Enqueue(task)
	}

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

// AutoReplyInfo holds the cancellation auto-reply state for an account,
// read once during inbound processing to decide whether to send a reply.
type AutoReplyInfo struct {
	Status            string
	AutoReplyEnabled  bool
	AutoReplyStartsAt *time.Time
	AutoReplyMessage  string
}

// HandleInbound processes a message received from a device agent. Returns the
// persisted Message so the caller can broadcast it to the web app — or nil if
// the message was a duplicate of one we've already stored.
func (r *Router) HandleInbound(ctx context.Context, payload models.DeviceInboundPayload) (*models.Message, error) {
	// Find the phone number that received this message
	var pn models.PhoneNumber
	err := r.db.QueryRow(ctx, `
		SELECT id, account_id FROM phone_numbers
		WHERE number = $1 OR imessage_address = $1
	`, payload.ToAddress).Scan(&pn.ID, &pn.AccountID)
	if err != nil {
		return nil, fmt.Errorf("phone number not found for address %s: %w", payload.ToAddress, err)
	}

	// Upsert contact
	contact, err := r.upsertContact(ctx, pn.AccountID, pn.ID, payload.FromAddress)
	if err != nil {
		return nil, fmt.Errorf("upsert contact: %w", err)
	}

	// Upsert conversation
	conv, err := r.upsertConversation(ctx, pn.AccountID, pn.ID, contact.ID)
	if err != nil {
		return nil, fmt.Errorf("upsert conversation: %w", err)
	}

	// Deduplicate by imessage_guid
	var exists bool
	_ = r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages WHERE imessage_guid = $1)`,
		payload.IMessageGUID).Scan(&exists)
	if exists {
		return nil, nil // already processed
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
		Attachments:    payload.Attachments,
		IMessageGUID:   &guid,
		Status:         models.MessageStatusDelivered,
		SentAt:         &payload.ReceivedAt,
		DeliveredAt:    &payload.ReceivedAt,
		CreatedAt:      time.Now(),
	}

	attachmentsJSON, _ := json.Marshal(payload.Attachments)
	if len(payload.Attachments) == 0 {
		attachmentsJSON = []byte("[]")
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO messages (id, conversation_id, account_id, phone_number_id, contact_id,
		                      direction, content, attachments, imessage_guid, status, sent_at, delivered_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, msg.ID, msg.ConversationID, msg.AccountID, msg.PhoneNumberID, msg.ContactID,
		msg.Direction, msg.Content, attachmentsJSON, msg.IMessageGUID, msg.Status,
		msg.SentAt, msg.DeliveredAt, msg.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert inbound message: %w", err)
	}

	// Update conversation. Show "[voice message]" / "[N attachment(s)]" when
	// the message is media-only so the sidebar isn't blank.
	preview := payload.Content
	if preview == "" && len(payload.Attachments) > 0 {
		preview = previewForAttachments(payload.Attachments)
	}
	_, _ = r.db.Exec(ctx, `
		UPDATE conversations
		SET last_message_at = $1, last_message_preview = $2,
		    message_count = message_count + 1, unread_count = unread_count + 1
		WHERE id = $3
	`, time.Now(), truncate(preview, 100), conv.ID)

	// Update contact stats
	_, _ = r.db.Exec(ctx, `
		UPDATE contacts SET last_message_at = $1, message_count = message_count + 1 WHERE id = $2
	`, time.Now(), contact.ID)

	// Check account cancellation state — skip GHL sync for cancelled accounts
	// and trigger auto-reply if conditions are met.
	var ari AutoReplyInfo
	_ = r.db.QueryRow(ctx, `
		SELECT status, auto_reply_enabled, auto_reply_starts_at, auto_reply_message
		FROM accounts WHERE id = $1
	`, pn.AccountID).Scan(&ari.Status, &ari.AutoReplyEnabled, &ari.AutoReplyStartsAt, &ari.AutoReplyMessage)

	if ari.Status != string(models.AccountStatusCancelled) {
		syncPayload, _ := json.Marshal(map[string]string{
			"message_id": msg.ID.String(),
			"account_id": pn.AccountID.String(),
		})
		task := asynq.NewTask(TaskSyncToGHL, syncPayload, asynq.MaxRetry(3))
		_, _ = r.asynq.Enqueue(task)
	}

	// Auto-reply for cancelled accounts: if auto-reply is enabled, the start
	// date has passed, and the sender hasn't already received an auto-reply in
	// the last 24h (to avoid reply loops), send the canned message.
	if ari.Status == string(models.AccountStatusCancelled) &&
		ari.AutoReplyEnabled &&
		ari.AutoReplyStartsAt != nil &&
		time.Now().After(*ari.AutoReplyStartsAt) &&
		ari.AutoReplyMessage != "" {

		r.maybeAutoReply(ctx, pn, payload.FromAddress, ari.AutoReplyMessage)
	}

	return msg, nil
}

// HandleOutbound processes outbound messages detected by the device agent
// (messages sent directly from Messages.app, not through the API). Returns
// the persisted Message so the caller can broadcast it — or nil if the
// message was a duplicate.
func (r *Router) HandleOutbound(ctx context.Context, payload models.DeviceInboundPayload) (*models.Message, error) {
	// FromAddress = the local account that sent it, ToAddress = the recipient
	var pn models.PhoneNumber
	err := r.db.QueryRow(ctx, `
		SELECT id, account_id FROM phone_numbers
		WHERE number = $1 OR imessage_address = $1
	`, payload.FromAddress).Scan(&pn.ID, &pn.AccountID)
	if err != nil {
		return nil, fmt.Errorf("phone number not found for sender %s: %w", payload.FromAddress, err)
	}

	contact, err := r.upsertContact(ctx, pn.AccountID, pn.ID, payload.ToAddress)
	if err != nil {
		return nil, fmt.Errorf("upsert contact: %w", err)
	}

	conv, err := r.upsertConversation(ctx, pn.AccountID, pn.ID, contact.ID)
	if err != nil {
		return nil, fmt.Errorf("upsert conversation: %w", err)
	}

	// Deduplicate
	var exists bool
	_ = r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages WHERE imessage_guid = $1)`,
		payload.IMessageGUID).Scan(&exists)
	if exists {
		return nil, nil
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
		Attachments:    payload.Attachments,
		IMessageGUID:   &guid,
		Status:         models.MessageStatusSent,
		SentAt:         &now,
		CreatedAt:      now,
	}

	attachmentsJSON, _ := json.Marshal(payload.Attachments)
	if len(payload.Attachments) == 0 {
		attachmentsJSON = []byte("[]")
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO messages (id, conversation_id, account_id, phone_number_id, contact_id,
		                      direction, content, attachments, imessage_guid, status, sent_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, msg.ID, msg.ConversationID, msg.AccountID, msg.PhoneNumberID, msg.ContactID,
		msg.Direction, msg.Content, attachmentsJSON, msg.IMessageGUID, msg.Status,
		msg.SentAt, msg.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert outbound message: %w", err)
	}

	preview := payload.Content
	if preview == "" && len(payload.Attachments) > 0 {
		preview = previewForAttachments(payload.Attachments)
	}
	_, _ = r.db.Exec(ctx, `
		UPDATE conversations
		SET last_message_at = $1, last_message_preview = $2, message_count = message_count + 1
		WHERE id = $3
	`, now, truncate(preview, 100), conv.ID)

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

	return msg, nil
}

func (r *Router) upsertContact(ctx context.Context, accountID, phoneNumberID uuid.UUID, address string) (*models.Contact, error) {
	// Normalize address — phone numbers to E.164, emails lowercased
	address = normalizeAddress(address)

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

// normalizeAddress converts phone numbers to E.164 format and lowercases emails.
func normalizeAddress(addr string) string {
	if addr == "" {
		return addr
	}
	// If it contains @, it's an email — lowercase and return
	if strings.Contains(addr, "@") {
		return strings.ToLower(strings.TrimSpace(addr))
	}
	// Phone number — strip all non-digits except leading +
	var sb strings.Builder
	hasPlus := false
	for i, c := range addr {
		if i == 0 && c == '+' {
			sb.WriteRune(c)
			hasPlus = true
			continue
		}
		if c >= '0' && c <= '9' {
			sb.WriteRune(c)
		}
	}
	result := sb.String()
	// Add + prefix if missing and it looks like a US number (10 or 11 digits)
	if !hasPlus {
		digits := result
		if len(digits) == 10 {
			result = "+1" + digits
		} else if len(digits) == 11 && digits[0] == '1' {
			result = "+" + digits
		} else if len(digits) > 0 {
			result = "+" + digits
		}
	}
	return result
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

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// previewForAttachments produces a sidebar preview string for media-only
// messages. Voice messages get a dedicated label so the conversation list
// reads naturally; otherwise we fall back to a generic count.
func previewForAttachments(atts []models.Attachment) string {
	if len(atts) == 0 {
		return ""
	}
	for _, a := range atts {
		if strings.HasPrefix(a.Type, "audio/") {
			return "🎙️ Voice message"
		}
	}
	for _, a := range atts {
		if strings.HasPrefix(a.Type, "image/") {
			return "📷 Photo"
		}
		if strings.HasPrefix(a.Type, "video/") {
			return "🎥 Video"
		}
	}
	return fmt.Sprintf("[%d attachment%s]", len(atts), plural(len(atts)))
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

// maybeAutoReply sends a one-per-24h auto-reply to the given sender address.
// This avoids reply loops where two systems keep answering each other.
func (r *Router) maybeAutoReply(ctx context.Context, pn models.PhoneNumber, toAddress, replyText string) {
	// Check if we've already auto-replied to this address in the last 24 hours.
	cacheKey := fmt.Sprintf("autoreply:%s:%s", pn.ID.String(), normalizeAddress(toAddress))
	already, _ := r.redis.Exists(ctx, cacheKey).Result()
	if already > 0 {
		log.Printf("Auto-reply skipped for %s (already sent in last 24h)", toAddress)
		return
	}

	if pn.DeviceID == nil {
		log.Printf("Auto-reply skipped for %s (phone number has no device)", toAddress)
		return
	}

	// Send via device hub
	autoReplyPayload := models.DeviceSendPayload{
		MessageID:       uuid.New().String(),
		PhoneNumber:     pn.Number,
		ToAddress:       toAddress,
		Content:         replyText,
		IMessageAddress: deref(pn.IMessageAddress),
	}

	if err := r.hub.SendToDevice(*pn.DeviceID, models.DeviceWSEvent{
		Type:    models.DeviceEventSendMessage,
		Payload: autoReplyPayload,
	}); err != nil {
		log.Printf("Auto-reply send failed for %s: %v", toAddress, err)
		return
	}

	// Mark as sent for 24 hours so we don't reply again
	r.redis.Set(ctx, cacheKey, "1", 24*time.Hour)
	log.Printf("Auto-reply sent to %s on number %s", toAddress, pn.Number)
}
