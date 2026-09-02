ALTER TABLE payment_events DROP CONSTRAINT chk_processing_status;
ALTER TABLE payment_events DROP COLUMN rejection_reason;
