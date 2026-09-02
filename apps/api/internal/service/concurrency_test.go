package service_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"carefund-api/internal/database"
	"carefund-api/internal/domain"
	"carefund-api/internal/service"
	"github.com/google/uuid"
)

func setupConcurrencyDependencies(t *testing.T) (*domain.User, *domain.Campaign, domain.DonationRepository, domain.PaymentRepository, domain.WebhookService, *domain.Payment) {
	db := setupTestDB(t)
	ctx := context.Background()

	txManager := database.NewTransactionManager(db)
	donRepo := database.NewDonationRepository(db)
	payRepo := database.NewPaymentRepository(db)
	eventRepo := database.NewPaymentEventRepository(db)
	userRepo := database.NewUserRepository(db)
	catRepo := database.NewCategoryRepository(db)
	campRepo := database.NewCampaignRepository(db)

	u := &domain.User{Email: "conc_" + uuid.New().String()[:8] + "@example.com", PasswordHash: "h", Name: "N", IsActive: true}
	_ = userRepo.Create(ctx, u)

	cat := &domain.Category{Name: "C", Slug: "c_conc" + uuid.New().String()[:8], IsActive: true}
	_ = catRepo.Create(ctx, cat)

	c := &domain.Campaign{OwnerID: u.ID, CategoryID: cat.ID, Title: "T", Description: "D", TargetAmount: 100, StartAt: time.Now(), EndAt: time.Now().Add(time.Hour), Status: "ACTIVE"}
	_ = campRepo.Create(ctx, c)

	webhookSvc := service.NewWebhookService(payRepo, donRepo, eventRepo, txManager)

	d := &domain.Donation{CampaignID: c.ID, DonorID: &u.ID, Amount: 10000, Status: "PENDING"}
	_ = donRepo.Create(ctx, d)
	p := &domain.Payment{
		DonationID:  d.ID,
		Provider:    "MIDTRANS",
		OrderID:     "ORD-" + uuid.New().String()[:8],
		GrossAmount: 10000,
		Status:      "PENDING",
	}
	_ = payRepo.Create(ctx, p)
	return u, c, donRepo, payRepo, webhookSvc, p
}

// TEST A  EDuplicate webhook (5+ times)
func TestConcurrencyDuplicateWebhook(t *testing.T) {
	_, _, _, payRepo, webhookSvc, p := setupConcurrencyDependencies(t)

	var wg sync.WaitGroup
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			notif := &domain.WebhookNotification{
				Provider:        "MIDTRANS",
				EventSource:     "WEBHOOK",
				ProviderEventID: "tx_123",
				OrderID:         p.OrderID,
				TransactionID:   "tx_123",
				GrossAmount:     10000,
				ProviderStatus:  "capture",
				FraudStatus:     "accept",
				RawPayload:      `{}`,
				IdempotencyKey:  "idem_key_dup",
			}
			_ = webhookSvc.ProcessNotification(ctx, notif)
		}()
	}
	wg.Wait()

	finalP, _ := payRepo.FindByID(ctx, p.ID)
	if finalP.Status != domain.PaymentStatusCaptured {
		t.Errorf("Expected CAPTURED, got %s", finalP.Status)
	}
}

// TEST C  EDifferent event keys (Same Payment)
func TestConcurrencyDifferentEventKeys(t *testing.T) {
	_, _, _, payRepo, webhookSvc, p := setupConcurrencyDependencies(t)

	var wg sync.WaitGroup
	ctx := context.Background()

	// Event A: settlement
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := webhookSvc.ProcessNotification(ctx, &domain.WebhookNotification{
			Provider: "MIDTRANS", EventSource: "WEBHOOK", OrderID: p.OrderID, TransactionID: "tx_A",
			GrossAmount: 10000, ProviderStatus: "settlement", FraudStatus: "accept", IdempotencyKey: "idem_A", RawPayload: "{}",
		})
		if err != nil {
			t.Errorf("Event A failed: %v", err)
		}
	}()

	// Event B: expire
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := webhookSvc.ProcessNotification(ctx, &domain.WebhookNotification{
			Provider: "MIDTRANS", EventSource: "WEBHOOK", OrderID: p.OrderID, TransactionID: "tx_B",
			GrossAmount: 10000, ProviderStatus: "expire", FraudStatus: "accept", IdempotencyKey: "idem_B", RawPayload: "{}",
		})
		if err != nil {
			t.Errorf("Event B failed with err (should reject cleanly): %v", err)
		}
	}()

	wg.Wait()

	finalP, _ := payRepo.FindByID(ctx, p.ID)
	// It should be either CAPTURED or EXPIRED, depending on which lock won first.
	if finalP.Status != domain.PaymentStatusCaptured && finalP.Status != domain.PaymentStatusExpired {
		t.Errorf("Expected CAPTURED or EXPIRED, got %s", finalP.Status)
	}
}

// TEST G  EAmount mismatch under concurrency
func TestConcurrencyAmountMismatch(t *testing.T) {
	_, _, _, payRepo, webhookSvc, p := setupConcurrencyDependencies(t)

	var wg sync.WaitGroup
	ctx := context.Background()

	// Event A: Valid Amount -> CAPTURED
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := webhookSvc.ProcessNotification(ctx, &domain.WebhookNotification{
			Provider: "MIDTRANS", EventSource: "WEBHOOK", OrderID: p.OrderID, TransactionID: "tx_G1",
			GrossAmount: 10000, ProviderStatus: "capture", FraudStatus: "accept", IdempotencyKey: "idem_G1", RawPayload: "{}",
		})
		if err != nil {
			t.Errorf("Event G1 failed: %v", err)
		}
	}()

	// Event B: Invalid Amount -> attempt EXPIRED
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := webhookSvc.ProcessNotification(ctx, &domain.WebhookNotification{
			Provider: "MIDTRANS", EventSource: "WEBHOOK", OrderID: p.OrderID, TransactionID: "tx_G2",
			GrossAmount: 99999, ProviderStatus: "expire", FraudStatus: "accept", IdempotencyKey: "idem_G2", RawPayload: "{}",
		})
		if err != nil {
			t.Errorf("Event G2 failed with err (should reject cleanly): %v", err)
		}
	}()

	wg.Wait()

	finalP, _ := payRepo.FindByID(ctx, p.ID)
	if finalP.Status != domain.PaymentStatusCaptured {
		t.Errorf("Expected CAPTURED, got %s", finalP.Status)
	}
}
