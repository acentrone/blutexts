-- BlueSend Initial Schema Migration
-- 001_initial.up.sql

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm"; -- for full-text search on messages

-- ============================================================
-- ACCOUNTS (SaaS customers / businesses)
-- ============================================================
CREATE TABLE accounts (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name                    TEXT NOT NULL,
    email                   TEXT NOT NULL UNIQUE,
    stripe_customer_id      TEXT UNIQUE,
    stripe_subscription_id  TEXT UNIQUE,
    plan                    TEXT NOT NULL DEFAULT 'pending' CHECK (plan IN ('pending', 'monthly', 'annual')),
    status                  TEXT NOT NULL DEFAULT 'pending' CHECK (
                                status IN ('pending', 'setting_up', 'active', 'past_due', 'cancelled', 'suspended')
                            ),
    setup_complete          BOOLEAN NOT NULL DEFAULT FALSE,
    setup_fee_paid          BOOLEAN NOT NULL DEFAULT FALSE,
    timezone                TEXT NOT NULL DEFAULT 'America/New_York',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- USERS (people who log into the dashboard)
-- ============================================================
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id      UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    email           TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    first_name      TEXT,
    last_name       TEXT,
    role            TEXT NOT NULL DEFAULT 'owner' CHECK (role IN ('owner', 'member', 'admin')),
    last_login_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_account_id ON users(account_id);
CREATE INDEX idx_users_email ON users(email);

-- ============================================================
-- DEVICES (physical Mac Minis / iPhones)
-- ============================================================
CREATE TABLE devices (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name             TEXT NOT NULL,
    type             TEXT NOT NULL CHECK (type IN ('mac_mini', 'iphone')),
    serial_number    TEXT UNIQUE,
    device_token     TEXT NOT NULL UNIQUE, -- used by agent to authenticate WebSocket
    status           TEXT NOT NULL DEFAULT 'offline' CHECK (
                         status IN ('online', 'offline', 'error', 'maintenance')
                     ),
    last_seen_at     TIMESTAMPTZ,
    ip_address       INET,
    agent_version    TEXT,
    os_version       TEXT,
    capacity         INT NOT NULL DEFAULT 5,  -- max phone numbers this device can host
    assigned_count   INT NOT NULL DEFAULT 0,
    error_message    TEXT,
    metadata         JSONB NOT NULL DEFAULT '{}',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- PHONE NUMBERS (dedicated numbers per account)
-- ============================================================
CREATE TABLE phone_numbers (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id              UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    device_id               UUID REFERENCES devices(id),
    number                  TEXT NOT NULL UNIQUE, -- E.164 format, e.g. +15551234567
    display_name            TEXT,
    imessage_address        TEXT, -- could differ from phone number (email-based iMessage)
    status                  TEXT NOT NULL DEFAULT 'provisioning' CHECK (
                                status IN ('provisioning', 'active', 'suspended', 'deprovisioned')
                            ),
    daily_new_contact_limit INT NOT NULL DEFAULT 50,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_phone_numbers_account_id ON phone_numbers(account_id);
CREATE INDEX idx_phone_numbers_device_id ON phone_numbers(device_id);
CREATE INDEX idx_phone_numbers_number ON phone_numbers(number);

-- ============================================================
-- CONTACTS
-- ============================================================
CREATE TABLE contacts (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id          UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    phone_number_id     UUID REFERENCES phone_numbers(id),
    imessage_address    TEXT NOT NULL,  -- the address used for iMessage (phone or email)
    name                TEXT,
    ghl_contact_id      TEXT,           -- linked GHL contact ID
    first_message_at    TIMESTAMPTZ,
    last_message_at     TIMESTAMPTZ,
    message_count       INT NOT NULL DEFAULT 0,
    tags                TEXT[] DEFAULT '{}',
    metadata            JSONB NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, imessage_address)
);

CREATE INDEX idx_contacts_account_id ON contacts(account_id);
CREATE INDEX idx_contacts_phone_number_id ON contacts(phone_number_id);
CREATE INDEX idx_contacts_imessage_address ON contacts(imessage_address);
CREATE INDEX idx_contacts_ghl_contact_id ON contacts(ghl_contact_id);

-- ============================================================
-- CONVERSATIONS
-- ============================================================
CREATE TABLE conversations (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id              UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    phone_number_id         UUID NOT NULL REFERENCES phone_numbers(id),
    contact_id              UUID NOT NULL REFERENCES contacts(id),
    ghl_conversation_id     TEXT,       -- linked GHL conversation ID
    last_message_at         TIMESTAMPTZ,
    last_message_preview    TEXT,
    message_count           INT NOT NULL DEFAULT 0,
    unread_count            INT NOT NULL DEFAULT 0,
    status                  TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed', 'archived')),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (phone_number_id, contact_id)
);

CREATE INDEX idx_conversations_account_id ON conversations(account_id);
CREATE INDEX idx_conversations_contact_id ON conversations(contact_id);
CREATE INDEX idx_conversations_phone_number_id ON conversations(phone_number_id);
CREATE INDEX idx_conversations_ghl_conversation_id ON conversations(ghl_conversation_id);
CREATE INDEX idx_conversations_last_message_at ON conversations(last_message_at DESC);

-- ============================================================
-- MESSAGES
-- ============================================================
CREATE TABLE messages (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    conversation_id     UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    account_id          UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    phone_number_id     UUID NOT NULL REFERENCES phone_numbers(id),
    contact_id          UUID NOT NULL REFERENCES contacts(id),
    direction           TEXT NOT NULL CHECK (direction IN ('inbound', 'outbound')),
    content             TEXT NOT NULL,
    imessage_guid       TEXT UNIQUE,    -- Apple's internal message GUID
    status              TEXT NOT NULL DEFAULT 'pending' CHECK (
                            status IN ('pending', 'sent', 'delivered', 'read', 'failed')
                        ),
    sent_at             TIMESTAMPTZ,
    delivered_at        TIMESTAMPTZ,
    read_at             TIMESTAMPTZ,
    failed_at           TIMESTAMPTZ,
    error_message       TEXT,
    ghl_message_id      TEXT,           -- synced GHL message ID
    ghl_synced_at       TIMESTAMPTZ,
    metadata            JSONB NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_messages_conversation_id ON messages(conversation_id);
CREATE INDEX idx_messages_account_id ON messages(account_id);
CREATE INDEX idx_messages_contact_id ON messages(contact_id);
CREATE INDEX idx_messages_imessage_guid ON messages(imessage_guid);
CREATE INDEX idx_messages_created_at ON messages(created_at DESC);
CREATE INDEX idx_messages_content_search ON messages USING gin(to_tsvector('english', content));

-- ============================================================
-- RATE LIMIT TRACKING
-- ============================================================
CREATE TABLE rate_limit_daily (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    phone_number_id     UUID NOT NULL REFERENCES phone_numbers(id) ON DELETE CASCADE,
    contact_address     TEXT NOT NULL,
    date                DATE NOT NULL DEFAULT CURRENT_DATE,
    is_new_contact      BOOLEAN NOT NULL DEFAULT TRUE,
    message_count       INT NOT NULL DEFAULT 1,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (phone_number_id, contact_address, date)
);

CREATE INDEX idx_rate_limit_phone_date ON rate_limit_daily(phone_number_id, date);

-- ============================================================
-- ONBOARDING SESSIONS (track multi-step signup)
-- ============================================================
CREATE TABLE onboarding_sessions (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email                   TEXT NOT NULL,
    account_id              UUID REFERENCES accounts(id),
    step                    TEXT NOT NULL DEFAULT 'account',
    stripe_payment_intent   TEXT,
    stripe_session_id       TEXT,
    plan_selected           TEXT,
    completed               BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at              TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '2 hours'),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- REFRESH TOKENS
-- ============================================================
CREATE TABLE refresh_tokens (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_token_hash ON refresh_tokens(token_hash);

-- ============================================================
-- AUDIT LOG
-- ============================================================
CREATE TABLE audit_log (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id  UUID REFERENCES accounts(id),
    user_id     UUID REFERENCES users(id),
    action      TEXT NOT NULL,
    entity_type TEXT,
    entity_id   UUID,
    details     JSONB NOT NULL DEFAULT '{}',
    ip_address  INET,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_log_account_id ON audit_log(account_id);
CREATE INDEX idx_audit_log_created_at ON audit_log(created_at DESC);

-- ============================================================
-- TRIGGERS: auto-update updated_at columns
-- ============================================================
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER accounts_updated_at BEFORE UPDATE ON accounts FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER devices_updated_at BEFORE UPDATE ON devices FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER phone_numbers_updated_at BEFORE UPDATE ON phone_numbers FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER contacts_updated_at BEFORE UPDATE ON contacts FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER conversations_updated_at BEFORE UPDATE ON conversations FOR EACH ROW EXECUTE FUNCTION update_updated_at();
