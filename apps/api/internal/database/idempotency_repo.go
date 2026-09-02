package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"carefund-api/internal/domain"
	"github.com/lib/pq"
)

type idempotencyRepository struct {
	db *sql.DB
}

func NewIdempotencyRepository(db *DB) domain.IdempotencyRepository {
	return &idempotencyRepository{db: db.DB}
}

// Get returns a non-expired idempotency record or domain.ErrNotFound.
func (r *idempotencyRepository) Get(ctx context.Context, userID, idempotencyKey string) (*domain.IdempotencyRecord, error) {
	query := `
		SELECT user_id, idempotency_key, request_hash, status, response_code, response_body, created_at, expires_at
		FROM idempotency_keys
		WHERE user_id = $1 AND idempotency_key = $2
	`

	var rec domain.IdempotencyRecord
	var respBody []byte
	runner := GetQueryRunner(ctx, r.db)
	err := runner.QueryRowContext(ctx, query, userID, idempotencyKey).Scan(
		&rec.UserID,
		&rec.IdempotencyKey,
		&rec.RequestHash,
		&rec.Status,
		&rec.ResponseCode,
		&respBody,
		&rec.CreatedAt,
		&rec.ExpiresAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get idempotency record: %w", err)
	}

	rec.ResponseBody = respBody
	return &rec, nil
}

func (r *idempotencyRepository) Reserve(ctx context.Context, userID, idempotencyKey, requestHash, orderID string, expiresAt time.Time) error {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		INSERT INTO idempotency_keys (user_id, idempotency_key, request_hash, order_id, status, created_at, expires_at)
		VALUES ($1, $2, $3, $4, 'PENDING', NOW(), $5)
	`
	_, err := runner.ExecContext(ctx, query, userID, idempotencyKey, requestHash, orderID, expiresAt)
	if err != nil {
		// PostgreSQL unique_violation error code is "23505"
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return domain.ErrIdempotencyConflict
		}
		return fmt.Errorf("failed to reserve idempotency key: %w", err)
	}
	return nil
}

// Complete marks an existing PENDING reservation as COMPLETED and stores the cached response.
// Called OUTSIDE the financial transaction, after a successful Midtrans call.
func (r *idempotencyRepository) Complete(ctx context.Context, userID, idempotencyKey string, responseCode int, responseBody []byte) error {
	query := `
		UPDATE idempotency_keys
		SET status = 'COMPLETED', response_code = $3, response_body = $4::jsonb
		WHERE user_id = $1 AND idempotency_key = $2 AND status = 'PENDING'
	`

	runner := GetQueryRunner(ctx, r.db)
	_, err := runner.ExecContext(ctx, query, userID, idempotencyKey, responseCode, string(responseBody))
	if err != nil {
		return fmt.Errorf("failed to complete idempotency key: %w", err)
	}

	return nil
}

func (r *idempotencyRepository) Fail(ctx context.Context, userID, idempotencyKey string) error {
	query := `
		UPDATE idempotency_keys
		SET status = 'FAILED'
		WHERE user_id = $1 AND idempotency_key = $2 AND status = 'PENDING'
	`
	runner := GetQueryRunner(ctx, r.db)
	_, err := runner.ExecContext(ctx, query, userID, idempotencyKey)
	if err != nil {
		return fmt.Errorf("failed to fail idempotency record: %w", err)
	}
	return nil
}

func (r *idempotencyRepository) RecoverCompletedByOrderID(ctx context.Context, orderID string, responseCode int, responseBody []byte) error {
	query := `
		UPDATE idempotency_keys
		SET status = 'COMPLETED', response_code = $2, response_body = $3
		WHERE order_id = $1 AND status = 'PENDING'
	`
	runner := GetQueryRunner(ctx, r.db)
	_, err := runner.ExecContext(ctx, query, orderID, responseCode, responseBody)
	if err != nil {
		return fmt.Errorf("failed to recover completed idempotency record: %w", err)
	}
	return nil
}

func (r *idempotencyRepository) RecoverFailedByOrderID(ctx context.Context, orderID string) error {
	query := `
		UPDATE idempotency_keys
		SET status = 'FAILED'
		WHERE order_id = $1 AND status = 'PENDING'
	`
	runner := GetQueryRunner(ctx, r.db)
	_, err := runner.ExecContext(ctx, query, orderID)
	if err != nil {
		return fmt.Errorf("failed to recover failed idempotency record: %w", err)
	}
	return nil
}
