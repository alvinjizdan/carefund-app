package metrics

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"carefund-api/internal/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	registry = prometheus.NewRegistry()

	// HTTP Metrics
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests processed, partitioned by method, normalized route, and status code.",
		},
		[]string{"method", "normalized_route", "status_code"},
	)

	httpRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Latency of HTTP requests in seconds.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 20},
		},
		[]string{"method", "normalized_route", "status_code"},
	)

	// Payment Metrics
	paymentsCreatedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payments_created_total",
			Help: "Total number of payments initiated, partitioned by provider.",
		},
		[]string{"provider"},
	)

	paymentsStatusTransitionTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payments_status_transition_total",
			Help: "Total count of payment state machine transitions.",
		},
		[]string{"from_status", "to_status"},
	)

	paymentsPending = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "payments_pending",
			Help: "Current count of pending payments partitioned by age bucket.",
		},
		[]string{"age_bucket"},
	)

	paymentsReconciliationMismatchesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payments_reconciliation_mismatches_total",
			Help: "Count of detected state mismatches between provider and local database.",
		},
		[]string{"provider_status", "local_status"},
	)

	// Provider Dependency Metrics
	paymentProviderRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payment_provider_requests_total",
			Help: "Total external requests to payment gateways, partitioned by provider, operation, and outcome.",
		},
		[]string{"provider", "operation", "result"},
	)

	paymentProviderRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "payment_provider_request_duration_seconds",
			Help:    "Latency of external calls to payment gateways.",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 20, 30},
		},
		[]string{"provider", "operation"},
	)

	// Outbox Metrics
	outboxEventsPending = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "outbox_events_pending",
			Help: "Current count of unleased pending events in the transactional outbox.",
		},
		[]string{"event_type"},
	)

	outboxEventsProcessing = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "outbox_events_processing",
			Help: "Current count of actively leased processing events in the transactional outbox.",
		},
		[]string{"event_type"},
	)

	outboxEventsDeadLetterTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "outbox_events_dead_letter_total",
			Help: "Total outbox events transitioned to DEAD_LETTER after retry exhaustion.",
		},
		[]string{"event_type"},
	)

	outboxEventsRetryTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "outbox_events_retry_total",
			Help: "Total retry attempts for transient outbox processing failures.",
		},
		[]string{"event_type"},
	)

	// Worker Metrics
	workerReconciliationDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "worker_reconciliation_duration_seconds",
			Help:    "Execution duration of payment reconciliation batches.",
			Buckets: []float64{0.1, 0.5, 1, 2.5, 5, 10, 30, 60},
		},
		[]string{"result"},
	)

	workerLastProgressTimestampSeconds = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "worker_last_progress_timestamp_seconds",
			Help: "Unix timestamp of the last successful processing progress made by the worker.",
		},
		[]string{"worker_type"},
	)

	workerLastHeartbeatTimestampSeconds = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "worker_last_heartbeat_timestamp_seconds",
			Help: "Unix timestamp of the last completed loop or sweep by the worker.",
		},
		[]string{"worker_type"},
	)

	workerStartTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "worker_start_total",
			Help: "Count of worker lifecycle start events.",
		},
		[]string{"worker_type"},
	)

	workerShutdownTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "worker_shutdown_total",
			Help: "Count of worker lifecycle shutdown events.",
		},
		[]string{"worker_type"},
	)

	// Refund Metrics
	refundsInitiatedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "refunds_initiated_total",
			Help: "Total count of refund requests initiated by donors or administrators.",
		},
	)

	refundsProviderResultTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "refunds_provider_result_total",
			Help: "Outcomes of refund execution with the upstream payment provider.",
		},
		[]string{"result"},
	)

	// Settlement Metrics
	settlementsApprovedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "settlements_approved_total",
			Help: "Total count of campaign settlements approved.",
		},
	)

	settlementsCompletedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "settlements_completed_total",
			Help: "Total count of campaign settlements successfully finalized.",
		},
	)

	settlementsFailedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "settlements_failed_total",
			Help: "Total count of campaign settlements that failed or were rejected due to inconsistencies.",
		},
	)

	// Financial Anomaly Signal
	financialAnomaliesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "financial_anomalies_total",
			Help: "Count of detected financial anomalies requiring operational triage.",
		},
		[]string{"anomaly_type"},
	)
)

func init() {
	registry.MustRegister(
		httpRequestsTotal,
		httpRequestDurationSeconds,
		paymentsCreatedTotal,
		paymentsStatusTransitionTotal,
		paymentsPending,
		paymentsReconciliationMismatchesTotal,
		paymentProviderRequestsTotal,
		paymentProviderRequestDurationSeconds,
		outboxEventsPending,
		outboxEventsProcessing,
		outboxEventsDeadLetterTotal,
		outboxEventsRetryTotal,
		workerReconciliationDurationSeconds,
		workerLastProgressTimestampSeconds,
		workerLastHeartbeatTimestampSeconds,
		workerStartTotal,
		workerShutdownTotal,
		refundsInitiatedTotal,
		refundsProviderResultTotal,
		settlementsApprovedTotal,
		settlementsCompletedTotal,
		settlementsFailedTotal,
		financialAnomaliesTotal,
	)
}

// Registry returns the isolated application prometheus registry.
func Registry() *prometheus.Registry {
	return registry
}

// Handler returns the HTTP handler exposing prometheus metrics.
func Handler() http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// RecordHTTPRequest tracks an incoming HTTP request and its duration.
func RecordHTTPRequest(method, normalizedRoute string, statusCode int, duration time.Duration) {
	codeStr := strconv.Itoa(statusCode)
	httpRequestsTotal.WithLabelValues(method, normalizedRoute, codeStr).Inc()
	httpRequestDurationSeconds.WithLabelValues(method, normalizedRoute, codeStr).Observe(duration.Seconds())
}

// RecordPaymentCreated increments payments_created_total.
func RecordPaymentCreated(provider string) {
	if provider == "" {
		provider = "UNKNOWN"
	}
	paymentsCreatedTotal.WithLabelValues(provider).Inc()
}

// RecordPaymentStatusTransition tracks a state change in a payment.
func RecordPaymentStatusTransition(fromStatus, toStatus string) {
	if fromStatus == "" {
		fromStatus = "NONE"
	}
	if toStatus == "" {
		toStatus = "UNKNOWN"
	}
	paymentsStatusTransitionTotal.WithLabelValues(fromStatus, toStatus).Inc()
}

// SetPaymentsPending updates the gauge for a pending payment age bucket.
func SetPaymentsPending(ageBucket string, count float64) {
	switch ageBucket {
	case "lt_15m", "15m_to_45m", "gt_45m":
		paymentsPending.WithLabelValues(ageBucket).Set(count)
	}
}

// RecordPaymentReconciliationMismatch records a divergence between provider and local state.
func RecordPaymentReconciliationMismatch(providerStatus, localStatus string) {
	if providerStatus == "" {
		providerStatus = "UNKNOWN"
	}
	if localStatus == "" {
		localStatus = "UNKNOWN"
	}
	paymentsReconciliationMismatchesTotal.WithLabelValues(providerStatus, localStatus).Inc()
}

// RecordProviderRequest tracks upstream gateway communication.
func RecordProviderRequest(provider, operation, result string, duration time.Duration) {
	if provider == "" {
		provider = "UNKNOWN"
	}
	if operation == "" {
		operation = "UNKNOWN"
	}
	switch result {
	case "success", "rejected", "timeout", "network_error", "provider_error":
	default:
		result = "provider_error"
	}
	paymentProviderRequestsTotal.WithLabelValues(provider, operation, result).Inc()
	paymentProviderRequestDurationSeconds.WithLabelValues(provider, operation).Observe(duration.Seconds())
}

// SetOutboxPending updates the pending outbox gauge for a given event type.
func SetOutboxPending(eventType string, count float64) {
	if eventType == "" {
		eventType = "UNKNOWN"
	}
	outboxEventsPending.WithLabelValues(eventType).Set(count)
}

// SetOutboxProcessing updates the in-flight processing gauge for a given event type.
func SetOutboxProcessing(eventType string, count float64) {
	if eventType == "" {
		eventType = "UNKNOWN"
	}
	outboxEventsProcessing.WithLabelValues(eventType).Set(count)
}

// RecordOutboxDeadLetter increments the dead letter counter.
func RecordOutboxDeadLetter(eventType string) {
	if eventType == "" {
		eventType = "UNKNOWN"
	}
	outboxEventsDeadLetterTotal.WithLabelValues(eventType).Inc()
}

// RecordOutboxRetry increments the outbox retry counter.
func RecordOutboxRetry(eventType string) {
	if eventType == "" {
		eventType = "UNKNOWN"
	}
	outboxEventsRetryTotal.WithLabelValues(eventType).Inc()
}

// RecordWorkerReconciliation records duration and result of a reconciliation sweep.
func RecordWorkerReconciliation(result string, duration time.Duration) {
	if result != "success" && result != "failure" {
		result = "failure"
	}
	workerReconciliationDurationSeconds.WithLabelValues(result).Observe(duration.Seconds())
}

// RecordWorkerProgress marks the unix timestamp of successful progress.
func RecordWorkerProgress(workerType string) {
	switch workerType {
	case "outbox", "reconciliation":
		workerLastProgressTimestampSeconds.WithLabelValues(workerType).Set(float64(time.Now().Unix()))
	}
}

// RecordWorkerHeartbeat marks the unix timestamp of a completed sweep/cycle regardless of whether new items were found.
func RecordWorkerHeartbeat(workerType string) {
	switch workerType {
	case "outbox", "reconciliation":
		workerLastHeartbeatTimestampSeconds.WithLabelValues(workerType).Set(float64(time.Now().Unix()))
	}
}

// RecordWorkerStart marks a worker starting.
func RecordWorkerStart(workerType string) {
	workerStartTotal.WithLabelValues(workerType).Inc()
}

// RecordWorkerShutdown marks a worker exiting cleanly.
func RecordWorkerShutdown(workerType string) {
	workerShutdownTotal.WithLabelValues(workerType).Inc()
}

// RecordRefundInitiated increments refunds_initiated_total.
func RecordRefundInitiated() {
	refundsInitiatedTotal.Inc()
}

// RecordRefundProviderResult tracks provider refund results.
func RecordRefundProviderResult(result string) {
	switch result {
	case "success", "rejected", "timeout", "network_error", "provider_error":
	default:
		result = "provider_error"
	}
	refundsProviderResultTotal.WithLabelValues(result).Inc()
}

// RecordSettlementApproved increments settlements_approved_total.
func RecordSettlementApproved() {
	settlementsApprovedTotal.Inc()
}

// RecordSettlementCompleted increments settlements_completed_total.
func RecordSettlementCompleted() {
	settlementsCompletedTotal.Inc()
}

// RecordSettlementFailed increments settlements_failed_total.
func RecordSettlementFailed() {
	settlementsFailedTotal.Inc()
}

// RecordFinancialAnomaly logs structured operational signal and increments the bounded anomaly counter.
func RecordFinancialAnomaly(ctx context.Context, anomalyType string, msg string, fields ...logger.Field) {
	validTypes := map[string]bool{
		"provider_success_local_pending":        true,
		"provider_failed_local_pending":         true,
		"payment_paid_idempotency_pending":      true,
		"refund_provider_success_local_pending": true,
		"settlement_inconsistency":              true,
		"dead_letter_growth":                    true,
		"long_lived_pending_payment":            true,
		"reconciliation_repeated_failures":      true,
	}

	if !validTypes[anomalyType] {
		anomalyType = "unknown_anomaly"
	}

	financialAnomaliesTotal.WithLabelValues(anomalyType).Inc()

	allFields := append([]logger.Field{
		logger.F("component", "FinancialAnomalyDetector"),
		logger.F("anomaly_type", anomalyType),
	}, fields...)

	logger.Error(ctx, "FINANCIAL ANOMALY DETECTED: "+msg, nil, allFields...)
}

// NormalizeRoute converts dynamic URL paths into bounded pattern strings.
func NormalizeRoute(path string) string {
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}

	switch path {
	case "/health", "/ready", "/metrics":
		return path
	case "/api/v1/auth/register", "/api/v1/auth/login", "/api/v1/auth/refresh", "/api/v1/auth/logout":
		return path
	case "/api/v1/me", "/api/v1/me/donations":
		return path
	case "/api/v1/campaigns":
		return path
	case "/api/v1/donations":
		return path
	case "/api/v1/webhooks/midtrans":
		return path
	}

	parts := strings.Split(path, "/")
	if len(parts) >= 4 && parts[1] == "api" && parts[2] == "v1" {
		switch parts[3] {
		case "campaigns":
			if len(parts) == 5 {
				return "/api/v1/campaigns/{id}"
			}
			if len(parts) == 6 {
				switch parts[5] {
				case "approve", "reject", "suspend", "submit-review":
					return "/api/v1/campaigns/{id}/" + parts[5]
				}
			}
		case "donations":
			if len(parts) == 5 {
				return "/api/v1/donations/{id}"
			}
		case "payments":
			if len(parts) == 5 {
				return "/api/v1/payments/{id}"
			}
		}
	}

	return "other"
}

var (
	dbStatsMu               sync.Mutex
	currentDBStatsCollector prometheus.Collector
)

type dbStatsCollector struct {
	db *sql.DB

	openConnsDesc    *prometheus.Desc
	inUseConnsDesc   *prometheus.Desc
	idleConnsDesc    *prometheus.Desc
	waitCountDesc    *prometheus.Desc
	waitDurationDesc *prometheus.Desc
}

func newDBStatsCollector(db *sql.DB) *dbStatsCollector {
	return &dbStatsCollector{
		db: db,
		openConnsDesc: prometheus.NewDesc(
			"db_open_connections",
			"Current number of established PostgreSQL connections both in use and idle.",
			nil, nil,
		),
		inUseConnsDesc: prometheus.NewDesc(
			"db_in_use_connections",
			"Current number of PostgreSQL connections in use.",
			nil, nil,
		),
		idleConnsDesc: prometheus.NewDesc(
			"db_idle_connections",
			"Current number of idle PostgreSQL connections in the pool.",
			nil, nil,
		),
		waitCountDesc: prometheus.NewDesc(
			"db_wait_count_total",
			"Total number of times a PostgreSQL connection had to be waited for.",
			nil, nil,
		),
		waitDurationDesc: prometheus.NewDesc(
			"db_wait_duration_seconds_total",
			"Total time blocked waiting for a new PostgreSQL connection in seconds.",
			nil, nil,
		),
	}
}

func (c *dbStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.openConnsDesc
	ch <- c.inUseConnsDesc
	ch <- c.idleConnsDesc
	ch <- c.waitCountDesc
	ch <- c.waitDurationDesc
}

func (c *dbStatsCollector) Collect(ch chan<- prometheus.Metric) {
	if c.db == nil {
		return
	}
	stats := c.db.Stats()
	ch <- prometheus.MustNewConstMetric(c.openConnsDesc, prometheus.GaugeValue, float64(stats.OpenConnections))
	ch <- prometheus.MustNewConstMetric(c.inUseConnsDesc, prometheus.GaugeValue, float64(stats.InUse))
	ch <- prometheus.MustNewConstMetric(c.idleConnsDesc, prometheus.GaugeValue, float64(stats.Idle))
	ch <- prometheus.MustNewConstMetric(c.waitCountDesc, prometheus.CounterValue, float64(stats.WaitCount))
	ch <- prometheus.MustNewConstMetric(c.waitDurationDesc, prometheus.CounterValue, stats.WaitDuration.Seconds())
}

// RegisterDBStats registers a prometheus collector for database/sql connection pool statistics.
// If a collector was previously registered, it is safely unregistered and replaced.
func RegisterDBStats(db *sql.DB) {
	if db == nil {
		return
	}
	dbStatsMu.Lock()
	defer dbStatsMu.Unlock()

	if currentDBStatsCollector != nil {
		registry.Unregister(currentDBStatsCollector)
	}

	collector := newDBStatsCollector(db)
	currentDBStatsCollector = collector
	_ = registry.Register(collector)
}
