package service_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"carefund-api/internal/database"
	"carefund-api/internal/domain"
	"carefund-api/internal/service"
)

func TestWebhookIdempotencyAndTransition(t *testing.T) {
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

	webhookSvc := service.NewWebhookService(payRepo, donRepo, eventRepo, txManager)

	// Setup data
	u := &domain.User{Email: "donor_wh@example.com", PasswordHash: "h", Name: "N", IsActive: true}
	_ = userRepo.Create(ctx, u)

	cat := &domain.Category{Name: "C", Slug: "c2", IsActive: true}
	_ = catRepo.Create(ctx, cat)

	c := &domain.Campaign{OwnerID: u.ID, CategoryID: cat.ID, Title: "T", Description: "D", TargetAmount: 100, StartAt: time.Now(), EndAt: time.Now().Add(time.Hour), Status: "ACTIVE"}
	_ = campRepo.Create(ctx, c)

	d1 := &domain.Donation{CampaignID: c.ID, DonorID: &u.ID, Amount: 10000, Status: "PENDING"}
	_ = donRepo.Create(ctx, d1)

	p1 := &domain.Payment{
		DonationID:  d1.ID,
		Provider:    "MIDTRANS",
		OrderID:     "ORDER-WH-123",
		GrossAmount: 10000,
		Status:      "PENDING",
	}
	_ = payRepo.Create(ctx, p1)

	// 1. Process valid settlement webhook
	notif := &domain.WebhookNotification{
		Provider:        "MIDTRANS",
		ProviderEventID: "tx-123",
		OrderID:         p1.OrderID,
		TransactionID:   "tx-123",
		GrossAmount:     10000,
		ProviderStatus:  "settlement",
		FraudStatus:     "accept",
		IdempotencyKey:  "key1",
		RawPayload:      "{}",
	}

	err := webhookSvc.ProcessNotification(ctx, notif)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	// Verify DB state
	p, _ := payRepo.FindByID(ctx, p1.ID)
	if p.Status != domain.PaymentStatusCaptured {
		t.Errorf("expected CAPTURED, got %s", p.Status)
	}

	don, _ := donRepo.FindByID(ctx, d1.ID)
	if don.Status != domain.DonationStatusPaid {
		t.Errorf("expected PAID donation, got %s", don.Status)
	}

	// 2. Process duplicate webhook (idempotency test)
	err = webhookSvc.ProcessNotification(ctx, notif)
	if err != nil {
		t.Errorf("expected no error for idempotent duplicate, got %v", err)
	}

	// 3. Process mismatch amount
	notifBadAmount := &domain.WebhookNotification{
		Provider:        "MIDTRANS",
		ProviderEventID: "tx-999",
		OrderID:         p1.OrderID,
		TransactionID:   "tx-999",
		GrossAmount:     99999, // wrong
		ProviderStatus:  "settlement",
		FraudStatus:     "accept",
		IdempotencyKey:  "key2",
		RawPayload:      "{}",
	}
	err = webhookSvc.ProcessNotification(ctx, notifBadAmount)
	if err != nil {
		t.Errorf("expected clean rejection for amount mismatch, got error: %v", err)
	}
}

func TestMidtransStatusMapping(t *testing.T) {
	if service.MapMidtransStatus("capture", "accept") != domain.PaymentStatusCaptured {
		t.Errorf("expected CAPTURED")
	}
	if service.MapMidtransStatus("capture", "challenge") != domain.PaymentStatusPending {
		t.Errorf("expected PENDING")
	}
	if service.MapMidtransStatus("settlement", "") != domain.PaymentStatusCaptured {
		t.Errorf("expected CAPTURED")
	}
	if service.MapMidtransStatus("deny", "") != domain.PaymentStatusFailed {
		t.Errorf("expected FAILED")
	}
}

func TestConcurrentWebhookIdempotency(t *testing.T) {
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

	webhookSvc := service.NewWebhookService(payRepo, donRepo, eventRepo, txManager)

	u := &domain.User{Email: "donor_wh2@example.com", PasswordHash: "h", Name: "N", IsActive: true}
	_ = userRepo.Create(ctx, u)
	cat := &domain.Category{Name: "C", Slug: "c3", IsActive: true}
	_ = catRepo.Create(ctx, cat)
	c := &domain.Campaign{OwnerID: u.ID, CategoryID: cat.ID, Title: "T", Description: "D", TargetAmount: 100, StartAt: time.Now(), EndAt: time.Now().Add(time.Hour), Status: "ACTIVE"}
	_ = campRepo.Create(ctx, c)
	d1 := &domain.Donation{CampaignID: c.ID, DonorID: &u.ID, Amount: 10000, Status: "PENDING"}
	_ = donRepo.Create(ctx, d1)
	p1 := &domain.Payment{DonationID: d1.ID, Provider: "MIDTRANS", OrderID: "ORDER-WH-CONC", GrossAmount: 10000, Status: "PENDING"}
	_ = payRepo.Create(ctx, p1)

	notif := &domain.WebhookNotification{
		Provider:        "MIDTRANS",
		ProviderEventID: "tx-conc",
		OrderID:         p1.OrderID,
		TransactionID:   "tx-conc",
		GrossAmount:     10000,
		ProviderStatus:  "settlement",
		FraudStatus:     "accept",
		IdempotencyKey:  "key-conc",
		RawPayload:      "{}",
	}

	var wg sync.WaitGroup
	errs := make(chan error, 10)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- webhookSvc.ProcessNotification(context.Background(), notif)
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("expected no errors, got: %v", err)
		}
	}

	p, _ := payRepo.FindByID(ctx, p1.ID)
	if p.Status != domain.PaymentStatusCaptured {
		t.Errorf("expected CAPTURED, got %s", p.Status)
	}
}
