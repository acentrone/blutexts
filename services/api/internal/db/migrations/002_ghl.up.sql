-- BlueSend GHL Integration Schema
-- 002_ghl.up.sql

-- ============================================================
-- GHL CONNECTIONS (per account)
-- ============================================================
CREATE TABLE ghl_connections (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id          UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE UNIQUE,
    location_id         TEXT NOT NULL UNIQUE,   -- GHL sub-account/location ID
    access_token        TEXT NOT NULL,
    refresh_token       TEXT NOT NULL,
    token_expires_at    TIMESTAMPTZ NOT NULL,
    pipeline_id         TEXT,                   -- GHL pipeline for BlueSend leads
    custom_channel_id   TEXT,                   -- GHL custom channel we register
    webhook_id          TEXT,                   -- GHL webhook registration ID
    connected           BOOLEAN NOT NULL DEFAULT FALSE,
    last_synced_at      TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ghl_connections_location_id ON ghl_connections(location_id);

-- ============================================================
-- GHL SYNC QUEUE (track pending syncs for retry)
-- ============================================================
CREATE TABLE ghl_sync_queue (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id      UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    entity_type     TEXT NOT NULL CHECK (entity_type IN ('message', 'contact', 'conversation')),
    entity_id       UUID NOT NULL,
    direction       TEXT NOT NULL CHECK (direction IN ('to_ghl', 'from_ghl')),
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    attempts        INT NOT NULL DEFAULT 0,
    last_error      TEXT,
    scheduled_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ghl_sync_queue_status ON ghl_sync_queue(status, scheduled_at);
CREATE INDEX idx_ghl_sync_queue_account_id ON ghl_sync_queue(account_id);

-- ============================================================
-- GHL WEBHOOK EVENTS LOG
-- ============================================================
CREATE TABLE ghl_webhook_events (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    location_id     TEXT NOT NULL,
    event_type      TEXT NOT NULL,
    payload         JSONB NOT NULL,
    processed       BOOLEAN NOT NULL DEFAULT FALSE,
    error           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ghl_webhook_events_location_id ON ghl_webhook_events(location_id);
CREATE INDEX idx_ghl_webhook_events_processed ON ghl_webhook_events(processed, created_at);

CREATE TRIGGER ghl_connections_updated_at BEFORE UPDATE ON ghl_connections FOR EACH ROW EXECUTE FUNCTION update_updated_at();
