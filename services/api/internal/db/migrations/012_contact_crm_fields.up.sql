-- 012_contact_crm_fields.up.sql
-- Adds CRM fields to contacts: email, company, notes, custom fields.

ALTER TABLE contacts
    ADD COLUMN email    TEXT,
    ADD COLUMN company  TEXT,
    ADD COLUMN notes    TEXT NOT NULL DEFAULT '',
    ADD COLUMN custom_fields JSONB NOT NULL DEFAULT '{}';

CREATE INDEX idx_contacts_email ON contacts(email) WHERE email IS NOT NULL;
CREATE INDEX idx_contacts_tags ON contacts USING gin(tags);
