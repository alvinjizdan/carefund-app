package database_test

import (
	"context"
	"testing"
	"time"

	"carefund-api/internal/config"
	"carefund-api/internal/database"
	"carefund-api/internal/domain"
)

func setupTestDB(t *testing.T) *database.DB {
	cfg := &config.Config{
		DBHost:     "localhost",
		DBPort:     "5432",
		DBUser:     "postgres",
		DBPassword: "234djisamSOE",
		DBName:     "carefund-app_test",
		DBSSLMode:  "disable",
	}

	db, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}

	// Clean up tables for a fresh state for tests that might need it
	// We do this by deleting data in reverse dependency order
	tables := []string{"payment_events", "refunds", "payments", "donations", "settlement_items", "settlements", "campaigns", "categories", "user_roles", "users"}
	for _, table := range tables {
		_, err := db.ExecContext(context.Background(), "DELETE FROM "+table)
		if err != nil {
			t.Logf("failed to clean table %s: %v", table, err)
		}
	}

	return db
}

func TestUserRepository(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := database.NewUserRepository(db)
	ctx := context.Background()

	// Test Create
	user := &domain.User{
		Email:        "test@example.com",
		PasswordHash: "hashed",
		Name:         "Test User",
		IsActive:     true,
	}
	err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	if user.ID == "" {
		t.Errorf("expected user ID to be set")
	}

	// Test Duplicate Email
	err = repo.Create(ctx, user)
	if err != domain.ErrDuplicate {
		t.Errorf("expected ErrDuplicate, got %v", err)
	}

	// Test FindByID
	foundUser, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to find user by ID: %v", err)
	}
	if foundUser.Email != user.Email {
		t.Errorf("expected email %s, got %s", user.Email, foundUser.Email)
	}

	// Test FindByEmail
	foundUser, err = repo.FindByEmail(ctx, user.Email)
	if err != nil {
		t.Fatalf("failed to find user by email: %v", err)
	}
	if foundUser.ID != user.ID {
		t.Errorf("expected ID %s, got %s", user.ID, foundUser.ID)
	}
}

func TestCampaignRepository(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := database.NewUserRepository(db)
	catRepo := database.NewCategoryRepository(db)
	campRepo := database.NewCampaignRepository(db)

	owner := &domain.User{Email: "owner@example.com", PasswordHash: "hash", Name: "Owner", IsActive: true}
	_ = userRepo.Create(ctx, owner)

	category := &domain.Category{Name: "Education", Slug: "education", IsActive: true}
	_ = catRepo.Create(ctx, category)

	// Test Create Campaign
	now := time.Now().Round(time.Microsecond)
	campaign := &domain.Campaign{
		OwnerID:       owner.ID,
		CategoryID:    category.ID,
		Title:         "Help School",
		Slug:          "help-school",
		Description:   "Desc",
		TargetAmount:  1000000,
		CurrentAmount: 0,
		StartAt:       now,
		EndAt:         now.Add(24 * time.Hour),
		Status:        "ACTIVE",
	}

	err := campRepo.Create(ctx, campaign)
	if err != nil {
		t.Fatalf("failed to create campaign: %v", err)
	}
	if campaign.ID == "" {
		t.Errorf("expected campaign ID to be set")
	}

	// Test Find
	found, err := campRepo.FindByID(ctx, campaign.ID)
	if err != nil {
		t.Fatalf("failed to find campaign: %v", err)
	}
	if found.Title != campaign.Title {
		t.Errorf("expected title %s, got %s", campaign.Title, found.Title)
	}

	// Test Update
	campaign.Title = "Updated Title"
	err = campRepo.Update(ctx, campaign)
	if err != nil {
		t.Fatalf("failed to update campaign: %v", err)
	}

	found, _ = campRepo.FindByID(ctx, campaign.ID)
	if found.Title != "Updated Title" {
		t.Errorf("expected updated title, got %s", found.Title)
	}

	// Test List
	list, err := campRepo.List(ctx, 10, 0)
	if err != nil || len(list) != 1 {
		t.Fatalf("failed to list campaigns")
	}

	// Test Owner Lookup
	ownerList, err := campRepo.ListByOwner(ctx, owner.ID, 10, 0)
	if err != nil || len(ownerList) != 1 {
		t.Fatalf("failed to list campaigns by owner")
	}
}

func TestDonationAndPaymentRepository(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := database.NewUserRepository(db)
	catRepo := database.NewCategoryRepository(db)
	campRepo := database.NewCampaignRepository(db)
	donRepo := database.NewDonationRepository(db)
	payRepo := database.NewPaymentRepository(db)

	owner := &domain.User{Email: "owner2@example.com", PasswordHash: "hash", Name: "Owner", IsActive: true}
	_ = userRepo.Create(ctx, owner)

	cat := &domain.Category{Name: "Health", Slug: "health", IsActive: true}
	_ = catRepo.Create(ctx, cat)

	now := time.Now()
	camp := &domain.Campaign{
		OwnerID:      owner.ID,
		CategoryID:   cat.ID,
		Title:        "Health Campaign",
		Slug:         "health-campaign",
		Description:  "Desc",
		TargetAmount: 1000,
		StartAt:      now,
		EndAt:        now.Add(24 * time.Hour),
		Status:       "ACTIVE",
	}
	_ = campRepo.Create(ctx, camp)

	// Test Donation
	donation := &domain.Donation{
		CampaignID:  camp.ID,
		Amount:      50000,
		IsAnonymous: false,
		Status:      "PENDING",
	}
	err := donRepo.Create(ctx, donation)
	if err != nil {
		t.Fatalf("failed to create donation: %v", err)
	}

	foundDon, err := donRepo.FindByID(ctx, donation.ID)
	if err != nil || foundDon.Amount != donation.Amount {
		t.Fatalf("failed to find donation properly")
	}

	// Test Payment
	payment := &domain.Payment{
		DonationID:  donation.ID,
		Provider:    "MIDTRANS",
		OrderID:     "ORDER-123",
		GrossAmount: 50000,
		Status:      "PENDING",
	}
	err = payRepo.Create(ctx, payment)
	if err != nil {
		t.Fatalf("failed to create payment: %v", err)
	}

	// Duplicate OrderID
	dupPayment := &domain.Payment{
		DonationID:  donation.ID, // DB unique constraint on donation_id will fail first, but let's just create a new donation
		Provider:    "MIDTRANS",
		OrderID:     "ORDER-123",
		GrossAmount: 50000,
		Status:      "PENDING",
	}
	don2 := &domain.Donation{CampaignID: camp.ID, Amount: 1, Status: "PENDING"}
	_ = donRepo.Create(ctx, don2)
	dupPayment.DonationID = don2.ID
	
	err = payRepo.Create(ctx, dupPayment)
	if err != domain.ErrDuplicate {
		t.Errorf("expected ErrDuplicate for order_id, got %v", err)
	}

	// Test Find Payment
	foundPay, err := payRepo.FindByOrderID(ctx, "ORDER-123")
	if err != nil || foundPay.ID != payment.ID {
		t.Fatalf("failed to find payment by order id")
	}
}

func TestTransactionRollback(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	txManager := database.NewTransactionManager(db)
	userRepo := database.NewUserRepository(db)

	email := "rollback@example.com"

	err := txManager.Do(ctx, func(txCtx context.Context) error {
		// Insert valid record
		err := userRepo.Create(txCtx, &domain.User{
			Email:        email,
			PasswordHash: "pwd",
			Name:         "Rollback Test",
			IsActive:     true,
		})
		if err != nil {
			return err
		}

		// Insert invalid record (duplicate email)
		err = userRepo.Create(txCtx, &domain.User{
			Email:        email, // duplicate
			PasswordHash: "pwd",
			Name:         "Duplicate",
			IsActive:     true,
		})
		
		// We expect this to fail, returning the error causes rollback
		return err
	})

	if err == nil {
		t.Fatalf("expected transaction to fail")
	}

	// Verify rollback
	_, err = userRepo.FindByEmail(ctx, email)
	if err != domain.ErrNotFound {
		t.Errorf("expected user to not exist due to rollback, but found or got err: %v", err)
	}
}
