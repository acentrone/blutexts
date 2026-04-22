DROP INDEX IF EXISTS idx_contacts_imessage_capable;
ALTER TABLE contacts
  DROP COLUMN IF EXISTS imessage_capable,
  DROP COLUMN IF EXISTS imessage_checked_at;
