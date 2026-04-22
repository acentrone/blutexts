-- Cache whether each contact is reachable via iMessage. Determined by the
-- device agent's IMHandleAvailabilityChecker query (which round-trips to
-- Apple's identity servers). Caching here avoids the 3-second wait on every
-- send and lets the web UI disable effects / voice messages for SMS-only
-- contacts up-front.
--
-- NULL = unknown (we haven't checked yet — first send will determine)
-- TRUE  = registered with iMessage (blue bubbles, effects, voice work)
-- FALSE = not on iMessage (route via SMS / Continuity, green bubbles)
ALTER TABLE contacts
  ADD COLUMN imessage_capable    BOOLEAN,
  ADD COLUMN imessage_checked_at TIMESTAMPTZ;

CREATE INDEX idx_contacts_imessage_capable ON contacts (imessage_capable)
  WHERE imessage_capable IS NOT NULL;
