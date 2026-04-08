-- BlueSend Billing Schema
-- 003_billing.up.sql

-- ============================================================
-- BILLING EVENTS (Stripe webhook log — idempotency)
-- ============================================================
CREATE TABLE billing_events (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id      UUID REFERENCES accounts(id) ON DELETE SET NULL,
    stripe_event_id TEXT NOT NULL UNIQUE,
    event_type      TEXT NOT NULL,
    payload         JSONB NOT NULL,
    processed       BOOLEAN NOT NULL DEFAULT FALSE,
    error           TEXT,
    processed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_billing_events_account_id ON billing_events(account_id);
CREATE INDEX idx_billing_events_stripe_event_id ON billing_events(stripe_event_id);

-- ============================================================
-- INVOICES (mirror of Stripe invoices for dashboard display)
-- ============================================================
CREATE TABLE invoices (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id          UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    stripe_invoice_id   TEXT NOT NULL UNIQUE,
    amount_due          INT NOT NULL,   -- cents
    amount_paid         INT NOT NULL DEFAULT 0,
    currency            TEXT NOT NULL DEFAULT 'usd',
    status              TEXT NOT NULL,
    invoice_pdf_url     TEXT,
    period_start        TIMESTAMPTZ,
    period_end          TIMESTAMPTZ,
    paid_at             TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_invoices_account_id ON invoices(account_id);
