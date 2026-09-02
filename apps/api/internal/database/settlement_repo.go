package database

import (
	"context"
	"database/sql"
	"errors"

	"carefund-api/internal/domain"
	"github.com/lib/pq"
)

type settlementRepo struct {
	db *DB
}

func NewSettlementRepository(db *DB) domain.SettlementRepository {
	return &settlementRepo{db: db}
}

func (r *settlementRepo) Create(ctx context.Context, s *domain.Settlement) error {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		INSERT INTO settlements (
			campaign_id, gross_amount, refund_amount, platform_fee, net_amount,
			status, calculated_at, approved_at, executed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at
	`
	err := runner.QueryRowContext(ctx, query,
		s.CampaignID, s.GrossAmount, s.RefundAmount, s.PlatformFee, s.NetAmount,
		s.Status, s.CalculatedAt, s.ApprovedAt, s.ExecutedAt,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" { // unique violation
			return domain.ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *settlementRepo) GetByCampaignID(ctx context.Context, campaignID string) (*domain.Settlement, error) {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		SELECT id, campaign_id, gross_amount, refund_amount, platform_fee, net_amount,
			status, calculated_at, approved_at, executed_at, created_at, updated_at
		FROM settlements
		WHERE campaign_id = $1
	`
	s := &domain.Settlement{}
	err := runner.QueryRowContext(ctx, query, campaignID).Scan(
		&s.ID, &s.CampaignID, &s.GrossAmount, &s.RefundAmount, &s.PlatformFee, &s.NetAmount,
		&s.Status, &s.CalculatedAt, &s.ApprovedAt, &s.ExecutedAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return s, nil
}

func (r *settlementRepo) Update(ctx context.Context, s *domain.Settlement) error {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		UPDATE settlements
		SET gross_amount = $1, refund_amount = $2, platform_fee = $3, net_amount = $4,
			status = $5, calculated_at = $6, approved_at = $7, executed_at = $8,
			updated_at = NOW()
		WHERE id = $9
		RETURNING updated_at
	`
	err := runner.QueryRowContext(ctx, query,
		s.GrossAmount, s.RefundAmount, s.PlatformFee, s.NetAmount,
		s.Status, s.CalculatedAt, s.ApprovedAt, s.ExecutedAt, s.ID,
	).Scan(&s.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrNotFound
		}
		return err
	}
	return nil
}

type settlementItemRepo struct {
	db *DB
}

func NewSettlementItemRepository(db *DB) domain.SettlementItemRepository {
	return &settlementItemRepo{db: db}
}

func (r *settlementItemRepo) Create(ctx context.Context, item *domain.SettlementItem) error {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		INSERT INTO settlement_items (
			settlement_id, donation_id, payment_id, eligible_amount
		) VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`
	err := runner.QueryRowContext(ctx, query,
		item.SettlementID, item.DonationID, item.PaymentID, item.EligibleAmount,
	).Scan(&item.ID, &item.CreatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" { // unique violation
			return domain.ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *settlementItemRepo) GetBySettlementID(ctx context.Context, settlementID string) ([]*domain.SettlementItem, error) {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		SELECT id, settlement_id, donation_id, payment_id, eligible_amount, created_at
		FROM settlement_items
		WHERE settlement_id = $1
	`
	rows, err := runner.QueryContext(ctx, query, settlementID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*domain.SettlementItem
	for rows.Next() {
		item := &domain.SettlementItem{}
		if err := rows.Scan(
			&item.ID, &item.SettlementID, &item.DonationID, &item.PaymentID,
			&item.EligibleAmount, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *settlementItemRepo) GetByPaymentID(ctx context.Context, paymentID string) (*domain.SettlementItem, error) {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		SELECT id, settlement_id, donation_id, payment_id, eligible_amount, created_at
		FROM settlement_items
		WHERE payment_id = $1
	`
	item := &domain.SettlementItem{}
	err := runner.QueryRowContext(ctx, query, paymentID).Scan(
		&item.ID, &item.SettlementID, &item.DonationID, &item.PaymentID,
		&item.EligibleAmount, &item.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return item, nil
}
