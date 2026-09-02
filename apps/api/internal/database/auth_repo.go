package database

import (
	"context"
	"database/sql"
	"errors"

	"carefund-api/internal/domain"
)

type refreshTokenRepo struct {
	db *DB
}

func NewRefreshTokenRepository(db *DB) domain.RefreshTokenRepository {
	return &refreshTokenRepo{db: db}
}

func (r *refreshTokenRepo) Create(ctx context.Context, token *domain.RefreshToken) error {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, NOW())
		RETURNING id, created_at
	`
	err := runner.QueryRowContext(ctx, query, token.UserID, token.TokenHash, token.ExpiresAt).Scan(&token.ID, &token.CreatedAt)
	if err != nil {
		return domain.ErrInternalError
	}
	return nil
}

func (r *refreshTokenRepo) FindByTokenHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		SELECT id, user_id, token_hash, expires_at, revoked_at, created_at, last_used_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`
	var token domain.RefreshToken
	err := runner.QueryRowContext(ctx, query, hash).Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.RevokedAt,
		&token.CreatedAt,
		&token.LastUsedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, domain.ErrInternalError
	}
	return &token, nil
}

func (r *refreshTokenRepo) Update(ctx context.Context, token *domain.RefreshToken) error {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		UPDATE refresh_tokens
		SET revoked_at = $1, last_used_at = $2
		WHERE id = $3
	`
	res, err := runner.ExecContext(ctx, query, token.RevokedAt, token.LastUsedAt, token.ID)
	if err != nil {
		return domain.ErrInternalError
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *refreshTokenRepo) RevokeByUserID(ctx context.Context, userID string) error {
	runner := GetQueryRunner(ctx, r.db.DB)
	query := `
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE user_id = $1 AND revoked_at IS NULL
	`
	_, err := runner.ExecContext(ctx, query, userID)
	if err != nil {
		return domain.ErrInternalError
	}
	return nil
}
