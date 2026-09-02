package database_test

import (
	"context"
	"testing"
	"time"

	"carefund-api/internal/database"
	"carefund-api/internal/domain"
	"github.com/google/uuid"
)

func TestPaymentConstraints(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	payRepo := database.NewPaymentRepository(db)

	orderID := "CF-" + uuid.New().String()
	p1 := &domain.Payment{
		DonationID:  uuid.New().String(),
		Provider:    "MIDTRANS",
		OrderID:     orderID,
		GrossAmount: 10000,
		Status:      "PENDING",
	}

	// Fake donationID constraint might fail because we don't have a real donation.
	// We need to create a real user, real campaign, real donation first.
	userRepo := database.NewUserRepository(db)
	u := &domain.User{Email: "pay_test_" + uuid.New().String()[:8] + "@example.com", PasswordHash: "h", Name: "N", IsActive: true}
	userRepo.Create(ctx, u)

	catRepo := database.NewCategoryRepository(db)
	cat := &domain.Category{Name: "C", Slug: "c", IsActive: true}
	catRepo.Create(ctx, cat)

	campRepo := database.NewCampaignRepository(db)
	c := &domain.Campaign{OwnerID: u.ID, CategoryID: cat.ID, Title: "T", Description: "D", TargetAmount: 100, StartAt: time.Now(), EndAt: time.Now().Add(time.Hour), Status: "ACTIVE"}
	if err := campRepo.Create(ctx, c); err != nil {
		t.Fatalf("failed to create campaign: %v", err)
	}

	donRepo := database.NewDonationRepository(db)
	d1 := &domain.Donation{CampaignID: c.ID, DonorID: &u.ID, Amount: 10000, Status: "PENDING"}
	if err := donRepo.Create(ctx, d1); err != nil {
		t.Fatalf("failed to create donation 1: %v", err)
	}
	
	p1.DonationID = d1.ID
	if err := payRepo.Create(ctx, p1); err != nil {
		t.Fatalf("failed to create first payment: %v", err)
	}

	d2 := &domain.Donation{CampaignID: c.ID, DonorID: &u.ID, Amount: 10000, Status: "PENDING"}
	if err := donRepo.Create(ctx, d2); err != nil {
		t.Fatalf("failed to create donation 2: %v", err)
	}

	p2 := &domain.Payment{
		DonationID:  d2.ID,
		Provider:    "MIDTRANS",
		OrderID:     orderID, // duplicate order id
		GrossAmount: 10000,
		Status:      "PENDING",
	}

	err := payRepo.Create(ctx, p2)
	if err != domain.ErrDuplicate {
		t.Errorf("expected duplicate order ID error, got: %v", err)
	}
}
func TestPaymentReconciliationBoundary(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	payRepo := database.NewPaymentRepository(db)
	userRepo := database.NewUserRepository(db)
	catRepo := database.NewCategoryRepository(db)
	campRepo := database.NewCampaignRepository(db)
	donRepo := database.NewDonationRepository(db)

	u := &domain.User{Email: "bound_" + uuid.New().String()[:8] + "@example.com", PasswordHash: "h", Name: "N", IsActive: true}
	_ = userRepo.Create(ctx, u)

	cat := &domain.Category{Name: "C", Slug: "c_bound", IsActive: true}
	_ = catRepo.Create(ctx, cat)

	c := &domain.Campaign{OwnerID: u.ID, CategoryID: cat.ID, Title: "T", Description: "D", TargetAmount: 100, StartAt: time.Now(), EndAt: time.Now().Add(time.Hour), Status: "ACTIVE"}
	_ = campRepo.Create(ctx, c)

	createPayment := func(orderID string, offset time.Duration) *domain.Payment {
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
		// Manipulate created_at precisely
		_, _ = db.ExecContext(ctx, "UPDATE payments SET created_at = $1 WHERE id = $2", time.Now().Add(-offset), p.ID)
		return p
	}

	// 1. age = 44m59s
	p1 := createPayment("BOUND-1", 44*time.Minute + 59*time.Second)
	// 2. age = exactly 45m
	p2 := createPayment("BOUND-2", 45*time.Minute)
	// 3. age = 45m01s
	p3 := createPayment("BOUND-3", 45*time.Minute + 1*time.Second)

	ttl := 45 * time.Minute
	cutoffTime := time.Now().Add(-ttl)

	stalePayments, err := payRepo.FindStalePendingPayments(ctx, cutoffTime, 10)
	if err != nil {
		t.Fatalf("failed to find stale payments: %v", err)
	}

	foundP1, foundP2, foundP3 := false, false, false
	for _, p := range stalePayments {
		if p.ID == p1.ID { foundP1 = true }
		if p.ID == p2.ID { foundP2 = true }
		if p.ID == p3.ID { foundP3 = true }
	}

	if foundP1 {
		t.Errorf("Expected age 44m59s NOT to be selected")
	}
	if !foundP2 {
		t.Errorf("Expected exactly 45m to be SELECTED")
	}
	if !foundP3 {
		t.Errorf("Expected age 45m01s to be SELECTED")
	}
}
