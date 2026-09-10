package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                string
	MetricsHost         string
	MetricsPort         string
	WorkerMetricsHost   string
	WorkerMetricsPort   string
	Env                 string
	DBHost              string
	DBPort              string
	DBUser              string
	DBPassword          string
	DBName              string
	DBSSLMode           string
	DBMaxOpenConns      int
	DBMaxIdleConns      int
	DBConnMaxLifetime   time.Duration
	DBStatementTimeout  time.Duration
	DBLockTimeout       time.Duration
	DBIdleTxTimeout     time.Duration
	JWTSecret           string
	JWTAccessTTL        time.Duration
	MidtransServerKey   string
	MidtransClientKey   string
	MidtransEnvironment string
	PaymentPendingTTL   time.Duration
	OutboxProcessingTTL time.Duration
	CORSAllowedOrigins  string
	TrustedProxyCIDRs   string
}

func Load() (*Config, error) {
	_ = godotenv.Load() // Ignore error if .env doesn't exist

	jwtTTLStr := getEnv("JWT_ACCESS_TTL", "15m")
	jwtTTL, err := time.ParseDuration(jwtTTLStr)
	if err != nil {
		jwtTTL = 15 * time.Minute
	}

	paymentTTLStr := getEnv("PAYMENT_PENDING_TTL", "45m")
	paymentTTL, err := time.ParseDuration(paymentTTLStr)
	if err != nil {
		paymentTTL = 45 * time.Minute
	}

	outboxTTLStr := getEnv("OUTBOX_PROCESSING_TTL", "15m")
	outboxTTL, err := time.ParseDuration(outboxTTLStr)
	if err != nil {
		outboxTTL = 15 * time.Minute
	}

	corsOrigins := getEnv("CORS_ALLOWED_ORIGINS", "")
	env := getEnv("ENV", "development")
	if corsOrigins == "" && env != "production" {
		corsOrigins = "http://localhost:3000"
	}

	// Database password: In production, no hardcoded fallback is permitted.
	dbPassword := os.Getenv("DB_PASSWORD")
	if env != "production" && dbPassword == "" {
		dbPassword = "234djisamSOE"
	}

	// Database connection pool configuration
	dbMaxOpenConns := getEnvInt("DB_MAX_OPEN_CONNS", 25)
	if dbMaxOpenConns <= 0 {
		dbMaxOpenConns = 25
	}
	dbMaxIdleConns := getEnvInt("DB_MAX_IDLE_CONNS", 10)
	if dbMaxIdleConns <= 0 {
		dbMaxIdleConns = 10
	}
	if dbMaxIdleConns > dbMaxOpenConns {
		dbMaxIdleConns = dbMaxOpenConns
	}

	dbConnMaxLifetimeStr := getEnv("DB_CONN_MAX_LIFETIME", "15m")
	dbConnMaxLifetime, err := time.ParseDuration(dbConnMaxLifetimeStr)
	if err != nil || dbConnMaxLifetime <= 0 {
		dbConnMaxLifetime = 15 * time.Minute
	}

	// PostgreSQL server-side timeouts
	// statement_timeout: limits query execution time (default 15s)
	dbStatementTimeoutStr := getEnv("DB_STATEMENT_TIMEOUT", "15s")
	dbStatementTimeout, err := time.ParseDuration(dbStatementTimeoutStr)
	if err != nil || dbStatementTimeout <= 0 {
		dbStatementTimeout = 15 * time.Second
	}

	// lock_timeout: limits time waiting to acquire a table or row lock (default 5s)
	dbLockTimeoutStr := getEnv("DB_LOCK_TIMEOUT", "5s")
	dbLockTimeout, err := time.ParseDuration(dbLockTimeoutStr)
	if err != nil || dbLockTimeout <= 0 {
		dbLockTimeout = 5 * time.Second
	}

	// idle_in_transaction_session_timeout: limits idle time within open transactions (default 10s)
	dbIdleTxTimeoutStr := getEnv("DB_IDLE_IN_TRANSACTION_TIMEOUT", "10s")
	dbIdleTxTimeout, err := time.ParseDuration(dbIdleTxTimeoutStr)
	if err != nil || dbIdleTxTimeout <= 0 {
		dbIdleTxTimeout = 10 * time.Second
	}

	// Metrics listener host binding: default loopback "127.0.0.1" for safety, overridable via env
	metricsHost := getEnv("METRICS_HOST", "127.0.0.1")
	workerMetricsHost := getEnv("WORKER_METRICS_HOST", "127.0.0.1")

	cfg := &Config{
		Port:                getEnv("PORT", "8080"),
		MetricsHost:         metricsHost,
		MetricsPort:         getEnv("METRICS_PORT", "9090"),
		WorkerMetricsHost:   workerMetricsHost,
		WorkerMetricsPort:   getEnv("WORKER_METRICS_PORT", "9091"),
		Env:                 env,
		DBHost:              getEnv("DB_HOST", "localhost"),
		DBPort:              getEnv("DB_PORT", "5432"),
		DBUser:              getEnv("DB_USER", "postgres"),
		DBPassword:          dbPassword,
		DBName:              getEnv("DB_NAME", "carefund-app"),
		DBSSLMode:           getEnv("DB_SSLMODE", "disable"),
		DBMaxOpenConns:      dbMaxOpenConns,
		DBMaxIdleConns:      dbMaxIdleConns,
		DBConnMaxLifetime:   dbConnMaxLifetime,
		DBStatementTimeout:  dbStatementTimeout,
		DBLockTimeout:       dbLockTimeout,
		DBIdleTxTimeout:     dbIdleTxTimeout,
		JWTSecret:           getEnv("JWT_SECRET", ""),
		JWTAccessTTL:        jwtTTL,
		MidtransServerKey:   getEnv("MIDTRANS_SERVER_KEY", ""),
		MidtransClientKey:   getEnv("MIDTRANS_CLIENT_KEY", ""),
		MidtransEnvironment: getEnv("MIDTRANS_ENVIRONMENT", "sandbox"),
		PaymentPendingTTL:   paymentTTL,
		OutboxProcessingTTL: outboxTTL,
		CORSAllowedOrigins:  corsOrigins,
		TrustedProxyCIDRs:   getEnv("TRUSTED_PROXY_CIDRS", ""),
	}

	if cfg.Env != "test" {
		if cfg.JWTSecret == "" {
			return nil, errors.New("JWT_SECRET is required")
		}
		if cfg.MidtransServerKey == "" {
			return nil, errors.New("MIDTRANS_SERVER_KEY is required")
		}
	}

	if cfg.Env == "production" {
		if cfg.DBPassword == "" {
			return nil, errors.New("DB_PASSWORD is required in production and cannot use a default fallback")
		}
		if cfg.CORSAllowedOrigins == "" || cfg.CORSAllowedOrigins == "http://localhost:3000" {
			return nil, errors.New("CORS_ALLOWED_ORIGINS is required and cannot default to localhost in production")
		}
		if cfg.DBSSLMode == "disable" || cfg.DBSSLMode == "" {
			return nil, errors.New("DB_SSLMODE cannot be 'disable' in production; encrypted PostgreSQL transport is required (e.g. require, verify-ca, verify-full)")
		}
	}

	return cfg, nil
}

// NewTestConfig loads configuration specifically for integration and unit testing.
// It prioritizes environment variables provided by the test runner (DB_HOST, DB_PORT,
// DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE), defaults DB_NAME to "carefund-app_test"
// if not explicitly set, and permits development fallbacks when running locally.
func NewTestConfig() (*Config, error) {
	if os.Getenv("ENV") == "" {
		_ = os.Setenv("ENV", "test")
	}
	cfg, err := Load()
	if err != nil {
		return nil, err
	}
	if os.Getenv("DB_NAME") == "" {
		cfg.DBName = "carefund-app_test"
	}
	return cfg, nil
}

// MetricsAddress returns the network listener address for the public API metrics server.
func (c *Config) MetricsAddress() string {
	if c.MetricsHost == "" {
		return ":" + c.MetricsPort
	}
	return fmt.Sprintf("%s:%s", c.MetricsHost, c.MetricsPort)
}

// WorkerMetricsAddress returns the network listener address for the worker metrics server.
func (c *Config) WorkerMetricsAddress() string {
	if c.WorkerMetricsHost == "" {
		return ":" + c.WorkerMetricsPort
	}
	return fmt.Sprintf("%s:%s", c.WorkerMetricsHost, c.WorkerMetricsPort)
}

func (c *Config) DSN() string {
	baseDSN := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode)

	// Append server-side timeouts in milliseconds when configured
	if c.DBStatementTimeout > 0 {
		baseDSN += fmt.Sprintf(" statement_timeout=%d", c.DBStatementTimeout.Milliseconds())
	}
	if c.DBLockTimeout > 0 {
		baseDSN += fmt.Sprintf(" lock_timeout=%d", c.DBLockTimeout.Milliseconds())
	}
	if c.DBIdleTxTimeout > 0 {
		baseDSN += fmt.Sprintf(" idle_in_transaction_session_timeout=%d", c.DBIdleTxTimeout.Milliseconds())
	}

	return baseDSN
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, exists := os.LookupEnv(key); exists {
		var i int
		if _, err := fmt.Sscanf(value, "%d", &i); err == nil {
			return i
		}
	}
	return fallback
}
