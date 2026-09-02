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
	"carefund-api/internal/service"
)

func TestSettlementEligibilityDomain(t *testing.T) {
	dPaid := &domain.Donation{Status: domain.DonationStatusPaid}
	pCap := &domain.Payment{Status: domain.PaymentStatusCaptured, GrossAmount: 10000}
	if !domain.IsEligibleForSettlement(pCap, dPaid) {
		t.Errorf("error")
	}
	pPending := &domain.Payment{Status: domain.PaymentStatusPending, GrossAmount: 10000}
	if domain.IsEligibleForSettlement(pPending, dPaid) {
		t.Errorf("error")
	}
	pZero := &domain.Payment{Status: domain.PaymentStatusCaptured, GrossAmount: 0}
	if domain.IsEligibleForSettlement(pZero, dPaid) {
		t.Errorf("error")
	}
	dPending := &domain.Donation{Status: domain.DonationStatusPending}
	if domain.IsEligibleForSettlement(pCap, dPending) {
		t.Errorf("error")
	}
}

func setupSettlementTestDB(t *testing.T) (*database.DB, domain.UserRepository, domain.CampaignRepository, domain.DonationRepository, domain.PaymentRepository, domain.SettlementRepository, domain.SettlementItemRepository, domain.CategoryRepository, domain.OutboxEventRepository, database.TransactionManager) {
	cfg := &config.Config{DBHost: "localhost", DBPort: "5432", DBUser: "postgres", DBPassword: "234djisamSOE", DBName: "carefund-app_test", DBSSLMode: "disable"}
	db, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	outboxRepo := database.NewOutboxEventRepository(db)
	return db, database.NewUserRepository(db), database.NewCampaignRepository(db), database.NewDonationRepository(db), database.NewPaymentRepository(db), database.NewSettlementRepository(db), database.NewSettlementItemRepository(db), database.NewCategoryRepository(db), outboxRepo, database.NewTransactionManager(db)
}

func mustCreate(t *testing.T, label string, err error) {
	if err != nil {
		t.Fatalf("Create %s failed: %v", label, err)
	}
}

func TestSettlementService(t *testing.T) {
	db, userRepo, campaignRepo, donationRepo, paymentRepo, settlementRepo, settlementItemRepo, categoryRepo, outboxRepo, tx := setupSettlementTestDB(t)
	defer db.Close()
	ctx := context.Background()
	svc := service.NewSettlementService(campaignRepo, paymentRepo, settlementRepo, settlementItemRepo, outboxRepo, tx)

	randStr := fmt.Sprintf("%d", time.Now().UnixNano())
	u := &domain.User{Email: "settle1_" + randStr + "@test.com", PasswordHash: "x", Name: "Test"}
	mustCreate(t, "user", userRepo.Create(ctx, u))

	cat := &domain.Category{Name: "Cat Settle1 " + randStr, Slug: "cat-settle1-" + randStr}
	mustCreate(t, "cat", categoryRepo.Create(ctx, cat))

	c := &domain.Campaign{OwnerID: u.ID, CategoryID: cat.ID, Title: "Settle Test", Slug: "settle-test-" + randStr, Description: "Desc", TargetAmount: 100000, StartAt: time.Now(), EndAt: time.Now().Add(time.Hour), Status: domain.CampaignStateActive}
	mustCreate(t, "camp", campaignRepo.Create(ctx, c))

	d1 := &domain.Donation{CampaignID: c.ID, DonorID: &u.ID, Amount: 10000, Status: domain.DonationStatusPaid}
	mustCreate(t, "don1", donationRepo.Create(ctx, d1))
	p1 := &domain.Payment{DonationID: d1.ID, Provider: "MIDTRANS", OrderID: "ORD-S1-" + randStr, GrossAmount: 10000, Status: domain.PaymentStatusCaptured}
	mustCreate(t, "pay1", paymentRepo.Create(ctx, p1))

	d2 := &domain.Donation{CampaignID: c.ID, DonorID: &u.ID, Amount: 20000, Status: domain.DonationStatusPending}
	mustCreate(t, "don2", donationRepo.Create(ctx, d2))
	p2 := &domain.Payment{DonationID: d2.ID, Provider: "MIDTRANS", OrderID: "ORD-S2-" + randStr, GrossAmount: 20000, Status: domain.PaymentStatusPending}
	mustCreate(t, "pay2", paymentRepo.Create(ctx, p2))

	d3 := &domain.Donation{CampaignID: c.ID, DonorID: &u.ID, Amount: 50000, Status: domain.DonationStatusPaid}
	mustCreate(t, "don3", donationRepo.Create(ctx, d3))
	p3 := &domain.Payment{DonationID: d3.ID, Provider: "MIDTRANS", OrderID: "ORD-S3-" + randStr, GrossAmount: 50000, Status: domain.PaymentStatusCaptured}
	mustCreate(t, "pay3", paymentRepo.Create(ctx, p3))

	settlement, err := svc.SettleCampaign(ctx, c.ID)
	if err != nil {
		t.Fatalf("Failed to settle campaign: %v", err)
	}
	if settlement.GrossAmount != 60000 {
		t.Errorf("Expected 60000, got %d", settlement.GrossAmount)
	}

	updatedP1, _ := paymentRepo.FindByID(ctx, p1.ID)
	if updatedP1.Status != domain.PaymentStatusSettled {
		t.Errorf("Expected SETTLED")
	}
	updatedP2, _ := paymentRepo.FindByID(ctx, p2.ID)
	if updatedP2.Status != domain.PaymentStatusPending {
		t.Errorf("Expected PENDING")
	}
	updatedP3, _ := paymentRepo.FindByID(ctx, p3.ID)
	if updatedP3.Status != domain.PaymentStatusSettled {
		t.Errorf("Expected SETTLED")
	}
	updatedCamp, _ := campaignRepo.FindByIDForUpdate(ctx, c.ID)
	if updatedCamp.CurrentAmount != 60000 {
		t.Errorf("Expected 60000, got %d", updatedCamp.CurrentAmount)
	}

	_, err = svc.SettleCampaign(ctx, c.ID)
	if err == nil {
		t.Errorf("Expected error double settling")
	}
}

func TestConcurrentSettlementSameCampaign(t *testing.T) {
	db, userRepo, campaignRepo, donationRepo, paymentRepo, settlementRepo, settlementItemRepo, categoryRepo, outboxRepo, tx := setupSettlementTestDB(t)
	defer db.Close()
	ctx := context.Background()
	svc := service.NewSettlementService(campaignRepo, paymentRepo, settlementRepo, settlementItemRepo, outboxRepo, tx)

	randStr := fmt.Sprintf("%d", time.Now().UnixNano())
	u := &domain.User{Email: "conc_settle_" + randStr + "@test.com", PasswordHash: "x", Name: "Test2"}
	mustCreate(t, "u2", userRepo.Create(ctx, u))

	cat := &domain.Category{Name: "Cat Settle2 " + randStr, Slug: "cat-settle2-" + randStr}
	mustCreate(t, "cat2", categoryRepo.Create(ctx, cat))

	c := &domain.Campaign{OwnerID: u.ID, CategoryID: cat.ID, Title: "Conc Settle", Slug: "conc-settle-" + randStr, Description: "desc", TargetAmount: 100000, StartAt: time.Now(), EndAt: time.Now().Add(time.Hour), Status: domain.CampaignStateActive}
	mustCreate(t, "c2", campaignRepo.Create(ctx, c))

	d1 := &domain.Donation{CampaignID: c.ID, DonorID: &u.ID, Amount: 25000, Status: domain.DonationStatusPaid}
	mustCreate(t, "d1", donationRepo.Create(ctx, d1))
	p1 := &domain.Payment{DonationID: d1.ID, Provider: "MIDTRANS", OrderID: "ORD-CS1-" + randStr, GrossAmount: 25000, Status: domain.PaymentStatusCaptured}
	mustCreate(t, "p1", paymentRepo.Create(ctx, p1))

	var wg sync.WaitGroup
	successes := 0
	var mu sync.Mutex
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.SettleCampaign(ctx, c.ID)
			if err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if successes != 1 {
		t.Errorf("Expected exactly 1 successful settlement, got %d", successes)
	}
	updatedCamp, _ := campaignRepo.FindByIDForUpdate(ctx, c.ID)
	if updatedCamp.CurrentAmount != 25000 {
		t.Errorf("Expected 25000, got %d", updatedCamp.CurrentAmount)
	}
}
