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
	"github.com/google/uuid"
)

func setupAuditEnv(t *testing.T) (
	*database.DB,
	database.TransactionManager,
	domain.UserRepository,
	domain.CategoryRepository,
	domain.CampaignRepository,
	domain.DonationRepository,
	domain.PaymentRepository,
	domain.PaymentEventRepository,
	domain.SettlementRepository,
	domain.SettlementItemRepository,
	domain.RefundRepository,
	domain.OutboxEventRepository,
	domain.WebhookService,
	service.RefundService,
	service.SettlementService,
) {
	cfg := &config.Config{
		DBHost:            "localhost",
		DBPort:            "5432",
		DBUser:            "postgres",
		DBPassword:        "234djisamSOE",
		DBName:            "carefund-app_test",
		DBSSLMode:         "disable",
		PaymentPendingTTL: 45 * time.Minute,
	}
	db, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}

	txManager := database.NewTransactionManager(db)
	userRepo := database.NewUserRepository(db)
	catRepo := database.NewCategoryRepository(db)
	campRepo := database.NewCampaignRepository(db)
	donRepo := database.NewDonationRepository(db)
	payRepo := database.NewPaymentRepository(db)
	eventRepo := database.NewPaymentEventRepository(db)
	settleRepo := database.NewSettlementRepository(db)
	settleItemRepo := database.NewSettlementItemRepository(db)
	refundRepo := database.NewRefundRepository(db)
	outboxRepo := database.NewOutboxEventRepository(db)

	webhookSvc := service.NewWebhookService(payRepo, donRepo, eventRepo, txManager, service.WithWebhookRefundRepository(refundRepo))
	refundSvc := service.NewRefundService(payRepo, refundRepo, donRepo, outboxRepo, txManager)
	settleSvc := service.NewSettlementService(campRepo, payRepo, settleRepo, settleItemRepo, outboxRepo, txManager)

	return db, txManager, userRepo, catRepo, campRepo, donRepo, payRepo, eventRepo, settleRepo, settleItemRepo, refundRepo, outboxRepo, webhookSvc, refundSvc, settleSvc
}

func createAuditFixtures(t *testing.T, db *database.DB) (*domain.User, *domain.Category, *domain.Campaign) {
	ctx := context.Background()
	userRepo := database.NewUserRepository(db)
	catRepo := database.NewCategoryRepository(db)
	campRepo := database.NewCampaignRepository(db)

	u := &domain.User{Email: "audit_" + uuid.New().String()[:8] + "@example.com", PasswordHash: "hash", Name: "Auditor", IsActive: true}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	cat := &domain.Category{Name: "Cat " + uuid.New().String()[:8], Slug: "slug_" + uuid.New().String()[:8], IsActive: true}
	if err := catRepo.Create(ctx, cat); err != nil {
		t.Fatalf("failed to create category: %v", err)
	}

	camp := &domain.Campaign{
		OwnerID:      u.ID,
		CategoryID:   cat.ID,
		Title:        "Campaign " + uuid.New().String()[:8],
		Slug:         "camp_" + uuid.New().String()[:8],
		Description:  "Desc",
		TargetAmount: 1000000,
		StartAt:      time.Now().Add(-2 * time.Hour),
		EndAt:        time.Now().Add(-1 * time.Hour),
		Status:       "ACTIVE",
	}
	if err := campRepo.Create(ctx, camp); err != nil {
		t.Fatalf("failed to create campaign: %v", err)
	}

	return u, cat, camp
}

func createAuditPayment(t *testing.T, db *database.DB, campaignID string, donorID string, amount int64, status string) (*domain.Donation, *domain.Payment) {
	ctx := context.Background()
	donRepo := database.NewDonationRepository(db)
	payRepo := database.NewPaymentRepository(db)

	donStatus := domain.DonationStatusPending
	if status == domain.PaymentStatusCaptured || status == domain.PaymentStatusSettled {
		donStatus = domain.DonationStatusPaid
	} else if status == domain.PaymentStatusExpired {
		donStatus = domain.DonationStatusExpired
	} else if status == domain.PaymentStatusFailed {
		donStatus = domain.DonationStatusFailed
	} else if status == domain.PaymentStatusCancelled {
		donStatus = domain.DonationStatusCancelled
	} else if status == domain.PaymentStatusRefunded {
		donStatus = domain.DonationStatusRefunded
	} else if status == domain.PaymentStatusPartiallyRefunded {
		donStatus = domain.DonationStatusPartiallyRefunded
	}

	d := &domain.Donation{CampaignID: campaignID, DonorID: &donorID, Amount: amount, Status: donStatus}
	if err := donRepo.Create(ctx, d); err != nil {
		t.Fatalf("failed to create donation: %v", err)
	}

	p := &domain.Payment{
		DonationID:  d.ID,
		Provider:    "MIDTRANS",
		OrderID:     "ORD-AUDIT-" + uuid.New().String()[:12],
		GrossAmount: amount,
		Status:      status,
	}
	if err := payRepo.Create(ctx, p); err != nil {
		t.Fatalf("failed to create payment: %v", err)
	}

	return d, p
}

func TestPhase5IAll27Scenarios(t *testing.T) {
	db, txManager, _, _, _, donRepo, payRepo, _, _, _, refundRepo, outboxRepo, webhookSvc, refundSvc, settleSvc := setupAuditEnv(t)
	defer db.Close()
	ctx := context.Background()

	// --- PAYMENT LIFECYCLE (1 - 5) ---
	t.Run("1_Payment_PENDING_to_CAPTURED", func(t *testing.T) {
		u, _, camp := createAuditFixtures(t, db)
		_, p := createAuditPayment(t, db, camp.ID, u.ID, 50000, domain.PaymentStatusPending)

		notif := &domain.WebhookNotification{
			Provider:        "MIDTRANS",
			EventSource:     "WEBHOOK",
			ProviderEventID: "evt_1",
			OrderID:         p.OrderID,
			TransactionID:   "tx_1",
			GrossAmount:     50000,
			ProviderStatus:  "capture",
			FraudStatus:     "accept",
			RawPayload:      `{}`,
			IdempotencyKey:  "idem_sc1_" + uuid.New().String()[:8],
		}
		if err := webhookSvc.ProcessNotification(ctx, notif); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		updatedP, _ := payRepo.FindByID(ctx, p.ID)
		if updatedP.Status != domain.PaymentStatusCaptured {
			t.Errorf("expected CAPTURED, got %s", updatedP.Status)
		}
	})

	t.Run("2_Payment_PENDING_to_EXPIRED_after_45m", func(t *testing.T) {
		u, _, camp := createAuditFixtures(t, db)
		_, p := createAuditPayment(t, db, camp.ID, u.ID, 50000, domain.PaymentStatusPending)

		// Backdate created_at to 50m ago and set order_id to unique ORDER-NOTFOUND string
		p.OrderID = "ORDER-NOTFOUND-" + uuid.New().String()[:8]
		_, _ = db.DB.Exec("UPDATE payments SET created_at = NOW() - INTERVAL '50 minutes', order_id = $1 WHERE id = $2", p.OrderID, p.ID)

		mockGw := midtrans.NewMockPaymentGateway()
		reconSvc := service.NewReconciliationService(payRepo, webhookSvc, mockGw, txManager)

		count, err := reconSvc.ReconcilePendingPayments(ctx, 10, 45*time.Minute)
		if err != nil || count == 0 {
			t.Fatalf("expected 1 reconciled payment, got count=%d, err=%v", count, err)
		}

		updatedP, _ := payRepo.FindByID(ctx, p.ID)
		if updatedP.Status != domain.PaymentStatusExpired {
			t.Errorf("expected EXPIRED, got %s", updatedP.Status)
		}
	})

	t.Run("3_EXPIRED_to_CAPTURED_rejected", func(t *testing.T) {
		u, _, camp := createAuditFixtures(t, db)
		_, p := createAuditPayment(t, db, camp.ID, u.ID, 50000, domain.PaymentStatusExpired)

		notif := &domain.WebhookNotification{
			Provider:        "MIDTRANS",
			EventSource:     "WEBHOOK",
			ProviderEventID: "evt_late",
			OrderID:         p.OrderID,
			TransactionID:   "tx_late",
			GrossAmount:     50000,
			ProviderStatus:  "capture",
			FraudStatus:     "accept",
			RawPayload:      `{}`,
			IdempotencyKey:  "idem_sc3_" + uuid.New().String()[:8],
		}

		// Webhook acknowledges with nil to gateway, but marks payment_events REJECTED internally
		err := webhookSvc.ProcessNotification(ctx, notif)
		if err != nil {
			t.Fatalf("expected nil return for gateway ack, got %v", err)
		}

		updatedP, _ := payRepo.FindByID(ctx, p.ID)
		if updatedP.Status != domain.PaymentStatusExpired {
			t.Errorf("expected status to remain EXPIRED, got %s", updatedP.Status)
		}

		var processingStatus, rejectionReason string
		_ = db.DB.QueryRow("SELECT processing_status, rejection_reason FROM payment_events WHERE idempotency_key = $1", notif.IdempotencyKey).Scan(&processingStatus, &rejectionReason)
		if processingStatus != domain.PaymentEventProcessingStatusRejected {
			t.Errorf("expected REJECTED audit status, got %s", processingStatus)
		}
	})

	t.Run("4_duplicate_webhook", func(t *testing.T) {
		u, _, camp := createAuditFixtures(t, db)
		_, p := createAuditPayment(t, db, camp.ID, u.ID, 50000, domain.PaymentStatusPending)

		idemKey := "idem_sc4_" + uuid.New().String()[:8]
		notif := &domain.WebhookNotification{
			Provider:        "MIDTRANS",
			EventSource:     "WEBHOOK",
			OrderID:         p.OrderID,
			GrossAmount:     50000,
			ProviderStatus:  "capture",
			FraudStatus:     "accept",
			RawPayload:      `{}`,
			IdempotencyKey:  idemKey,
		}

		err1 := webhookSvc.ProcessNotification(ctx, notif)
		err2 := webhookSvc.ProcessNotification(ctx, notif)
		if err1 != nil || err2 != nil {
			t.Errorf("duplicate webhooks should both return nil, got err1=%v, err2=%v", err1, err2)
		}
	})

	t.Run("5_concurrent_webhook_and_reconciliation", func(t *testing.T) {
		u, _, camp := createAuditFixtures(t, db)
		_, p := createAuditPayment(t, db, camp.ID, u.ID, 50000, domain.PaymentStatusPending)

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			_ = webhookSvc.ProcessNotification(ctx, &domain.WebhookNotification{
				Provider: "MIDTRANS", EventSource: "WEBHOOK", OrderID: p.OrderID,
				GrossAmount: 50000, ProviderStatus: "capture", FraudStatus: "accept",
				IdempotencyKey: "idem_sc5_wh_" + uuid.New().String()[:8], RawPayload: `{}`,
			})
		}()

		go func() {
			defer wg.Done()
			_ = webhookSvc.ProcessNotification(ctx, &domain.WebhookNotification{
				Provider: "MIDTRANS", EventSource: "RECONCILIATION", OrderID: p.OrderID,
				GrossAmount: 50000, ProviderStatus: "settlement", FraudStatus: "accept",
				IdempotencyKey: "idem_sc5_rec_" + uuid.New().String()[:8], RawPayload: `{}`,
			})
		}()

		wg.Wait()
		updatedP, _ := payRepo.FindByID(ctx, p.ID)
		if updatedP.Status != domain.PaymentStatusCaptured {
			t.Errorf("expected CAPTURED, got %s", updatedP.Status)
		}
	})

	// --- SETTLEMENT (6 - 9) ---
	t.Run("6_CAPTURED_to_SETTLED", func(t *testing.T) {
		u, _, camp := createAuditFixtures(t, db)
		_, p := createAuditPayment(t, db, camp.ID, u.ID, 50000, domain.PaymentStatusCaptured)

		settlement, err := settleSvc.SettleCampaign(ctx, camp.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if settlement.Status != domain.SettlementStatusApproved || settlement.GrossAmount != 50000 {
			t.Errorf("expected APPROVED settlement for 50000, got status=%s, gross=%d", settlement.Status, settlement.GrossAmount)
		}

		updatedP, _ := payRepo.FindByID(ctx, p.ID)
		if updatedP.Status != domain.PaymentStatusSettled {
			t.Errorf("expected SETTLED payment, got %s", updatedP.Status)
		}
	})

	t.Run("7_duplicate_settlement_prevented", func(t *testing.T) {
		u, _, camp := createAuditFixtures(t, db)
		createAuditPayment(t, db, camp.ID, u.ID, 50000, domain.PaymentStatusCaptured)

		_, err1 := settleSvc.SettleCampaign(ctx, camp.ID)
		_, err2 := settleSvc.SettleCampaign(ctx, camp.ID)
		if err1 != nil {
			t.Fatalf("first settlement failed: %v", err1)
		}
		if err2 != domain.ErrInvalidStateTransition {
			t.Errorf("expected ErrInvalidStateTransition for duplicate settlement, got %v", err2)
		}
	})

	t.Run("8_concurrent_settlement_same_campaign", func(t *testing.T) {
		u, _, camp := createAuditFixtures(t, db)
		createAuditPayment(t, db, camp.ID, u.ID, 50000, domain.PaymentStatusCaptured)

		var wg sync.WaitGroup
		var successCount int
		var mu sync.Mutex

		wg.Add(2)
		for i := 0; i < 2; i++ {
			go func() {
				defer wg.Done()
				_, err := settleSvc.SettleCampaign(ctx, camp.ID)
				if err == nil {
					mu.Lock()
					successCount++
					mu.Unlock()
				}
			}()
		}
		wg.Wait()

		if successCount != 1 {
			t.Errorf("expected exactly 1 successful settlement, got %d", successCount)
		}
	})

	t.Run("9_already_settled_payment_excluded", func(t *testing.T) {
		u, _, camp := createAuditFixtures(t, db)
		createAuditPayment(t, db, camp.ID, u.ID, 50000, domain.PaymentStatusCaptured)

		// First settlement
		_, _ = settleSvc.SettleCampaign(ctx, camp.ID)

		// Delete settlement row to test if payment query includes already-settled items
		_, _ = db.DB.Exec("DELETE FROM outbox_events WHERE aggregate_type = 'SETTLEMENT'")
		_, _ = db.DB.Exec("DELETE FROM settlements WHERE campaign_id = $1", camp.ID)

		// Check eligible payments again (settlement_items table still holds the item)
		eligible, err := payRepo.FindEligibleForSettlement(ctx, camp.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(eligible) != 0 {
			t.Errorf("expected 0 eligible payments (already in settlement_items), got %d", len(eligible))
		}
	})

	// --- REFUND (10 - 18) ---
	t.Run("10_CAPTURED_to_PENDING_refund", func(t *testing.T) {
		u, _, camp := createAuditFixtures(t, db)
		_, p := createAuditPayment(t, db, camp.ID, u.ID, 50000, domain.PaymentStatusCaptured)

		refund, err := refundSvc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{
			PaymentID:      p.ID,
			Amount:         20000,
			IdempotencyKey: "idem_sc10_" + uuid.New().String()[:8],
			Reason:         "Partial",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if refund.Status != domain.RefundStatusPending || refund.Amount != 20000 {
			t.Errorf("expected PENDING refund for 20000, got status=%s, amount=%d", refund.Status, refund.Amount)
		}
	})

	t.Run("11_concurrent_refunds_cannot_exceed_gross_amount", func(t *testing.T) {
		u, _, camp := createAuditFixtures(t, db)
		_, p := createAuditPayment(t, db, camp.ID, u.ID, 100000, domain.PaymentStatusCaptured)

		var wg sync.WaitGroup
		var successes int
		var mu sync.Mutex

		amounts := []int64{70000, 50000}
		wg.Add(2)
		for _, amt := range amounts {
			go func(a int64) {
				defer wg.Done()
				_, err := refundSvc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{
					PaymentID:      p.ID,
					Amount:         a,
					IdempotencyKey: fmt.Sprintf("idem_sc11_%d_%s", a, uuid.New().String()[:8]),
					Reason:         "Concurrent overdraw test",
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
			t.Errorf("expected exactly 1 successful refund intent, got %d", successes)
		}
	})

	t.Run("12_PENDING_refund_reserves_balance", func(t *testing.T) {
		u, _, camp := createAuditFixtures(t, db)
		_, p := createAuditPayment(t, db, camp.ID, u.ID, 100000, domain.PaymentStatusCaptured)

		_, _ = refundSvc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{
			PaymentID:      p.ID,
			Amount:         60000,
			IdempotencyKey: "idem_sc12_1_" + uuid.New().String()[:8],
			Reason:         "R1",
		})

		// Second refund for 50000 must fail because 60000 is reserved (100000 - 60000 = 40000 available)
		_, err := refundSvc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{
			PaymentID:      p.ID,
			Amount:         50000,
			IdempotencyKey: "idem_sc12_2_" + uuid.New().String()[:8],
			Reason:         "R2",
		})

		if err != domain.ErrInvalidInput {
			t.Errorf("expected ErrInvalidInput due to balance reservation, got %v", err)
		}
	})

	t.Run("13_FAILED_refund_releases_reservation", func(t *testing.T) {
		u, _, camp := createAuditFixtures(t, db)
		_, p := createAuditPayment(t, db, camp.ID, u.ID, 100000, domain.PaymentStatusCaptured)

		refund, _ := refundSvc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{
			PaymentID:      p.ID,
			Amount:         80000,
			IdempotencyKey: "idem_sc13_" + uuid.New().String()[:8],
			Reason:         "R1",
		})

		// Provider rejects refund -> FAILED
		_ = refundSvc.FinalizeRefund(ctx, refund.ID, "", domain.RefundStatusFailed)

		// Active reservation should be 0, allowing new 80000 refund
		_, err := refundSvc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{
			PaymentID:      p.ID,
			Amount:         80000,
			IdempotencyKey: "idem_sc13_2_" + uuid.New().String()[:8],
			Reason:         "R2",
		})

		if err != nil {
			t.Errorf("failed refund should release reservation, got %v", err)
		}
	})

	t.Run("14_and_15_COMPLETED_refund_updates_Payment_and_Donation", func(t *testing.T) {
		u, _, camp := createAuditFixtures(t, db)
		d, p := createAuditPayment(t, db, camp.ID, u.ID, 100000, domain.PaymentStatusCaptured)

		refund, _ := refundSvc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{
			PaymentID:      p.ID,
			Amount:         100000,
			IdempotencyKey: "idem_sc14_" + uuid.New().String()[:8],
			Reason:         "Full refund",
		})

		_ = refundSvc.FinalizeRefund(ctx, refund.ID, "PROV_REF_100", domain.RefundStatusCompleted)

		updatedP, _ := payRepo.FindByID(ctx, p.ID)
		updatedD, _ := donRepo.FindByID(ctx, d.ID)

		if updatedP.Status != domain.PaymentStatusRefunded {
			t.Errorf("expected Payment status REFUNDED, got %s", updatedP.Status)
		}
		if updatedD.Status != domain.DonationStatusRefunded {
			t.Errorf("expected Donation status REFUNDED, got %s", updatedD.Status)
		}
	})

	t.Run("16_duplicate_refund_request", func(t *testing.T) {
		u, _, camp := createAuditFixtures(t, db)
		_, p := createAuditPayment(t, db, camp.ID, u.ID, 100000, domain.PaymentStatusCaptured)

		idemKey := "idem_sc16_" + uuid.New().String()[:8]
		_, err1 := refundSvc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{PaymentID: p.ID, Amount: 30000, IdempotencyKey: idemKey, Reason: "Dup"})
		_, err2 := refundSvc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{PaymentID: p.ID, Amount: 30000, IdempotencyKey: idemKey, Reason: "Dup"})

		if err1 != nil {
			t.Fatalf("first refund failed: %v", err1)
		}
		if err2 == nil {
			t.Errorf("expected error for duplicate idempotency key")
		}
	})

	t.Run("17_provider_async_refund_response", func(t *testing.T) {
		_, _ = db.DB.Exec("TRUNCATE TABLE outbox_events CASCADE")
		u, _, camp := createAuditFixtures(t, db)
		_, p := createAuditPayment(t, db, camp.ID, u.ID, 100000, domain.PaymentStatusCaptured)

		mockGw := &midtrans.MockPaymentGateway{ShouldStayPending: true}
		worker := service.NewOutboxWorker(outboxRepo, 15*time.Minute, service.WithPaymentGateway(mockGw), service.WithRefundService(refundSvc), service.WithRefundRepository(refundRepo), service.WithPaymentRepository(payRepo))

		refund, _ := refundSvc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{PaymentID: p.ID, Amount: 40000, IdempotencyKey: "idem_sc17_" + uuid.New().String()[:8], Reason: "Async"})
		hasMore, err := worker.ProcessNext(ctx)

		if err != nil || !hasMore {
			t.Fatalf("worker failed: %v", err)
		}

		updatedR, _ := refundRepo.FindByID(ctx, refund.ID)
		if updatedR.Status != domain.RefundStatusPending {
			t.Errorf("async acceptance must keep refund PENDING, got %s", updatedR.Status)
		}
	})

	t.Run("18_provider_definitive_rejection", func(t *testing.T) {
		_, _ = db.DB.Exec("TRUNCATE TABLE outbox_events CASCADE")
		u, _, camp := createAuditFixtures(t, db)
		_, p := createAuditPayment(t, db, camp.ID, u.ID, 100000, domain.PaymentStatusCaptured)

		mockGw := &midtrans.MockPaymentGateway{ShouldReject: true}
		worker := service.NewOutboxWorker(outboxRepo, 15*time.Minute, service.WithPaymentGateway(mockGw), service.WithRefundService(refundSvc), service.WithRefundRepository(refundRepo), service.WithPaymentRepository(payRepo))

		refund, _ := refundSvc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{PaymentID: p.ID, Amount: 40000, IdempotencyKey: "idem_sc18_" + uuid.New().String()[:8], Reason: "Rej"})
		_, _ = worker.ProcessNext(ctx)

		updatedR, _ := refundRepo.FindByID(ctx, refund.ID)
		if updatedR.Status != domain.RefundStatusFailed {
			t.Errorf("definitive rejection must mark refund FAILED, got %s", updatedR.Status)
		}
	})

	// --- CROSS-PHASE & CRASH SCENARIOS (19 - 27) ---
	t.Run("19_settlement_vs_refund_concurrency", func(t *testing.T) {
		u, _, camp := createAuditFixtures(t, db)
		_, p := createAuditPayment(t, db, camp.ID, u.ID, 100000, domain.PaymentStatusCaptured)

		// Start a pending refund
		_, err := refundSvc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{
			PaymentID:      p.ID,
			Amount:         50000,
			IdempotencyKey: "idem_sc19_" + uuid.New().String()[:8],
			Reason:         "Pending refund before settlement",
		})
		if err != nil {
			t.Fatalf("failed to create pending refund: %v", err)
		}

		// Settle campaign: since payment has a PENDING refund, FindEligibleForSettlement MUST exclude it!
		settlement, err := settleSvc.SettleCampaign(ctx, camp.ID)
		if err != nil {
			t.Fatalf("settlement failed: %v", err)
		}

		if settlement.GrossAmount != 0 {
			t.Errorf("expected settlement gross amount 0 (payment excluded due to pending refund), got %d", settlement.GrossAmount)
		}
	})

	t.Run("20_webhook_vs_refund_finalization", func(t *testing.T) {
		u, _, camp := createAuditFixtures(t, db)
		_, p := createAuditPayment(t, db, camp.ID, u.ID, 100000, domain.PaymentStatusCaptured)

		refund, _ := refundSvc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{
			PaymentID:      p.ID,
			Amount:         100000,
			IdempotencyKey: "idem_sc20_" + uuid.New().String()[:8],
			Reason:         "Webhook finalization",
		})

		// Midtrans refund webhook arrives
		notif := &domain.WebhookNotification{
			Provider:        "MIDTRANS",
			EventSource:     "WEBHOOK",
			OrderID:         p.OrderID,
			GrossAmount:     100000,
			ProviderStatus:  "refund",
			FraudStatus:     "accept",
			RawPayload:      `{}`,
			IdempotencyKey:  "idem_sc20_wh_" + uuid.New().String()[:8],
			ProviderEventID: "prov_evt_sc20",
		}
		_ = webhookSvc.ProcessNotification(ctx, notif)

		updatedR, _ := refundRepo.FindByID(ctx, refund.ID)
		if updatedR.Status != domain.RefundStatusCompleted {
			t.Errorf("expected refund status COMPLETED via webhook, got %s", updatedR.Status)
		}
	})

	t.Run("21_webhook_vs_settlement", func(t *testing.T) {
		u, _, camp := createAuditFixtures(t, db)
		_, p := createAuditPayment(t, db, camp.ID, u.ID, 100000, domain.PaymentStatusCaptured)

		// Settle payment
		_, _ = settleSvc.SettleCampaign(ctx, camp.ID)

		// Late webhook arrives attempting to capture payment again
		notif := &domain.WebhookNotification{
			Provider:       "MIDTRANS",
			EventSource:    "WEBHOOK",
			OrderID:        p.OrderID,
			GrossAmount:    100000,
			ProviderStatus: "capture",
			FraudStatus:    "accept",
			RawPayload:     `{}`,
			IdempotencyKey: "idem_sc21_wh_" + uuid.New().String()[:8],
		}
		err := webhookSvc.ProcessNotification(ctx, notif)
		if err != nil {
			t.Fatalf("expected nil for gateway ack, got %v", err)
		}

		updatedP, _ := payRepo.FindByID(ctx, p.ID)
		if updatedP.Status != domain.PaymentStatusSettled {
			t.Errorf("expected payment to remain SETTLED, got %s", updatedP.Status)
		}
	})

	t.Run("22_reconciliation_vs_webhook", func(t *testing.T) {
		u, _, camp := createAuditFixtures(t, db)
		_, p := createAuditPayment(t, db, camp.ID, u.ID, 100000, domain.PaymentStatusPending)

		// Both process identical capture status
		_ = webhookSvc.ProcessNotification(ctx, &domain.WebhookNotification{
			Provider: "MIDTRANS", EventSource: "WEBHOOK", OrderID: p.OrderID, GrossAmount: 100000,
			ProviderStatus: "capture", FraudStatus: "accept", IdempotencyKey: "idem_sc22_wh_" + uuid.New().String()[:8], RawPayload: `{}`,
		})
		_ = webhookSvc.ProcessNotification(ctx, &domain.WebhookNotification{
			Provider: "MIDTRANS", EventSource: "RECONCILIATION", OrderID: p.OrderID, GrossAmount: 100000,
			ProviderStatus: "capture", FraudStatus: "accept", IdempotencyKey: "idem_sc22_rec_" + uuid.New().String()[:8], RawPayload: `{}`,
		})

		updatedP, _ := payRepo.FindByID(ctx, p.ID)
		if updatedP.Status != domain.PaymentStatusCaptured {
			t.Errorf("expected CAPTURED, got %s", updatedP.Status)
		}
	})

	t.Run("23_outbox_retry_after_provider_success", func(t *testing.T) {
		u, _, camp := createAuditFixtures(t, db)
		_, p := createAuditPayment(t, db, camp.ID, u.ID, 100000, domain.PaymentStatusCaptured)

		refund, _ := refundSvc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{
			PaymentID:      p.ID,
			Amount:         100000,
			IdempotencyKey: "idem_sc23_" + uuid.New().String()[:8],
			Reason:         "Retry test",
		})

		// Finalize refund locally
		_ = refundSvc.FinalizeRefund(ctx, refund.ID, "PROV_23", domain.RefundStatusCompleted)

		// Outbox worker retries processing event
		mockGw := midtrans.NewMockPaymentGateway()
		worker := service.NewOutboxWorker(outboxRepo, 15*time.Minute, service.WithPaymentGateway(mockGw), service.WithRefundService(refundSvc), service.WithRefundRepository(refundRepo), service.WithPaymentRepository(payRepo))

		hasMore, err := worker.ProcessNext(ctx)
		if err != nil || !hasMore {
			t.Fatalf("worker failed: %v", err)
		}

		completedSum, _ := refundRepo.SumCompletedRefunds(ctx, p.ID)
		if completedSum != 100000 {
			t.Errorf("expected completed total 100000 (no double count), got %d", completedSum)
		}
	})

	t.Run("24_outbox_lease_reclamation", func(t *testing.T) {
		u, _, camp := createAuditFixtures(t, db)
		_, p := createAuditPayment(t, db, camp.ID, u.ID, 100000, domain.PaymentStatusCaptured)

		_, _ = refundSvc.ProcessLocalRefund(ctx, service.ProcessRefundRequest{PaymentID: p.ID, Amount: 50000, IdempotencyKey: "idem_sc24_" + uuid.New().String()[:8], Reason: "Lease"})

		event, err := outboxRepo.ClaimNext(ctx)
		if err != nil {
			t.Fatalf("failed to claim event: %v", err)
		}

		// Backdate lease
		_, _ = db.DB.Exec("UPDATE outbox_events SET processing_started_at = NOW() - INTERVAL '20 minutes' WHERE id = $1", event.ID)

		reclaimed, err := outboxRepo.ReclaimExpiredLeases(ctx, 15*time.Minute)
		if err != nil || reclaimed == 0 {
			t.Fatalf("expected lease to be reclaimed, reclaimed=%d, err=%v", reclaimed, err)
		}
	})

	t.Run("25_duplicate_outbox_event", func(t *testing.T) {
		now := time.Now()
		evt := &domain.OutboxEvent{
			IdempotencyKey: "idem_sc25_dup",
			AggregateType:  "REFUND",
			AggregateID:    uuid.New().String(),
			EventType:      "REFUND_REQUESTED",
			Payload:        []byte(`{}`),
			Status:         "PENDING",
			AvailableAt:    now,
		}
		err1 := outboxRepo.Create(ctx, evt)
		err2 := outboxRepo.Create(ctx, evt)

		if err1 != nil {
			t.Fatalf("first outbox insert failed: %v", err1)
		}
		if err2 != domain.ErrDuplicate {
			t.Errorf("expected ErrDuplicate for duplicate outbox event, got %v", err2)
		}
	})

	t.Run("26_late_provider_event_after_EXPIRED", func(t *testing.T) {
		u, _, camp := createAuditFixtures(t, db)
		_, p := createAuditPayment(t, db, camp.ID, u.ID, 100000, domain.PaymentStatusExpired)

		notif := &domain.WebhookNotification{
			Provider:       "MIDTRANS",
			EventSource:    "WEBHOOK",
			OrderID:        p.OrderID,
			GrossAmount:    100000,
			ProviderStatus: "capture",
			FraudStatus:    "accept",
			RawPayload:     `{}`,
			IdempotencyKey: "idem_sc26_" + uuid.New().String()[:8],
		}
		_ = webhookSvc.ProcessNotification(ctx, notif)

		updatedP, _ := payRepo.FindByID(ctx, p.ID)
		if updatedP.Status != domain.PaymentStatusExpired {
			t.Errorf("late event must NOT revive EXPIRED payment, got %s", updatedP.Status)
		}
	})

	t.Run("27_late_provider_event_after_REFUNDED", func(t *testing.T) {
		u, _, camp := createAuditFixtures(t, db)
		_, p := createAuditPayment(t, db, camp.ID, u.ID, 100000, domain.PaymentStatusRefunded)

		notif := &domain.WebhookNotification{
			Provider:       "MIDTRANS",
			EventSource:    "WEBHOOK",
			OrderID:        p.OrderID,
			GrossAmount:    100000,
			ProviderStatus: "capture",
			FraudStatus:    "accept",
			RawPayload:     `{}`,
			IdempotencyKey: "idem_sc27_" + uuid.New().String()[:8],
		}
		_ = webhookSvc.ProcessNotification(ctx, notif)

		updatedP, _ := payRepo.FindByID(ctx, p.ID)
		if updatedP.Status != domain.PaymentStatusRefunded {
			t.Errorf("late event must NOT revert REFUNDED payment, got %s", updatedP.Status)
		}
	})
}
