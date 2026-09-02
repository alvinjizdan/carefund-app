package database

import (
	"context"
	"database/sql"
	"errors"

	"carefund-api/internal/domain"
)

type donationRepo struct {
	db *sql.DB
}

func NewDonationRepository(db *DB) domain.DonationRepository {
	return &donationRepo{db: db.DB}
}

func (r *donationRepo) Create(ctx context.Context, donation *domain.Donation) error {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		INSERT INTO donations (campaign_id, donor_id, amount, is_anonymous, message, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`
	err := runner.QueryRowContext(ctx, query,
		donation.CampaignID, donation.DonorID, donation.Amount, donation.IsAnonymous, donation.Message, donation.Status,
	).Scan(&donation.ID, &donation.CreatedAt, &donation.UpdatedAt)
	if err != nil {
		return err
	}
	return nil
}

func (r *donationRepo) FindByID(ctx context.Context, id string) (*domain.Donation, error) {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		SELECT id, campaign_id, donor_id, amount, is_anonymous, message, status, created_at, updated_at
		FROM donations WHERE id = $1
	`
	var d domain.Donation
	err := runner.QueryRowContext(ctx, query, id).
		Scan(&d.ID, &d.CampaignID, &d.DonorID, &d.Amount, &d.IsAnonymous, &d.Message, &d.Status, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}

func (r *donationRepo) ListByCampaign(ctx context.Context, campaignID string, limit, offset int) ([]*domain.Donation, error) {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		SELECT id, campaign_id, donor_id, amount, is_anonymous, message, status, created_at, updated_at
		FROM donations
		WHERE campaign_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := runner.QueryContext(ctx, query, campaignID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var donations []*domain.Donation
	for rows.Next() {
		var d domain.Donation
		if err := rows.Scan(&d.ID, &d.CampaignID, &d.DonorID, &d.Amount, &d.IsAnonymous, &d.Message, &d.Status, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		donations = append(donations, &d)
	}
	return donations, rows.Err()
}

func (r *donationRepo) UpdateStatus(ctx context.Context, id string, status string) error {
	runner := GetQueryRunner(ctx, r.db)
	query := `UPDATE donations SET status = $1, updated_at = NOW() WHERE id = $2`
	res, err := runner.ExecContext(ctx, query, status, id)
	if err != nil {
		return err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *donationRepo) ListByUser(ctx context.Context, userID string, limit, offset int) ([]*domain.Donation, error) {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		SELECT id, campaign_id, donor_id, amount, is_anonymous, message, status, created_at, updated_at
		FROM donations
		WHERE donor_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := runner.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var donations []*domain.Donation
	for rows.Next() {
		var d domain.Donation
		if err := rows.Scan(&d.ID, &d.CampaignID, &d.DonorID, &d.Amount, &d.IsAnonymous, &d.Message, &d.Status, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		donations = append(donations, &d)
	}
	return donations, rows.Err()
}

func (r *donationRepo) Update(ctx context.Context, donation *domain.Donation) error {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		UPDATE donations
		SET status = $1, message = $2, is_anonymous = $3, updated_at = NOW()
		WHERE id = $4
		RETURNING updated_at
	`
	err := runner.QueryRowContext(ctx, query, donation.Status, donation.Message, donation.IsAnonymous, donation.ID).Scan(&donation.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrNotFound
		}
		return err
	}
	return nil
}
