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

func TestDonationCreationRules(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	txManager := database.NewTransactionManager(db)

	campRepo := database.NewCampaignRepository(db)
	donRepo := database.NewDonationRepository(db)
	payRepo := database.NewPaymentRepository(db)
	userRepo := database.NewUserRepository(db)
	roleRepo := database.NewRoleRepository(db)
	authSvc := service.NewAuthService("secret", 15*time.Minute)
	userSvc := service.NewUserService(userRepo, roleRepo, authSvc, txManager)

	mockGw := midtrans.NewMockPaymentGateway()
	donSvc := service.NewDonationService(donRepo, payRepo, campRepo, mockGw, txManager)

	// Create user
	email := "donor_" + time.Now().Format("150405.000") + "@example.com"
	user, _ := userSvc.RegisterUser(ctx, email, "pass", "Donor")

	catRepo := database.NewCategoryRepository(db)
	cat := &domain.Category{Name: "C", Slug: "c2", IsActive: true}
	catRepo.Create(ctx, cat)

	// Create ineligible campaign (DRAFT)
	c := &domain.Campaign{
		OwnerID:      user.ID,
		CategoryID:   cat.ID,
		Title:        "Test Camp",
		Description:  "Desc",
		TargetAmount: 1000000,
		StartAt:      time.Now(),
		EndAt:        time.Now().Add(24 * time.Hour),
		Status:       domain.CampaignStateDraft,
	}
	if err := campRepo.Create(ctx, c); err != nil {
		t.Fatalf("failed to create campaign: %v", err)
	}

	// Attempt donation on ineligible
	_, _, _, err := donSvc.CreateDonation(ctx, user.ID, user.Email, user.Name, c.ID, 50000, false, "Good luck")
	if err != domain.ErrInvalidStateTransition {
		t.Errorf("expected ErrInvalidStateTransition for ineligible campaign, got %v", err)
	}

	// Make campaign ACTIVE
	c.Status = domain.CampaignStateActive
	campRepo.Update(ctx, c)

	// Attempt valid donation
	don, pay, _, err := donSvc.CreateDonation(ctx, user.ID, user.Email, user.Name, c.ID, 50000, false, "Good luck")
	if err != nil {
		t.Fatalf("expected successful donation, got %v", err)
	}

	if don.Status != domain.DonationStatusPending || pay.Status != domain.PaymentStatusPending {
		t.Errorf("expected pending initial states")
	}
	if pay.GrossAmount != 50000 {
		t.Errorf("expected payment amount to match donation amount")
	}

	// Check persistence
	savedDon, _ := donRepo.FindByID(ctx, don.ID)
	savedPay, _ := payRepo.FindByID(ctx, pay.ID)

	if savedDon.Amount != 50000 || savedPay.OrderID == "" {
		t.Errorf("persistence failed")
	}
}
