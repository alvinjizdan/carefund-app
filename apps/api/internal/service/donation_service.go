package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"carefund-api/internal/database"
	"carefund-api/internal/domain"
	"github.com/google/uuid"
)

type DonationService interface {
	CreateDonation(ctx context.Context, donorID string, donorEmail, donorName string, campaignID string, amount int64, isAnonymous bool, message string) (*domain.Donation, *domain.Payment, *domain.PaymentCreationResult, error)
	// CreateDonationIdempotent is the concurrency-safe path for HTTP requests carrying
	// an Idempotency-Key. It atomically reserves the idempotency slot inside the same
	// PostgreSQL transaction that creates the Donation and Payment rows.
	// If the slot is already taken (ErrIdempotencyConflict), no financial records are
	// created and the caller is responsible for looking up and returning the existing result.
	CreateDonationIdempotent(ctx context.Context, donorID string, donorEmail, donorName string, campaignID string, amount int64, isAnonymous bool, message string, idempotencyRepo domain.IdempotencyRepository, idempotencyKey, requestHash string, expiresAt time.Time) (*domain.Donation, *domain.Payment, *domain.PaymentCreationResult, error)
	GetDonation(ctx context.Context, id string) (*domain.Donation, error)
	GetDonationForUser(ctx context.Context, userID string, roles []string, id string) (*domain.Donation, error)
	ListUserDonations(ctx context.Context, userID string, limit, offset int) ([]*domain.Donation, error)
	GetPayment(ctx context.Context, paymentID string) (*domain.Payment, error)
	GetPaymentForUser(ctx context.Context, userID string, roles []string, paymentID string) (*domain.Payment, error)
}

type donationService struct {
	donationRepo domain.DonationRepository
	paymentRepo  domain.PaymentRepository
	campaignRepo domain.CampaignRepository
	paymentGw    domain.PaymentGateway
	tx           database.TransactionManager
}

func NewDonationService(donationRepo domain.DonationRepository, paymentRepo domain.PaymentRepository, campaignRepo domain.CampaignRepository, paymentGw domain.PaymentGateway, tx database.TransactionManager) DonationService {
	return &donationService{
		donationRepo: donationRepo,
		paymentRepo:  paymentRepo,
		campaignRepo: campaignRepo,
		paymentGw:    paymentGw,
		tx:           tx,
	}
}

func (s *donationService) CreateDonation(ctx context.Context, donorID string, donorEmail, donorName string, campaignID string, amount int64, isAnonymous bool, message string) (*domain.Donation, *domain.Payment, *domain.PaymentCreationResult, error) {
	if amount <= 0 {
		return nil, nil, nil, domain.ErrInvalidInput
	}

	var d *domain.Donation
	var p *domain.Payment

	err := s.tx.Do(ctx, func(txCtx context.Context) error {
		c, err := s.campaignRepo.FindByID(txCtx, campaignID)
		if err != nil {
			return err
		}

		if c.Status != domain.CampaignStateActive {
			return domain.ErrInvalidStateTransition
		}

		d = &domain.Donation{
			CampaignID:  c.ID,
			DonorID:     &donorID,
			Amount:      amount,
			IsAnonymous: isAnonymous,
			Message:     message,
			Status:      domain.DonationStatusPending,
		}

		if err := s.donationRepo.Create(txCtx, d); err != nil {
			return err
		}

		orderID := fmt.Sprintf("CF-%d-%s", time.Now().UnixMilli(), strings.Split(uuid.New().String(), "-")[0])

		p = &domain.Payment{
			DonationID:  d.ID,
			Provider:    "MIDTRANS",
			OrderID:     orderID,
			GrossAmount: amount,
			Status:      domain.PaymentStatusPending,
		}

		if err := s.paymentRepo.Create(txCtx, p); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, nil, nil, err
	}

	// 2. Call Payment Gateway (OUTSIDE OF TRANSACTION)
	res, err := s.paymentGw.CreatePayment(ctx, p, d, donorEmail, donorName)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("payment provider error, status is unknown: %w", err)
	}

	return d, p, res, nil
}

// CreateDonationIdempotent atomically reserves the idempotency slot (via idempotencyRepo.Reserve)
// inside the same PostgreSQL transaction that creates Donation + Payment rows.
//
// Concurrency guarantee:
//
//	PostgreSQL's UNIQUE constraint on (user_id, idempotency_key) ensures that at most ONE
//	goroutine can INSERT the reservation row within an uncommitted transaction. All other
//	concurrent goroutines that attempt to INSERT the same key receive a unique_violation (23505)
//	error, which causes their transaction to roll back — and therefore their Donation + Payment
//	INSERT statements are also rolled back. The losing goroutines receive ErrIdempotencyConflict
//	and must return the existing result to the caller.
//
// Note: this holds for the transactional INSERT path. PostgreSQL's row-level locking
// ensures that only one transaction wins the UNIQUE constraint race.
func (s *donationService) CreateDonationIdempotent(
	ctx context.Context,
	donorID string,
	donorEmail, donorName string,
	campaignID string,
	amount int64,
	isAnonymous bool,
	message string,
	idempotencyRepo domain.IdempotencyRepository,
	idempotencyKey string,
	requestHash string,
	expiresAt time.Time,
) (*domain.Donation, *domain.Payment, *domain.PaymentCreationResult, error) {
	if amount <= 0 {
		return nil, nil, nil, domain.ErrInvalidInput
	}

	var d *domain.Donation
	var p *domain.Payment

	err := s.tx.Do(ctx, func(txCtx context.Context) error {
		orderID := fmt.Sprintf("CF-%d-%s", time.Now().UnixMilli(), strings.Split(uuid.New().String(), "-")[0])

		// STEP 1: Atomically reserve the idempotency slot FIRST, before any financial records.
		// If a concurrent request already holds the slot, Reserve returns ErrIdempotencyConflict
		// and this transaction rolls back — Donation and Payment are never created.
		// We pass orderID to create a durable link for recovery in case Complete() fails.
		if err := idempotencyRepo.Reserve(txCtx, donorID, idempotencyKey, requestHash, orderID, expiresAt); err != nil {
			return err // propagates ErrIdempotencyConflict up through tx.Do → rollback
		}

		// STEP 2: Financial records — only reached if reservation succeeded.
		c, err := s.campaignRepo.FindByID(txCtx, campaignID)
		if err != nil {
			return err
		}

		if c.Status != domain.CampaignStateActive {
			return domain.ErrInvalidStateTransition
		}

		d = &domain.Donation{
			CampaignID:  c.ID,
			DonorID:     &donorID,
			Amount:      amount,
			IsAnonymous: isAnonymous,
			Message:     message,
			Status:      domain.DonationStatusPending,
		}

		if err := s.donationRepo.Create(txCtx, d); err != nil {
			return err
		}

		p = &domain.Payment{
			DonationID:  d.ID,
			Provider:    "MIDTRANS",
			OrderID:     orderID,
			GrossAmount: amount,
			Status:      domain.PaymentStatusPending,
		}

		if err := s.paymentRepo.Create(txCtx, p); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, nil, nil, err
	}

	// STEP 3: Call Payment Gateway OUTSIDE the transaction.
	// The idempotency record is PENDING at this point.
	res, err := s.paymentGw.CreatePayment(ctx, p, d, donorEmail, donorName)
	if err != nil {
		// Ambiguous timeout / network failure: leave idempotency record PENDING.
		// The next retry will find the PENDING record and return the appropriate state.
		// Reconciliation will determine the actual Midtrans state later.
		return nil, nil, nil, fmt.Errorf("payment provider error, status is unknown: %w", err)
	}

	return d, p, res, nil
}


func (s *donationService) GetDonation(ctx context.Context, id string) (*domain.Donation, error) {
	return s.donationRepo.FindByID(ctx, id)
}

func (s *donationService) GetDonationForUser(ctx context.Context, userID string, roles []string, id string) (*domain.Donation, error) {
	donation, err := s.donationRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 1. Admin bypass
	for _, r := range roles {
		if r == "ADMIN" {
			return donation, nil
		}
	}

	// 2. Donor ownership
	if donation.DonorID != nil && *donation.DonorID == userID {
		return donation, nil
	}

	// 3. Campaign owner ownership
	campaign, err := s.campaignRepo.FindByID(ctx, donation.CampaignID)
	if err == nil && campaign.OwnerID == userID {
		return donation, nil
	}

	return nil, domain.ErrForbidden
}

func (s *donationService) ListUserDonations(ctx context.Context, userID string, limit, offset int) ([]*domain.Donation, error) {
	return s.donationRepo.ListByUser(ctx, userID, limit, offset)
}

func (s *donationService) GetPayment(ctx context.Context, id string) (*domain.Payment, error) {
	return s.paymentRepo.FindByID(ctx, id)
}

func (s *donationService) GetPaymentForUser(ctx context.Context, userID string, roles []string, paymentID string) (*domain.Payment, error) {
	payment, err := s.paymentRepo.FindByID(ctx, paymentID)
	if err != nil {
		return nil, err
	}

	donation, err := s.donationRepo.FindByID(ctx, payment.DonationID)
	if err != nil {
		return nil, err
	}

	// 1. Admin bypass
	for _, r := range roles {
		if r == "ADMIN" {
			return payment, nil
		}
	}

	// 2. Donor ownership
	if donation.DonorID != nil && *donation.DonorID == userID {
		return payment, nil
	}

	// 3. Campaign owner ownership
	campaign, err := s.campaignRepo.FindByID(ctx, donation.CampaignID)
	if err == nil && campaign.OwnerID == userID {
		return payment, nil
	}

	return nil, domain.ErrForbidden
}
