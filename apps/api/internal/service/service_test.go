package service_test

import (
	"context"
	"testing"
	"time"

	"carefund-api/internal/config"
	"carefund-api/internal/database"
	"carefund-api/internal/domain"
	"carefund-api/internal/service"
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

	// Clean up tables
	tables := []string{"outbox_events", "settlement_items", "settlements", "payment_events", "refunds", "payments", "donations", "campaigns", "categories", "user_roles", "roles", "users"}
	for _, table := range tables {
		_, err := db.ExecContext(context.Background(), "DELETE FROM "+table)
		if err != nil {
			t.Logf("failed to clean table %s: %v", table, err)
		}
	}

	// Insert standard roles
	_, _ = db.ExecContext(context.Background(), "INSERT INTO roles (name) VALUES ('DONOR'), ('CAMPAIGN_OWNER'), ('ADMIN')")

	return db
}

func TestAuthService(t *testing.T) {
	authSvc := service.NewAuthService("test-secret-key", 15*time.Minute)

	hash, err := authSvc.HashPassword("strong-password")
	if err != nil {
		t.Fatalf("hashing failed: %v", err)
	}

	if !authSvc.VerifyPassword("strong-password", hash) {
		t.Errorf("expected password to verify successfully")
	}

	if authSvc.VerifyPassword("wrong-password", hash) {
		t.Errorf("expected wrong password to fail verification")
	}
}

func TestUserRegistrationAndLogin(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	txManager := database.NewTransactionManager(db)
	userRepo := database.NewUserRepository(db)
	roleRepo := database.NewRoleRepository(db)
	authSvc := service.NewAuthService("secret", 15*time.Minute)
	userSvc := service.NewUserService(userRepo, roleRepo, authSvc, txManager)

	email := "test_" + time.Now().Format("150405.000") + "@example.com"
	user, err := userSvc.RegisterUser(ctx, email, "password123", "Test User")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	// 2. Login valid
	loggedInUser, roles, err := userSvc.Login(ctx, email, "password123")
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}
	if loggedInUser.ID != user.ID {
		t.Errorf("login returned wrong user")
	}

	hasDonor := false
	for _, r := range roles {
		if r == "DONOR" {
			hasDonor = true
		}
	}
	if !hasDonor {
		t.Errorf("expected user to have DONOR role")
	}

	// 3. Login invalid password
	_, _, err = userSvc.Login(ctx, email, "wrong")
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound/Unauthorized for bad password, got %v", err)
	}
}

func TestCampaignLifecycle(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	txManager := database.NewTransactionManager(db)
	campRepo := database.NewCampaignRepository(db)
	catRepo := database.NewCategoryRepository(db)
	userRepo := database.NewUserRepository(db)

	campSvc := service.NewCampaignService(campRepo, txManager)

	// Setup owner and category
	owner := &domain.User{Email: "owner@campaign.com", PasswordHash: "h", Name: "O", IsActive: true}
	_ = userRepo.Create(ctx, owner)
	admin := &domain.User{Email: "admin@campaign.com", PasswordHash: "h", Name: "A", IsActive: true}
	_ = userRepo.Create(ctx, admin)
	cat := &domain.Category{Name: "Edu", Slug: "edu", IsActive: true}
	_ = catRepo.Create(ctx, cat)

	now := time.Now().Round(time.Microsecond)

	// Create Campaign
	camp, err := campSvc.CreateCampaign(ctx, owner.ID, cat.ID, "Title", "Desc", 1000, now, now.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("failed to create: %v", err)
	}

	// 1. Unauthorized Update (by admin ID)
	_, err = campSvc.UpdateCampaign(ctx, admin.ID, camp.ID, "New Title", "Desc", cat.ID, 2000, now, now.Add(24*time.Hour))
	if err != domain.ErrForbidden {
		t.Errorf("expected ErrForbidden when non-owner updates, got %v", err)
	}

	// 2. Submit for review (Owner)
	err = campSvc.SubmitForReview(ctx, owner.ID, camp.ID)
	if err != nil {
		t.Fatalf("failed to submit for review: %v", err)
	}

	// 3. Approve (Admin)
	err = campSvc.ApproveCampaign(ctx, admin.ID, camp.ID)
	if err != nil {
		t.Fatalf("failed to approve: %v", err)
	}

	// 4. Reject (Admin) - Should fail because it's already active
	err = campSvc.RejectCampaign(ctx, admin.ID, camp.ID, "Bad")
	if err != domain.ErrInvalidStateTransition {
		t.Errorf("expected state transition error rejecting active campaign, got %v", err)
	}
}
