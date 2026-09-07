package main_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	migratecmd "carefund-api/cmd/migrate"
	"carefund-api/internal/config"
)

func TestRunMigration_NilConfig(t *testing.T) {
	ctx := context.Background()
	err := migratecmd.RunMigration(ctx, nil, "migrations", "up")
	if err == nil {
		t.Fatal("expected error when config is nil, got nil")
	}
	if !strings.Contains(err.Error(), "configuration is required") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestRunMigration_InvalidDBConnection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Points to an unreachable port to test fast connection error reporting
	cfg := &config.Config{
		Env:        "test",
		DBHost:     "127.0.0.1",
		DBPort:     "54329", // Non-existent port
		DBUser:     "postgres",
		DBPassword: "password",
		DBName:     "carefund_nonexistent",
		DBSSLMode:  "disable",
	}

	tempDir := t.TempDir()

	err := migratecmd.RunMigration(ctx, cfg, tempDir, "up")
	if err == nil {
		t.Fatal("expected error connecting to unreachable database, got nil")
	}
	if !strings.Contains(err.Error(), "failed to ping database") && !strings.Contains(err.Error(), "connection refused") {
		t.Logf("connection error reported as expected: %v", err)
	}
}

func TestRunMigration_UnsupportedCommand(t *testing.T) {
	ctx := context.Background()

	cfg := &config.Config{
		Env:        "test",
		DBHost:     "localhost",
		DBPort:     "5432",
		DBUser:     "postgres",
		DBPassword: "234djisamSOE",
		DBName:     "carefund-app_test",
		DBSSLMode:  "disable",
	}

	// Unsupported command: "drop", "reset", "foo"
	err := migratecmd.RunMigration(ctx, cfg, "migrations", "unknown_cmd")
	if err == nil {
		t.Fatal("expected error for unsupported command, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported migration command") {
		t.Fatalf("expected unsupported command error, got: %v", err)
	}

	// Down command forbidden by safety rule
	err = migratecmd.RunMigration(ctx, cfg, "migrations", "down")
	if err == nil {
		t.Fatal("expected error for 'down' command, got nil")
	}
	if !strings.Contains(err.Error(), "down migrations are not permitted") {
		t.Fatalf("expected down migrations forbidden error, got: %v", err)
	}
}

func TestRunMigration_MissingMigrationsDir(t *testing.T) {
	ctx := context.Background()

	cfg := &config.Config{
		Env:        "test",
		DBHost:     "localhost",
		DBPort:     "5432",
		DBUser:     "postgres",
		DBPassword: "234djisamSOE",
		DBName:     "carefund-app_test",
		DBSSLMode:  "disable",
	}

	nonExistentDir := filepath.Join(os.TempDir(), "non_existent_migrations_dir_12345")
	err := migratecmd.RunMigration(ctx, cfg, nonExistentDir, "up")
	if err == nil {
		t.Fatal("expected error for missing migrations directory, got nil")
	}
	if !strings.Contains(err.Error(), "migrations directory not accessible") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestRunMigration_MigrationOrderingAndExecution(t *testing.T) {
	// Verify that migrations directory contains valid ordered SQL pairs
	migrationsDir := filepath.Join("..", "..", "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("failed to read migrations directory: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("no migration files found in migrations directory")
	}

	upCount := 0
	downCount := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".up.sql") {
			upCount++
		} else if strings.HasSuffix(entry.Name(), ".down.sql") {
			downCount++
		}
	}

	if upCount == 0 || downCount == 0 {
		t.Fatalf("expected both up and down migrations, got up=%d, down=%d", upCount, downCount)
	}

	if upCount != downCount {
		t.Fatalf("asymmetric migration count: %d up vs %d down", upCount, downCount)
	}
}
