package database

import (
	"context"
	"database/sql"

	"carefund-api/internal/domain"
)

type refundRepo struct {
	db *DB
}

func NewRefundRepository(db *DB) domain.RefundRepository {
	return &refundRepo{db: db}
}

func (r *refundRepo) Create(ctx context.Context, refund *domain.Refund) error {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		INSERT INTO refunds (payment_id, idempotency_key, amount, provider_refund_id, status, reason, requested_at, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`
	err := runner.QueryRowContext(ctx, query,
		refund.PaymentID, refund.IdempotencyKey, refund.Amount, refund.ProviderRefundID, refund.Status, refund.Reason, refund.RequestedAt, refund.CompletedAt,
	).Scan(&refund.ID)
	return err
}

func (r *refundRepo) FindByID(ctx context.Context, id string) (*domain.Refund, error) {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		SELECT id, payment_id, idempotency_key, amount, provider_refund_id, status, reason, requested_at, completed_at
		FROM refunds
		WHERE id = $1
	`
	refund := &domain.Refund{}
	err := runner.QueryRowContext(ctx, query, id).Scan(
		&refund.ID, &refund.PaymentID, &refund.IdempotencyKey, &refund.Amount, &refund.ProviderRefundID, &refund.Status, &refund.Reason, &refund.RequestedAt, &refund.CompletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return refund, nil
}

func (r *refundRepo) ListByPayment(ctx context.Context, paymentID string) ([]*domain.Refund, error) {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		SELECT id, payment_id, idempotency_key, amount, provider_refund_id, status, reason, requested_at, completed_at
		FROM refunds
		WHERE payment_id = $1
		ORDER BY requested_at ASC
	`
	rows, err := runner.QueryContext(ctx, query, paymentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refunds []*domain.Refund
	for rows.Next() {
		refund := &domain.Refund{}
		if err := rows.Scan(
			&refund.ID, &refund.PaymentID, &refund.IdempotencyKey, &refund.Amount, &refund.ProviderRefundID, &refund.Status, &refund.Reason, &refund.RequestedAt, &refund.CompletedAt,
		); err != nil {
			return nil, err
		}
		refunds = append(refunds, refund)
	}
	return refunds, nil
}

func (r *refundRepo) FindByIDForUpdate(ctx context.Context, id string) (*domain.Refund, error) {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		SELECT id, payment_id, idempotency_key, amount, provider_refund_id, status, reason, requested_at, completed_at
		FROM refunds
		WHERE id = $1 FOR UPDATE
	`
	refund := &domain.Refund{}
	err := runner.QueryRowContext(ctx, query, id).Scan(
		&refund.ID, &refund.PaymentID, &refund.IdempotencyKey, &refund.Amount, &refund.ProviderRefundID, &refund.Status, &refund.Reason, &refund.RequestedAt, &refund.CompletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return refund, nil
}

func (r *refundRepo) SumActiveRefunds(ctx context.Context, paymentID string) (int64, error) {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		SELECT COALESCE(SUM(amount), 0)
		FROM refunds
		WHERE payment_id = $1 AND status IN ('PENDING', 'COMPLETED')
	`
	var total int64
	err := runner.QueryRowContext(ctx, query, paymentID).Scan(&total)
	return total, err
}

func (r *refundRepo) SumCompletedRefunds(ctx context.Context, paymentID string) (int64, error) {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		SELECT COALESCE(SUM(amount), 0)
		FROM refunds
		WHERE payment_id = $1 AND status = 'COMPLETED'
	`
	var total int64
	err := runner.QueryRowContext(ctx, query, paymentID).Scan(&total)
	return total, err
}

func (r *refundRepo) Update(ctx context.Context, refund *domain.Refund) error {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		UPDATE refunds
		SET status = $1, provider_refund_id = $2, completed_at = $3
		WHERE id = $4
	`
	res, err := runner.ExecContext(ctx, query, refund.Status, refund.ProviderRefundID, refund.CompletedAt, refund.ID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}
