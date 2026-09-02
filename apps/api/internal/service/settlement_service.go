package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"carefund-api/internal/database"
	"carefund-api/internal/domain"
)

type SettlementService interface {
	SettleCampaign(ctx context.Context, campaignID string) (*domain.Settlement, error)
}

type settlementService struct {
	campaignRepo       domain.CampaignRepository
	paymentRepo        domain.PaymentRepository
	settlementRepo     domain.SettlementRepository
	settlementItemRepo domain.SettlementItemRepository
	outboxRepo         domain.OutboxEventRepository
	tx                 database.TransactionManager
}

func NewSettlementService(
	campaignRepo domain.CampaignRepository,
	paymentRepo domain.PaymentRepository,
	settlementRepo domain.SettlementRepository,
	settlementItemRepo domain.SettlementItemRepository,
	outboxRepo domain.OutboxEventRepository,
	tx database.TransactionManager,
) SettlementService {
	return &settlementService{
		campaignRepo:       campaignRepo,
		paymentRepo:        paymentRepo,
		settlementRepo:     settlementRepo,
		settlementItemRepo: settlementItemRepo,
		outboxRepo:         outboxRepo,
		tx:                 tx,
	}
}

func (s *settlementService) SettleCampaign(ctx context.Context, campaignID string) (*domain.Settlement, error) {
	var finalSettlement *domain.Settlement

	err := s.tx.Do(ctx, func(txCtx context.Context) error {
		// 1. Lock campaign / settlement authority
		campaign, err := s.campaignRepo.FindByIDForUpdate(txCtx, campaignID)
		if err != nil {
			return fmt.Errorf("failed to lock campaign: %w", err)
		}

		_, err = s.settlementRepo.GetByCampaignID(txCtx, campaign.ID)
		if err == nil {
			return domain.ErrInvalidStateTransition
		}
		if err != domain.ErrNotFound {
			return fmt.Errorf("failed to check existing settlement: %w", err)
		}

		// 2. Identify eligible payments
		payments, err := s.paymentRepo.FindEligibleForSettlement(txCtx, campaign.ID)
		if err != nil {
			return fmt.Errorf("failed to find eligible payments: %w", err)
		}

		var eligibleAmount int64 = 0
		var items []*domain.SettlementItem

		for _, p := range payments {
			eligibleAmount += p.GrossAmount
			items = append(items, &domain.SettlementItem{
				DonationID:     p.DonationID,
				PaymentID:      p.ID,
				EligibleAmount: p.GrossAmount,
			})
		}

		now := time.Now()
		settlement := &domain.Settlement{
			CampaignID:   campaign.ID,
			GrossAmount:  eligibleAmount,
			RefundAmount: 0,
			PlatformFee:  0,
			NetAmount:    eligibleAmount,
			Status:       domain.SettlementStatusApproved,
			CalculatedAt: &now,
			ApprovedAt:   &now,
		}

		if err := s.settlementRepo.Create(txCtx, settlement); err != nil {
			return fmt.Errorf("failed to create settlement: %w", err)
		}

		// 5. Create settlement_items
		for _, item := range items {
			item.SettlementID = settlement.ID
			if err := s.settlementItemRepo.Create(txCtx, item); err != nil {
				return fmt.Errorf("failed to create settlement item: %w", err)
			}

			// 6. Update payment state CAPTURED -> SETTLED
			if err := s.paymentRepo.UpdateState(txCtx, item.PaymentID, domain.PaymentStatusSettled); err != nil {
				return fmt.Errorf("failed to update payment state to SETTLED: %w", err)
			}
		}

		// 7. Update relevant financial state
		if eligibleAmount > 0 {
			campaign.CurrentAmount += eligibleAmount
			if err := s.campaignRepo.Update(txCtx, campaign); err != nil {
				return fmt.Errorf("failed to update campaign current_amount: %w", err)
			}
		}

		// 8. Create Outbox Event Atomically
		payload, _ := json.Marshal(map[string]interface{}{
			"settlement_id": settlement.ID,
			"campaign_id":   settlement.CampaignID,
			"gross_amount":  settlement.GrossAmount,
			"net_amount":    settlement.NetAmount,
			"status":        settlement.Status,
			"calculated_at": settlement.CalculatedAt,
		})

		idempotencyKey := fmt.Sprintf("settlement_%s_approved", settlement.ID)
		outboxEvent := &domain.OutboxEvent{
			IdempotencyKey: idempotencyKey,
			AggregateType:  domain.OutboxAggregateSettlement,
			AggregateID:    settlement.ID,
			EventType:      domain.OutboxEventSettlementApproved,
			Payload:        payload,
			Status:         domain.OutboxStatusPending,
			AvailableAt:    now,
		}

		if err := s.outboxRepo.Create(txCtx, outboxEvent); err != nil {
			return fmt.Errorf("failed to create outbox event: %w", err)
		}

		finalSettlement = settlement
		return nil
	})

	if err != nil {
		return nil, err
	}

	return finalSettlement, nil
}
