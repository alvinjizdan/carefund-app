package database

import (
	"context"
	"database/sql"
	"errors"

	"carefund-api/internal/domain"
	"github.com/lib/pq"
)

type userRepo struct {
	db *sql.DB
}

func NewUserRepository(db *DB) domain.UserRepository {
	return &userRepo{db: db.DB}
}

func (r *userRepo) Create(ctx context.Context, user *domain.User) error {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		INSERT INTO users (email, password_hash, name, phone, is_active)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`
	err := runner.QueryRowContext(ctx, query,
		user.Email, user.PasswordHash, user.Name, user.Phone, user.IsActive,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" { // unique_violation
			return domain.ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *userRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		SELECT id, email, password_hash, name, phone, is_active, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	var u domain.User
	err := runner.QueryRowContext(ctx, query, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Phone, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *userRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		SELECT id, email, password_hash, name, phone, is_active, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	var u domain.User
	err := runner.QueryRowContext(ctx, query, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Phone, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}
