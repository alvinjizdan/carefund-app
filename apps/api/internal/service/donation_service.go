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
	// Passing customer info
	res, err := s.paymentGw.CreatePayment(ctx, p, d, donorEmail, donorName)
	if err != nil {
		// CRITICAL CORRECTION:
		// A Midtrans network timeout or ambiguous provider error MUST NOT automatically transition Payment -> FAILED
		// A timeout means the provider outcome may be UNKNOWN.
		// The local system MUST NOT assume payment creation failed.
		// Therefore, remain PENDING. Reconciliation will resolve the actual provider state later.
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
