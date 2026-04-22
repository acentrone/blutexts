-- Account-level custom field schema: defines which custom fields exist for this account.
-- Format: [{"key":"lead_source","label":"Lead Source","type":"select","required":false,"options":["Web","Referral","Cold Call"]}]
-- Supported types: text, number, select, date, url
ALTER TABLE accounts ADD COLUMN custom_field_schema JSONB NOT NULL DEFAULT '[]';
