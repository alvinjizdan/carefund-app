package database

import (
	"context"
	"database/sql"
	"fmt"
)

type contextKey string

const txKey = contextKey("tx")

type TransactionManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type txManager struct {
	db *sql.DB
}

func NewTransactionManager(db *DB) TransactionManager {
	return &txManager{db: db.DB}
}

func (tm *txManager) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := tm.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	txCtx := context.WithValue(ctx, txKey, tx)

	err = fn(txCtx)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx err: %v, rollback err: %v", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetQueryRunner extracts either the transaction or the DB from context
// so repositories don't need to know if they are in a transaction or not.
func GetQueryRunner(ctx context.Context, db *sql.DB) QueryRunner {
	if tx, ok := ctx.Value(txKey).(*sql.Tx); ok {
		return tx
	}
	return db
}

// QueryRunner interface matches both *sql.DB and *sql.Tx
type QueryRunner interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
