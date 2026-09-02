package database

import (
	"context"
	"database/sql"
	"errors"

	"carefund-api/internal/domain"
	"github.com/lib/pq"
)

type campaignRepo struct {
	db *sql.DB
}

func NewCampaignRepository(db *DB) domain.CampaignRepository {
	return &campaignRepo{db: db.DB}
}

func (r *campaignRepo) Create(ctx context.Context, campaign *domain.Campaign) error {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		INSERT INTO campaigns (owner_id, category_id, title, slug, description, target_amount, current_amount, start_at, end_at, status, rejection_reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`
	err := runner.QueryRowContext(ctx, query,
		campaign.OwnerID, campaign.CategoryID, campaign.Title, campaign.Slug, campaign.Description,
		campaign.TargetAmount, campaign.CurrentAmount, campaign.StartAt, campaign.EndAt, campaign.Status, campaign.RejectionReason,
	).Scan(&campaign.ID, &campaign.CreatedAt, &campaign.UpdatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return domain.ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *campaignRepo) FindByID(ctx context.Context, id string) (*domain.Campaign, error) {
	return r.findByID(ctx, id, false)
}

func (r *campaignRepo) FindByIDForUpdate(ctx context.Context, id string) (*domain.Campaign, error) {
	return r.findByID(ctx, id, true)
}

func (r *campaignRepo) findByID(ctx context.Context, id string, forUpdate bool) (*domain.Campaign, error) {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		SELECT id, owner_id, category_id, title, slug, description, target_amount, current_amount, start_at, end_at, status, rejection_reason, created_at, updated_at
		FROM campaigns
		WHERE id = $1
	`
	if forUpdate {
		query += " FOR UPDATE"
	}

	var c domain.Campaign
	err := runner.QueryRowContext(ctx, query, id).Scan(
		&c.ID, &c.OwnerID, &c.CategoryID, &c.Title, &c.Slug, &c.Description, &c.TargetAmount, &c.CurrentAmount, &c.StartAt, &c.EndAt, &c.Status, &c.RejectionReason, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, domain.ErrInternalError
	}
	return &c, nil
}

func (r *campaignRepo) FindBySlug(ctx context.Context, slug string) (*domain.Campaign, error) {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		SELECT id, owner_id, category_id, title, slug, description, target_amount, current_amount, start_at, end_at, status, rejection_reason, created_at, updated_at
		FROM campaigns WHERE slug = $1
	`
	var c domain.Campaign
	err := runner.QueryRowContext(ctx, query, slug).
		Scan(&c.ID, &c.OwnerID, &c.CategoryID, &c.Title, &c.Slug, &c.Description, &c.TargetAmount, &c.CurrentAmount, &c.StartAt, &c.EndAt, &c.Status, &c.RejectionReason, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *campaignRepo) Update(ctx context.Context, campaign *domain.Campaign) error {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		UPDATE campaigns
		SET title = $1, description = $2, category_id = $3, target_amount = $4, start_at = $5, end_at = $6, status = $7, rejection_reason = $8, current_amount = $9, updated_at = NOW()
		WHERE id = $10
		RETURNING updated_at
	`
	err := runner.QueryRowContext(ctx, query,
		campaign.Title, campaign.Description, campaign.CategoryID, campaign.TargetAmount,
		campaign.StartAt, campaign.EndAt, campaign.Status, campaign.RejectionReason, campaign.CurrentAmount, campaign.ID,
	).Scan(&campaign.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrNotFound
		}
		return err
	}
	return nil
}

func (r *campaignRepo) List(ctx context.Context, limit, offset int) ([]*domain.Campaign, error) {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		SELECT id, owner_id, category_id, title, slug, description, target_amount, current_amount, start_at, end_at, status, rejection_reason, created_at, updated_at
		FROM campaigns
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := runner.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var campaigns []*domain.Campaign
	for rows.Next() {
		var c domain.Campaign
		if err := rows.Scan(&c.ID, &c.OwnerID, &c.CategoryID, &c.Title, &c.Slug, &c.Description, &c.TargetAmount, &c.CurrentAmount, &c.StartAt, &c.EndAt, &c.Status, &c.RejectionReason, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		campaigns = append(campaigns, &c)
	}
	return campaigns, rows.Err()
}

func (r *campaignRepo) ListByOwner(ctx context.Context, ownerID string, limit, offset int) ([]*domain.Campaign, error) {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		SELECT id, owner_id, category_id, title, slug, description, target_amount, current_amount, start_at, end_at, status, rejection_reason, created_at, updated_at
		FROM campaigns
		WHERE owner_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := runner.QueryContext(ctx, query, ownerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var campaigns []*domain.Campaign
	for rows.Next() {
		var c domain.Campaign
		if err := rows.Scan(&c.ID, &c.OwnerID, &c.CategoryID, &c.Title, &c.Slug, &c.Description, &c.TargetAmount, &c.CurrentAmount, &c.StartAt, &c.EndAt, &c.Status, &c.RejectionReason, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		campaigns = append(campaigns, &c)
	}
	return campaigns, rows.Err()
}
