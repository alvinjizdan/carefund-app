package database

import (
	"context"
	"database/sql"

	"carefund-api/internal/domain"
	"github.com/lib/pq"
)

type paymentEventRepo struct {
	db *sql.DB
}

func NewPaymentEventRepository(db *DB) domain.PaymentEventRepository {
	return &paymentEventRepo{db: db.DB}
}

func (r *paymentEventRepo) Create(ctx context.Context, event *domain.PaymentEvent) error {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		INSERT INTO payment_events (
			payment_id, provider, event_source, idempotency_key, event_type, 
			provider_status, payload, processing_status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, received_at
	`
	err := runner.QueryRowContext(ctx, query,
		event.PaymentID, event.Provider, event.EventSource, event.IdempotencyKey, event.EventType,
		event.ProviderStatus, event.Payload, event.ProcessingStatus,
	).Scan(&event.ID, &event.ReceivedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" { // unique_violation
			return domain.ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *paymentEventRepo) MarkProcessed(ctx context.Context, id string) error {
	runner := GetQueryRunner(ctx, r.db)
	query := `UPDATE payment_events SET processing_status = 'PROCESSED', processed_at = NOW() WHERE id = $1`
	_, err := runner.ExecContext(ctx, query, id)
	return err
}

func (r *paymentEventRepo) MarkRejected(ctx context.Context, id string, reason string) error {
	runner := GetQueryRunner(ctx, r.db)
	query := `UPDATE payment_events SET processing_status = 'REJECTED', rejection_reason = $2, processed_at = NOW() WHERE id = $1`
	_, err := runner.ExecContext(ctx, query, id, reason)
	return err
}
