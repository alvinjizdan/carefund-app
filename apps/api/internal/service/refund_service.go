package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"carefund-api/internal/database"
	"carefund-api/internal/domain"
)

type ProcessRefundRequest struct {
	PaymentID      string
	IdempotencyKey string
	Amount         int64
	Reason         string
}

type RefundService interface {
	ProcessLocalRefund(ctx context.Context, req ProcessRefundRequest) (*domain.Refund, error)
	FinalizeRefund(ctx context.Context, refundID string, providerRefundID string, targetStatus string) error
}

type refundService struct {
	paymentRepo  domain.PaymentRepository
	refundRepo   domain.RefundRepository
	donationRepo domain.DonationRepository
	outboxRepo   domain.OutboxEventRepository
	tx           database.TransactionManager
}

func NewRefundService(
	paymentRepo domain.PaymentRepository,
	refundRepo domain.RefundRepository,
	donationRepo domain.DonationRepository,
	outboxRepo domain.OutboxEventRepository,
	tx database.TransactionManager,
) RefundService {
	return &refundService{
		paymentRepo:  paymentRepo,
		refundRepo:   refundRepo,
		donationRepo: donationRepo,
		outboxRepo:   outboxRepo,
		tx:           tx,
	}
}

func (s *refundService) ProcessLocalRefund(ctx context.Context, req ProcessRefundRequest) (*domain.Refund, error) {
	var finalRefund *domain.Refund

	err := s.tx.Do(ctx, func(txCtx context.Context) error {
		payment, err := s.paymentRepo.FindByIDForUpdate(txCtx, req.PaymentID)
		if err != nil {
			return fmt.Errorf("failed to lock payment: %w", err)
		}

		if !domain.IsPaymentEligibleForRefund(payment) {
			return domain.ErrInvalidStateTransition
		}

		activeRefunds, err := s.refundRepo.SumActiveRefunds(txCtx, payment.ID)
		if err != nil {
			return fmt.Errorf("failed to sum active refunds: %w", err)
		}

		refundableAmount := domain.CalculateRefundableAmount(payment, activeRefunds)

		if req.Amount <= 0 {
			return domain.ErrInvalidInput
		}
		if req.Amount > refundableAmount {
			return domain.ErrInvalidInput
		}

		now := time.Now()
		refund := &domain.Refund{
			PaymentID:      payment.ID,
			IdempotencyKey: req.IdempotencyKey,
			Amount:         req.Amount,
			Status:         domain.RefundStatusPending,
			Reason:         req.Reason,
			RequestedAt:    now,
		}

		if err := refund.Validate(); err != nil {
			return err
		}

		if err := s.refundRepo.Create(txCtx, refund); err != nil {
			return fmt.Errorf("failed to create refund: %w", err)
		}

		// Create Outbox Event for Phase 5H execution
		payload, _ := json.Marshal(map[string]interface{}{
			"refund_id":  refund.ID,
			"payment_id": payment.ID,
			"order_id":   payment.OrderID,
			"amount":     refund.Amount,
			"reason":     refund.Reason,
		})
		outboxEvent := &domain.OutboxEvent{
			AggregateType:  "REFUND",
			AggregateID:    refund.ID,
			AvailableAt:    time.Now(),
			IdempotencyKey: fmt.Sprintf("REFUND_%s", refund.ID),
			EventType:      "REFUND_REQUESTED",
			Payload:        payload,
			Status:         domain.OutboxStatusPending,
		}
		if err := s.outboxRepo.Create(txCtx, outboxEvent); err != nil {
			return fmt.Errorf("failed to create outbox event: %w", err)
		}

		finalRefund = refund
		return nil
	})

	if err != nil {
		return nil, err
	}

	return finalRefund, nil
}

func (s *refundService) FinalizeRefund(ctx context.Context, refundID string, providerRefundID string, targetStatus string) error {
	return s.tx.Do(ctx, func(txCtx context.Context) error {
		// 1. Initial lookup to get PaymentID
		initialRefund, err := s.refundRepo.FindByID(txCtx, refundID)
		if err != nil {
			return fmt.Errorf("failed to find refund: %w", err)
		}

		// 2. Lock Payment FOR UPDATE (Deterministic Lock Ordering: Payment -> Refund -> Donation)
		payment, err := s.paymentRepo.FindByIDForUpdate(txCtx, initialRefund.PaymentID)
		if err != nil {
			return fmt.Errorf("failed to lock payment: %w", err)
		}

		// 3. Lock Refund FOR UPDATE
		refund, err := s.refundRepo.FindByIDForUpdate(txCtx, refundID)
		if err != nil {
			return fmt.Errorf("failed to lock refund: %w", err)
		}

		if refund.Status == targetStatus {
			return nil // Idempotent
		}

		if !refund.IsValidTransition(targetStatus) {
			return domain.ErrInvalidStateTransition
		}

		now := time.Now()
		refund.Status = targetStatus
		if providerRefundID != "" {
			refund.ProviderRefundID = &providerRefundID
		}

		if targetStatus == domain.RefundStatusCompleted {
			refund.CompletedAt = &now
			if err := s.refundRepo.Update(txCtx, refund); err != nil {
				return fmt.Errorf("failed to update refund: %w", err)
			}

			// Calculate new completed refunds total
			completedTotal, err := s.refundRepo.SumCompletedRefunds(txCtx, payment.ID)
			if err != nil {
				return fmt.Errorf("failed to sum completed refunds: %w", err)
			}

			var nextPaymentStatus string
			var nextDonationStatus string
			if completedTotal >= payment.GrossAmount {
				nextPaymentStatus = domain.PaymentStatusRefunded
				nextDonationStatus = domain.DonationStatusRefunded
			} else {
				nextPaymentStatus = domain.PaymentStatusPartiallyRefunded
				nextDonationStatus = domain.DonationStatusPartiallyRefunded
			}

			if payment.Status != nextPaymentStatus {
				payment.Status = nextPaymentStatus
				if err := s.paymentRepo.UpdateState(txCtx, payment.ID, nextPaymentStatus); err != nil {
					return fmt.Errorf("failed to update payment state: %w", err)
				}
			}

			if err := s.donationRepo.UpdateStatus(txCtx, payment.DonationID, nextDonationStatus); err != nil {
				return fmt.Errorf("failed to update donation status: %w", err)
			}
		} else {
			// FAILED or CANCELLED: reservation is released automatically
			if err := s.refundRepo.Update(txCtx, refund); err != nil {
				return fmt.Errorf("failed to update refund: %w", err)
			}
		}

		return nil
	})
}
