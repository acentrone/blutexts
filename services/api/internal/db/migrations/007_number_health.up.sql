-- Add area code preference to accounts
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS preferred_area_code TEXT;

-- Add health tracking to phone numbers
ALTER TABLE phone_numbers ADD COLUMN IF NOT EXISTS health_status TEXT NOT NULL DEFAULT 'healthy'
    CHECK (health_status IN ('healthy', 'warning', 'flagged', 'blocked'));
ALTER TABLE phone_numbers ADD COLUMN IF NOT EXISTS health_notes TEXT;
ALTER TABLE phone_numbers ADD COLUMN IF NOT EXISTS last_send_success_at TIMESTAMPTZ;
ALTER TABLE phone_numbers ADD COLUMN IF NOT EXISTS last_send_failure_at TIMESTAMPTZ;
ALTER TABLE phone_numbers ADD COLUMN IF NOT EXISTS consecutive_failures INT NOT NULL DEFAULT 0;
