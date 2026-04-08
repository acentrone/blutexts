package messaging

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const (
	DefaultDailyNewContactLimit = 50
)

// RateLimiter enforces the 50 new contacts/day rule per phone number.
// Existing contacts (those with prior conversation history) bypass the limit entirely.
type RateLimiter struct {
	db    *pgxpool.Pool
	redis *redis.Client
}

func NewRateLimiter(db *pgxpool.Pool, rdb *redis.Client) *RateLimiter {
	return &RateLimiter{db: db, redis: rdb}
}

type CheckResult struct {
	Allowed      bool
	IsNewContact bool
	Used         int
	Limit        int
	ResetsAt     time.Time
}

// Check determines whether sending to toAddress from phoneNumberID is allowed.
// It uses Redis for fast counter reads and PostgreSQL as the source of truth.
func (r *RateLimiter) Check(ctx context.Context, phoneNumberID uuid.UUID, toAddress string) (*CheckResult, error) {
	// 1. Determine if this is a new contact (no prior messages)
	isNew, err := r.isNewContact(ctx, phoneNumberID, toAddress)
	if err != nil {
		return nil, fmt.Errorf("check new contact: %w", err)
	}

	// Existing contacts are always allowed
	if !isNew {
		return &CheckResult{Allowed: true, IsNewContact: false}, nil
	}

	// 2. For new contacts, check the daily counter
	limit, err := r.getDailyLimit(ctx, phoneNumberID)
	if err != nil {
		return nil, fmt.Errorf("get daily limit: %w", err)
	}

	used, err := r.getDailyCount(ctx, phoneNumberID)
	if err != nil {
		return nil, fmt.Errorf("get daily count: %w", err)
	}

	resetsAt := midnight()

	if used >= limit {
		return &CheckResult{
			Allowed:      false,
			IsNewContact: true,
			Used:         used,
			Limit:        limit,
			ResetsAt:     resetsAt,
		}, nil
	}

	return &CheckResult{
		Allowed:      true,
		IsNewContact: true,
		Used:         used,
		Limit:        limit,
		ResetsAt:     resetsAt,
	}, nil
}

// Record increments the daily new contact counter after a successful send.
// Must be called after a message is successfully queued.
func (r *RateLimiter) Record(ctx context.Context, phoneNumberID uuid.UUID, toAddress string, isNew bool) error {
	if !isNew {
		return nil // no counter to update for existing contacts
	}

	// Upsert in PostgreSQL
	_, err := r.db.Exec(ctx, `
		INSERT INTO rate_limit_daily (phone_number_id, contact_address, date, is_new_contact, message_count)
		VALUES ($1, $2, CURRENT_DATE, true, 1)
		ON CONFLICT (phone_number_id, contact_address, date)
		DO UPDATE SET message_count = rate_limit_daily.message_count + 1
	`, phoneNumberID, toAddress)
	if err != nil {
		return fmt.Errorf("upsert rate limit entry: %w", err)
	}

	// Increment Redis counter (fast path for future checks)
	key := dailyCountKey(phoneNumberID)
	pipe := r.redis.Pipeline()
	pipe.Incr(ctx, key)
	pipe.ExpireAt(ctx, key, midnight())
	_, err = pipe.Exec(ctx)
	return err
}

// isNewContact returns true if there are no prior messages between
// this phone number and this contact address.
func (r *RateLimiter) isNewContact(ctx context.Context, phoneNumberID uuid.UUID, toAddress string) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM contacts
		WHERE phone_number_id = $1 AND imessage_address = $2
	`, phoneNumberID, toAddress).Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

// getDailyLimit fetches the configured limit for this phone number.
func (r *RateLimiter) getDailyLimit(ctx context.Context, phoneNumberID uuid.UUID) (int, error) {
	var limit int
	err := r.db.QueryRow(ctx, `
		SELECT daily_new_contact_limit FROM phone_numbers WHERE id = $1
	`, phoneNumberID).Scan(&limit)
	if err != nil {
		return DefaultDailyNewContactLimit, err
	}
	return limit, nil
}

// getDailyCount returns the number of new contacts messaged today.
// Uses Redis for speed; falls back to PostgreSQL on cache miss.
func (r *RateLimiter) getDailyCount(ctx context.Context, phoneNumberID uuid.UUID) (int, error) {
	key := dailyCountKey(phoneNumberID)
	val, err := r.redis.Get(ctx, key).Int()
	if err == nil {
		return val, nil
	}
	if err != redis.Nil {
		return 0, fmt.Errorf("redis get daily count: %w", err)
	}

	// Cache miss: query postgres and warm the cache
	var count int
	err = r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(message_count), 0)
		FROM rate_limit_daily
		WHERE phone_number_id = $1 AND date = CURRENT_DATE AND is_new_contact = true
	`, phoneNumberID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("db count: %w", err)
	}

	// Warm cache
	r.redis.SetEx(ctx, key, count, time.Until(midnight()))
	return count, nil
}

func dailyCountKey(phoneNumberID uuid.UUID) string {
	return fmt.Sprintf("rate_limit:daily:%s:%s", phoneNumberID.String(), time.Now().Format("2006-01-02"))
}

// midnight returns the next midnight in UTC.
func midnight() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
}
