package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// Account
// ============================================================

type AccountStatus string
type AccountPlan string

const (
	AccountStatusPending    AccountStatus = "pending"
	AccountStatusSettingUp  AccountStatus = "setting_up"
	AccountStatusActive     AccountStatus = "active"
	AccountStatusPastDue    AccountStatus = "past_due"
	AccountStatusCancelled  AccountStatus = "cancelled"
	AccountStatusSuspended  AccountStatus = "suspended"

	AccountPlanPending  AccountPlan = "pending"
	AccountPlanMonthly  AccountPlan = "monthly"
	AccountPlanAnnual   AccountPlan = "annual"
)

type Account struct {
	ID                   uuid.UUID     `json:"id" db:"id"`
	Name                 string        `json:"name" db:"name"`
	Email                string        `json:"email" db:"email"`
	StripeCustomerID     *string       `json:"stripe_customer_id,omitempty" db:"stripe_customer_id"`
	StripeSubscriptionID *string       `json:"stripe_subscription_id,omitempty" db:"stripe_subscription_id"`
	Plan                 AccountPlan   `json:"plan" db:"plan"`
	Status               AccountStatus `json:"status" db:"status"`
	SetupComplete        bool          `json:"setup_complete" db:"setup_complete"`
	SetupFeePaid         bool          `json:"setup_fee_paid" db:"setup_fee_paid"`
	Timezone             string        `json:"timezone" db:"timezone"`
	CallingEnabled       bool          `json:"calling_enabled" db:"calling_enabled"`
	// Cancellation lifecycle
	CancelledAt        *time.Time `json:"cancelled_at,omitempty" db:"cancelled_at"`
	GracePeriodEndsAt  *time.Time `json:"grace_period_ends_at,omitempty" db:"grace_period_ends_at"`
	AutoReplyStartsAt  *time.Time `json:"auto_reply_starts_at,omitempty" db:"auto_reply_starts_at"`
	AutoReplyEnabled   bool       `json:"auto_reply_enabled" db:"auto_reply_enabled"`
	AutoReplyMessage   string          `json:"auto_reply_message" db:"auto_reply_message"`
	CustomFieldSchema  json.RawMessage `json:"custom_field_schema" db:"custom_field_schema"`
	CreatedAt          time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at" db:"updated_at"`
}

// CustomFieldDefinition describes a single custom field in the account schema.
type CustomFieldDefinition struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`     // text, number, select, date, url
	Required bool     `json:"required"`
	Options  []string `json:"options,omitempty"` // for select type
}

// ============================================================
// User
// ============================================================

type UserRole string

const (
	UserRoleOwner  UserRole = "owner"
	UserRoleMember UserRole = "member"
	UserRoleAdmin  UserRole = "admin"
)

type User struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	AccountID    uuid.UUID  `json:"account_id" db:"account_id"`
	Email        string     `json:"email" db:"email"`
	PasswordHash string     `json:"-" db:"password_hash"`
	FirstName    *string    `json:"first_name,omitempty" db:"first_name"`
	LastName     *string    `json:"last_name,omitempty" db:"last_name"`
	Role         UserRole   `json:"role" db:"role"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
}

// ============================================================
// Device
// ============================================================

type DeviceStatus string
type DeviceType string

const (
	DeviceStatusOnline      DeviceStatus = "online"
	DeviceStatusOffline     DeviceStatus = "offline"
	DeviceStatusError       DeviceStatus = "error"
	DeviceStatusMaintenance DeviceStatus = "maintenance"

	DeviceTypeMacMini DeviceType = "mac_mini"
	DeviceTypeIPhone  DeviceType = "iphone"
)

type Device struct {
	ID            uuid.UUID    `json:"id" db:"id"`
	Name          string       `json:"name" db:"name"`
	Type          DeviceType   `json:"type" db:"type"`
	SerialNumber  *string      `json:"serial_number,omitempty" db:"serial_number"`
	DeviceToken   string       `json:"-" db:"device_token"` // never expose to clients
	Status        DeviceStatus `json:"status" db:"status"`
	LastSeenAt    *time.Time   `json:"last_seen_at,omitempty" db:"last_seen_at"`
	IPAddress     *string      `json:"ip_address,omitempty" db:"ip_address"`
	AgentVersion  *string      `json:"agent_version,omitempty" db:"agent_version"`
	OSVersion     *string      `json:"os_version,omitempty" db:"os_version"`
	Capacity      int          `json:"capacity" db:"capacity"`
	AssignedCount int          `json:"assigned_count" db:"assigned_count"`
	ErrorMessage  *string      `json:"error_message,omitempty" db:"error_message"`
	CreatedAt     time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at" db:"updated_at"`
}

// ============================================================
// PhoneNumber
// ============================================================

type PhoneNumberStatus string

const (
	PhoneNumberStatusProvisioning  PhoneNumberStatus = "provisioning"
	PhoneNumberStatusActive        PhoneNumberStatus = "active"
	PhoneNumberStatusSuspended     PhoneNumberStatus = "suspended"
	PhoneNumberStatusDeprovisioned PhoneNumberStatus = "deprovisioned"
)

type PhoneNumber struct {
	ID                   uuid.UUID         `json:"id" db:"id"`
	AccountID            uuid.UUID         `json:"account_id" db:"account_id"`
	DeviceID             *uuid.UUID        `json:"device_id,omitempty" db:"device_id"`
	Number               string            `json:"number" db:"number"`
	DisplayName          *string           `json:"display_name,omitempty" db:"display_name"`
	IMessageAddress      *string           `json:"imessage_address,omitempty" db:"imessage_address"`
	Status               PhoneNumberStatus `json:"status" db:"status"`
	DailyNewContactLimit int               `json:"daily_new_contact_limit" db:"daily_new_contact_limit"`
	VoiceEnabled         bool              `json:"voice_enabled" db:"voice_enabled"`
	CreatedAt            time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time         `json:"updated_at" db:"updated_at"`
}

// ============================================================
// Contact
// ============================================================

type Contact struct {
	ID               uuid.UUID              `json:"id" db:"id"`
	AccountID        uuid.UUID              `json:"account_id" db:"account_id"`
	PhoneNumberID    *uuid.UUID             `json:"phone_number_id,omitempty" db:"phone_number_id"`
	IMessageAddress  string                 `json:"imessage_address" db:"imessage_address"`
	Name             *string                `json:"name,omitempty" db:"name"`
	Email            *string                `json:"email,omitempty" db:"email"`
	Company          *string                `json:"company,omitempty" db:"company"`
	Notes            string                 `json:"notes" db:"notes"`
	Tags             []string               `json:"tags" db:"tags"`
	CustomFields     map[string]interface{} `json:"custom_fields" db:"custom_fields"`
	GHLContactID     *string                `json:"ghl_contact_id,omitempty" db:"ghl_contact_id"`
	IMessageCapable    *bool      `json:"imessage_capable" db:"imessage_capable"`
	IMessageCheckedAt  *time.Time `json:"imessage_checked_at,omitempty" db:"imessage_checked_at"`
	FirstMessageAt   *time.Time             `json:"first_message_at,omitempty" db:"first_message_at"`
	LastMessageAt    *time.Time             `json:"last_message_at,omitempty" db:"last_message_at"`
	MessageCount     int                    `json:"message_count" db:"message_count"`
	CreatedAt        time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at" db:"updated_at"`
}

// ============================================================
// Conversation
// ============================================================

type ConversationStatus string

const (
	ConversationStatusOpen     ConversationStatus = "open"
	ConversationStatusClosed   ConversationStatus = "closed"
	ConversationStatusArchived ConversationStatus = "archived"
)

type Conversation struct {
	ID                  uuid.UUID          `json:"id" db:"id"`
	AccountID           uuid.UUID          `json:"account_id" db:"account_id"`
	PhoneNumberID       uuid.UUID          `json:"phone_number_id" db:"phone_number_id"`
	ContactID           uuid.UUID          `json:"contact_id" db:"contact_id"`
	GHLConversationID   *string            `json:"ghl_conversation_id,omitempty" db:"ghl_conversation_id"`
	LastMessageAt       *time.Time         `json:"last_message_at,omitempty" db:"last_message_at"`
	LastMessagePreview  *string            `json:"last_message_preview,omitempty" db:"last_message_preview"`
	MessageCount        int                `json:"message_count" db:"message_count"`
	UnreadCount         int                `json:"unread_count" db:"unread_count"`
	Status              ConversationStatus `json:"status" db:"status"`
	CreatedAt           time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time          `json:"updated_at" db:"updated_at"`
	// Joined fields
	Contact     *Contact     `json:"contact,omitempty" db:"-"`
	PhoneNumber *PhoneNumber `json:"phone_number,omitempty" db:"-"`
}

// ============================================================
// Message
// ============================================================

type MessageDirection string
type MessageStatus string

const (
	MessageDirectionInbound  MessageDirection = "inbound"
	MessageDirectionOutbound MessageDirection = "outbound"

	MessageStatusPending   MessageStatus = "pending"
	MessageStatusSent      MessageStatus = "sent"
	MessageStatusDelivered MessageStatus = "delivered"
	MessageStatusRead      MessageStatus = "read"
	MessageStatusFailed    MessageStatus = "failed"
)

type Message struct {
	ID              uuid.UUID        `json:"id" db:"id"`
	ConversationID  uuid.UUID        `json:"conversation_id" db:"conversation_id"`
	AccountID       uuid.UUID        `json:"account_id" db:"account_id"`
	PhoneNumberID   uuid.UUID        `json:"phone_number_id" db:"phone_number_id"`
	ContactID       uuid.UUID        `json:"contact_id" db:"contact_id"`
	Direction       MessageDirection `json:"direction" db:"direction"`
	Content         string           `json:"content" db:"content"`
	Attachments     []Attachment     `json:"attachments,omitempty" db:"attachments"`
	IMessageGUID    *string          `json:"imessage_guid,omitempty" db:"imessage_guid"`
	Status          MessageStatus    `json:"status" db:"status"`
	SentAt          *time.Time       `json:"sent_at,omitempty" db:"sent_at"`
	DeliveredAt     *time.Time       `json:"delivered_at,omitempty" db:"delivered_at"`
	ReadAt          *time.Time       `json:"read_at,omitempty" db:"read_at"`
	FailedAt        *time.Time       `json:"failed_at,omitempty" db:"failed_at"`
	ErrorMessage    *string          `json:"error_message,omitempty" db:"error_message"`
	GHLMessageID    *string          `json:"ghl_message_id,omitempty" db:"ghl_message_id"`
	GHLSyncedAt     *time.Time       `json:"ghl_synced_at,omitempty" db:"ghl_synced_at"`
	Service         string           `json:"service" db:"service"` // "imessage" or "sms"
	CreatedAt       time.Time        `json:"created_at" db:"created_at"`
}

// ============================================================
// GHL Connection
// ============================================================

type GHLConnection struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	AccountID       uuid.UUID  `json:"account_id" db:"account_id"`
	LocationID      string     `json:"location_id" db:"location_id"`
	AccessToken     string     `json:"-" db:"access_token"`
	RefreshToken    string     `json:"-" db:"refresh_token"`
	TokenExpiresAt  time.Time  `json:"token_expires_at" db:"token_expires_at"`
	PipelineID      *string    `json:"pipeline_id,omitempty" db:"pipeline_id"`
	CustomChannelID *string    `json:"custom_channel_id,omitempty" db:"custom_channel_id"`
	WebhookID       *string    `json:"webhook_id,omitempty" db:"webhook_id"`
	Connected       bool       `json:"connected" db:"connected"`
	LastSyncedAt    *time.Time `json:"last_synced_at,omitempty" db:"last_synced_at"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
}

// ============================================================
// API Request/Response types
// ============================================================

type SignupRequest struct {
	Email             string `json:"email" validate:"required,email"`
	Password          string `json:"password" validate:"required,min=8"`
	FirstName         string `json:"first_name" validate:"required"`
	LastName          string `json:"last_name" validate:"required"`
	Company           string `json:"company" validate:"required"`
	PreferredAreaCode string `json:"preferred_area_code"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         *User  `json:"user"`
	Account      *Account `json:"account"`
}

type SendMessageRequest struct {
	PhoneNumberID string       `json:"phone_number_id" validate:"required,uuid"`
	ToAddress     string       `json:"to_address" validate:"required"`
	Content       string       `json:"content" validate:"max=5000"`
	Attachments   []Attachment `json:"attachments,omitempty"`
	Effect        string       `json:"effect,omitempty"` // iMessage effect ID (e.g. "slam", "confetti")

	// GHLMessageID is set internally when this send was initiated by a GHL
	// delivery webhook. The router stores it on the message so the GHL syncer
	// skips re-pushing it (which would cause double logging in GHL).
	GHLMessageID string `json:"-"`
}

// Attachment represents a media file attached to a message.
type Attachment struct {
	URL      string `json:"url"`
	Type     string `json:"type"`     // image/jpeg, video/mp4, audio/m4a, etc.
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	WebURL   string `json:"web_url,omitempty"` // Browser-playable version (e.g., webm for audio)
}

type SendMessageResponse struct {
	Message    *Message `json:"message"`
	RateLimited bool    `json:"rate_limited"`
	RateLimit  *RateLimitInfo `json:"rate_limit,omitempty"`
}

type RateLimitInfo struct {
	DailyNewContactsUsed  int    `json:"daily_new_contacts_used"`
	DailyNewContactsLimit int    `json:"daily_new_contacts_limit"`
	IsNewContact          bool   `json:"is_new_contact"`
	ResetsAt              string `json:"resets_at"`
}

// ServiceStats are the per-service counts within a date range. Used for the
// "iMessage vs SMS" comparison block on the dashboard so the customer can see
// at a glance which channel is actually working for their list.
//
// Reply-rate semantics (deliberate — see GetDashboardStats):
//   contacts_messaged = distinct contacts that received an outbound on this
//                       service inside the window
//   contacts_replied  = distinct contacts that sent ANY inbound inside the
//                       window AND received an outbound on this service in
//                       the same window
//   reply_rate        = contacts_replied / contacts_messaged
//
// A contact who got both iMessage and SMS sends and replied counts for both
// services. That's the right behavior for "is this channel landing replies?"
// — we're measuring channel performance, not deduping the customer.
type ServiceStats struct {
	Sent             int     `json:"sent"`
	Delivered        int     `json:"delivered"`
	ContactsMessaged int     `json:"contacts_messaged"`
	ContactsReplied  int     `json:"contacts_replied"`
	ReplyRate        float64 `json:"reply_rate"`
}

type ServiceBreakdown struct {
	IMessage ServiceStats `json:"imessage"`
	SMS      ServiceStats `json:"sms"`
}

type DashboardStats struct {
	// ── Top-level counters (aggregate across both services in the window) ──
	TotalSent        int     `json:"total_sent"`
	TotalDelivered   int     `json:"total_delivered"`
	TotalReplied     int     `json:"total_replied"`
	ResponseRate     float64 `json:"response_rate"`
	ActiveConvos     int     `json:"active_conversations"`
	TodayNewContacts int     `json:"today_new_contacts"`
	DailyLimit       int     `json:"daily_limit"`

	// ── Per-service comparison + the date window the row covers ───────────
	Breakdown ServiceBreakdown `json:"breakdown"`
	From      time.Time        `json:"from"`
	To        time.Time        `json:"to"`
}

type CreateCheckoutRequest struct {
	Plan      string `json:"plan" validate:"required,oneof=monthly annual"`
	Email     string `json:"email" validate:"required,email"`
	FirstName string `json:"first_name" validate:"required"`
	LastName  string `json:"last_name" validate:"required"`
	Company   string `json:"company" validate:"required"`
}

type CreateCheckoutResponse struct {
	URL          string `json:"url"`
	SessionID    string `json:"session_id"`
	ClientSecret string `json:"client_secret,omitempty"`
	CustomerID   string `json:"customer_id"`
}

// WebSocket event types sent to frontend
type WSEvent struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

const (
	WSEventNewMessage         = "new_message"
	WSEventMessageDelivered   = "message_delivered"
	WSEventMessageRead        = "message_read"
	WSEventMessageFailed      = "message_failed"
	WSEventDeviceStatusChange = "device_status_change"
	WSEventAccountStatusChange = "account_status_change"
)

// Device agent WebSocket event types
type DeviceWSEvent struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

const (
	DeviceEventSendMessage     = "send_message"
	DeviceEventMessageStatus   = "message_status"
	DeviceEventInboundMessage  = "inbound_message"
	DeviceEventOutboundMessage = "outbound_message"
	DeviceEventHeartbeat       = "heartbeat"
	DeviceEventRegister        = "register"
	DeviceEventInitiateCall    = "initiate_call"
	DeviceEventCallControl     = "call_control"
	DeviceEventCallStatus      = "call_status"
)

// DeviceCallPayload instructs the device agent to join an Agora channel
// and place a FaceTime Audio call to the contact. Audio is bridged through
// BlackHole on the hosted Mac.
type DeviceCallPayload struct {
	CallID       string `json:"call_id"`
	To           string `json:"to"`       // contact phone number (E.164) or email (for FaceTime Audio)
	FromNumber   string `json:"from_number"`
	AgoraChannel string `json:"agora_channel"`
	AgoraToken   string `json:"agora_token"`
	AgoraUID     uint32 `json:"agora_uid"`
	AgoraAppID   string `json:"agora_app_id"`
}

// DeviceCallControl tells the device agent to end/cancel an in-progress call.
type DeviceCallControl struct {
	CallID string `json:"call_id"`
	Action string `json:"action"` // "end", "cancel"
}

// DeviceCallStatusPayload is reported from the device agent back to the server
// as the call progresses (ringing, connected, ended with duration, failed).
type DeviceCallStatusPayload struct {
	CallID   string `json:"call_id"`
	Status   string `json:"status"`   // "ringing", "connected", "ended", "failed"
	Duration int    `json:"duration"` // seconds, set on ended
	Error    string `json:"error,omitempty"`
}

// CallDirection and CallStatus mirror the call_logs check constraints.
type CallDirection string
type CallStatus string

const (
	CallDirectionInbound  CallDirection = "inbound"
	CallDirectionOutbound CallDirection = "outbound"

	CallStatusInitiated CallStatus = "initiated"
	CallStatusRinging   CallStatus = "ringing"
	CallStatusConnected CallStatus = "connected"
	CallStatusCompleted CallStatus = "completed"
	CallStatusFailed    CallStatus = "failed"
	CallStatusMissed    CallStatus = "missed"
	CallStatusCancelled CallStatus = "cancelled"
)

type CallLog struct {
	ID              uuid.UUID     `json:"id" db:"id"`
	AccountID       uuid.UUID     `json:"account_id" db:"account_id"`
	PhoneNumberID   *uuid.UUID    `json:"phone_number_id,omitempty" db:"phone_number_id"`
	DeviceID        *uuid.UUID    `json:"device_id,omitempty" db:"device_id"`
	Direction       CallDirection `json:"direction" db:"direction"`
	FromNumber      string        `json:"from_number" db:"from_number"`
	ToNumber        string        `json:"to_number" db:"to_number"`
	AgoraChannel    string        `json:"-" db:"agora_channel"`
	Status          CallStatus    `json:"status" db:"status"`
	FailureReason   *string       `json:"failure_reason,omitempty" db:"failure_reason"`
	DurationSeconds *int          `json:"duration_seconds,omitempty" db:"duration_seconds"`
	StartedAt       *time.Time    `json:"started_at,omitempty" db:"started_at"`
	ConnectedAt     *time.Time    `json:"connected_at,omitempty" db:"connected_at"`
	EndedAt         *time.Time    `json:"ended_at,omitempty" db:"ended_at"`
	CreatedAt       time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at" db:"updated_at"`
}

type DeviceSendPayload struct {
	MessageID       string       `json:"message_id"`
	PhoneNumber     string       `json:"phone_number"`     // sending from
	ToAddress       string       `json:"to_address"`       // sending to
	Content         string       `json:"content"`
	IMessageAddress string       `json:"imessage_address"` // iMessage handle to send from
	Attachments     []Attachment `json:"attachments,omitempty"`
	Effect          string       `json:"effect,omitempty"` // iMessage effect ID
	// IMessageCapable: nil = unknown (device must check), true = use iMessage,
	// false = use SMS via Continuity. Cached on the contact server-side.
	IMessageCapable *bool `json:"imessage_capable,omitempty"`
}

type DeviceInboundPayload struct {
	IMessageGUID string       `json:"imessage_guid"`
	FromAddress  string       `json:"from_address"`
	ToAddress    string       `json:"to_address"` // which number/address on device received it
	Content      string       `json:"content"`
	Attachments  []Attachment `json:"attachments,omitempty"`
	ReceivedAt   time.Time    `json:"received_at"`
}

type DeviceStatusPayload struct {
	DeviceID     string `json:"device_id"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// ============================================================
// ScheduledMessage
// ============================================================

type ScheduledMessageStatus string

const (
	ScheduledStatusPending   ScheduledMessageStatus = "pending"
	ScheduledStatusSent      ScheduledMessageStatus = "sent"
	ScheduledStatusFailed    ScheduledMessageStatus = "failed"
	ScheduledStatusCancelled ScheduledMessageStatus = "cancelled"
)

type ScheduledMessage struct {
	ID            uuid.UUID              `json:"id" db:"id"`
	AccountID     uuid.UUID              `json:"account_id" db:"account_id"`
	PhoneNumberID uuid.UUID              `json:"phone_number_id" db:"phone_number_id"`
	ToAddress     string                 `json:"to_address" db:"to_address"`
	Content       string                 `json:"content" db:"content"`
	Attachments   []Attachment           `json:"attachments" db:"attachments"`
	Effect        *string                `json:"effect,omitempty" db:"effect"`
	ScheduledAt   time.Time              `json:"scheduled_at" db:"scheduled_at"`
	Status        ScheduledMessageStatus `json:"status" db:"status"`
	SentAt        *time.Time             `json:"sent_at,omitempty" db:"sent_at"`
	ErrorMessage  *string                `json:"error_message,omitempty" db:"error_message"`
	CreatedBy     uuid.UUID              `json:"created_by" db:"created_by"`
	CreatedAt     time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at" db:"updated_at"`
}

type CreateScheduledMessageRequest struct {
	PhoneNumberID string       `json:"phone_number_id" validate:"required,uuid"`
	ToAddress     string       `json:"to_address" validate:"required"`
	Content       string       `json:"content"`
	Attachments   []Attachment `json:"attachments,omitempty"`
	Effect        string       `json:"effect,omitempty"`
	ScheduledAt   string       `json:"scheduled_at" validate:"required"` // RFC3339
}

// ============================================================
// Invitation (team members)
// ============================================================

type Invitation struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	AccountID  uuid.UUID  `json:"account_id" db:"account_id"`
	Email      string     `json:"email" db:"email"`
	Role       UserRole   `json:"role" db:"role"`
	Token      string     `json:"-" db:"token"`
	InvitedBy  uuid.UUID  `json:"invited_by" db:"invited_by"`
	ExpiresAt  time.Time  `json:"expires_at" db:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty" db:"accepted_at"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
}

type InviteRequest struct {
	Email string `json:"email" validate:"required,email"`
	Role  string `json:"role" validate:"required,oneof=member admin"`
}
