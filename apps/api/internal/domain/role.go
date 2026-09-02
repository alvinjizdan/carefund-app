package domain

import (
	"context"
	"time"
)

type Role struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

type RoleRepository interface {
	FindByName(ctx context.Context, name string) (*Role, error)
	GetUserRoles(ctx context.Context, userID string) ([]*Role, error)
	AssignRole(ctx context.Context, userID string, roleID string) error
}
