-- Add status column to idempotency_keys for atomic reservation state tracking.
-- PENDING: key reserved inside the financial transaction; response not yet recorded.
-- COMPLETED: Midtrans responded successfully; response cached.
-- FAILED: Midtrans returned a definitive rejection; operation failed permanently.
ALTER TABLE idempotency_keys
    ALTER COLUMN response_code DROP NOT NULL,
    ALTER COLUMN response_body DROP NOT NULL,
    ALTER COLUMN expires_at SET DEFAULT NOW() + INTERVAL '24 hours',
    ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'PENDING',
    ADD COLUMN order_id VARCHAR(64);

-- Partial index to allow fast lookup of completed records
CREATE INDEX IF NOT EXISTS idx_idempotency_status ON idempotency_keys(user_id, idempotency_key, status);
CREATE INDEX IF NOT EXISTS idx_idempotency_order ON idempotency_keys(order_id);
