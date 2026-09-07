package service

import (
	"context"
	"fmt"
	"time"

	"carefund-api/internal/database"
	"carefund-api/internal/domain"
	"carefund-api/internal/logger"
	"carefund-api/internal/metrics"
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
	start := time.Now()
	cutoffTime := time.Now().Add(-staleThreshold)
	payments, err := s.paymentRepo.FindStalePendingPayments(ctx, cutoffTime, batchSize)
	if err != nil {
		metrics.RecordWorkerReconciliation("failure", time.Since(start))
		metrics.RecordFinancialAnomaly(ctx, "reconciliation_repeated_failures", "Failed to query stale pending payments", logger.F("err", err.Error()))
		return 0, fmt.Errorf("failed to fetch stale payments: %w", err)
	}

	successCount := 0
	for _, p := range payments {
		err := s.reconcilePayment(ctx, p, staleThreshold)
		if err != nil {
			logger.Error(ctx, "Failed to reconcile payment", err,
				logger.F("component", "Reconciliation"),
				logger.F("payment_id", p.ID),
				logger.F("order_id", p.OrderID),
			)
			continue
		}
		successCount++
	}

	duration := time.Since(start)
	metrics.RecordWorkerReconciliation("success", duration)
	metrics.RecordWorkerHeartbeat("reconciliation")
	if successCount > 0 {
		metrics.RecordWorkerProgress("reconciliation")
	}

	return successCount, nil
}

func (s *reconciliationService) reconcilePayment(ctx context.Context, p *domain.Payment, ttl time.Duration) error {
	logger.Info(ctx, "Checking provider status for reconciliation",
		logger.F("component", "Reconciliation"),
		logger.F("payment_id", p.ID),
		logger.F("order_id", p.OrderID),
	)

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

	if (statusRes.ProviderStatus == "capture" || statusRes.ProviderStatus == "settlement") && p.Status == domain.PaymentStatusPending {
		metrics.RecordFinancialAnomaly(ctx, "provider_success_local_pending", "Reconciliation discovered captured provider status for local pending payment",
			logger.F("order_id", p.OrderID),
			logger.F("payment_id", p.ID),
			logger.F("provider_status", statusRes.ProviderStatus),
		)
	} else if (statusRes.ProviderStatus == "expire" || statusRes.ProviderStatus == "deny" || statusRes.ProviderStatus == "cancel") && p.Status == domain.PaymentStatusPending {
		metrics.RecordFinancialAnomaly(ctx, "provider_failed_local_pending", "Reconciliation discovered terminal failure for local pending payment",
			logger.F("order_id", p.OrderID),
			logger.F("payment_id", p.ID),
			logger.F("provider_status", statusRes.ProviderStatus),
		)
	}

	if time.Since(p.CreatedAt) > 2*time.Hour && p.Status == domain.PaymentStatusPending {
		metrics.RecordFinancialAnomaly(ctx, "long_lived_pending_payment", "Payment remained PENDING beyond 2 hour threshold",
			logger.F("order_id", p.OrderID),
			logger.F("payment_id", p.ID),
			logger.F("age", time.Since(p.CreatedAt).String()),
		)
	}

	err = s.webhookSvc.ProcessNotification(ctx, notif)
	if err != nil {
		return fmt.Errorf("failed to process notification: %w", err)
	}

	return nil
}
