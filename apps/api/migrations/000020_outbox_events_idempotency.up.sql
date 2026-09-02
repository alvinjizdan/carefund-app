ALTER TABLE outbox_events ADD COLUMN idempotency_key VARCHAR(255);
CREATE UNIQUE INDEX idx_outbox_events_idempotency_key ON outbox_events(idempotency_key);
