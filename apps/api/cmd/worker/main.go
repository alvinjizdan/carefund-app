package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"carefund-api/internal/config"
	"carefund-api/internal/database"
	"carefund-api/internal/infrastructure/payment/midtrans"
	"carefund-api/internal/logger"
	"carefund-api/internal/metrics"
	"carefund-api/internal/service"
)

func main() {
	runOnce := flag.Bool("once", false, "Execute a single worker cycle and exit")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal(context.Background(), "Failed to load config", err, logger.F("component", "Worker"))
	}

	db, err := database.Connect(cfg)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to connect to database", err, logger.F("component", "Worker"))
	}
	defer db.Close()

	// Dependency initialization - fail-fast on any missing component
	txManager := database.NewTransactionManager(db)
	donRepo := database.NewDonationRepository(db)
	payRepo := database.NewPaymentRepository(db)
	eventRepo := database.NewPaymentEventRepository(db)
	refundRepo := database.NewRefundRepository(db)
	outboxRepo := database.NewOutboxEventRepository(db)
	idempotencyRepo := database.NewIdempotencyRepository(db)

	midtransGw := midtrans.NewGateway(cfg)
	webhookSvc := service.NewWebhookService(payRepo, donRepo, eventRepo, txManager, service.WithWebhookIdempotencyRepository(idempotencyRepo))
	reconSvc := service.NewReconciliationService(payRepo, webhookSvc, midtransGw, txManager)
	refundSvc := service.NewRefundService(payRepo, refundRepo, donRepo, outboxRepo, txManager)

	outboxWorker := service.NewOutboxWorker(
		outboxRepo,
		cfg.OutboxProcessingTTL,
		service.WithPaymentGateway(midtransGw),
		service.WithRefundService(refundSvc),
		service.WithRefundRepository(refundRepo),
		service.WithPaymentRepository(payRepo),
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	batchSize := 50
	outboxInterval := 5 * time.Second
	reconInterval := 1 * time.Minute

	// Single-pass mode: used for CronJobs, manual recovery sweeps, or integration tests
	if *runOnce || os.Getenv("WORKER_RUN_ONCE") == "true" {
		logger.Info(ctx, "Executing single-pass worker cycle...", logger.F("component", "Worker"))

		// 1. Drain pending outbox events
		outboxCount := 0
		for {
			hasMore, err := outboxWorker.ProcessNext(ctx)
			if err != nil {
				logger.Error(ctx, "Error processing outbox event", err, logger.F("component", "Worker"))
				break
			}
			if !hasMore {
				break
			}
			outboxCount++
		}
		logger.Info(ctx, "Outbox pass completed", logger.F("component", "Worker"), logger.F("processed", outboxCount))

		// 2. Reconcile pending payments
		reconCtx, reconCancel := context.WithTimeout(ctx, 45*time.Second)
		defer reconCancel()
		reconCount, err := reconSvc.ReconcilePendingPayments(reconCtx, batchSize, cfg.PaymentPendingTTL)
		if err != nil {
			logger.Fatal(ctx, "Reconciliation failed", err, logger.F("component", "Worker"))
		}
		// 3. Collect gauges
		collectDatabaseGauges(ctx, db)

		logger.Info(ctx, "Reconciliation pass completed", logger.F("component", "Worker"), logger.F("processed", reconCount))
		return
	}

	// Daemon mode: long-running background worker
	logger.Info(ctx, "Starting background worker daemon...", logger.F("component", "Worker"))
	metrics.RecordWorkerStart("outbox")
	metrics.RecordWorkerStart("reconciliation")

	var wg sync.WaitGroup

	// Worker Routine 1: Outbox Event Processor
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer metrics.RecordWorkerShutdown("outbox")
		logger.Info(ctx, "Outbox worker loop started", logger.F("component", "OutboxWorker"), logger.F("interval", outboxInterval.String()))
		outboxWorker.Start(ctx, outboxInterval)
		logger.Info(ctx, "Outbox worker loop stopped", logger.F("component", "OutboxWorker"))
	}()

	// Worker Routine 2: Payment Reconciliation Processor & Gauge Sweeper
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer metrics.RecordWorkerShutdown("reconciliation")
		logger.Info(ctx, "Reconciliation loop started", logger.F("component", "ReconciliationWorker"), logger.F("interval", reconInterval.String()))
		ticker := time.NewTicker(reconInterval)
		defer ticker.Stop()

		// Initial immediate pass
		reconCtx, reconCancel := context.WithTimeout(ctx, 45*time.Second)
		processed, err := reconSvc.ReconcilePendingPayments(reconCtx, batchSize, cfg.PaymentPendingTTL)
		collectDatabaseGauges(reconCtx, db)
		reconCancel()
		if err != nil {
			logger.Error(ctx, "Initial reconciliation pass error", err, logger.F("component", "ReconciliationWorker"))
		} else {
			logger.Info(ctx, "Initial reconciliation pass finished", logger.F("component", "ReconciliationWorker"), logger.F("processed", processed))
		}

		for {
			select {
			case <-ctx.Done():
				logger.Info(ctx, "Reconciliation loop stopping", logger.F("component", "ReconciliationWorker"))
				return
			case <-ticker.C:
				reconCtx, reconCancel := context.WithTimeout(ctx, 45*time.Second)
				processed, err := reconSvc.ReconcilePendingPayments(reconCtx, batchSize, cfg.PaymentPendingTTL)
				collectDatabaseGauges(reconCtx, db)
				reconCancel()
				if err != nil {
					logger.Error(ctx, "Periodic reconciliation error", err, logger.F("component", "ReconciliationWorker"))
				} else if processed > 0 {
					logger.Info(ctx, "Periodic reconciliation finished", logger.F("component", "ReconciliationWorker"), logger.F("processed", processed))
				}
			}
		}
	}()

	// Worker Routine 3: Dedicated Worker Metrics HTTP Server
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", metrics.Handler())

	metricsSrv := &http.Server{
		Addr:              cfg.WorkerMetricsAddress(),
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info(ctx, "Worker metrics server listening", logger.F("component", "WorkerMetricsServer"), logger.F("addr", cfg.WorkerMetricsAddress()))
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(ctx, "Worker metrics server failed unexpectedly", err, logger.F("component", "WorkerMetricsServer"))
		}
	}()

	// Block until SIGINT/SIGTERM received
	<-ctx.Done()
	logger.Info(context.Background(), "Shutdown signal received, draining workers gracefully...", logger.F("component", "Worker"))

	// Trigger metrics server shutdown concurrently
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
		logger.Error(context.Background(), "Worker metrics server forced to shutdown", err, logger.F("component", "WorkerMetricsServer"))
	}

	// Graceful shutdown with 15-second deadline
	shutdownDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(shutdownDone)
	}()

	select {
	case <-shutdownDone:
		logger.Info(context.Background(), "Worker daemon exited cleanly", logger.F("component", "Worker"))
	case <-time.After(15 * time.Second):
		logger.Warn(context.Background(), "Worker shutdown timed out, terminating", logger.F("component", "Worker"))
	}
}

// collectDatabaseGauges safely samples table aggregates for gauge telemetry without burdening HTTP requests.
func collectDatabaseGauges(ctx context.Context, db *database.DB) {
	// 1. Pending Payments Age Buckets
	var lt15, m15to45, gt45 int64
	queryPayments := `
		SELECT
			COUNT(*) FILTER (WHERE created_at >= NOW() - INTERVAL '15 minutes') AS lt_15m,
			COUNT(*) FILTER (WHERE created_at < NOW() - INTERVAL '15 minutes' AND created_at >= NOW() - INTERVAL '45 minutes') AS m15_to_45m,
			COUNT(*) FILTER (WHERE created_at < NOW() - INTERVAL '45 minutes') AS gt_45m
		FROM payments
		WHERE status = 'PENDING'
	`
	if err := db.DB.QueryRowContext(ctx, queryPayments).Scan(&lt15, &m15to45, &gt45); err == nil {
		metrics.SetPaymentsPending("lt_15m", float64(lt15))
		metrics.SetPaymentsPending("15m_to_45m", float64(m15to45))
		metrics.SetPaymentsPending("gt_45m", float64(gt45))
	}

	// 2. Outbox Pending & Processing Counts by Event Type
	queryOutbox := `
		SELECT status, event_type, COUNT(*)
		FROM outbox_events
		WHERE status IN ('PENDING', 'PROCESSING')
		GROUP BY status, event_type
	`
	rows, err := db.DB.QueryContext(ctx, queryOutbox)
	if err == nil {
		defer rows.Close()
		metrics.SetOutboxPending("REFUND_REQUESTED", 0)
		metrics.SetOutboxProcessing("REFUND_REQUESTED", 0)
		metrics.SetOutboxPending("SETTLEMENT_APPROVED", 0)
		metrics.SetOutboxProcessing("SETTLEMENT_APPROVED", 0)

		for rows.Next() {
			var status, eventType string
			var count int64
			if err := rows.Scan(&status, &eventType, &count); err == nil {
				if status == "PENDING" {
					metrics.SetOutboxPending(eventType, float64(count))
				} else if status == "PROCESSING" {
					metrics.SetOutboxProcessing(eventType, float64(count))
				}
			}
		}
	}
}
