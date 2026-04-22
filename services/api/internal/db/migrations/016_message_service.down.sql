DROP INDEX IF EXISTS idx_messages_service;
ALTER TABLE messages DROP COLUMN IF EXISTS service;
