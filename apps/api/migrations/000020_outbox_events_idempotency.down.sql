DROP INDEX idx_outbox_events_idempotency_key;
ALTER TABLE outbox_events DROP COLUMN idempotency_key;
