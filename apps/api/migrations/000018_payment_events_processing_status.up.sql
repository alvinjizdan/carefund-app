ALTER TABLE payment_events ADD COLUMN rejection_reason VARCHAR(100) NULL;
ALTER TABLE payment_events ALTER COLUMN payment_id DROP NOT NULL;
ALTER TABLE payment_events ADD CONSTRAINT chk_processing_status CHECK (processing_status IN ('RECEIVED', 'PROCESSED', 'REJECTED'));
