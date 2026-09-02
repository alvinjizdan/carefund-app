CREATE TABLE settlements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL UNIQUE,
    gross_amount BIGINT NOT NULL CHECK (gross_amount >= 0),
    refund_amount BIGINT NOT NULL CHECK (refund_amount >= 0),
    platform_fee BIGINT NOT NULL CHECK (platform_fee >= 0),
    net_amount BIGINT NOT NULL CHECK (net_amount >= 0),
    status VARCHAR(30) NOT NULL,
    calculated_at TIMESTAMPTZ NULL,
    approved_at TIMESTAMPTZ NULL,
    executed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (campaign_id) REFERENCES campaigns(id)
);
CREATE UNIQUE INDEX idx_settlements_campaign_id ON settlements(campaign_id);
CREATE INDEX idx_settlements_status ON settlements(status);
