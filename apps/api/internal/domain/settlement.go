package domain

import (
	"context"
	"time"
)

const (
	SettlementStatusPending  = "PENDING"
	SettlementStatusApproved = "APPROVED"
	SettlementStatusExecuted = "EXECUTED"
)

type Settlement struct {
	ID           string
	CampaignID   string
	GrossAmount  int64
	RefundAmount int64
	PlatformFee  int64
	NetAmount    int64
	Status       string
	CalculatedAt *time.Time
	ApprovedAt   *time.Time
	ExecutedAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type SettlementItem struct {
	ID             string
	SettlementID   string
	DonationID     string
	PaymentID      string
	EligibleAmount int64
	CreatedAt      time.Time
}

// Eligibility rules:
// payment.status == CAPTURED
// payment.gross_amount > 0
// donation exists and is PAID
func IsEligibleForSettlement(p *Payment, d *Donation) bool {
	if p == nil || d == nil {
		return false
	}
	if p.Status != PaymentStatusCaptured {
		return false
	}
	if p.GrossAmount <= 0 {
		return false
	}
	if d.Status != DonationStatusPaid {
		return false
	}
	return true
}

type SettlementRepository interface {
	Create(ctx context.Context, s *Settlement) error
	GetByCampaignID(ctx context.Context, campaignID string) (*Settlement, error)
	Update(ctx context.Context, s *Settlement) error
}

type SettlementItemRepository interface {
	Create(ctx context.Context, item *SettlementItem) error
	GetBySettlementID(ctx context.Context, settlementID string) ([]*SettlementItem, error)
	GetByPaymentID(ctx context.Context, paymentID string) (*SettlementItem, error)
}
