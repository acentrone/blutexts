package models

import (
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
	CreatedAt            time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time     `json:"updated_at" db:"updated_at"`
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
	CreatedAt            time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time         `json:"updated_at" db:"updated_at"`
}

// ============================================================
// Contact
// ============================================================

type Contact struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	AccountID        uuid.UUID  `json:"account_id" db:"account_id"`
	PhoneNumberID    *uuid.UUID `json:"phone_number_id,omitempty" db:"phone_number_id"`
	IMessageAddress  string     `json:"imessage_address" db:"imessage_address"`
	Name             *string    `json:"name,omitempty" db:"name"`
	GHLContactID     *string    `json:"ghl_contact_id,omitempty" db:"ghl_contact_id"`
	FirstMessageAt   *time.Time `json:"first_message_at,omitempty" db:"first_message_at"`
	LastMessageAt    *time.Time `json:"last_message_at,omitempty" db:"last_message_at"`
	MessageCount     int        `json:"message_count" db:"message_count"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
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
	IMessageGUID    *string          `json:"imessage_guid,omitempty" db:"imessage_guid"`
	Status          MessageStatus    `json:"status" db:"status"`
	SentAt          *time.Time       `json:"sent_at,omitempty" db:"sent_at"`
	DeliveredAt     *time.Time       `json:"delivered_at,omitempty" db:"delivered_at"`
	ReadAt          *time.Time       `json:"read_at,omitempty" db:"read_at"`
	FailedAt        *time.Time       `json:"failed_at,omitempty" db:"failed_at"`
	ErrorMessage    *string          `json:"error_message,omitempty" db:"error_message"`
	GHLMessageID    *string          `json:"ghl_message_id,omitempty" db:"ghl_message_id"`
	GHLSyncedAt     *time.Time       `json:"ghl_synced_at,omitempty" db:"ghl_synced_at"`
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
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=8"`
	FirstName string `json:"first_name" validate:"required"`
	LastName  string `json:"last_name" validate:"required"`
	Company   string `json:"company" validate:"required"`
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
	PhoneNumberID string `json:"phone_number_id" validate:"required,uuid"`
	ToAddress     string `json:"to_address" validate:"required"`
	Content       string `json:"content" validate:"required,max=5000"`
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

type DashboardStats struct {
	TotalSent       int     `json:"total_sent"`
	TotalDelivered  int     `json:"total_delivered"`
	TotalReplied    int     `json:"total_replied"`
	ResponseRate    float64 `json:"response_rate"`
	ActiveConvos    int     `json:"active_conversations"`
	TodayNewContacts int    `json:"today_new_contacts"`
	DailyLimit      int     `json:"daily_limit"`
}

type CreateCheckoutRequest struct {
	Plan      string `json:"plan" validate:"required,oneof=monthly annual"`
	Email     string `json:"email" validate:"required,email"`
	FirstName string `json:"first_name" validate:"required"`
	LastName  string `json:"last_name" validate:"required"`
	Company   string `json:"company" validate:"required"`
}

type CreateCheckoutResponse struct {
	ClientSecret string `json:"client_secret"`
	CustomerID   string `json:"customer_id"`
	SessionID    string `json:"session_id"`
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
)

type DeviceSendPayload struct {
	MessageID      string `json:"message_id"`
	PhoneNumber    string `json:"phone_number"`    // sending from
	ToAddress      string `json:"to_address"`      // sending to
	Content        string `json:"content"`
	IMessageAddress string `json:"imessage_address"` // iMessage handle to send from
}

type DeviceInboundPayload struct {
	IMessageGUID string    `json:"imessage_guid"`
	FromAddress  string    `json:"from_address"`
	ToAddress    string    `json:"to_address"` // which number/address on device received it
	Content      string    `json:"content"`
	ReceivedAt   time.Time `json:"received_at"`
}

type DeviceStatusPayload struct {
	DeviceID     string `json:"device_id"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message,omitempty"`
}
