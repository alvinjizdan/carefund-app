package metrics_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"carefund-api/internal/api/middleware"
	"carefund-api/internal/logger"
	"carefund-api/internal/metrics"
)

func TestHTTPMetricsAndRouteNormalization(t *testing.T) {
	// 1. Verify route normalizer
	tests := []struct {
		input    string
		expected string
	}{
		{"/health", "/health"},
		{"/ready", "/ready"},
		{"/metrics", "/metrics"},
		{"/api/v1/auth/login", "/api/v1/auth/login"},
		{"/api/v1/campaigns", "/api/v1/campaigns"},
		{"/api/v1/campaigns/982f1b0a-7431-4c6e-8219-c08197779f64", "/api/v1/campaigns/{id}"},
		{"/api/v1/campaigns/982f1b0a-7431-4c6e-8219-c08197779f64/approve", "/api/v1/campaigns/{id}/approve"},
		{"/api/v1/donations/d8495038-1234-5678-9012-123456789012", "/api/v1/donations/{id}"},
		{"/api/v1/payments/pay-uuid-test-1234", "/api/v1/payments/{id}"},
		{"/api/v1/unknown/endpoint", "other"},
	}

	for _, tc := range tests {
		got := metrics.NormalizeRoute(tc.input)
		if got != tc.expected {
			t.Errorf("NormalizeRoute(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}

	// 2. Test HTTP middleware records metrics
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/error" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	instrumented := middleware.Metrics()(dummyHandler)

	reqOk := httptest.NewRequest("GET", "/api/v1/donations", nil)
	recOk := httptest.NewRecorder()
	instrumented.ServeHTTP(recOk, reqOk)

	reqErr := httptest.NewRequest("POST", "/error", nil)
	recErr := httptest.NewRecorder()
	instrumented.ServeHTTP(recErr, reqErr)

	// Fetch /metrics output
	metricsReq := httptest.NewRequest("GET", "/metrics", nil)
	metricsRec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(metricsRec, metricsReq)

	body := metricsRec.Body.String()
	if !strings.Contains(body, `http_requests_total{method="GET",normalized_route="/api/v1/donations",status_code="200"}`) {
		t.Errorf("expected 200 HTTP metric in /metrics, got:\n%s", body)
	}
	if !strings.Contains(body, `http_requests_total{method="POST",normalized_route="other",status_code="500"}`) {
		t.Errorf("expected 500 HTTP metric in /metrics, got:\n%s", body)
	}
	if !strings.Contains(body, `http_request_duration_seconds_bucket`) {
		t.Errorf("expected http_request_duration_seconds histogram in /metrics, got:\n%s", body)
	}
}

func TestPaymentAndProviderMetrics(t *testing.T) {
	metrics.RecordPaymentCreated("MIDTRANS")
	metrics.RecordPaymentStatusTransition("PENDING", "CAPTURED")
	metrics.SetPaymentsPending("lt_15m", 12)
	metrics.SetPaymentsPending("15m_to_45m", 4)
	metrics.SetPaymentsPending("gt_45m", 1)
	metrics.RecordPaymentReconciliationMismatch("settlement", "PENDING")

	metrics.RecordProviderRequest("MIDTRANS", "create_transaction", "success", 120*time.Millisecond)
	metrics.RecordProviderRequest("MIDTRANS", "check_transaction", "timeout", 5*time.Second)
	metrics.RecordProviderRequest("MIDTRANS", "direct_refund", "rejected", 300*time.Millisecond)
	metrics.RecordProviderRequest("MIDTRANS", "create_transaction", "network_error", 10*time.Millisecond)

	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body := rec.Body.String()

	if !strings.Contains(body, `payments_created_total{provider="MIDTRANS"}`) {
		t.Errorf("expected payments_created_total in /metrics")
	}
	if !strings.Contains(body, `payments_status_transition_total{from_status="PENDING",to_status="CAPTURED"}`) {
		t.Errorf("expected payments_status_transition_total in /metrics")
	}
	if !strings.Contains(body, `payments_pending{age_bucket="lt_15m"} 12`) {
		t.Errorf("expected payments_pending lt_15m in /metrics")
	}
	if !strings.Contains(body, `payments_reconciliation_mismatches_total{local_status="PENDING",provider_status="settlement"}`) {
		t.Errorf("expected payments_reconciliation_mismatches_total in /metrics")
	}
	if !strings.Contains(body, `payment_provider_requests_total{operation="check_transaction",provider="MIDTRANS",result="timeout"}`) {
		t.Errorf("expected provider timeout metric in /metrics")
	}
	if !strings.Contains(body, `payment_provider_requests_total{operation="direct_refund",provider="MIDTRANS",result="rejected"}`) {
		t.Errorf("expected provider rejected metric in /metrics")
	}
	if !strings.Contains(body, `payment_provider_requests_total{operation="create_transaction",provider="MIDTRANS",result="network_error"}`) {
		t.Errorf("expected provider network_error metric in /metrics")
	}
}

func TestOutboxWorkerRefundAndSettlementMetrics(t *testing.T) {
	metrics.SetOutboxPending("REFUND_REQUESTED", 3)
	metrics.SetOutboxProcessing("REFUND_REQUESTED", 1)
	metrics.RecordOutboxDeadLetter("REFUND_REQUESTED")
	metrics.RecordOutboxRetry("REFUND_REQUESTED")

	metrics.RecordWorkerStart("outbox")
	metrics.RecordWorkerProgress("outbox")
	metrics.RecordWorkerShutdown("outbox")

	metrics.RecordWorkerReconciliation("success", 1500*time.Millisecond)
	metrics.RecordWorkerProgress("reconciliation")

	metrics.RecordRefundInitiated()
	metrics.RecordRefundProviderResult("success")
	metrics.RecordRefundProviderResult("rejected")

	metrics.RecordSettlementApproved()
	metrics.RecordSettlementCompleted()
	metrics.RecordSettlementFailed()

	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body := rec.Body.String()

	if !strings.Contains(body, `outbox_events_pending{event_type="REFUND_REQUESTED"} 3`) {
		t.Errorf("expected outbox_events_pending in /metrics")
	}
	if !strings.Contains(body, `outbox_events_processing{event_type="REFUND_REQUESTED"} 1`) {
		t.Errorf("expected outbox_events_processing in /metrics")
	}
	if !strings.Contains(body, `outbox_events_dead_letter_total{event_type="REFUND_REQUESTED"}`) {
		t.Errorf("expected outbox_events_dead_letter_total in /metrics")
	}
	if !strings.Contains(body, `outbox_events_retry_total{event_type="REFUND_REQUESTED"}`) {
		t.Errorf("expected outbox_events_retry_total in /metrics")
	}
	if !strings.Contains(body, `worker_last_progress_timestamp_seconds{worker_type="outbox"}`) {
		t.Errorf("expected worker_last_progress_timestamp_seconds for outbox in /metrics")
	}
	if !strings.Contains(body, `worker_reconciliation_duration_seconds_bucket`) {
		t.Errorf("expected worker_reconciliation_duration_seconds in /metrics")
	}
	if !strings.Contains(body, `refunds_initiated_total`) {
		t.Errorf("expected refunds_initiated_total in /metrics")
	}
	if !strings.Contains(body, `refunds_provider_result_total{result="success"}`) {
		t.Errorf("expected refunds_provider_result_total in /metrics")
	}
	if !strings.Contains(body, `settlements_approved_total`) {
		t.Errorf("expected settlements_approved_total in /metrics")
	}
	if !strings.Contains(body, `settlements_completed_total`) {
		t.Errorf("expected settlements_completed_total in /metrics")
	}
	if !strings.Contains(body, `settlements_failed_total`) {
		t.Errorf("expected settlements_failed_total in /metrics")
	}
}

func TestFinancialAnomalySignalsAndSecurity(t *testing.T) {
	var buf bytes.Buffer
	logger.SetOutput(&buf)
	defer logger.SetOutput(io.Discard)

	ctx := context.Background()

	// Record anomaly
	metrics.RecordFinancialAnomaly(ctx, "provider_success_local_pending", "Discovered unconfirmed capture",
		logger.F("order_id", "CF-ANOMALY-001"),
	)
	metrics.RecordFinancialAnomaly(ctx, "dead_letter_growth", "Outbox event exhausted retries",
		logger.F("event_id", "evt-999"),
	)

	// Verify log was emitted
	logStr := buf.String()
	if !strings.Contains(logStr, "FINANCIAL ANOMALY DETECTED") {
		t.Errorf("expected financial anomaly warning in log output, got: %s", logStr)
	}
	if !strings.Contains(logStr, "anomaly_type=provider_success_local_pending") {
		t.Errorf("expected anomaly_type in log output, got: %s", logStr)
	}

	// Verify metric incremented
	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	metricsBody := rec.Body.String()

	if !strings.Contains(metricsBody, `financial_anomalies_total{anomaly_type="provider_success_local_pending"}`) {
		t.Errorf("expected financial_anomalies_total in /metrics, got:\n%s", metricsBody)
	}
	if !strings.Contains(metricsBody, `financial_anomalies_total{anomaly_type="dead_letter_growth"}`) {
		t.Errorf("expected financial_anomalies_total dead_letter_growth in /metrics, got:\n%s", metricsBody)
	}

	// Security check: Verify NO secrets, tokens, or PII leak into /metrics
	forbiddenSubstrings := []string{
		"supersecretpassword",
		"SB-Mid-server",
		"eyJhbGci",
		"snap-token",
		"CF-ANOMALY-001", // no order IDs in labels
		"evt-999",         // no event IDs in labels
	}
	for _, forbidden := range forbiddenSubstrings {
		if strings.Contains(metricsBody, forbidden) {
			t.Errorf("SECURITY VIOLATION: metrics exposition contains forbidden value %q", forbidden)
		}
	}
}

func TestWorkerHeartbeatVsProgressAndScrapeEndpoint(t *testing.T) {
	// A. Healthy worker with zero work (heartbeat advances, progress does not)
	metrics.RecordWorkerStart("outbox")
	metrics.RecordWorkerHeartbeat("outbox")

	recA := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(recA, httptest.NewRequest("GET", "/metrics", nil))
	bodyA := recA.Body.String()

	if !strings.Contains(bodyA, `worker_last_heartbeat_timestamp_seconds{worker_type="outbox"}`) {
		t.Errorf("expected worker_last_heartbeat_timestamp_seconds for outbox in /metrics")
	}

	// B. Healthy worker with successful work (both heartbeat and progress advance)
	metrics.RecordWorkerProgress("outbox")
	recB := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(recB, httptest.NewRequest("GET", "/metrics", nil))
	bodyB := recB.Body.String()

	if !strings.Contains(bodyB, `worker_last_progress_timestamp_seconds{worker_type="outbox"}`) {
		t.Errorf("expected worker_last_progress_timestamp_seconds for outbox in /metrics")
	}

	// C. Reconciliation worker sweep: test failure case
	metrics.RecordWorkerReconciliation("failure", 500*time.Millisecond)
	recC := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(recC, httptest.NewRequest("GET", "/metrics", nil))
	bodyC := recC.Body.String()

	if !strings.Contains(bodyC, `worker_reconciliation_duration_seconds_bucket{result="failure"`) {
		t.Errorf("expected reconciliation failure metric in /metrics")
	}

	// D & E & F. Worker shutdown and restart lifecycle
	metrics.RecordWorkerShutdown("outbox")
	metrics.RecordWorkerStart("outbox")

	recD := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(recD, httptest.NewRequest("GET", "/metrics", nil))
	bodyD := recD.Body.String()

	if !strings.Contains(bodyD, `worker_shutdown_total{worker_type="outbox"}`) {
		t.Errorf("expected worker_shutdown_total in /metrics")
	}
	if !strings.Contains(bodyD, `worker_start_total{worker_type="outbox"}`) {
		t.Errorf("expected worker_start_total in /metrics")
	}
}
