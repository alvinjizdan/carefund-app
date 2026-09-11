package config_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"carefund-api/internal/config"
)

func clearEnv(t *testing.T) func() {
	t.Helper()
	keys := []string{
		"ENV", "DB_PASSWORD", "PORT", "METRICS_HOST", "METRICS_PORT",
		"WORKER_METRICS_HOST", "WORKER_METRICS_PORT", "DB_HOST", "DB_PORT",
		"DB_USER", "DB_NAME", "DB_SSLMODE", "DB_MAX_OPEN_CONNS", "DB_MAX_IDLE_CONNS",
		"DB_CONN_MAX_LIFETIME", "DB_STATEMENT_TIMEOUT", "DB_LOCK_TIMEOUT",
		"DB_IDLE_IN_TRANSACTION_TIMEOUT", "JWT_SECRET", "MIDTRANS_SERVER_KEY",
		"CORS_ALLOWED_ORIGINS",
	}

	saved := make(map[string]string)
	for _, k := range keys {
		if v, exists := os.LookupEnv(k); exists {
			saved[k] = v
		}
		os.Unsetenv(k)
	}

	return func() {
		for _, k := range keys {
			if v, exists := saved[k]; exists {
				os.Setenv(k, v)
			} else {
				os.Unsetenv(k)
			}
		}
	}
}

func TestLoad_ProductionRequiresDBPassword(t *testing.T) {
	restore := clearEnv(t)
	defer restore()

	os.Setenv("ENV", "production")
	os.Setenv("JWT_SECRET", "prod-secret")
	os.Setenv("MIDTRANS_SERVER_KEY", "prod-midtrans-key")
	os.Setenv("CORS_ALLOWED_ORIGINS", "https://carefund.org")
	os.Setenv("DB_SSLMODE", "require")

	// 1. Missing DB_PASSWORD in production -> Must fail fast
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when DB_PASSWORD is not set in production, got nil")
	}
	if !strings.Contains(err.Error(), "DB_PASSWORD is required in production") {
		t.Fatalf("unexpected error message: %v", err)
	}
	// Verify error does NOT leak any fallback password string
	if strings.Contains(err.Error(), "234djisamSOE") {
		t.Fatal("error message leaked fallback password")
	}

	// 2. Explicit DB_PASSWORD in production -> Must succeed
	os.Setenv("DB_PASSWORD", "super-secret-production-pw")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error when DB_PASSWORD is provided in production: %v", err)
	}
	if cfg.DBPassword != "super-secret-production-pw" {
		t.Fatalf("expected DBPassword 'super-secret-production-pw', got %s", cfg.DBPassword)
	}
}

func TestLoad_ProductionRequiresEncryptedDBSSLMode(t *testing.T) {
	setupProd := func(t *testing.T) func() {
		restore := clearEnv(t)
		os.Setenv("ENV", "production")
		os.Setenv("JWT_SECRET", "prod-secret-32bytes-for-testing")
		os.Setenv("MIDTRANS_SERVER_KEY", "prod-midtrans-key")
		os.Setenv("CORS_ALLOWED_ORIGINS", "https://carefund.org")
		os.Setenv("DB_PASSWORD", "prod-secure-db-password")
		return restore
	}

	t.Run("production default (unset) DB_SSLMODE fails fast", func(t *testing.T) {
		restore := setupProd(t)
		defer restore()

		_, err := config.Load()
		if err == nil {
			t.Fatal("expected error when DB_SSLMODE is unset in production, got nil")
		}
		if !strings.Contains(err.Error(), "DB_SSLMODE cannot be 'disable' in production") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("production explicit DB_SSLMODE=disable fails fast", func(t *testing.T) {
		restore := setupProd(t)
		defer restore()

		os.Setenv("DB_SSLMODE", "disable")
		_, err := config.Load()
		if err == nil {
			t.Fatal("expected error when DB_SSLMODE is disable in production, got nil")
		}
		if !strings.Contains(err.Error(), "DB_SSLMODE cannot be 'disable' in production") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("production valid encrypted modes succeed", func(t *testing.T) {
		for _, mode := range []string{"require", "verify-ca", "verify-full"} {
			restore := setupProd(t)
			os.Setenv("DB_SSLMODE", mode)

			cfg, err := config.Load()
			if err != nil {
				t.Fatalf("expected mode %q to succeed in production, got: %v", mode, err)
			}
			if cfg.DBSSLMode != mode {
				t.Fatalf("expected DBSSLMode %q, got %q", mode, cfg.DBSSLMode)
			}
			restore()
		}
	})

	t.Run("development permits default and explicit disable", func(t *testing.T) {
		restore := clearEnv(t)
		defer restore()

		os.Setenv("ENV", "development")
		os.Setenv("JWT_SECRET", "dev-secret")
		os.Setenv("MIDTRANS_SERVER_KEY", "dev-key")

		// Unset -> defaults to disable
		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("unexpected error in development with default SSLMODE: %v", err)
		}
		if cfg.DBSSLMode != "disable" {
			t.Fatalf("expected default 'disable', got %q", cfg.DBSSLMode)
		}

		// Explicit disable
		os.Setenv("DB_SSLMODE", "disable")
		cfg, err = config.Load()
		if err != nil {
			t.Fatalf("unexpected error in development with explicit disable: %v", err)
		}
		if cfg.DBSSLMode != "disable" {
			t.Fatalf("expected 'disable', got %q", cfg.DBSSLMode)
		}
	})

	t.Run("test environment permits disable", func(t *testing.T) {
		restore := clearEnv(t)
		defer restore()

		os.Setenv("ENV", "test")
		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("unexpected error in test env: %v", err)
		}
		if cfg.DBSSLMode != "disable" {
			t.Fatalf("expected 'disable', got %q", cfg.DBSSLMode)
		}
	})
}

func TestLoad_DevelopmentFallbackDBPassword(t *testing.T) {
	restore := clearEnv(t)
	defer restore()

	os.Setenv("ENV", "development")
	os.Setenv("JWT_SECRET", "dev-secret")
	os.Setenv("MIDTRANS_SERVER_KEY", "dev-midtrans-key")

	// Missing DB_PASSWORD in development -> uses dev fallback
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error in development: %v", err)
	}
	if cfg.DBPassword != "234djisamSOE" {
		t.Fatalf("expected fallback '234djisamSOE', got %s", cfg.DBPassword)
	}

	// Explicit DB_PASSWORD in development -> overrides fallback
	os.Setenv("DB_PASSWORD", "custom-dev-password")
	cfg, err = config.Load()
	if err != nil {
		t.Fatalf("unexpected error in development: %v", err)
	}
	if cfg.DBPassword != "custom-dev-password" {
		t.Fatalf("expected 'custom-dev-password', got %s", cfg.DBPassword)
	}
}

func TestMetricsAddress_DefaultAndOverride(t *testing.T) {
	restore := clearEnv(t)
	defer restore()

	os.Setenv("ENV", "test")

	// Defaults: METRICS_HOST and WORKER_METRICS_HOST default to 127.0.0.1
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}

	if cfg.MetricsHost != "127.0.0.1" {
		t.Fatalf("expected default MetricsHost '127.0.0.1', got '%s'", cfg.MetricsHost)
	}
	if cfg.MetricsAddress() != "127.0.0.1:9090" {
		t.Fatalf("expected default MetricsAddress '127.0.0.1:9090', got '%s'", cfg.MetricsAddress())
	}

	if cfg.WorkerMetricsHost != "127.0.0.1" {
		t.Fatalf("expected default WorkerMetricsHost '127.0.0.1', got '%s'", cfg.WorkerMetricsHost)
	}
	if cfg.WorkerMetricsAddress() != "127.0.0.1:9091" {
		t.Fatalf("expected default WorkerMetricsAddress '127.0.0.1:9091', got '%s'", cfg.WorkerMetricsAddress())
	}

	// Explicit override: e.g. binding to a private interface or 0.0.0.0 in a secure container network
	os.Setenv("METRICS_HOST", "10.0.0.5")
	os.Setenv("METRICS_PORT", "9100")
	os.Setenv("WORKER_METRICS_HOST", "0.0.0.0")
	os.Setenv("WORKER_METRICS_PORT", "9101")

	cfg, err = config.Load()
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}

	if cfg.MetricsAddress() != "10.0.0.5:9100" {
		t.Fatalf("expected overridden MetricsAddress '10.0.0.5:9100', got '%s'", cfg.MetricsAddress())
	}
	if cfg.WorkerMetricsAddress() != "0.0.0.0:9101" {
		t.Fatalf("expected overridden WorkerMetricsAddress '0.0.0.0:9101', got '%s'", cfg.WorkerMetricsAddress())
	}
}

func TestDatabaseConnectionPoolAndDSNTimeouts(t *testing.T) {
	restore := clearEnv(t)
	defer restore()

	os.Setenv("ENV", "test")

	// 1. Defaults verification
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}

	if cfg.DBMaxOpenConns != 25 {
		t.Fatalf("expected default DBMaxOpenConns 25, got %d", cfg.DBMaxOpenConns)
	}
	if cfg.DBMaxIdleConns != 10 {
		t.Fatalf("expected default DBMaxIdleConns 10, got %d", cfg.DBMaxIdleConns)
	}
	if cfg.DBConnMaxLifetime != 15*time.Minute {
		t.Fatalf("expected default DBConnMaxLifetime 15m, got %v", cfg.DBConnMaxLifetime)
	}
	if cfg.DBStatementTimeout != 15*time.Second {
		t.Fatalf("expected default DBStatementTimeout 15s, got %v", cfg.DBStatementTimeout)
	}
	if cfg.DBLockTimeout != 5*time.Second {
		t.Fatalf("expected default DBLockTimeout 5s, got %v", cfg.DBLockTimeout)
	}
	if cfg.DBIdleTxTimeout != 10*time.Second {
		t.Fatalf("expected default DBIdleTxTimeout 10s, got %v", cfg.DBIdleTxTimeout)
	}

	dsn := cfg.DSN()
	if !strings.Contains(dsn, "statement_timeout=15000") {
		t.Fatalf("DSN missing statement_timeout=15000: %s", dsn)
	}
	if !strings.Contains(dsn, "lock_timeout=5000") {
		t.Fatalf("DSN missing lock_timeout=5000: %s", dsn)
	}
	if !strings.Contains(dsn, "idle_in_transaction_session_timeout=10000") {
		t.Fatalf("DSN missing idle_in_transaction_session_timeout=10000: %s", dsn)
	}

	// 2. Custom values and MaxIdleConns clamping
	os.Setenv("DB_MAX_OPEN_CONNS", "20")
	os.Setenv("DB_MAX_IDLE_CONNS", "50") // Greater than open -> should clamp to 20
	os.Setenv("DB_CONN_MAX_LIFETIME", "30m")
	os.Setenv("DB_STATEMENT_TIMEOUT", "8s")
	os.Setenv("DB_LOCK_TIMEOUT", "3s")
	os.Setenv("DB_IDLE_IN_TRANSACTION_TIMEOUT", "6s")

	cfg, err = config.Load()
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}

	if cfg.DBMaxOpenConns != 20 {
		t.Fatalf("expected DBMaxOpenConns 20, got %d", cfg.DBMaxOpenConns)
	}
	if cfg.DBMaxIdleConns != 20 {
		t.Fatalf("expected DBMaxIdleConns clamped to 20, got %d", cfg.DBMaxIdleConns)
	}
	if cfg.DBConnMaxLifetime != 30*time.Minute {
		t.Fatalf("expected DBConnMaxLifetime 30m, got %v", cfg.DBConnMaxLifetime)
	}

	dsn = cfg.DSN()
	if !strings.Contains(dsn, "statement_timeout=8000") {
		t.Fatalf("DSN missing custom statement_timeout=8000: %s", dsn)
	}
	if !strings.Contains(dsn, "lock_timeout=3000") {
		t.Fatalf("DSN missing custom lock_timeout=3000: %s", dsn)
	}
	if !strings.Contains(dsn, "idle_in_transaction_session_timeout=6000") {
		t.Fatalf("DSN missing custom idle_in_transaction_session_timeout=6000: %s", dsn)
	}
}

func TestLoad_ProductionRequiresAppSecrets(t *testing.T) {
	restore := clearEnv(t)
	defer restore()

	setupValidProd := func() {
		os.Setenv("ENV", "production")
		os.Setenv("DB_PASSWORD", "super-secret-db-pw")
		os.Setenv("DB_SSLMODE", "require")
		os.Setenv("JWT_SECRET", "super-secret-jwt")
		os.Setenv("MIDTRANS_SERVER_KEY", "super-secret-midtrans")
		os.Setenv("CORS_ALLOWED_ORIGINS", "https://carefund.org")
	}

	t.Run("missing JWT_SECRET fails fast", func(t *testing.T) {
		setupValidProd()
		os.Unsetenv("JWT_SECRET")

		_, err := config.Load()
		if err == nil {
			t.Fatal("expected error when JWT_SECRET is unset in production, got nil")
		}
		if !strings.Contains(err.Error(), "JWT_SECRET is required") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("missing MIDTRANS_SERVER_KEY fails fast", func(t *testing.T) {
		setupValidProd()
		os.Unsetenv("MIDTRANS_SERVER_KEY")

		_, err := config.Load()
		if err == nil {
			t.Fatal("expected error when MIDTRANS_SERVER_KEY is unset in production, got nil")
		}
		if !strings.Contains(err.Error(), "MIDTRANS_SERVER_KEY is required") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("missing CORS_ALLOWED_ORIGINS fails fast", func(t *testing.T) {
		setupValidProd()
		os.Unsetenv("CORS_ALLOWED_ORIGINS")

		_, err := config.Load()
		if err == nil {
			t.Fatal("expected error when CORS_ALLOWED_ORIGINS is unset in production, got nil")
		}
		if !strings.Contains(err.Error(), "CORS_ALLOWED_ORIGINS is required") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("localhost CORS_ALLOWED_ORIGINS fails fast in production", func(t *testing.T) {
		setupValidProd()
		os.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")

		_, err := config.Load()
		if err == nil {
			t.Fatal("expected error when CORS_ALLOWED_ORIGINS is localhost in production, got nil")
		}
		if !strings.Contains(err.Error(), "CORS_ALLOWED_ORIGINS is required and cannot default to localhost") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestLoadMigrateConfig_ProductionIsolation(t *testing.T) {
	restore := clearEnv(t)
	defer restore()

	setupValidMigrateProd := func() {
		os.Setenv("ENV", "production")
		os.Setenv("DB_HOST", "managed-db.internal")
		os.Setenv("DB_PORT", "5432")
		os.Setenv("DB_USER", "carefund_app")
		os.Setenv("DB_PASSWORD", "prod-migrate-db-password")
		os.Setenv("DB_NAME", "carefund_prod")
		os.Setenv("DB_SSLMODE", "require")
		// Explicitly ensure application-layer secrets and CORS are UNSET
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("MIDTRANS_SERVER_KEY")
		os.Unsetenv("CORS_ALLOWED_ORIGINS")
	}

	t.Run("succeeds with only database configuration present", func(t *testing.T) {
		setupValidMigrateProd()

		cfg, err := config.LoadMigrateConfig()
		if err != nil {
			t.Fatalf("expected LoadMigrateConfig to succeed with only DB config, got: %v", err)
		}

		if cfg.DBHost != "managed-db.internal" {
			t.Errorf("expected DBHost 'managed-db.internal', got '%s'", cfg.DBHost)
		}
		if cfg.DBPassword != "prod-migrate-db-password" {
			t.Errorf("expected DBPassword 'prod-migrate-db-password', got '%s'", cfg.DBPassword)
		}
		if cfg.DBSSLMode != "require" {
			t.Errorf("expected DBSSLMode 'require', got '%s'", cfg.DBSSLMode)
		}
		// Confirm DSN is correctly generated
		dsn := cfg.DSN()
		if !strings.Contains(dsn, "host=managed-db.internal") || !strings.Contains(dsn, "sslmode=require") {
			t.Fatalf("unexpected DSN generated: %s", dsn)
		}
	})

	t.Run("fails when DB_PASSWORD is absent in production", func(t *testing.T) {
		setupValidMigrateProd()
		os.Unsetenv("DB_PASSWORD")

		_, err := config.LoadMigrateConfig()
		if err == nil {
			t.Fatal("expected error when DB_PASSWORD is unset in production, got nil")
		}
		if !strings.Contains(err.Error(), "DB_PASSWORD is required in production") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("fails when DB_SSLMODE is disable or empty in production", func(t *testing.T) {
		setupValidMigrateProd()
		os.Setenv("DB_SSLMODE", "disable")

		_, err := config.LoadMigrateConfig()
		if err == nil {
			t.Fatal("expected error when DB_SSLMODE is disable in production, got nil")
		}
		if !strings.Contains(err.Error(), "DB_SSLMODE cannot be 'disable' in production") {
			t.Fatalf("unexpected error message: %v", err)
		}

		os.Unsetenv("DB_SSLMODE")
		_, err = config.LoadMigrateConfig()
		if err == nil {
			t.Fatal("expected error when DB_SSLMODE is empty in production, got nil")
		}
	})

	t.Run("development permits default fallback password and disable SSL", func(t *testing.T) {
		os.Setenv("ENV", "development")
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("DB_SSLMODE")

		cfg, err := config.LoadMigrateConfig()
		if err != nil {
			t.Fatalf("unexpected error in development LoadMigrateConfig: %v", err)
		}
		if cfg.DBPassword != "234djisamSOE" {
			t.Errorf("expected fallback '234djisamSOE', got '%s'", cfg.DBPassword)
		}
		if cfg.DBSSLMode != "disable" {
			t.Errorf("expected default 'disable', got '%s'", cfg.DBSSLMode)
		}
	})
}
