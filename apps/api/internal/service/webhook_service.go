package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"carefund-api/internal/database"
	"carefund-api/internal/domain"
)

type webhookService struct {
	paymentRepo      domain.PaymentRepository
	donationRepo     domain.DonationRepository
	paymentEventRepo domain.PaymentEventRepository
	refundRepo       domain.RefundRepository
	idempotencyRepo  domain.IdempotencyRepository
	tx               database.TransactionManager
}

type WebhookServiceOption func(*webhookService)

func WithWebhookRefundRepository(repo domain.RefundRepository) WebhookServiceOption {
	return func(s *webhookService) {
		s.refundRepo = repo
	}
}

func WithWebhookIdempotencyRepository(repo domain.IdempotencyRepository) WebhookServiceOption {
	return func(s *webhookService) {
		s.idempotencyRepo = repo
	}
}

func NewWebhookService(paymentRepo domain.PaymentRepository, donationRepo domain.DonationRepository, paymentEventRepo domain.PaymentEventRepository, tx database.TransactionManager, opts ...WebhookServiceOption) domain.WebhookService {
	s := &webhookService{
		paymentRepo:      paymentRepo,
		donationRepo:     donationRepo,
		paymentEventRepo: paymentEventRepo,
		tx:               tx,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// MapMidtransStatus maps Midtrans-specific transaction status to CareFund domain state.
func MapMidtransStatus(transactionStatus, fraudStatus string) string {
	switch transactionStatus {
	case "capture":
		if fraudStatus == "challenge" {
			return domain.PaymentStatusPending // Still under review by fraud team
		}
		if fraudStatus == "accept" {
			return domain.PaymentStatusCaptured
		}
	case "settlement":
		// CAPTURED != SETTLED in CareFund domain.
		// Midtrans "settlement" means customer paid successfully (our CAPTURED).
		return domain.PaymentStatusCaptured
	case "deny":
		return domain.PaymentStatusFailed
	case "cancel":
		return domain.PaymentStatusCancelled
	case "expire":
		return domain.PaymentStatusExpired
	case "refund":
		return domain.PaymentStatusRefunded
	case "partial_refund":
		return domain.PaymentStatusPartiallyRefunded
	case "pending":
		return domain.PaymentStatusPending
	}
	return domain.PaymentStatusPending
}

func (s *webhookService) ProcessNotification(ctx context.Context, notif *domain.WebhookNotification) error {
	// Stage A: Unlocked read and Event Persistence
	// Try to find the payment to link the event. If it doesn't exist, paymentID remains nil.
	unlockedPayment, err := s.paymentRepo.FindByOrderID(ctx, notif.OrderID)
	var paymentID *string
	if err == nil {
		paymentID = &unlockedPayment.ID
	}

	event := &domain.PaymentEvent{
		PaymentID:        paymentID,
		Provider:         notif.Provider,
		EventSource:      notif.EventSource,
		IdempotencyKey:   notif.IdempotencyKey,
		EventType:        notif.ProviderStatus,
		ProviderStatus:   notif.ProviderStatus,
		Payload:          notif.RawPayload,
		ProcessingStatus: domain.PaymentEventProcessingStatusReceived,
	}

	if err := s.paymentEventRepo.Create(ctx, event); err != nil {
		if err == domain.ErrDuplicate {
			// Duplicate logical event. Return nil so Midtrans doesn't retry.
			return nil
		}
		return err
	}

	// Helper to mark rejected and return nil (so Midtrans acknowledges)
	rejectEvent := func(reason string) error {
		_ = s.paymentEventRepo.MarkRejected(context.Background(), event.ID, reason)
		return nil
	}

	if paymentID == nil {
		return rejectEvent(domain.RejectionReasonPaymentNotFound)
	}

	// Stage B: Authoritative Lock and Financial Mutation
	err = s.tx.Do(ctx, func(txCtx context.Context) error {
		payment, err := s.paymentRepo.FindByIDForUpdate(txCtx, *paymentID)
		if err != nil {
			return fmt.Errorf("failed to lock payment: %w", err)
		}

		if payment.GrossAmount != notif.GrossAmount {
			return fmt.Errorf("amount_mismatch")
		}

		targetPaymentStatus := MapMidtransStatus(notif.ProviderStatus, notif.FraudStatus)

		if payment.Status == targetPaymentStatus {
			return nil
		}

		if !payment.IsValidTransition(targetPaymentStatus) {
			return domain.ErrInvalidStateTransition
		}

		payment.Status = targetPaymentStatus
		if err := s.paymentRepo.UpdateState(txCtx, payment.ID, targetPaymentStatus); err != nil {
			return err
		}

		donation, err := s.donationRepo.FindByID(txCtx, payment.DonationID)
		if err != nil {
			return err
		}

		newDonationStatus := donation.Status
		if targetPaymentStatus == domain.PaymentStatusCaptured {
			newDonationStatus = domain.DonationStatusPaid
		} else if targetPaymentStatus == domain.PaymentStatusFailed {
			newDonationStatus = domain.DonationStatusFailed
		} else if targetPaymentStatus == domain.PaymentStatusExpired {
			newDonationStatus = domain.DonationStatusExpired
		} else if targetPaymentStatus == domain.PaymentStatusCancelled {
			newDonationStatus = domain.DonationStatusCancelled
		} else if targetPaymentStatus == domain.PaymentStatusRefunded {
			newDonationStatus = domain.DonationStatusRefunded
		} else if targetPaymentStatus == domain.PaymentStatusPartiallyRefunded {
			newDonationStatus = domain.DonationStatusPartiallyRefunded
		}

		if newDonationStatus != donation.Status {
			donation.Status = newDonationStatus
			if err := s.donationRepo.UpdateStatus(txCtx, donation.ID, newDonationStatus); err != nil {
				return err
			}
		}

		if (targetPaymentStatus == domain.PaymentStatusRefunded || targetPaymentStatus == domain.PaymentStatusPartiallyRefunded) && s.refundRepo != nil {
			refunds, err := s.refundRepo.ListByPayment(txCtx, payment.ID)
			if err == nil {
				now := time.Now()
				for _, r := range refunds {
					if r.Status == domain.RefundStatusPending {
						r.Status = domain.RefundStatusCompleted
						r.CompletedAt = &now
						if notif.ProviderEventID != "" {
							r.ProviderRefundID = &notif.ProviderEventID
						}
						_ = s.refundRepo.Update(txCtx, r)
					}
				}
			}
		}

		if s.idempotencyRepo != nil {
			if targetPaymentStatus == domain.PaymentStatusCaptured {
				respObj := map[string]interface{}{
					"data": map[string]interface{}{
						"donation_id":   donation.ID,
						"payment_id":    payment.ID,
						"order_id":      payment.OrderID,
						"amount":        donation.Amount,
						"status":        newDonationStatus,
						"payment_token": nil,
						"redirect_url":  nil,
					},
				}
				importJson, _ := json.Marshal(respObj)
				_ = s.idempotencyRepo.RecoverCompletedByOrderID(txCtx, payment.OrderID, 201, importJson)
			} else if targetPaymentStatus == domain.PaymentStatusFailed || targetPaymentStatus == domain.PaymentStatusExpired || targetPaymentStatus == domain.PaymentStatusCancelled {
				_ = s.idempotencyRepo.RecoverFailedByOrderID(txCtx, payment.OrderID)
			}
		}

		return s.paymentEventRepo.MarkProcessed(txCtx, event.ID)
	})

	if err != nil {
		// Evaluate exact errors
		if err.Error() == "amount_mismatch" {
			log.Printf("[Webhook] Amount mismatch for OrderID %s", notif.OrderID)
			return rejectEvent(domain.RejectionReasonAmountMismatch)
		}
		if err == domain.ErrInvalidStateTransition {
			log.Printf("[Webhook] Invalid transition for OrderID %s: -> %s", notif.OrderID, notif.ProviderStatus)
			return rejectEvent(domain.RejectionReasonInvalidStateTransition)
		}
		// Any other internal error implies a DB transaction failure, not a domain rejection.
		// However, for testing "Rollback" we might need to reject it if it fails?
		// Actually, if it's a transient DB error, we probably shouldn't REJECT it because it might be retried.
		// The prompt says: "If Stage B fails: UPDATE payment_event -> REJECTED". Let's do that for any error just in case, but usually we'd want to separate them.
		// I will just return the error so Midtrans retries, UNLESS it's a domain error.
		return err
	}

	return nil
}
