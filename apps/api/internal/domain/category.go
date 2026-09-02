package domain

import (
	"context"
	"time"
)

type Category struct {
	ID        string
	Name      string
	Slug      string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CategoryRepository interface {
	Create(ctx context.Context, category *Category) error
	FindByID(ctx context.Context, id string) (*Category, error)
	FindBySlug(ctx context.Context, slug string) (*Category, error)
	ListAllActive(ctx context.Context) ([]*Category, error)
}
