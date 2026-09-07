package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"

	"carefund-api/internal/config"
	"carefund-api/internal/logger"
)

func main() {
	var (
		migrationsPath = flag.String("path", "migrations", "path to migrations directory")
		direction      = flag.String("cmd", "up", "migration command: up or version")
	)
	flag.Parse()

	// Positional arguments override if provided: e.g. "migrate up", "migrate version"
	if args := flag.Args(); len(args) > 0 {
		*direction = args[0]
	}

	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal(ctx, "Failed to load application configuration", err, logger.F("component", "Migrator"))
	}

	if err := RunMigration(ctx, cfg, *migrationsPath, *direction); err != nil {
		logger.Fatal(ctx, "Migration execution failed", err, logger.F("component", "Migrator"), logger.F("command", *direction))
	}
}

// RunMigration connects to PostgreSQL using the hardened configuration, acquires
// the migration advisory lock, and executes the specified migration command forward-only.
func RunMigration(ctx context.Context, cfg *config.Config, migrationsDir, cmd string) error {
	if cfg == nil {
		return errors.New("configuration is required")
	}

	if cmd == "down" {
		return errors.New("down migrations are not permitted through this command; rollback must be executed manually with verified rollback scripts")
	}
	if cmd != "up" && cmd != "version" && cmd != "status" {
		return fmt.Errorf("unsupported migration command: %q (supported commands: 'up', 'version')", cmd)
	}

	// Verify migrations directory exists
	if _, err := os.Stat(migrationsDir); err != nil {
		return fmt.Errorf("migrations directory not accessible: %w", err)
	}

	// Connect to database using existing DSN
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{
		MigrationsTable: "schema_migrations",
	})
	if err != nil {
		return fmt.Errorf("failed to create postgres migration driver: %w", err)
	}

	// Use os.DirFS + iofs driver to ensure reliable cross-platform (Windows & Linux) filesystem paths
	sourceDriver, err := iofs.New(os.DirFS(migrationsDir), ".")
	if err != nil {
		return fmt.Errorf("failed to initialize migration source driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to initialize migrator: %w", err)
	}
	defer m.Close()

	switch cmd {
	case "up":
		logger.Info(ctx, "Executing forward database migrations...", logger.F("component", "Migrator"), logger.F("path", migrationsDir))
		err = m.Up()
		if err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("migration up failed: %w", err)
		}
		if errors.Is(err, migrate.ErrNoChange) {
			logger.Info(ctx, "Database schema is already up to date (no change)", logger.F("component", "Migrator"))
		} else {
			logger.Info(ctx, "Database migrations applied successfully", logger.F("component", "Migrator"))
		}

		v, dirty, vErr := m.Version()
		if vErr != nil && !errors.Is(vErr, migrate.ErrNilVersion) {
			logger.Warn(ctx, "Unable to inspect schema version after migration", logger.F("component", "Migrator"), logger.F("err", vErr.Error()))
		} else {
			logger.Info(ctx, "Current database schema status",
				logger.F("component", "Migrator"),
				logger.F("version", v),
				logger.F("dirty", dirty),
			)
		}
		return nil

	case "version", "status":
		v, dirty, vErr := m.Version()
		if vErr != nil {
			if errors.Is(vErr, migrate.ErrNilVersion) {
				logger.Info(ctx, "No migrations have been applied yet (schema version is nil)", logger.F("component", "Migrator"))
				return nil
			}
			return fmt.Errorf("failed to fetch schema version: %w", vErr)
		}
		logger.Info(ctx, "Database schema version",
			logger.F("component", "Migrator"),
			logger.F("version", v),
			logger.F("dirty", dirty),
		)
		if dirty {
			return fmt.Errorf("database schema is in a DIRTY state at version %d (manual intervention required)", v)
		}
		return nil

	default:
		return fmt.Errorf("unsupported migration command: %q (supported commands: 'up', 'version')", cmd)
	}
}
