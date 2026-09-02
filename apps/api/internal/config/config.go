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
	Env                 string
	DBHost              string
	DBPort              string
	DBUser              string
	DBPassword          string
	DBName              string
	DBSSLMode           string
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

	cfg := &Config{
		Port:                getEnv("PORT", "8080"),
		Env:                 env,
		DBHost:              getEnv("DB_HOST", "localhost"),
		DBPort:              getEnv("DB_PORT", "5432"),
		DBUser:              getEnv("DB_USER", "postgres"),
		DBPassword:          getEnv("DB_PASSWORD", "234djisamSOE"),
		DBName:              getEnv("DB_NAME", "carefund-app"),
		DBSSLMode:           getEnv("DB_SSLMODE", "disable"),
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
		if cfg.CORSAllowedOrigins == "" || cfg.CORSAllowedOrigins == "http://localhost:3000" {
			return nil, errors.New("CORS_ALLOWED_ORIGINS is required and cannot default to localhost in production")
		}
	}

	return cfg, nil
}


func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode)
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
