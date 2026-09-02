DROP INDEX IF EXISTS idx_refunds_idempotency_key;
ALTER TABLE refunds DROP COLUMN IF EXISTS idempotency_key;

