-- Track which underlying service delivered each message (iMessage vs SMS via
-- iPhone Continuity). Default 'imessage' since that's what every prior message
-- went over.
ALTER TABLE messages
  ADD COLUMN service TEXT NOT NULL DEFAULT 'imessage'
  CHECK (service IN ('imessage', 'sms'));

CREATE INDEX idx_messages_service ON messages (service);
