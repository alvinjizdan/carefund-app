package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"carefund-api/internal/logger"
)

func TestLoggerContextAndSanitization(t *testing.T) {
	var buf bytes.Buffer
	logger.SetOutput(&buf)

	ctx := logger.ContextWithRequestID(context.Background(), "test-req-12345")

	logger.Info(ctx, "Operation succeeded",
		logger.F("component", "TestComponent"),
		logger.F("order_id", "ORD-999"),
		logger.F("password", "supersecretpassword"),
		logger.F("jwt", "eyJhbGciOiJIUzI1Ni..."),
		logger.F("idempotency_key", "idem-safe-123"),
	)

	out := buf.String()

	if !strings.Contains(out, "[req_id=test-req-12345]") {
		t.Errorf("expected req_id in log, got: %s", out)
	}
	if !strings.Contains(out, "[component=TestComponent]") {
		t.Errorf("expected component in log, got: %s", out)
	}
	if !strings.Contains(out, "order_id=ORD-999") {
		t.Errorf("expected order_id in log, got: %s", out)
	}
	if !strings.Contains(out, "idempotency_key=idem-safe-123") {
		t.Errorf("expected safe idempotency_key in log, got: %s", out)
	}
	if strings.Contains(out, "supersecretpassword") {
		t.Errorf("password was NOT redacted! got: %s", out)
	}
	if !strings.Contains(out, "password=[REDACTED]") {
		t.Errorf("expected password=[REDACTED], got: %s", out)
	}
	if strings.Contains(out, "eyJhbGciOiJIUzI1Ni") {
		t.Errorf("jwt was NOT redacted! got: %s", out)
	}
	if !strings.Contains(out, "jwt=[REDACTED]") {
		t.Errorf("expected jwt=[REDACTED], got: %s", out)
	}

	buf.Reset()
	logger.Error(ctx, "Operation failed", errors.New("db connection lost"),
		logger.F("component", "DB"),
		logger.F("payment_id", "pay-abc-123"),
	)
	outErr := buf.String()
	if !strings.Contains(outErr, `err="db connection lost"`) {
		t.Errorf("expected formatted err in log, got: %s", outErr)
	}
	if !strings.Contains(outErr, "payment_id=pay-abc-123") {
		t.Errorf("expected payment_id in log, got: %s", outErr)
	}
}

func TestLogger_JSONFormat(t *testing.T) {
	origFormat := os.Getenv("LOG_FORMAT")
	defer func() {
		if origFormat != "" {
			os.Setenv("LOG_FORMAT", origFormat)
		} else {
			os.Unsetenv("LOG_FORMAT")
		}
	}()

	os.Setenv("LOG_FORMAT", "json")

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	ctx := logger.ContextWithRequestID(context.Background(), "req-json-999")

	logger.Info(ctx, "JSON log test",
		logger.F("component", "JSONTester"),
		logger.F("user_id", "usr-123"),
		logger.F("password", "supersecret123"),
	)

	out := buf.String()
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &payload); err != nil {
		t.Fatalf("expected valid JSON output, got error %v; raw: %s", err, out)
	}

	if payload["level"] != "INFO" {
		t.Errorf("expected level 'INFO', got %v", payload["level"])
	}
	if payload["message"] != "JSON log test" {
		t.Errorf("expected message 'JSON log test', got %v", payload["message"])
	}
	if payload["request_id"] != "req-json-999" {
		t.Errorf("expected request_id 'req-json-999', got %v", payload["request_id"])
	}
	if payload["component"] != "JSONTester" {
		t.Errorf("expected component 'JSONTester', got %v", payload["component"])
	}
	if payload["user_id"] != "usr-123" {
		t.Errorf("expected user_id 'usr-123', got %v", payload["user_id"])
	}
	if payload["password"] != "[REDACTED]" {
		t.Errorf("expected password to be [REDACTED], got %v", payload["password"])
	}
	if _, ok := payload["time"]; !ok {
		t.Errorf("expected timestamp 'time' in JSON payload, got none")
	}

	// Test Error with err object
	buf.Reset()
	logger.Error(ctx, "Database write failed", errors.New("timeout connecting to pg"),
		logger.F("db_host", "127.0.0.1"),
	)

	var errPayload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &errPayload); err != nil {
		t.Fatalf("expected valid JSON output for error, got %v; raw: %s", err, buf.String())
	}

	if errPayload["level"] != "ERROR" {
		t.Errorf("expected level 'ERROR', got %v", errPayload["level"])
	}
	if errPayload["error"] != "timeout connecting to pg" {
		t.Errorf("expected error string 'timeout connecting to pg', got %v", errPayload["error"])
	}
}
