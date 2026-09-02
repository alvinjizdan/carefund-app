package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"carefund-api/internal/domain"
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
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 1. Reclaim expired leases
			reclaimed, err := w.repo.ReclaimExpiredLeases(ctx, w.ttl)
			if err != nil {
				log.Printf("[OutboxWorker] Failed to reclaim expired leases: %v", err)
			} else if reclaimed > 0 {
				log.Printf("[OutboxWorker] Reclaimed %d expired outbox leases", reclaimed)
			}

			// 2. Process all pending
			for {
				hasMore, err := w.ProcessNext(ctx)
				if err != nil && err != domain.ErrNotFound {
					log.Printf("[OutboxWorker] Error processing event: %v", err)
					break
				}
				if !hasMore {
					break
				}
			}
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
			log.Printf("[OutboxWorker] Event %s reached max retries (%d). Marking DEAD_LETTER.", event.ID, event.RetryCount)
			_ = w.repo.MarkDeadLetter(ctx, event.ID, err.Error())
			return true, fmt.Errorf("event %s reached max retries (%d) and moved to DEAD_LETTER: %w", event.ID, event.RetryCount, err)
		}

		nextAvailable := time.Now().Add(getBackoffDuration(event.RetryCount))
		_ = w.repo.MarkFailed(ctx, event.ID, nextAvailable)
		return true, fmt.Errorf("event processing failed: %w", err)
	}

	if err := w.repo.MarkProcessed(ctx, event.ID); err != nil {
		return true, fmt.Errorf("failed to mark event processed: %w", err)
	}

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
	log.Printf("[OutboxWorker] Processing event: %s %s", event.EventType, event.AggregateID)

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
				log.Printf("[OutboxWorker] Refund definitively rejected by provider for RefundID %s: %v", refund.ID, err)
				_ = w.refundSvc.FinalizeRefund(ctx, refund.ID, "", domain.RefundStatusFailed)
				return nil
			}

			// Ambiguous/transient error: Do NOT mark refund as FAILED; return error to trigger Outbox retry
			log.Printf("[OutboxWorker] Ambiguous provider failure for RefundID %s: %v. Retrying with backoff...", refund.ID, err)
			return err
		}

		if res != nil && res.IsCompleted {
			if err := w.refundSvc.FinalizeRefund(ctx, refund.ID, res.ProviderRefundID, domain.RefundStatusCompleted); err != nil {
				return fmt.Errorf("failed to finalize completed refund: %w", err)
			}
		}

		return nil
	}

	return nil
}
