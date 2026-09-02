CREATE INDEX idx_outbox_events_dead_letter ON outbox_events(status) WHERE status = 'DEAD_LETTER';
