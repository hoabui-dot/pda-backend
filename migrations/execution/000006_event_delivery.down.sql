DROP TABLE IF EXISTS event_dlq;
DROP TABLE IF EXISTS event_inbox;
DROP INDEX IF EXISTS domain_outbox_pending_idx;
ALTER TABLE domain_outbox DROP COLUMN IF EXISTS last_error;
ALTER TABLE domain_outbox DROP COLUMN IF EXISTS available_at;
ALTER TABLE domain_outbox DROP COLUMN IF EXISTS attempts;
