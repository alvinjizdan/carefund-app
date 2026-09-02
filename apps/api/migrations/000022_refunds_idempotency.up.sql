TRUNCATE TABLE refunds CASCADE;
ALTER TABLE refunds ADD COLUMN idempotency_key VARCHAR(100) NOT NULL;
ALTER TABLE refunds ADD CONSTRAINT refunds_idempotency_key_key UNIQUE (idempotency_key);

