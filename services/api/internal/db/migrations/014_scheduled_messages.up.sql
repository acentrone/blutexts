CREATE TABLE scheduled_messages (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id      UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    phone_number_id UUID NOT NULL REFERENCES phone_numbers(id),
    to_address      TEXT NOT NULL,
    content         TEXT NOT NULL DEFAULT '',
    attachments     JSONB NOT NULL DEFAULT '[]',
    effect          TEXT,
    scheduled_at    TIMESTAMPTZ NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed', 'cancelled')),
    sent_at         TIMESTAMPTZ,
    error_message   TEXT,
    created_by      UUID NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_scheduled_messages_account ON scheduled_messages(account_id);
CREATE INDEX idx_scheduled_messages_fire ON scheduled_messages(scheduled_at) WHERE status = 'pending';
