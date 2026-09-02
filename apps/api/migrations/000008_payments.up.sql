CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    donation_id UUID NOT NULL UNIQUE,
    provider VARCHAR(30) NOT NULL,
    order_id VARCHAR(100) UNIQUE NOT NULL,
    transaction_id VARCHAR(150) NULL,
    payment_type VARCHAR(50) NULL,
    gross_amount BIGINT NOT NULL CHECK (gross_amount > 0),
    status VARCHAR(30) NOT NULL,
    fraud_status VARCHAR(30) NULL,
    transaction_at TIMESTAMPTZ NULL,
    settled_at TIMESTAMPTZ NULL,
    expired_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (donation_id) REFERENCES donations(id)
);
CREATE UNIQUE INDEX idx_payments_order_id ON payments(order_id);
CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_donation_id ON payments(donation_id);
