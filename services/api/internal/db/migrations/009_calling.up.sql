-- ============================================================
-- FaceTime Audio calling via Agora bridge
-- ============================================================
-- Feature gate on accounts (upsell)
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS calling_enabled BOOLEAN NOT NULL DEFAULT FALSE;

-- Per-number opt-in (so a tenant with multiple numbers can enable
-- calling on a subset).
ALTER TABLE phone_numbers
    ADD COLUMN IF NOT EXISTS voice_enabled BOOLEAN NOT NULL DEFAULT FALSE;

-- ============================================================
-- CALL LOGS
-- ============================================================
CREATE TABLE IF NOT EXISTS call_logs (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id          UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    phone_number_id     UUID REFERENCES phone_numbers(id) ON DELETE SET NULL,
    device_id           UUID REFERENCES devices(id) ON DELETE SET NULL,
    direction           TEXT NOT NULL CHECK (direction IN ('inbound', 'outbound')),
    from_number         TEXT NOT NULL,
    to_number           TEXT NOT NULL,
    agora_channel       TEXT NOT NULL UNIQUE,
    status              TEXT NOT NULL DEFAULT 'initiated' CHECK (
                            status IN ('initiated', 'ringing', 'connected', 'completed', 'failed', 'missed', 'cancelled')
                        ),
    failure_reason      TEXT,
    duration_seconds    INT,
    started_at          TIMESTAMPTZ,
    connected_at        TIMESTAMPTZ,
    ended_at            TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_call_logs_account_id ON call_logs(account_id);
CREATE INDEX IF NOT EXISTS idx_call_logs_phone_number_id ON call_logs(phone_number_id);
CREATE INDEX IF NOT EXISTS idx_call_logs_created_at ON call_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_call_logs_status ON call_logs(status);
