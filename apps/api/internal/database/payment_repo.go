package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"carefund-api/internal/domain"
	"github.com/lib/pq"
)

type paymentRepo struct {
	db *sql.DB
}

func NewPaymentRepository(db *DB) domain.PaymentRepository {
	return &paymentRepo{db: db.DB}
}

func (r *paymentRepo) Create(ctx context.Context, payment *domain.Payment) error {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		INSERT INTO payments (donation_id, provider, order_id, transaction_id, payment_type, gross_amount, status, fraud_status, transaction_at, settled_at, expired_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`
	err := runner.QueryRowContext(ctx, query,
		payment.DonationID, payment.Provider, payment.OrderID, payment.TransactionID, payment.PaymentType, payment.GrossAmount,
		payment.Status, payment.FraudStatus, payment.TransactionAt, payment.SettledAt, payment.ExpiredAt,
	).Scan(&payment.ID, &payment.CreatedAt, &payment.UpdatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" { // unique_violation
			return domain.ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *paymentRepo) FindByID(ctx context.Context, id string) (*domain.Payment, error) {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		SELECT id, donation_id, provider, order_id, transaction_id, payment_type, gross_amount, status, fraud_status, transaction_at, settled_at, expired_at, created_at, updated_at
		FROM payments WHERE id = $1
	`
	var p domain.Payment
	err := runner.QueryRowContext(ctx, query, id).
		Scan(&p.ID, &p.DonationID, &p.Provider, &p.OrderID, &p.TransactionID, &p.PaymentType, &p.GrossAmount, &p.Status, &p.FraudStatus, &p.TransactionAt, &p.SettledAt, &p.ExpiredAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *paymentRepo) FindByIDForUpdate(ctx context.Context, id string) (*domain.Payment, error) {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		SELECT id, donation_id, provider, order_id, transaction_id, payment_type, gross_amount, status, fraud_status, transaction_at, settled_at, expired_at, created_at, updated_at
		FROM payments WHERE id = $1 FOR UPDATE
	`
	var p domain.Payment
	err := runner.QueryRowContext(ctx, query, id).
		Scan(&p.ID, &p.DonationID, &p.Provider, &p.OrderID, &p.TransactionID, &p.PaymentType, &p.GrossAmount, &p.Status, &p.FraudStatus, &p.TransactionAt, &p.SettledAt, &p.ExpiredAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *paymentRepo) FindByOrderID(ctx context.Context, orderID string) (*domain.Payment, error) {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		SELECT id, donation_id, provider, order_id, transaction_id, payment_type, gross_amount, status, fraud_status, transaction_at, settled_at, expired_at, created_at, updated_at
		FROM payments WHERE order_id = $1
	`
	var p domain.Payment
	err := runner.QueryRowContext(ctx, query, orderID).
		Scan(&p.ID, &p.DonationID, &p.Provider, &p.OrderID, &p.TransactionID, &p.PaymentType, &p.GrossAmount, &p.Status, &p.FraudStatus, &p.TransactionAt, &p.SettledAt, &p.ExpiredAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *paymentRepo) FindByOrderIDForUpdate(ctx context.Context, orderID string) (*domain.Payment, error) {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		SELECT id, donation_id, provider, order_id, transaction_id, payment_type, gross_amount, status, fraud_status, transaction_at, settled_at, expired_at, created_at, updated_at
		FROM payments WHERE order_id = $1 FOR UPDATE
	`
	var p domain.Payment
	err := runner.QueryRowContext(ctx, query, orderID).
		Scan(&p.ID, &p.DonationID, &p.Provider, &p.OrderID, &p.TransactionID, &p.PaymentType, &p.GrossAmount, &p.Status, &p.FraudStatus, &p.TransactionAt, &p.SettledAt, &p.ExpiredAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *paymentRepo) Update(ctx context.Context, payment *domain.Payment) error {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		UPDATE payments
		SET transaction_id = $1, payment_type = $2, status = $3, fraud_status = $4, transaction_at = $5, settled_at = $6, expired_at = $7, updated_at = NOW()
		WHERE id = $8
		RETURNING updated_at
	`
	err := runner.QueryRowContext(ctx, query,
		payment.TransactionID, payment.PaymentType, payment.Status, payment.FraudStatus, payment.TransactionAt, payment.SettledAt, payment.ExpiredAt, payment.ID,
	).Scan(&payment.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrNotFound
		}
		return err
	}
	return nil
}

func (r *paymentRepo) UpdateState(ctx context.Context, id string, status string) error {
	runner := GetQueryRunner(ctx, r.db)
	query := `UPDATE payments SET status = $1, updated_at = NOW() WHERE id = $2`
	res, err := runner.ExecContext(ctx, query, status, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *paymentRepo) FindStalePendingPayments(ctx context.Context, cutoffTime time.Time, limit int) ([]*domain.Payment, error) {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		SELECT id, donation_id, provider, order_id, transaction_id, payment_type, gross_amount, status, fraud_status, transaction_at, settled_at, expired_at, created_at, updated_at
		FROM payments
		WHERE status = 'PENDING' AND created_at <= $1
		ORDER BY created_at ASC
		LIMIT $2
	`
	rows, err := runner.QueryContext(ctx, query, cutoffTime, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []*domain.Payment
	for rows.Next() {
		var p domain.Payment
		if err := rows.Scan(
			&p.ID, &p.DonationID, &p.Provider, &p.OrderID, &p.TransactionID, &p.PaymentType,
			&p.GrossAmount, &p.Status, &p.FraudStatus, &p.TransactionAt, &p.SettledAt, &p.ExpiredAt,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		payments = append(payments, &p)
	}

	return payments, rows.Err()
}
func (r *paymentRepo) FindEligibleForSettlement(ctx context.Context, campaignID string) ([]*domain.Payment, error) {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		SELECT p.id, p.donation_id, p.provider, p.order_id, p.transaction_id, p.payment_type, p.gross_amount, p.status, p.fraud_status, p.transaction_at, p.settled_at, p.expired_at, p.created_at, p.updated_at
		FROM payments p
		JOIN donations d ON p.donation_id = d.id
		WHERE d.campaign_id = $1
		  AND d.status = 'PAID'
		  AND p.status = 'CAPTURED'
		  AND p.gross_amount > 0
		  AND NOT EXISTS (
			  SELECT 1 FROM settlement_items si WHERE si.payment_id = p.id
		  )
		  AND NOT EXISTS (
			  SELECT 1 FROM refunds r WHERE r.payment_id = p.id AND r.status IN ('PENDING', 'COMPLETED')
		  )
	`
	rows, err := runner.QueryContext(ctx, query, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []*domain.Payment
	for rows.Next() {
		var p domain.Payment
		if err := rows.Scan(
			&p.ID, &p.DonationID, &p.Provider, &p.OrderID, &p.TransactionID, &p.PaymentType,
			&p.GrossAmount, &p.Status, &p.FraudStatus, &p.TransactionAt, &p.SettledAt, &p.ExpiredAt,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		payments = append(payments, &p)
	}
	return payments, rows.Err()
}
