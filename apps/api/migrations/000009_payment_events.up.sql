CREATE TABLE payment_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id UUID NOT NULL,
    provider VARCHAR(30) NOT NULL,
    idempotency_key VARCHAR(255) UNIQUE NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    provider_status VARCHAR(50) NULL,
    payload JSONB NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ NULL,
    processing_status VARCHAR(30) NOT NULL,
    FOREIGN KEY (payment_id) REFERENCES payments(id)
);
CREATE UNIQUE INDEX idx_payment_events_idempotency_key ON payment_events(idempotency_key);
CREATE INDEX idx_payment_events_payment_id ON payment_events(payment_id);
