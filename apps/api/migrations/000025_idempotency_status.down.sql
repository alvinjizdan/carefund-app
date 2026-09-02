ALTER TABLE idempotency_keys
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS order_id,
    ALTER COLUMN response_code SET NOT NULL,
    ALTER COLUMN response_body SET NOT NULL,
    ALTER COLUMN expires_at DROP DEFAULT;

DROP INDEX IF EXISTS idx_idempotency_status;
DROP INDEX IF EXISTS idx_idempotency_order;
