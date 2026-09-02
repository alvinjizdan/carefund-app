package service

import (
	"context"
	"time"

	"carefund-api/internal/database"
	"carefund-api/internal/domain"
)

type CampaignService interface {
	CreateCampaign(ctx context.Context, ownerID, categoryID, title, desc string, target int64, start, end time.Time) (*domain.Campaign, error)
	GetCampaign(ctx context.Context, id string) (*domain.Campaign, error)
	ListCampaigns(ctx context.Context, limit, offset int) ([]*domain.Campaign, error)
	UpdateCampaign(ctx context.Context, actorID, id, title, desc, categoryID string, target int64, start, end time.Time) (*domain.Campaign, error)
	SubmitForReview(ctx context.Context, actorID, id string) error
	ApproveCampaign(ctx context.Context, adminID, id string) error
	RejectCampaign(ctx context.Context, adminID, id, reason string) error
	SuspendCampaign(ctx context.Context, adminID, id string) error
	CompleteCampaign(ctx context.Context, actorID, id string) error
}

type campaignService struct {
	campaignRepo domain.CampaignRepository
	tx           database.TransactionManager
}

func NewCampaignService(campRepo domain.CampaignRepository, tx database.TransactionManager) CampaignService {
	return &campaignService{
		campaignRepo: campRepo,
		tx:           tx,
	}
}

func (s *campaignService) CreateCampaign(ctx context.Context, ownerID, categoryID, title, desc string, target int64, start, end time.Time) (*domain.Campaign, error) {
	if target <= 0 || end.Before(start) {
		return nil, domain.ErrInvalidInput
	}

	campaign := &domain.Campaign{
		OwnerID:       ownerID,
		CategoryID:    categoryID,
		Title:         title,
		Slug:          title + "-" + time.Now().Format("20060102150405"),
		Description:   desc,
		TargetAmount:  target,
		CurrentAmount: 0,
		StartAt:       start,
		EndAt:         end,
		Status:        domain.CampaignStateDraft,
	}

	if err := s.campaignRepo.Create(ctx, campaign); err != nil {
		return nil, err
	}
	return campaign, nil
}

func (s *campaignService) GetCampaign(ctx context.Context, id string) (*domain.Campaign, error) {
	return s.campaignRepo.FindByID(ctx, id)
}

func (s *campaignService) ListCampaigns(ctx context.Context, limit, offset int) ([]*domain.Campaign, error) {
	return s.campaignRepo.List(ctx, limit, offset)
}

func (s *campaignService) UpdateCampaign(ctx context.Context, actorID, id, title, desc, categoryID string, target int64, start, end time.Time) (*domain.Campaign, error) {
	if target <= 0 || end.Before(start) {
		return nil, domain.ErrInvalidInput
	}

	var campaign *domain.Campaign
	err := s.tx.Do(ctx, func(txCtx context.Context) error {
		c, err := s.campaignRepo.FindByIDForUpdate(txCtx, id)
		if err != nil {
			return err
		}

		// Owner check
		if c.OwnerID != actorID {
			return domain.ErrForbidden
		}

		// Domain rules: usually you can only modify if DRAFT or PENDING_REVIEW or ACTIVE if extending end_at etc.
		// For simplicity, we just allow updates unless terminal.
		if c.Status == domain.CampaignStateCompleted || c.Status == domain.CampaignStateCancelled {
			return domain.ErrInvalidStateTransition
		}

		c.Title = title
		c.Description = desc
		c.CategoryID = categoryID
		c.TargetAmount = target
		c.StartAt = start
		c.EndAt = end

		if err := s.campaignRepo.Update(txCtx, c); err != nil {
			return err
		}
		campaign = c
		return nil
	})

	return campaign, err
}

func (s *campaignService) SubmitForReview(ctx context.Context, actorID, id string) error {
	return s.changeState(ctx, actorID, id, domain.CampaignStatePendingReview, false, "")
}

func (s *campaignService) ApproveCampaign(ctx context.Context, adminID, id string) error {
	// Admin check is done at HTTP/handler level via RBAC middleware
	return s.changeState(ctx, adminID, id, domain.CampaignStateActive, true, "")
}

func (s *campaignService) RejectCampaign(ctx context.Context, adminID, id, reason string) error {
	return s.changeState(ctx, adminID, id, domain.CampaignStateRejected, true, reason)
}

func (s *campaignService) SuspendCampaign(ctx context.Context, adminID, id string) error {
	return s.changeState(ctx, adminID, id, domain.CampaignStateSuspended, true, "")
}

func (s *campaignService) CompleteCampaign(ctx context.Context, actorID, id string) error {
	return s.changeState(ctx, actorID, id, domain.CampaignStateCompleted, false, "")
}

func (s *campaignService) changeState(ctx context.Context, actorID, id, nextState string, isAdminAction bool, reason string) error {
	return s.tx.Do(ctx, func(txCtx context.Context) error {
		c, err := s.campaignRepo.FindByIDForUpdate(txCtx, id)
		if err != nil {
			return err
		}

		if !isAdminAction && c.OwnerID != actorID {
			return domain.ErrForbidden
		}

		if !c.IsValidTransition(nextState) {
			return domain.ErrInvalidStateTransition
		}

		c.Status = nextState
		if reason != "" {
			c.RejectionReason = &reason
		}

		return s.campaignRepo.Update(txCtx, c)
	})
}
