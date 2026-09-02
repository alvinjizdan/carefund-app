package domain

import (
	"context"
	"time"
)

// Campaign States
const (
	CampaignStateDraft         = "DRAFT"
	CampaignStatePendingReview = "PENDING_REVIEW"
	CampaignStateRejected      = "REJECTED"
	CampaignStateActive        = "ACTIVE"
	CampaignStateSuspended     = "SUSPENDED"
	CampaignStateCompleted     = "COMPLETED"
	CampaignStateCancelled     = "CANCELLED"
)

type Campaign struct {
	ID              string
	OwnerID         string
	CategoryID      string
	Title           string
	Slug            string
	Description     string
	TargetAmount    int64
	CurrentAmount   int64
	StartAt         time.Time
	EndAt           time.Time
	Status          string
	RejectionReason *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// IsValidTransition checks if the state transition is permitted
func (c *Campaign) IsValidTransition(nextState string) bool {
	switch c.Status {
	case CampaignStateDraft:
		return nextState == CampaignStatePendingReview
	case CampaignStatePendingReview:
		return nextState == CampaignStateActive || nextState == CampaignStateRejected
	case CampaignStateActive:
		return nextState == CampaignStateSuspended || nextState == CampaignStateCompleted || nextState == CampaignStateCancelled
	case CampaignStateSuspended:
		return nextState == CampaignStateActive || nextState == CampaignStateCancelled
	default:
		return false // REJECTED, COMPLETED, CANCELLED are terminal or not defined further
	}
}

type CampaignRepository interface {
	Create(ctx context.Context, campaign *Campaign) error
	FindByID(ctx context.Context, id string) (*Campaign, error)
	FindByIDForUpdate(ctx context.Context, id string) (*Campaign, error)
	FindBySlug(ctx context.Context, slug string) (*Campaign, error)
	Update(ctx context.Context, campaign *Campaign) error
	List(ctx context.Context, limit, offset int) ([]*Campaign, error)
	ListByOwner(ctx context.Context, ownerID string, limit, offset int) ([]*Campaign, error)
}
