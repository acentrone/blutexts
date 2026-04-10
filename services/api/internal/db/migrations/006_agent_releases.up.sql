-- Agent release management for auto-updates
-- 006_agent_releases.up.sql

CREATE TABLE agent_releases (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    version      TEXT NOT NULL UNIQUE,
    download_url TEXT NOT NULL,
    notes        TEXT NOT NULL DEFAULT '',
    required     BOOLEAN NOT NULL DEFAULT FALSE,
    active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
