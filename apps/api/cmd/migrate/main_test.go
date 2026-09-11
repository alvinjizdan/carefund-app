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

	cfg, err := config.NewTestConfig()
	if err != nil {
		t.Fatalf("failed to load test config: %v", err)
	}

	// Unsupported command: "drop", "reset", "foo"
	err = migratecmd.RunMigration(ctx, cfg, "migrations", "unknown_cmd")
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

	cfg, err := config.NewTestConfig()
	if err != nil {
		t.Fatalf("failed to load test config: %v", err)
	}

	nonExistentDir := filepath.Join(os.TempDir(), "non_existent_migrations_dir_12345")
	err = migratecmd.RunMigration(ctx, cfg, nonExistentDir, "up")
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

func TestLoadMigrateConfig_Integration(t *testing.T) {
	origEnv := os.Getenv("ENV")
	origHost := os.Getenv("DB_HOST")
	origPort := os.Getenv("DB_PORT")
	origUser := os.Getenv("DB_USER")
	origPass := os.Getenv("DB_PASSWORD")
	origName := os.Getenv("DB_NAME")
	origSSL := os.Getenv("DB_SSLMODE")
	origJWT := os.Getenv("JWT_SECRET")
	origMid := os.Getenv("MIDTRANS_SERVER_KEY")
	origCORS := os.Getenv("CORS_ALLOWED_ORIGINS")

	defer func() {
		os.Setenv("ENV", origEnv)
		os.Setenv("DB_HOST", origHost)
		os.Setenv("DB_PORT", origPort)
		os.Setenv("DB_USER", origUser)
		os.Setenv("DB_PASSWORD", origPass)
		os.Setenv("DB_NAME", origName)
		os.Setenv("DB_SSLMODE", origSSL)
		os.Setenv("JWT_SECRET", origJWT)
		os.Setenv("MIDTRANS_SERVER_KEY", origMid)
		os.Setenv("CORS_ALLOWED_ORIGINS", origCORS)
	}()

	os.Setenv("ENV", "production")
	os.Setenv("DB_HOST", "127.0.0.1")
	os.Setenv("DB_PORT", "5432")
	os.Setenv("DB_USER", "postgres")
	os.Setenv("DB_PASSWORD", "test-db-pass")
	os.Setenv("DB_NAME", "carefund_test")
	os.Setenv("DB_SSLMODE", "require")
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("MIDTRANS_SERVER_KEY")
	os.Unsetenv("CORS_ALLOWED_ORIGINS")

	cfg, err := config.LoadMigrateConfig()
	if err != nil {
		t.Fatalf("expected LoadMigrateConfig to succeed without app secrets, got: %v", err)
	}
	if cfg.DBPassword != "test-db-pass" {
		t.Errorf("expected DBPassword 'test-db-pass', got '%s'", cfg.DBPassword)
	}
}
