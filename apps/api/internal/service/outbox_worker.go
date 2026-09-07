package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"carefund-api/internal/domain"
	"carefund-api/internal/logger"
	"carefund-api/internal/metrics"
)

type OutboxWorker interface {
	ProcessNext(ctx context.Context) (bool, error)
	Start(ctx context.Context, interval time.Duration)
}

type outboxWorker struct {
	repo        domain.OutboxEventRepository
	ttl         time.Duration
	paymentGw   domain.PaymentGateway
	refundSvc   RefundService
	refundRepo  domain.RefundRepository
	paymentRepo domain.PaymentRepository
}

type OutboxWorkerOption func(*outboxWorker)

func WithPaymentGateway(gw domain.PaymentGateway) OutboxWorkerOption {
	return func(w *outboxWorker) {
		w.paymentGw = gw
	}
}

func WithRefundService(svc RefundService) OutboxWorkerOption {
	return func(w *outboxWorker) {
		w.refundSvc = svc
	}
}

func WithRefundRepository(repo domain.RefundRepository) OutboxWorkerOption {
	return func(w *outboxWorker) {
		w.refundRepo = repo
	}
}

func WithPaymentRepository(repo domain.PaymentRepository) OutboxWorkerOption {
	return func(w *outboxWorker) {
		w.paymentRepo = repo
	}
}

func NewOutboxWorker(repo domain.OutboxEventRepository, ttl time.Duration, opts ...OutboxWorkerOption) OutboxWorker {
	w := &outboxWorker{
		repo: repo,
		ttl:  ttl,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

func (w *outboxWorker) Start(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	runCycle := func() {
		if ctx.Err() != nil {
			return
		}
		// 1. Reclaim expired leases
		reclaimed, err := w.repo.ReclaimExpiredLeases(ctx, w.ttl)
		if err != nil {
			logger.Error(ctx, "Failed to reclaim expired leases", err, logger.F("component", "OutboxWorker"))
		} else if reclaimed > 0 {
			logger.Info(ctx, "Reclaimed expired outbox leases", logger.F("component", "OutboxWorker"), logger.F("count", reclaimed))
		}

		// 2. Process all pending
		for {
			if ctx.Err() != nil {
				return
			}
			hasMore, err := w.ProcessNext(ctx)
			if err != nil && err != domain.ErrNotFound {
				logger.Error(ctx, "Error processing event", err, logger.F("component", "OutboxWorker"))
				break
			}
			if !hasMore {
				break
			}
		}

		metrics.RecordWorkerHeartbeat("outbox")
	}

	// Initial eager pass
	runCycle()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runCycle()
		}
	}
}

func (w *outboxWorker) ProcessNext(ctx context.Context) (bool, error) {
	event, err := w.repo.ClaimNext(ctx)
	if err != nil {
		if err == domain.ErrNotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to claim outbox event: %w", err)
	}

	err = w.processEvent(ctx, event)

	if err != nil {
		if event.RetryCount >= domain.MaxOutboxRetryCount {
			metrics.RecordOutboxDeadLetter(event.EventType)
			metrics.RecordFinancialAnomaly(ctx, "dead_letter_growth", "Outbox event moved to DEAD_LETTER after retry exhaustion",
				logger.F("event_id", event.ID),
				logger.F("event_type", event.EventType),
				logger.F("retry_count", event.RetryCount),
			)
			logger.Error(ctx, "Event reached max retries, moving to DEAD_LETTER", err,
				logger.F("component", "OutboxWorker"),
				logger.F("event_id", event.ID),
				logger.F("event_type", event.EventType),
				logger.F("aggregate_id", event.AggregateID),
				logger.F("retry_count", event.RetryCount),
			)
			_ = w.repo.MarkDeadLetter(ctx, event.ID, err.Error())
			return true, fmt.Errorf("event %s reached max retries (%d) and moved to DEAD_LETTER: %w", event.ID, event.RetryCount, err)
		}

		nextAvailable := time.Now().Add(getBackoffDuration(event.RetryCount))
		metrics.RecordOutboxRetry(event.EventType)
		_ = w.repo.MarkFailed(ctx, event.ID, nextAvailable)
		logger.Warn(ctx, "Event processing failed, scheduled backoff retry",
			logger.F("component", "OutboxWorker"),
			logger.F("event_id", event.ID),
			logger.F("event_type", event.EventType),
			logger.F("aggregate_id", event.AggregateID),
			logger.F("retry_count", event.RetryCount),
			logger.F("next_available", nextAvailable.Format(time.RFC3339)),
		)
		return true, fmt.Errorf("event processing failed: %w", err)
	}

	if err := w.repo.MarkProcessed(ctx, event.ID); err != nil {
		return true, fmt.Errorf("failed to mark event processed: %w", err)
	}

	metrics.RecordWorkerProgress("outbox")

	return true, nil
}

func getBackoffDuration(retryCount int) time.Duration {
	switch retryCount {
	case 1:
		return 1 * time.Minute
	case 2:
		return 2 * time.Minute
	case 3:
		return 5 * time.Minute
	case 4:
		return 10 * time.Minute
	case 5:
		return 30 * time.Minute
	default:
		if retryCount >= 6 {
			return 60 * time.Minute
		}
		return 1 * time.Minute
	}
}

func (w *outboxWorker) processEvent(ctx context.Context, event *domain.OutboxEvent) error {
	logger.Info(ctx, "Processing outbox event",
		logger.F("component", "OutboxWorker"),
		logger.F("event_id", event.ID),
		logger.F("event_type", event.EventType),
		logger.F("aggregate_id", event.AggregateID),
	)

	if event.EventType == "REFUND_REQUESTED" {
		if w.paymentGw == nil || w.refundSvc == nil || w.refundRepo == nil || w.paymentRepo == nil {
			// No handler injected; treat as simulated success
			return nil
		}

		var payload struct {
			RefundID  string `json:"refund_id"`
			PaymentID string `json:"payment_id"`
			OrderID   string `json:"order_id"`
			Amount    int64  `json:"amount"`
			Reason    string `json:"reason"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return fmt.Errorf("invalid refund outbox payload: %w", err)
		}

		refund, err := w.refundRepo.FindByID(ctx, payload.RefundID)
		if err != nil {
			return fmt.Errorf("failed to find refund for outbox event: %w", err)
		}

		if refund.Status == domain.RefundStatusCompleted || refund.Status == domain.RefundStatusFailed || refund.Status == domain.RefundStatusCancelled {
			// Already finalized
			return nil
		}

		payment, err := w.paymentRepo.FindByID(ctx, payload.PaymentID)
		if err != nil {
			return fmt.Errorf("failed to find payment for refund outbox event: %w", err)
		}

		refundReq := &domain.RefundRequest{
			OrderID:        payment.OrderID,
			RefundID:       refund.ID,
			IdempotencyKey: refund.IdempotencyKey,
			Amount:         refund.Amount,
			Reason:         refund.Reason,
		}

		res, err := w.paymentGw.RefundPayment(ctx, refundReq)
		if err != nil {
			var rejectionErr *domain.ProviderRejectionError
			if errors.Is(err, domain.ErrProviderRejected) || errors.As(err, &rejectionErr) {
				// Definitive provider rejection: mark Refund as FAILED and Outbox as PROCESSED
				metrics.RecordRefundProviderResult("rejected")
				logger.Warn(ctx, "Refund definitively rejected by provider",
					logger.F("component", "OutboxWorker"),
					logger.F("refund_id", refund.ID),
					logger.F("payment_id", payment.ID),
					logger.F("order_id", payment.OrderID),
					logger.F("err", err.Error()),
				)
				_ = w.refundSvc.FinalizeRefund(ctx, refund.ID, "", domain.RefundStatusFailed)
				return nil
			}

			// Ambiguous/transient error: Do NOT mark refund as FAILED; return error to trigger Outbox retry
			metrics.RecordRefundProviderResult("timeout")
			logger.Warn(ctx, "Ambiguous provider failure for refund, retrying with backoff",
				logger.F("component", "OutboxWorker"),
				logger.F("refund_id", refund.ID),
				logger.F("payment_id", payment.ID),
				logger.F("order_id", payment.OrderID),
				logger.F("err", err.Error()),
			)
			return err
		}

		if res != nil && res.IsCompleted {
			metrics.RecordRefundProviderResult("success")
			if err := w.refundSvc.FinalizeRefund(ctx, refund.ID, res.ProviderRefundID, domain.RefundStatusCompleted); err != nil {
				return fmt.Errorf("failed to finalize completed refund: %w", err)
			}
			logger.Info(ctx, "Refund finalized successfully",
				logger.F("component", "OutboxWorker"),
				logger.F("refund_id", refund.ID),
				logger.F("payment_id", payment.ID),
				logger.F("order_id", payment.OrderID),
			)
		}

		return nil
	}

	return nil
}
