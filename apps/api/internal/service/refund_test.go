package service_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"carefund-api/internal/config"
	"carefund-api/internal/database"
	"carefund-api/internal/domain"
	"carefund-api/internal/infrastructure/payment/midtrans"
	"carefund-api/internal/service"
)

func setupRefundTestDB(t *testing.T) (*database.DB, service.RefundService, domain.PaymentRepository, domain.DonationRepository, domain.RefundRepository, domain.OutboxEventRepository) {
	cfg := &config.Config{DBHost: "localhost", DBPort: "5432", DBUser: "postgres", DBPassword: "234djisamSOE", DBName: "carefund-app_test", DBSSLMode: "disable"}
	db, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}

	_, _ = db.DB.Exec("TRUNCATE TABLE outbox_events CASCADE")

	paymentRepo := database.NewPaymentRepository(db)
	donationRepo := database.NewDonationRepository(db)
	refundRepo := database.NewRefundRepository(db)
	outboxRepo := database.NewOutboxEventRepository(db)
	txManager := database.NewTransactionManager(db)

	svc := service.NewRefundService(paymentRepo, refundRepo, donationRepo, outboxRepo, txManager)
	return db, svc, paymentRepo, donationRepo, refundRepo, outboxRepo
}

func createTestPaymentForRefund(ctx context.Context, db *database.DB, grossAmount int64) *domain.Payment {
	txManager := database.NewTransactionManager(db)
	var payment *domain.Payment
	err := txManager.Do(ctx, func(txCtx context.Context) error {
		donationRepo := database.NewDonationRepository(db)
		paymentRepo := database.NewPaymentRepository(db)

		userID := "00000000-0000-0000-0000-000000000001"
		_, err := db.DB.Exec("INSERT INTO users (id, email, password_hash, name, created_at, updated_at) VALUES ($1, 'guest@test.com', 'x', 'Guest', NOW(), NOW()) ON CONFLICT DO NOTHING", userID)
		if err != nil {
			return err
		}

		catID := "00000000-0000-0000-0000-000000000002"
		_, err = db.DB.Exec("INSERT INTO categories (id, name, slug, created_at, updated_at) VALUES ($1, 'Test', 'test-cat', NOW(), NOW()) ON CONFLICT DO NOTHING", catID)
		if err != nil {
			return err
		}

		campaignID := "00000000-0000-0000-0000-000000000003"
		_, err = db.DB.Exec("INSERT INTO campaigns (id, title, slug, description, target_amount, current_amount, status, owner_id, category_id, start_at, end_at, created_at, updated_at) VALUES ($1, 'Test Campaign', 'test-camp', 'Desc', 1000000, 0, 'ACTIVE', $2, $3, NOW(), NOW() + interval '30 days', NOW(), NOW()) ON CONFLICT DO NOTHING", campaignID, userID, catID)
		if err != nil {
			return err
		}

		don := &domain.Donation{
			CampaignID: campaignID,
			Amount:     grossAmount,
			Status:     domain.DonationStatusPaid,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		if err := donationRepo.Create(txCtx, don); err != nil {
			return err
		}

		payment = &domain.Payment{
			DonationID:  don.ID,
			Provider:    "MIDTRANS",
			OrderID:     fmt.Sprintf("ORDER-REFUND-%d", time.Now().UnixNano()),
			GrossAmount: grossAmount,
			Status:      domain.PaymentStatusCaptured,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		if err := paymentRepo.Create(txCtx, payment); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		panic("createTestPaymentForRefund failed: " + err.Error())
	}
	return payment
}

func TestRefundScenariosA_J(t *testing.T) {
	db, svc, paymentRepo, donationRepo, refundRepo, outboxRepo := setupRefundTestDB(t)
	defer db.Close()
	ctx := context.Background()

	t.Run("Scenario_A_Refund_Intent_And_Outbox_Pending", func(t *testing.T) {
		p := createTestPaymentForRefund(ctx, db, 100000)

		idemKey := fmt.Sprintf("IDEM-A-%d", time.Now().UnixNano())
		refund, err := svc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{
			PaymentID:      p.ID,
			Amount:         50000,
			IdempotencyKey: idemKey,
			Reason:         "Scenario A Test",
		})
		if err != nil {
			t.Fatalf("Failed to process refund intent: %v", err)
		}

		if refund.Status != domain.RefundStatusPending {
			t.Errorf("Expected refund status PENDING, got %s", refund.Status)
		}

		// Payment status remains CAPTURED until provider confirmation
		updatedPayment, _ := paymentRepo.FindByID(ctx, p.ID)
		if updatedPayment.Status != domain.PaymentStatusCaptured {
			t.Errorf("Expected payment status to remain CAPTURED, got %s", updatedPayment.Status)
		}

		// Active reservation should be 50000
		active, _ := refundRepo.SumActiveRefunds(ctx, p.ID)
		if active != 50000 {
			t.Errorf("Expected active refunds 50000, got %d", active)
		}
	})

	db.DB.Exec("TRUNCATE TABLE outbox_events CASCADE")
	t.Run("Scenario_B_Worker_Sends_Refund_Successfully", func(t *testing.T) {
		p := createTestPaymentForRefund(ctx, db, 100000)
		mockGw := midtrans.NewMockPaymentGateway()

		worker := service.NewOutboxWorker(
			outboxRepo,
			15*time.Minute,
			service.WithPaymentGateway(mockGw),
			service.WithRefundService(svc),
			service.WithRefundRepository(refundRepo),
			service.WithPaymentRepository(paymentRepo),
		)

		idemKey := fmt.Sprintf("IDEM-B-%d", time.Now().UnixNano())
		refund, err := svc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{
			PaymentID:      p.ID,
			Amount:         100000,
			IdempotencyKey: idemKey,
			Reason:         "Full Refund",
		})
		if err != nil {
			t.Fatalf("Failed to create refund: %v", err)
		}

		hasMore, err := worker.ProcessNext(ctx)
		if err != nil {
			t.Fatalf("Worker failed to process refund: %v", err)
		}
		if !hasMore {
			t.Fatalf("Expected event to be processed")
		}

		// Verify refund is COMPLETED
		updatedRefund, _ := refundRepo.FindByID(ctx, refund.ID)
		if updatedRefund.Status != domain.RefundStatusCompleted {
			t.Errorf("Expected refund status COMPLETED, got %s", updatedRefund.Status)
		}

		// Verify Payment and Donation are REFUNDED
		updatedPayment, _ := paymentRepo.FindByID(ctx, p.ID)
		if updatedPayment.Status != domain.PaymentStatusRefunded {
			t.Errorf("Expected payment status REFUNDED, got %s", updatedPayment.Status)
		}

		updatedDonation, _ := donationRepo.FindByID(ctx, p.DonationID)
		if updatedDonation.Status != domain.DonationStatusRefunded {
			t.Errorf("Expected donation status REFUNDED, got %s", updatedDonation.Status)
		}
	})

	db.DB.Exec("TRUNCATE TABLE outbox_events CASCADE")
	t.Run("Scenario_C_Worker_Times_Out_After_Provider_Call", func(t *testing.T) {
		p := createTestPaymentForRefund(ctx, db, 100000)
		mockGw := midtrans.NewMockPaymentGateway()
		mockGw.ShouldTimeout = true

		worker := service.NewOutboxWorker(
			outboxRepo,
			15*time.Minute,
			service.WithPaymentGateway(mockGw),
			service.WithRefundService(svc),
			service.WithRefundRepository(refundRepo),
			service.WithPaymentRepository(paymentRepo),
		)

		idemKey := fmt.Sprintf("IDEM-C-%d", time.Now().UnixNano())
		refund, err := svc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{
			PaymentID:      p.ID,
			Amount:         60000,
			IdempotencyKey: idemKey,
			Reason:         "Timeout Scenario",
		})
		if err != nil {
			t.Fatalf("Failed to create refund: %v", err)
		}

		_, err = worker.ProcessNext(ctx)
		if err == nil {
			t.Fatalf("Expected worker to return error on timeout")
		}

		// Invariant: Refund must NOT be marked FAILED on ambiguous timeout! Must stay PENDING!
		updatedRefund, _ := refundRepo.FindByID(ctx, refund.ID)
		if updatedRefund.Status != domain.RefundStatusPending {
			t.Errorf("Expected refund status to remain PENDING after timeout, got %s", updatedRefund.Status)
		}

		// Payment remains CAPTURED
		updatedPayment, _ := paymentRepo.FindByID(ctx, p.ID)
		if updatedPayment.Status != domain.PaymentStatusCaptured {
			t.Errorf("Expected payment status CAPTURED, got %s", updatedPayment.Status)
		}
	})

	db.DB.Exec("TRUNCATE TABLE outbox_events CASCADE")
	t.Run("Scenario_D_Worker_Crashes_Before_Call_Lease_Reclaimed", func(t *testing.T) {
		p := createTestPaymentForRefund(ctx, db, 100000)

		idemKey := fmt.Sprintf("IDEM-D-%d", time.Now().UnixNano())
		refund, _ := svc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{
			PaymentID:      p.ID,
			Amount:         40000,
			IdempotencyKey: idemKey,
			Reason:         "Crash Lease Test",
		})

		// Simulate claiming lease by a crashed worker 20 minutes ago
		event, err := outboxRepo.ClaimNext(ctx)
		if err != nil {
			t.Fatalf("Failed to claim event: %v", err)
		}

		// Backdate lease
		_, err = db.DB.Exec("UPDATE outbox_events SET processing_started_at = NOW() - INTERVAL '20 minutes' WHERE id = $1", event.ID)
		if err != nil {
			t.Fatalf("Failed to backdate lease: %v", err)
		}

		reclaimed, err := outboxRepo.ReclaimExpiredLeases(ctx, 15*time.Minute)
		if err != nil || reclaimed == 0 {
			t.Fatalf("Expected lease to be reclaimed, err: %v, reclaimed: %d", err, reclaimed)
		}

		// Refund should still be PENDING
		updatedRefund, _ := refundRepo.FindByID(ctx, refund.ID)
		if updatedRefund.Status != domain.RefundStatusPending {
			t.Errorf("Expected refund to remain PENDING, got %s", updatedRefund.Status)
		}
	})

	db.DB.Exec("TRUNCATE TABLE outbox_events CASCADE")
	t.Run("Scenario_E_Worker_Retry_After_Crash_Idempotency_Key", func(t *testing.T) {
		p := createTestPaymentForRefund(ctx, db, 100000)
		mockGw := midtrans.NewMockPaymentGateway()

		worker := service.NewOutboxWorker(
			outboxRepo,
			15*time.Minute,
			service.WithPaymentGateway(mockGw),
			service.WithRefundService(svc),
			service.WithRefundRepository(refundRepo),
			service.WithPaymentRepository(paymentRepo),
		)

		idemKey := fmt.Sprintf("IDEM-E-%d", time.Now().UnixNano())
		refund, _ := svc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{
			PaymentID:      p.ID,
			Amount:         30000,
			IdempotencyKey: idemKey,
			Reason:         "Retry Idempotency",
		})

		// First execution succeeds
		_, _ = worker.ProcessNext(ctx)

		// Verify Midtrans received the exact same refund idempotency key
		if mockGw.LastRefundRequest == nil || mockGw.LastRefundRequest.IdempotencyKey != refund.IdempotencyKey {
			t.Errorf("Expected provider request to use Refund IdempotencyKey %s", refund.IdempotencyKey)
		}

		// Second redundant finalize call on already completed refund is safely idempotent
		err := svc.FinalizeRefund(ctx, refund.ID, "mock_provider_refund_"+refund.ID, domain.RefundStatusCompleted)
		if err != nil {
			t.Errorf("Expected idempotent FinalizeRefund, got %v", err)
		}
	})

	t.Run("Scenario_F_Duplicate_Refund_Local_Idempotency_Key", func(t *testing.T) {
		p := createTestPaymentForRefund(ctx, db, 100000)
		idemKey := fmt.Sprintf("IDEM-F-%d", time.Now().UnixNano())

		_, err := svc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{
			PaymentID:      p.ID,
			Amount:         50000,
			IdempotencyKey: idemKey,
			Reason:         "First attempt",
		})
		if err != nil {
			t.Fatalf("First attempt failed: %v", err)
		}

		// Second attempt with exact same idempotency key must fail
		_, err = svc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{
			PaymentID:      p.ID,
			Amount:         50000,
			IdempotencyKey: idemKey,
			Reason:         "Second attempt duplicate",
		})
		if err == nil {
			t.Fatalf("Expected duplicate idempotency key to be rejected")
		}
	})

	t.Run("Scenario_G_Concurrent_Balance_Exhaustion", func(t *testing.T) {
		p := createTestPaymentForRefund(ctx, db, 100000)

		var wg sync.WaitGroup
		var successes int
		var mu sync.Mutex

		reqs := []int64{70000, 50000}

		for _, amt := range reqs {
			wg.Add(1)
			go func(amount int64) {
				defer wg.Done()
				_, err := svc.ProcessLocalRefund(context.Background(), service.ProcessRefundRequest{
					PaymentID:      p.ID,
					Amount:         amount,
					IdempotencyKey: fmt.Sprintf("IDEM-G-%d-%d", amount, time.Now().UnixNano()),
					Reason:         "Concurrent Balance Test",
				})
				if err == nil {
					mu.Lock()
					successes++
					mu.Unlock()
				}
			}(amt)
		}

		wg.Wait()

		if successes != 1 {
			t.Errorf("Expected exactly 1 successful refund intent, got %d", successes)
		}

		active, _ := refundRepo.SumActiveRefunds(ctx, p.ID)
		if active != 70000 && active != 50000 {
			t.Errorf("Expected active reservation to be 70000 or 50000, got %d", active)
		}
	})

	t.Run("Scenario_H_Provider_Confirms_Full_Refund", func(t *testing.T) {
		p := createTestPaymentForRefund(ctx, db, 100000)

		refund, _ := svc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{
			PaymentID:      p.ID,
			Amount:         100000,
			IdempotencyKey: fmt.Sprintf("IDEM-H-%d", time.Now().UnixNano()),
			Reason:         "Full Refund Scenario",
		})

		err := svc.FinalizeRefund(ctx, refund.ID, "MIDTRANS-REF-FULL", domain.RefundStatusCompleted)
		if err != nil {
			t.Fatalf("Finalize failed: %v", err)
		}

		updatedPayment, _ := paymentRepo.FindByID(ctx, p.ID)
		if updatedPayment.Status != domain.PaymentStatusRefunded {
			t.Errorf("Expected payment REFUNDED, got %s", updatedPayment.Status)
		}

		updatedDonation, _ := donationRepo.FindByID(ctx, p.DonationID)
		if updatedDonation.Status != domain.DonationStatusRefunded {
			t.Errorf("Expected donation REFUNDED, got %s", updatedDonation.Status)
		}
	})

	t.Run("Scenario_I_Provider_Confirms_Partial_Refund", func(t *testing.T) {
		p := createTestPaymentForRefund(ctx, db, 100000)

		refund, _ := svc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{
			PaymentID:      p.ID,
			Amount:         30000,
			IdempotencyKey: fmt.Sprintf("IDEM-I-%d", time.Now().UnixNano()),
			Reason:         "Partial Refund Scenario",
		})

		err := svc.FinalizeRefund(ctx, refund.ID, "MIDTRANS-REF-PARTIAL", domain.RefundStatusCompleted)
		if err != nil {
			t.Fatalf("Finalize failed: %v", err)
		}

		updatedPayment, _ := paymentRepo.FindByID(ctx, p.ID)
		if updatedPayment.Status != domain.PaymentStatusPartiallyRefunded {
			t.Errorf("Expected payment PARTIALLY_REFUNDED, got %s", updatedPayment.Status)
		}

		updatedDonation, _ := donationRepo.FindByID(ctx, p.DonationID)
		if updatedDonation.Status != domain.DonationStatusPartiallyRefunded {
			t.Errorf("Expected donation PARTIALLY_REFUNDED, got %s", updatedDonation.Status)
		}
	})

	db.DB.Exec("TRUNCATE TABLE outbox_events CASCADE")
	t.Run("Scenario_J_Provider_Rejects_Refund_Releases_Reservation", func(t *testing.T) {
		p := createTestPaymentForRefund(ctx, db, 100000)
		mockGw := midtrans.NewMockPaymentGateway()
		mockGw.ShouldReject = true

		worker := service.NewOutboxWorker(
			outboxRepo,
			15*time.Minute,
			service.WithPaymentGateway(mockGw),
			service.WithRefundService(svc),
			service.WithRefundRepository(refundRepo),
			service.WithPaymentRepository(paymentRepo),
		)

		refund, _ := svc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{
			PaymentID:      p.ID,
			Amount:         70000,
			IdempotencyKey: fmt.Sprintf("IDEM-J-%d", time.Now().UnixNano()),
			Reason:         "Rejection Scenario",
		})

		// Before worker run: active reservation is 70000
		activeBefore, _ := refundRepo.SumActiveRefunds(ctx, p.ID)
		if activeBefore != 70000 {
			t.Errorf("Expected active reservation 70000 before rejection, got %d", activeBefore)
		}

		// Worker processes rejection
		hasMore, err := worker.ProcessNext(ctx)
		if err != nil {
			t.Fatalf("Worker returned unexpected error: %v", err)
		}
		if !hasMore {
			t.Fatalf("Expected event to be processed")
		}

		// Refund should be FAILED
		updatedRefund, _ := refundRepo.FindByID(ctx, refund.ID)
		if updatedRefund.Status != domain.RefundStatusFailed {
			t.Errorf("Expected refund status FAILED, got %s", updatedRefund.Status)
		}

		// Payment status remains CAPTURED
		updatedPayment, _ := paymentRepo.FindByID(ctx, p.ID)
		if updatedPayment.Status != domain.PaymentStatusCaptured {
			t.Errorf("Expected payment to remain CAPTURED, got %s", updatedPayment.Status)
		}

		// Active reservation should be released (0)
		activeAfter, _ := refundRepo.SumActiveRefunds(ctx, p.ID)
		if activeAfter != 0 {
			t.Errorf("Expected active reservation to be released (0), got %d", activeAfter)
		}
	})
}

func TestRefundAuditScenarios(t *testing.T) {
	db, svc, paymentRepo, donationRepo, refundRepo, outboxRepo := setupRefundTestDB(t)
	defer db.Close()
	ctx := context.Background()
	txManager := database.NewTransactionManager(db)
	eventRepo := database.NewPaymentEventRepository(db)

	t.Run("Audit_1_Async_Provider_Response_Stays_Pending", func(t *testing.T) {
		db.DB.Exec("TRUNCATE TABLE outbox_events CASCADE")
		p := createTestPaymentForRefund(ctx, db, 100000)

		// Create mock gateway that returns accepted but NOT completed (async pending)
		mockGw := &midtrans.MockPaymentGateway{ShouldStayPending: true}
		worker := service.NewOutboxWorker(
			outboxRepo,
			15*time.Minute,
			service.WithPaymentGateway(mockGw),
			service.WithRefundService(svc),
			service.WithRefundRepository(refundRepo),
			service.WithPaymentRepository(paymentRepo),
		)

		refund, err := svc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{
			PaymentID:      p.ID,
			Amount:         50000,
			IdempotencyKey: fmt.Sprintf("IDEM-ASYNC-%d", time.Now().UnixNano()),
			Reason:         "Async refund test",
		})
		if err != nil {
			t.Fatalf("Failed to create refund: %v", err)
		}

		hasMore, err := worker.ProcessNext(ctx)
		if err != nil {
			t.Fatalf("Worker failed to process async acceptance: %v", err)
		}
		if !hasMore {
			t.Fatalf("Expected outbox event to be processed")
		}

		// Refund must remain PENDING because provider hasn't finalized it
		refundAfter, _ := refundRepo.FindByID(ctx, refund.ID)
		if refundAfter.Status != domain.RefundStatusPending {
			t.Errorf("Expected status to remain PENDING for async refund, got %s", refundAfter.Status)
		}

		// Payment must remain CAPTURED
		paymentAfter, _ := paymentRepo.FindByID(ctx, p.ID)
		if paymentAfter.Status != domain.PaymentStatusCaptured {
			t.Errorf("Expected payment to remain CAPTURED for async refund, got %s", paymentAfter.Status)
		}
	})

	t.Run("Audit_2_Worker_Crash_After_Success_Retry_Idempotency", func(t *testing.T) {
		db.DB.Exec("TRUNCATE TABLE outbox_events CASCADE")
		p := createTestPaymentForRefund(ctx, db, 100000)
		mockGw := midtrans.NewMockPaymentGateway()

		refund, _ := svc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{
			PaymentID:      p.ID,
			Amount:         100000,
			IdempotencyKey: fmt.Sprintf("IDEM-CRASH-%d", time.Now().UnixNano()),
			Reason:         "Crash after provider success",
		})

		// A. First execution succeeds at provider & domain level
		err := svc.FinalizeRefund(ctx, refund.ID, "MOCK-PROV-1", domain.RefundStatusCompleted)
		if err != nil {
			t.Fatalf("First finalization failed: %v", err)
		}

		// Verify state after 1st execution
		payment1, _ := paymentRepo.FindByID(ctx, p.ID)
		donation1, _ := donationRepo.FindByID(ctx, p.DonationID)
		if payment1.Status != domain.PaymentStatusRefunded || donation1.Status != domain.DonationStatusRefunded {
			t.Fatalf("Expected REFUNDED status, got payment=%s, donation=%s", payment1.Status, donation1.Status)
		}

		// B. Worker crashes before OutboxEvent is marked PROCESSED (simulate lease expiration and retry)
		event, err := outboxRepo.ClaimNext(ctx)
		if err == nil && event != nil {
			// Simulate backdating lease
			_, _ = db.DB.Exec("UPDATE outbox_events SET processing_started_at = NOW() - INTERVAL '20 minutes' WHERE id = $1", event.ID)
			_, _ = outboxRepo.ReclaimExpiredLeases(ctx, 15*time.Minute)
		}

		worker := service.NewOutboxWorker(
			outboxRepo,
			15*time.Minute,
			service.WithPaymentGateway(mockGw),
			service.WithRefundService(svc),
			service.WithRefundRepository(refundRepo),
			service.WithPaymentRepository(paymentRepo),
		)

		// C. Worker retries the event
		hasMore, err := worker.ProcessNext(ctx)
		if err != nil {
			t.Fatalf("Worker retry failed: %v", err)
		}
		if !hasMore {
			t.Fatalf("Expected event to be processed")
		}

		// D. Verify idempotency: Sum of completed refunds remains exactly 100000 (NOT doubled)
		completedTotal, _ := refundRepo.SumCompletedRefunds(ctx, p.ID)
		if completedTotal != 100000 {
			t.Errorf("Expected completed total to remain 100000, got %d", completedTotal)
		}

		payment2, _ := paymentRepo.FindByID(ctx, p.ID)
		if payment2.Status != domain.PaymentStatusRefunded {
			t.Errorf("Expected payment status REFUNDED, got %s", payment2.Status)
		}
	})

	t.Run("Audit_3_Webhook_Auditability_And_Sync", func(t *testing.T) {
		db.DB.Exec("TRUNCATE TABLE outbox_events CASCADE")
		p := createTestPaymentForRefund(ctx, db, 100000)

		// Create a local refund intent in PENDING state
		refund, _ := svc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{
			PaymentID:      p.ID,
			Amount:         100000,
			IdempotencyKey: fmt.Sprintf("IDEM-WH-%d", time.Now().UnixNano()),
			Reason:         "Webhook sync test",
		})

		webhookSvc := service.NewWebhookService(
			paymentRepo,
			donationRepo,
			eventRepo,
			txManager,
			service.WithWebhookRefundRepository(refundRepo),
		)

		notif := &domain.WebhookNotification{
			Provider:        "MIDTRANS",
			EventSource:     "WEBHOOK",
			ProviderEventID: "EVT-REFUND-100",
			OrderID:         p.OrderID,
			TransactionID:   "TX-REFUND-100",
			GrossAmount:     100000,
			ProviderStatus:  "refund",
			FraudStatus:     "accept",
			RawPayload:      `{"status_code":"200","transaction_status":"refund","order_id":"` + p.OrderID + `"}`,
			IdempotencyKey:  fmt.Sprintf("IDEM-WH-EVT-%d", time.Now().UnixNano()),
		}

		// Process webhook notification
		err := webhookSvc.ProcessNotification(ctx, notif)
		if err != nil {
			t.Fatalf("Failed to process webhook notification: %v", err)
		}

		// Verify PaymentEvent is PROCESSED (Audit log)
		var processingStatus string
		err = db.DB.QueryRow("SELECT processing_status FROM payment_events WHERE idempotency_key = $1", notif.IdempotencyKey).Scan(&processingStatus)
		if err != nil {
			t.Fatalf("Expected payment event to be persisted in DB: %v", err)
		}
		if processingStatus != domain.PaymentEventProcessingStatusProcessed {
			t.Errorf("Expected event status PROCESSED, got %s", processingStatus)
		}

		// Verify Refund is COMPLETED
		updatedRefund, _ := refundRepo.FindByID(ctx, refund.ID)
		if updatedRefund.Status != domain.RefundStatusCompleted {
			t.Errorf("Expected refund status COMPLETED via webhook sync, got %s", updatedRefund.Status)
		}

		// Verify Payment and Donation are REFUNDED
		updatedPayment, _ := paymentRepo.FindByID(ctx, p.ID)
		if updatedPayment.Status != domain.PaymentStatusRefunded {
			t.Errorf("Expected payment REFUNDED, got %s", updatedPayment.Status)
		}

		// Duplicate webhook with same idempotency key is acknowledged idempotently without error
		err = webhookSvc.ProcessNotification(ctx, notif)
		if err != nil {
			t.Errorf("Expected duplicate webhook to return nil, got %v", err)
		}
	})

	t.Run("Audit_4_Invalid_State_Transitions_Rejected", func(t *testing.T) {
		r := &domain.Refund{Status: domain.RefundStatusCompleted}
		if r.IsValidTransition(domain.RefundStatusPending) {
			t.Error("COMPLETED -> PENDING must be rejected")
		}
		if r.IsValidTransition(domain.RefundStatusFailed) {
			t.Error("COMPLETED -> FAILED must be rejected")
		}

		rFailed := &domain.Refund{Status: domain.RefundStatusFailed}
		if rFailed.IsValidTransition(domain.RefundStatusCompleted) {
			t.Error("FAILED -> COMPLETED must be rejected")
		}

		rCancelled := &domain.Refund{Status: domain.RefundStatusCancelled}
		if rCancelled.IsValidTransition(domain.RefundStatusCompleted) {
			t.Error("CANCELLED -> COMPLETED must be rejected")
		}
	})
}
