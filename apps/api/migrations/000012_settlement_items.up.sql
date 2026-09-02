CREATE TABLE settlement_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    settlement_id UUID NOT NULL,
    donation_id UUID NOT NULL,
    payment_id UUID NOT NULL,
    eligible_amount BIGINT NOT NULL CHECK (eligible_amount >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (settlement_id) REFERENCES settlements(id),
    FOREIGN KEY (donation_id) REFERENCES donations(id),
    FOREIGN KEY (payment_id) REFERENCES payments(id)
);
