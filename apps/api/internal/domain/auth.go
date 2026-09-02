package domain

import (
	"context"
	"time"
)

type RefreshToken struct {
	ID         string
	UserID     string
	TokenHash  string
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

func (rt *RefreshToken) IsValid() bool {
	if rt.RevokedAt != nil {
		return false
	}
	if time.Now().After(rt.ExpiresAt) {
		return false
	}
	return true
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, token *RefreshToken) error
	FindByTokenHash(ctx context.Context, hash string) (*RefreshToken, error)
	Update(ctx context.Context, token *RefreshToken) error
	RevokeByUserID(ctx context.Context, userID string) error
}
