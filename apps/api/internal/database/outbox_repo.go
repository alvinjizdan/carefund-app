package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"carefund-api/internal/domain"
	"github.com/lib/pq"
)

type outboxEventRepo struct {
	db *DB
}

func NewOutboxEventRepository(db *DB) domain.OutboxEventRepository {
	return &outboxEventRepo{db: db}
}

func (r *outboxEventRepo) Create(ctx context.Context, event *domain.OutboxEvent) error {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		INSERT INTO outbox_events (idempotency_key, aggregate_type, aggregate_id, event_type, payload, status, available_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, retry_count, created_at
	`
	err := runner.QueryRowContext(ctx, query,
		event.IdempotencyKey, event.AggregateType, event.AggregateID, event.EventType, event.Payload, event.Status, event.AvailableAt,
	).Scan(&event.ID, &event.RetryCount, &event.CreatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" { // unique violation
			return domain.ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *outboxEventRepo) ClaimNext(ctx context.Context) (*domain.OutboxEvent, error) {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		UPDATE outbox_events
		SET status = 'PROCESSING', 
		    retry_count = retry_count + 1,
		    processing_started_at = NOW()
		WHERE id = (
			SELECT id
			FROM outbox_events
			WHERE status IN ('PENDING', 'FAILED') AND available_at <= NOW()
			ORDER BY available_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING id, idempotency_key, aggregate_type, aggregate_id, event_type, payload, status, retry_count, available_at, processing_started_at, processed_at, created_at
	`
	event := &domain.OutboxEvent{}
	err := runner.QueryRowContext(ctx, query).Scan(
		&event.ID, &event.IdempotencyKey, &event.AggregateType, &event.AggregateID, &event.EventType,
		&event.Payload, &event.Status, &event.RetryCount, &event.AvailableAt, &event.ProcessingStartedAt, &event.ProcessedAt, &event.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return event, nil
}

func (r *outboxEventRepo) MarkProcessed(ctx context.Context, id string) error {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		UPDATE outbox_events
		SET status = 'PROCESSED', processed_at = NOW(), processing_started_at = NULL
		WHERE id = $1
	`
	_, err := runner.ExecContext(ctx, query, id)
	return err
}

func (r *outboxEventRepo) MarkFailed(ctx context.Context, id string, nextAvailableAt time.Time) error {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		UPDATE outbox_events
		SET status = 'FAILED', available_at = $2, processing_started_at = NULL
		WHERE id = $1
	`
	_, err := runner.ExecContext(ctx, query, id, nextAvailableAt)
	return err
}

func (r *outboxEventRepo) MarkDeadLetter(ctx context.Context, id string, reason string) error {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		UPDATE outbox_events
		SET status = 'DEAD_LETTER', processing_started_at = NULL
		WHERE id = $1
	`
	_, err := runner.ExecContext(ctx, query, id)
	return err
}

func (r *outboxEventRepo) ReplayDeadLetter(ctx context.Context, id string) error {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		UPDATE outbox_events
		SET status = 'PENDING', retry_count = 0, available_at = NOW(), processing_started_at = NULL
		WHERE id = $1 AND status = 'DEAD_LETTER'
	`
	res, err := runner.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *outboxEventRepo) FindByID(ctx context.Context, id string) (*domain.OutboxEvent, error) {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		SELECT id, idempotency_key, aggregate_type, aggregate_id, event_type, payload, status, retry_count, available_at, processing_started_at, processed_at, created_at
		FROM outbox_events WHERE id = $1
	`
	event := &domain.OutboxEvent{}
	err := runner.QueryRowContext(ctx, query, id).Scan(
		&event.ID, &event.IdempotencyKey, &event.AggregateType, &event.AggregateID, &event.EventType,
		&event.Payload, &event.Status, &event.RetryCount, &event.AvailableAt, &event.ProcessingStartedAt, &event.ProcessedAt, &event.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return event, nil
}

func (r *outboxEventRepo) ReclaimExpiredLeases(ctx context.Context, ttl time.Duration) (int64, error) {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		UPDATE outbox_events
		SET status = 'PENDING', processing_started_at = NULL
		WHERE status = 'PROCESSING' AND processing_started_at <= NOW() - $1::interval
	`
	intervalStr := ttl.String()
	res, err := runner.ExecContext(ctx, query, intervalStr)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
