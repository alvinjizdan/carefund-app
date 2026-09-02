package database

import (
	"context"
	"database/sql"
	"errors"

	"carefund-api/internal/domain"
	"github.com/lib/pq"
)

type categoryRepo struct {
	db *sql.DB
}

func NewCategoryRepository(db *DB) domain.CategoryRepository {
	return &categoryRepo{db: db.DB}
}

func (r *categoryRepo) Create(ctx context.Context, category *domain.Category) error {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		INSERT INTO categories (name, slug, is_active)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at
	`
	err := runner.QueryRowContext(ctx, query, category.Name, category.Slug, category.IsActive).
		Scan(&category.ID, &category.CreatedAt, &category.UpdatedAt)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" { // unique_violation
			return domain.ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *categoryRepo) FindByID(ctx context.Context, id string) (*domain.Category, error) {
	runner := GetQueryRunner(ctx, r.db)
	query := `SELECT id, name, slug, is_active, created_at, updated_at FROM categories WHERE id = $1`
	var c domain.Category
	err := runner.QueryRowContext(ctx, query, id).
		Scan(&c.ID, &c.Name, &c.Slug, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *categoryRepo) FindBySlug(ctx context.Context, slug string) (*domain.Category, error) {
	runner := GetQueryRunner(ctx, r.db)
	query := `SELECT id, name, slug, is_active, created_at, updated_at FROM categories WHERE slug = $1`
	var c domain.Category
	err := runner.QueryRowContext(ctx, query, slug).
		Scan(&c.ID, &c.Name, &c.Slug, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *categoryRepo) ListAllActive(ctx context.Context) ([]*domain.Category, error) {
	runner := GetQueryRunner(ctx, r.db)
	query := `SELECT id, name, slug, is_active, created_at, updated_at FROM categories WHERE is_active = true`
	rows, err := runner.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []*domain.Category
	for rows.Next() {
		var c domain.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		categories = append(categories, &c)
	}
	return categories, rows.Err()
}
