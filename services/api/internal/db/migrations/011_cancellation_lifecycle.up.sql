-- 011_cancellation_lifecycle.up.sql
-- Adds cancellation lifecycle columns to accounts for grace period,
-- auto-reply scheduling, and reinstatement support.

ALTER TABLE accounts
    ADD COLUMN cancelled_at         TIMESTAMPTZ,
    ADD COLUMN grace_period_ends_at TIMESTAMPTZ,
    ADD COLUMN auto_reply_starts_at TIMESTAMPTZ,
    ADD COLUMN auto_reply_enabled   BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN auto_reply_message   TEXT NOT NULL DEFAULT 'This number is no longer in service. Please contact the business directly.';

-- Index for future cron that processes accounts whose grace period has expired.
CREATE INDEX idx_accounts_grace_period ON accounts(grace_period_ends_at)
    WHERE grace_period_ends_at IS NOT NULL;
