ALTER TABLE outbox_events ADD COLUMN processing_started_at TIMESTAMPTZ NULL;
