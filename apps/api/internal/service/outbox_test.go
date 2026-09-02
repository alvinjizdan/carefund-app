package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"carefund-api/internal/config"
	"carefund-api/internal/database"
	"carefund-api/internal/domain"
	"carefund-api/internal/service"
)

func setupOutboxTestDB(t *testing.T) (*database.DB, domain.OutboxEventRepository) {
	cfg := &config.Config{DBHost: "localhost", DBPort: "5432", DBUser: "postgres", DBPassword: "234djisamSOE", DBName: "carefund-app_test", DBSSLMode: "disable"}
	db, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	return db, database.NewOutboxEventRepository(db)
}

func drainQueue(ctx context.Context, worker service.OutboxWorker) {
	for {
		hasMore, _ := worker.ProcessNext(ctx)
		if !hasMore {
			break
		}
	}
}

func createEvent(ctx context.Context, repo domain.OutboxEventRepository, availAt time.Time) *domain.OutboxEvent {
	evt := &domain.OutboxEvent{
		IdempotencyKey: fmt.Sprintf("test_event_%d", time.Now().UnixNano()),
		AggregateType:  "TEST",
		AggregateID:    "00000000-0000-0000-0000-000000000001",
		EventType:      "TEST_EVENT",
		Payload:        json.RawMessage(`{"foo":"bar"}`),
		Status:         domain.OutboxStatusPending,
		AvailableAt:    availAt,
	}
	repo.Create(ctx, evt)
	return evt
}

func TestOutboxReliability(t *testing.T) {
	db, repo := setupOutboxTestDB(t)
	defer db.Close()
	ctx := context.Background()
	worker := service.NewOutboxWorker(repo, 15*time.Minute)
	db.DB.Exec("TRUNCATE TABLE outbox_events CASCADE")
	db.DB.Exec("TRUNCATE TABLE outbox_events CASCADE")

	// drain queue
	for {
		hasMore, _ := worker.ProcessNext(ctx)
		if !hasMore {
			break
		}
	}

	t.Run("1_and_2_Lease_Reclamation", func(t *testing.T) {
		evt1 := createEvent(ctx, repo, time.Now().Add(-time.Hour))
		repo.ClaimNext(ctx)
		db.DB.Exec("UPDATE outbox_events SET processing_started_at = NOW() - interval '5 minutes' WHERE id = $1", evt1.ID)

		evt2 := createEvent(ctx, repo, time.Now().Add(-time.Hour))
		repo.ClaimNext(ctx)
		db.DB.Exec("UPDATE outbox_events SET processing_started_at = NOW() - interval '20 minutes' WHERE id = $1", evt2.ID)

		reclaimed, err := repo.ReclaimExpiredLeases(ctx, 15*time.Minute)
		if err != nil {
			t.Fatalf("reclaim failed: %v", err)
		}
		if reclaimed != 1 {
			t.Errorf("Expected 1 reclaimed event, got %d", reclaimed)
		}
	})

	t.Run("3_Concurrent_Claim", func(t *testing.T) {
		drainQueue(ctx, worker)
		createEvent(ctx, repo, time.Now().Add(-time.Minute))
		var wg sync.WaitGroup
		var successes int
		var mu sync.Mutex

		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				hasMore, err := worker.ProcessNext(ctx)
				if err == nil && hasMore {
					mu.Lock()
					successes++
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		if successes != 1 {
			t.Errorf("Expected exactly 1 successful claim, got %d", successes)
		}
	})

	t.Run("4_5_6_8_Retry_Backoff", func(t *testing.T) {
		drainQueue(ctx, worker)
		evt := createEvent(ctx, repo, time.Now().Add(-time.Minute))
		// Claim to process
		claimed, _ := repo.ClaimNext(ctx)
		if claimed.RetryCount != 1 {
			t.Errorf("Expected retry count 1 on first claim, got %d", claimed.RetryCount)
		}

		// Simulate failure
		err := repo.MarkFailed(ctx, evt.ID, time.Now().Add(5*time.Minute))
		if err != nil {
			t.Errorf("MarkFailed error: %v", err)
		}

		// Check DB state
		var status string
		var count int
		db.DB.QueryRow("SELECT status, retry_count FROM outbox_events WHERE id = $1", evt.ID).Scan(&status, &count)
		if status != domain.OutboxStatusFailed {
			t.Errorf("Expected FAILED")
		}
		if count != 1 {
			t.Errorf("Expected count 1")
		}
	})

	t.Run("7_9_Success_Processed", func(t *testing.T) {
		drainQueue(ctx, worker)
		evt := createEvent(ctx, repo, time.Now().Add(-time.Minute))
		repo.ClaimNext(ctx)
		repo.MarkProcessed(ctx, evt.ID)

		var status string
		db.DB.QueryRow("SELECT status FROM outbox_events WHERE id = $1", evt.ID).Scan(&status)
		if status != domain.OutboxStatusProcessed {
			t.Errorf("Expected PROCESSED")
		}

		// Attempt reclaim to ensure PROCESSED cannot return to PENDING
		db.DB.Exec("UPDATE outbox_events SET processing_started_at = NOW() - interval '20 minutes' WHERE id = $1", evt.ID)
		repo.ReclaimExpiredLeases(ctx, 15*time.Minute)

		db.DB.QueryRow("SELECT status FROM outbox_events WHERE id = $1", evt.ID).Scan(&status)
		if status != domain.OutboxStatusProcessed {
			t.Errorf("Expected PROCESSED")
		}
	})

	t.Run("10_Max_Retry_Threshold_And_Dead_Letter_Replay", func(t *testing.T) {
		drainQueue(ctx, worker)
		evt := createEvent(ctx, repo, time.Now().Add(-time.Minute))

		// Manually set retry_count = 10
		db.DB.Exec("UPDATE outbox_events SET retry_count = 10 WHERE id = $1", evt.ID)

		// Mark Dead Letter
		err := repo.MarkDeadLetter(ctx, evt.ID, "max retry threshold reached")
		if err != nil {
			t.Fatalf("MarkDeadLetter failed: %v", err)
		}

		var status string
		db.DB.QueryRow("SELECT status FROM outbox_events WHERE id = $1", evt.ID).Scan(&status)
		if status != domain.OutboxStatusDeadLetter {
			t.Errorf("Expected status DEAD_LETTER, got %s", status)
		}

		// Replay Dead Letter
		err = repo.ReplayDeadLetter(ctx, evt.ID)
		if err != nil {
			t.Fatalf("ReplayDeadLetter failed: %v", err)
		}

		var count int
		db.DB.QueryRow("SELECT status, retry_count FROM outbox_events WHERE id = $1", evt.ID).Scan(&status, &count)
		if status != domain.OutboxStatusPending {
			t.Errorf("Expected status PENDING after replay, got %s", status)
		}
		if count != 0 {
			t.Errorf("Expected retry_count reset to 0 after replay, got %d", count)
		}
	})
}

