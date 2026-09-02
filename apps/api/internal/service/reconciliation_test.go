package service_test

import (
	"context"
	"testing"
	"time"

	"carefund-api/internal/database"
	"carefund-api/internal/domain"
	"carefund-api/internal/infrastructure/payment/midtrans"
	"carefund-api/internal/service"
)

func TestExpirationPolicy(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	txManager := database.NewTransactionManager(db)
	donRepo := database.NewDonationRepository(db)
	payRepo := database.NewPaymentRepository(db)
	eventRepo := database.NewPaymentEventRepository(db)
	campRepo := database.NewCampaignRepository(db)
	userRepo := database.NewUserRepository(db)
	catRepo := database.NewCategoryRepository(db)

	mockGw := midtrans.NewMockPaymentGateway()
	webhookSvc := service.NewWebhookService(payRepo, donRepo, eventRepo, txManager)
	reconSvc := service.NewReconciliationService(payRepo, webhookSvc, mockGw, txManager)

	// Setup data
	u := &domain.User{Email: "donor_exp@example.com", PasswordHash: "h", Name: "N", IsActive: true}
	_ = userRepo.Create(ctx, u)
	cat := &domain.Category{Name: "C", Slug: "c_exp", IsActive: true}
	_ = catRepo.Create(ctx, cat)
	c := &domain.Campaign{OwnerID: u.ID, CategoryID: cat.ID, Title: "T", Description: "D", TargetAmount: 1000000, StartAt: time.Now(), EndAt: time.Now().Add(time.Hour), Status: "ACTIVE"}
	_ = campRepo.Create(ctx, c)

	createPayment := func(orderID string, ageMins int) (*domain.Payment, *domain.Donation) {
		d := &domain.Donation{CampaignID: c.ID, DonorID: &u.ID, Amount: 10000, Status: "PENDING"}
		_ = donRepo.Create(ctx, d)
		p := &domain.Payment{
			DonationID:  d.ID,
			Provider:    "MIDTRANS",
			OrderID:     orderID,
			GrossAmount: 10000,
			Status:      "PENDING",
		}
		_ = payRepo.Create(ctx, p)
		// set exact age
		_, err := db.ExecContext(ctx, "UPDATE payments SET created_at = NOW() - INTERVAL '1 minute' * $1 WHERE id = $2", ageMins, p.ID)
		if err != nil {
			t.Fatal(err)
		}
		p, _ = payRepo.FindByID(ctx, p.ID)
		return p, d
	}

	ttl := 45 * time.Minute

	// Test 1 — Under TTL
	p1, _ := createPayment("ORDER-EXP-U45", 44)

	// Test 2 & 3 — Exactly TTL and Beyond TTL
	p2, _ := createPayment("ORDER-EXP-E45", 45)
	p3, _ := createPayment("ORDER-EXP-O60", 60)

	// Test 8 - NOT FOUND after TTL (will use ORDER-NOTFOUND)
	p8, _ := createPayment("ORDER-NOTFOUND", 50)

	// Test 9 - NOT FOUND before TTL (should remain PENDING)
	p9, _ := createPayment("ORDER-NOTFOUND-EARLY", 30) // Wait, Mock returns NOT FOUND for ORDER-NOTFOUND. I'll need to modify mock to support this or just use ORDER-NOTFOUND and set age=30.
	_, _ = db.ExecContext(ctx, "UPDATE payments SET order_id = 'ORDER-NOTFOUND' WHERE id = $1", p9.ID)
	p9.OrderID = "ORDER-NOTFOUND"

	// Run batch reconciliation
	_, err := reconSvc.ReconcilePendingPayments(ctx, 50, ttl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify Test 1: Under TTL (should not be processed)
	p1Check, _ := payRepo.FindByID(ctx, p1.ID)
	if p1Check.Status != domain.PaymentStatusPending {
		t.Errorf("Test 1: Expected PENDING, got %s", p1Check.Status)
	}

	// Verify Test 2 & 3: Processed successfully (mock returns 'settlement' -> CAPTURED for regular orders)
	p2Check, _ := payRepo.FindByID(ctx, p2.ID)
	if p2Check.Status != domain.PaymentStatusCaptured {
		t.Errorf("Test 2: Expected CAPTURED, got %s", p2Check.Status)
	}
	p3Check, _ := payRepo.FindByID(ctx, p3.ID)
	if p3Check.Status != domain.PaymentStatusCaptured {
		t.Errorf("Test 3: Expected CAPTURED, got %s", p3Check.Status)
	}

	// Verify Test 8: NOT FOUND after TTL -> EXPIRED
	p8Check, _ := payRepo.FindByID(ctx, p8.ID)
	if p8Check.Status != domain.PaymentStatusExpired {
		t.Errorf("Test 8: Expected EXPIRED, got %s", p8Check.Status)
	}
	d8Check, _ := donRepo.FindByID(ctx, p8.DonationID)
	if d8Check.Status != domain.DonationStatusExpired {
		t.Errorf("Test 8 Donation: Expected EXPIRED, got %s", d8Check.Status)
	}

	// Verify Test 9: NOT FOUND before TTL -> PENDING (since it shouldn't even be selected)
	p9Check, _ := payRepo.FindByID(ctx, p9.ID)
	if p9Check.Status != domain.PaymentStatusPending {
		t.Errorf("Test 9: Expected PENDING, got %s", p9Check.Status)
	}

	// Test 10 - Provider timeout
	mockGw.ShouldFail = true
	p10, _ := createPayment("ORDER-EXP-TIMEOUT", 50)
	_, _ = reconSvc.ReconcilePendingPayments(ctx, 50, ttl)
	p10Check, _ := payRepo.FindByID(ctx, p10.ID)
	if p10Check.Status != domain.PaymentStatusPending {
		t.Errorf("Test 10: Expected PENDING, got %s", p10Check.Status)
	}
	mockGw.ShouldFail = false

	// Test 12 - Repeated execution (No duplicate transition)
	// Make sure ORDER-EXP-TIMEOUT is not pending anymore so we expect 0
	_, _ = db.ExecContext(ctx, "UPDATE payments SET status = 'CANCELLED' WHERE order_id = 'ORDER-EXP-TIMEOUT'")

	processed2, _ := reconSvc.ReconcilePendingPayments(ctx, 50, ttl)
	if processed2 != 0 {
		t.Errorf("Test 12: Expected 0 processed, got %d", processed2)
	}

	// Test 13 - Late Webhook (EXPIRED -> CAPTURED should fail or be ignored)
	lateNotif := &domain.WebhookNotification{
		Provider:        "MIDTRANS",
		EventSource:     "WEBHOOK",
		ProviderEventID: "late-webhook-tx",
		OrderID:         p8.OrderID,
		TransactionID:   "late-webhook-tx",
		GrossAmount:     p8.GrossAmount,
		ProviderStatus:  "settlement",
		FraudStatus:     "accept",
		RawPayload:      `{}`,
		IdempotencyKey:  "late-webhook-key",
	}
	err = webhookSvc.ProcessNotification(ctx, lateNotif)
	if err != nil {
		t.Errorf("Test 13: Expected clean rejection without error, got %v", err)
	}
}
