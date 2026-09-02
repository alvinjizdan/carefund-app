package database

import (
	"context"
	"database/sql"
	"errors"

	"carefund-api/internal/domain"
	"github.com/lib/pq"
)

type roleRepo struct {
	db *sql.DB
}

func NewRoleRepository(db *DB) domain.RoleRepository {
	return &roleRepo{db: db.DB}
}

func (r *roleRepo) FindByName(ctx context.Context, name string) (*domain.Role, error) {
	runner := GetQueryRunner(ctx, r.db)
	query := `SELECT id, name, created_at FROM roles WHERE name = $1`
	var role domain.Role
	err := runner.QueryRowContext(ctx, query, name).Scan(&role.ID, &role.Name, &role.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &role, nil
}

func (r *roleRepo) GetUserRoles(ctx context.Context, userID string) ([]*domain.Role, error) {
	runner := GetQueryRunner(ctx, r.db)
	query := `
		SELECT r.id, r.name, r.created_at
		FROM roles r
		JOIN user_roles ur ON r.id = ur.role_id
		WHERE ur.user_id = $1
	`
	rows, err := runner.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []*domain.Role
	for rows.Next() {
		var role domain.Role
		if err := rows.Scan(&role.ID, &role.Name, &role.CreatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, &role)
	}
	return roles, rows.Err()
}

func (r *roleRepo) AssignRole(ctx context.Context, userID string, roleID string) error {
	runner := GetQueryRunner(ctx, r.db)
	query := `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`
	_, err := runner.ExecContext(ctx, query, userID, roleID)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" { // unique_violation
			return domain.ErrDuplicate
		}
		return err
	}
	return nil
}
