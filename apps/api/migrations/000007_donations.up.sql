CREATE TABLE donations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL,
    donor_id UUID NULL,
    amount BIGINT NOT NULL CHECK (amount > 0),
    is_anonymous BOOLEAN NOT NULL DEFAULT false,
    message TEXT NULL,
    status VARCHAR(30) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (campaign_id) REFERENCES campaigns(id),
    FOREIGN KEY (donor_id) REFERENCES users(id)
);
CREATE INDEX idx_donations_campaign_id ON donations(campaign_id);
CREATE INDEX idx_donations_donor_id ON donations(donor_id);
CREATE INDEX idx_donations_status ON donations(status);
