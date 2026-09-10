package logger_test

import (
	"bytes"
	"context"
	"errors"
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
