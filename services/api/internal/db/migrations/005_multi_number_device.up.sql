-- Support multi-number devices (Mac Mini hub + forwarded iPhones)
-- 005_multi_number_device.up.sql

-- Increase default device capacity to reflect hub model
ALTER TABLE devices ALTER COLUMN capacity SET DEFAULT 10;

-- Track which handles a device has reported (for routing inbound messages)
ALTER TABLE devices ADD COLUMN IF NOT EXISTS reported_handles TEXT[] DEFAULT '{}';
