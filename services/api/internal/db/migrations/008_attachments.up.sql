-- Add attachments support to messages
-- 008_attachments.up.sql

ALTER TABLE messages ADD COLUMN IF NOT EXISTS attachments JSONB NOT NULL DEFAULT '[]';

CREATE INDEX IF NOT EXISTS idx_messages_has_attachments
    ON messages ((jsonb_array_length(attachments) > 0))
    WHERE jsonb_array_length(attachments) > 0;
