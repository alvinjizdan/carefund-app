package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"carefund-api/internal/database"
	"carefund-api/internal/domain"
)

type reconciliationService struct {
	paymentRepo domain.PaymentRepository
	webhookSvc  domain.WebhookService
	paymentGw   domain.PaymentGateway
	txManager   database.TransactionManager
}

func NewReconciliationService(
	paymentRepo domain.PaymentRepository,
	webhookSvc domain.WebhookService,
	paymentGw domain.PaymentGateway,
	txManager database.TransactionManager,
) *reconciliationService {
	return &reconciliationService{
		paymentRepo: paymentRepo,
		webhookSvc:  webhookSvc,
		paymentGw:   paymentGw,
		txManager:   txManager,
	}
}

// ReconcilePendingPayments fetches a bounded batch of stale pending payments and checks their status.
func (s *reconciliationService) ReconcilePendingPayments(ctx context.Context, batchSize int, staleThreshold time.Duration) (int, error) {
	// Query for pending payments older than the threshold
	// The repository needs a method to fetch these.
	cutoffTime := time.Now().Add(-staleThreshold)
	payments, err := s.paymentRepo.FindStalePendingPayments(ctx, cutoffTime, batchSize)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch stale payments: %w", err)
	}

	successCount := 0
	for _, p := range payments {
		err := s.reconcilePayment(ctx, p, staleThreshold)
		if err != nil {
			log.Printf("[Reconciliation] Failed to reconcile OrderID %s: %v", p.OrderID, err)
			continue
		}
		successCount++
	}

	return successCount, nil
}

func (s *reconciliationService) reconcilePayment(ctx context.Context, p *domain.Payment, ttl time.Duration) error {
	log.Printf("[Reconciliation] Checking status for OrderID %s", p.OrderID)

	statusRes, err := s.paymentGw.GetPaymentStatus(ctx, p.OrderID)
	if err != nil {
		if err.Error() == "transaction not found" {
			// Enforce expiration rule only if payment age >= TTL
			if time.Since(p.CreatedAt) >= ttl {
				statusRes = &domain.PaymentStatusResult{
					OrderID:        p.OrderID,
					TransactionID:  "",
					GrossAmount:    p.GrossAmount,
					ProviderStatus: "expire", // Map to EXPIRED
					FraudStatus:    "accept",
					RawPayload:     `{"status_message":"synthetic expiration due to 404 after TTL"}`,
				}
			} else {
				// Younger than TTL, stay pending
				return fmt.Errorf("transaction not found but payment is younger than TTL")
			}
		} else {
			return fmt.Errorf("gateway error: %w", err)
		}
	}

	// Construct a WebhookNotification to reuse the robust webhook logic
	idempotencyKey := fmt.Sprintf("reconcile_%s_%s", p.OrderID, statusRes.ProviderStatus)

	notif := &domain.WebhookNotification{
		Provider:        "MIDTRANS",
		EventSource:     "RECONCILIATION",
		ProviderEventID: statusRes.TransactionID,
		OrderID:         statusRes.OrderID,
		TransactionID:   statusRes.TransactionID,
		GrossAmount:     statusRes.GrossAmount,
		ProviderStatus:  statusRes.ProviderStatus,
		FraudStatus:     statusRes.FraudStatus,
		RawPayload:      statusRes.RawPayload,
		IdempotencyKey:  idempotencyKey,
	}

	err = s.webhookSvc.ProcessNotification(ctx, notif)
	if err != nil {
		return fmt.Errorf("failed to process notification: %w", err)
	}

	return nil
}
