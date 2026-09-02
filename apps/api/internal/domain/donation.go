package domain

import (
	"context"
	"time"
)

const (
	DonationStatusPending           = "PENDING"
	DonationStatusPaid              = "PAID"
	DonationStatusFailed            = "FAILED"
	DonationStatusExpired           = "EXPIRED"
	DonationStatusRefunded          = "REFUNDED"
	DonationStatusPartiallyRefunded = "PARTIALLY_REFUNDED"
	DonationStatusCancelled         = "CANCELLED"
)

type Donation struct {
	ID          string
	CampaignID  string
	DonorID     *string
	Amount      int64
	IsAnonymous bool
	Message     string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (d *Donation) Validate() error {
	if d.Amount <= 0 {
		return ErrInvalidInput
	}
	if d.CampaignID == "" {
		return ErrInvalidInput
	}
	return nil
}

type DonationRepository interface {
	Create(ctx context.Context, donation *Donation) error
	FindByID(ctx context.Context, id string) (*Donation, error)
	ListByUser(ctx context.Context, userID string, limit, offset int) ([]*Donation, error)
	ListByCampaign(ctx context.Context, campaignID string, limit, offset int) ([]*Donation, error)
	Update(ctx context.Context, donation *Donation) error
	UpdateStatus(ctx context.Context, id string, status string) error
}
