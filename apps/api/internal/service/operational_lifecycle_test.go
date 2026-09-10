package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"carefund-api/internal/config"
	"carefund-api/internal/database"
	"carefund-api/internal/domain"
	"carefund-api/internal/logger"
	"carefund-api/internal/service"
)

func TestOutboxWorkerStartupLifecycle(t *testing.T) {
	cfg, err := config.NewTestConfig()
	if err != nil {
		t.Fatalf("failed to load test config: %v", err)
	}
	db, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()

	repo := database.NewOutboxEventRepository(db)
	_, _ = db.DB.Exec("TRUNCATE TABLE outbox_events CASCADE")

	// 1. Create a pending outbox event
	evt := &domain.OutboxEvent{
		IdempotencyKey: fmt.Sprintf("lifecycle_test_%d", time.Now().UnixNano()),
		AggregateType:  "TEST",
		AggregateID:    "00000000-0000-0000-0000-000000000099",
		EventType:      "TEST_EVENT",
		Payload:        json.RawMessage(`{"lifecycle":"active"}`),
		Status:         domain.OutboxStatusPending,
		AvailableAt:    time.Now().Add(-1 * time.Second),
	}
	if err := repo.Create(context.Background(), evt); err != nil {
		t.Fatalf("failed to create event: %v", err)
	}

	worker := service.NewOutboxWorker(repo, 15*time.Minute)

	// 2. Start worker in a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		worker.Start(ctx, 20*time.Millisecond)
	}()

	// 3. Wait for event to reach PROCESSED terminal state
	deadline := time.Now().Add(3 * time.Second)
	var finalStatus string
	for time.Now().Before(deadline) {
		row := db.DB.QueryRow("SELECT status FROM outbox_events WHERE id = $1", evt.ID)
		_ = row.Scan(&finalStatus)
		if finalStatus == domain.OutboxStatusProcessed {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}

	if finalStatus != domain.OutboxStatusProcessed {
		t.Errorf("expected event status %s, got %s", domain.OutboxStatusProcessed, finalStatus)
	}

	// 4. Cancel context and verify graceful shutdown
	cancel()

	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case <-doneCh:
		// Clean exit
	case <-time.After(2 * time.Second):
		t.Fatalf("worker failed to exit gracefully on context cancellation")
	}
}

func TestContextTimeoutRollback(t *testing.T) {
	cfg, err := config.NewTestConfig()
	if err != nil {
		t.Fatalf("failed to load test config: %v", err)
	}
	db, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()

	txManager := database.NewTransactionManager(db)
	outboxRepo := database.NewOutboxEventRepository(db)

	_, _ = db.DB.Exec("TRUNCATE TABLE outbox_events CASCADE")

	// Create an already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = txManager.Do(ctx, func(txCtx context.Context) error {
		evt := &domain.OutboxEvent{
			IdempotencyKey: fmt.Sprintf("timeout_test_%d", time.Now().UnixNano()),
			AggregateType:  "TEST",
			AggregateID:    "00000000-0000-0000-0000-000000000088",
			EventType:      "TIMEOUT_EVENT",
			Payload:        json.RawMessage(`{}`),
			Status:         domain.OutboxStatusPending,
			AvailableAt:    time.Now(),
		}
		return outboxRepo.Create(txCtx, evt)
	})

	if err == nil {
		t.Fatalf("expected error from cancelled context, got nil")
	}

	// Verify nothing was committed to the database
	var count int
	_ = db.DB.QueryRow("SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = '00000000-0000-0000-0000-000000000088'").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 events committed on cancelled context, got %d", count)
	}
}

func TestRequestIDAndSensitiveLogRedaction(t *testing.T) {
	var buf bytes.Buffer
	logger.SetOutput(&buf)
	defer logger.SetOutput(os.Stdout)

	testReqID := "cf-trace-987654"
	ctx := logger.ContextWithRequestID(context.Background(), testReqID)

	logger.Info(ctx, "Processed transaction",
		logger.F("component", "PaymentEngine"),
		logger.F("order_id", "CF-ORDER-111"),
		logger.F("amount", 50000),
		logger.F("server_key", "SB-Mid-server-REALKEY123"),
		logger.F("payment_token", "snap-token-xyz-secret"),
		logger.F("password", "UserPassword456!"),
	)

	out := buf.String()

	if !strings.Contains(out, "[req_id="+testReqID+"]") {
		t.Errorf("expected req_id in log, got: %s", out)
	}
	if !strings.Contains(out, "[component=PaymentEngine]") {
		t.Errorf("expected component in log, got: %s", out)
	}
	if !strings.Contains(out, "order_id=CF-ORDER-111") {
		t.Errorf("expected order_id in log, got: %s", out)
	}
	if strings.Contains(out, "SB-Mid-server-REALKEY123") {
		t.Errorf("server_key was NOT redacted: %s", out)
	}
	if !strings.Contains(out, "server_key=[REDACTED]") {
		t.Errorf("expected server_key=[REDACTED], got: %s", out)
	}
	if strings.Contains(out, "snap-token-xyz-secret") {
		t.Errorf("payment_token was NOT redacted: %s", out)
	}
	if !strings.Contains(out, "payment_token=[REDACTED]") {
		t.Errorf("expected payment_token=[REDACTED], got: %s", out)
	}
	if strings.Contains(out, "UserPassword456!") {
		t.Errorf("password was NOT redacted: %s", out)
	}
	if !strings.Contains(out, "password=[REDACTED]") {
		t.Errorf("expected password=[REDACTED], got: %s", out)
	}
}
