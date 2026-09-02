package domain

import (
	"context"
	"time"
)

const (
	PaymentStatusPending           = "PENDING"
	PaymentStatusAuthorized        = "AUTHORIZED"
	PaymentStatusCaptured          = "CAPTURED"
	PaymentStatusSettled           = "SETTLED"
	PaymentStatusFailed            = "FAILED"
	PaymentStatusExpired           = "EXPIRED"
	PaymentStatusCancelled         = "CANCELLED"
	PaymentStatusRefunded          = "REFUNDED"
	PaymentStatusPartiallyRefunded = "PARTIALLY_REFUNDED"
)

type Payment struct {
	ID            string
	DonationID    string
	Provider      string
	OrderID       string
	TransactionID *string
	PaymentType   *string
	GrossAmount   int64
	Status        string
	FraudStatus   *string
	TransactionAt *time.Time
	SettledAt     *time.Time
	ExpiredAt     *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (p *Payment) Validate() error {
	if p.GrossAmount <= 0 {
		return ErrInvalidInput
	}
	if p.DonationID == "" || p.Provider == "" || p.OrderID == "" {
		return ErrInvalidInput
	}
	return nil
}

func (p *Payment) IsValidTransition(nextState string) bool {
	// Simple state machine checking based on business rules
	// Only allow progression
	if p.Status == nextState {
		return true // Idempotency allowance
	}

	switch p.Status {
	case PaymentStatusPending:
		// from pending it can be authorized, captured, failed, expired, cancelled
		return nextState == PaymentStatusAuthorized || 
		       nextState == PaymentStatusCaptured || 
		       nextState == PaymentStatusFailed || 
		       nextState == PaymentStatusExpired || 
		       nextState == PaymentStatusCancelled
	case PaymentStatusAuthorized:
		return nextState == PaymentStatusCaptured || 
		       nextState == PaymentStatusFailed || 
		       nextState == PaymentStatusCancelled
	case PaymentStatusCaptured:
		// Funds captured, next is settled or refunded
		return nextState == PaymentStatusSettled || 
		       nextState == PaymentStatusRefunded || 
		       nextState == PaymentStatusPartiallyRefunded
	case PaymentStatusSettled:
		// Already settled. It can be refunded.
		return nextState == PaymentStatusRefunded || 
		       nextState == PaymentStatusPartiallyRefunded
	case PaymentStatusPartiallyRefunded:
		// Can progress to fully refunded
		return nextState == PaymentStatusRefunded
	case PaymentStatusFailed, PaymentStatusExpired, PaymentStatusCancelled, PaymentStatusRefunded:
		// Terminal states
		return false
	default:
		return false
	}
}

type PaymentRepository interface {
	Create(ctx context.Context, payment *Payment) error
	FindByID(ctx context.Context, id string) (*Payment, error)
	FindByIDForUpdate(ctx context.Context, id string) (*Payment, error)
	FindByOrderID(ctx context.Context, orderID string) (*Payment, error)
	FindByOrderIDForUpdate(ctx context.Context, orderID string) (*Payment, error)
	UpdateState(ctx context.Context, id string, status string) error
	Update(ctx context.Context, payment *Payment) error
	FindStalePendingPayments(ctx context.Context, cutoffTime time.Time, limit int) ([]*Payment, error)
	FindEligibleForSettlement(ctx context.Context, campaignID string) ([]*Payment, error)
}

type PaymentEvent struct {
	ID               string
	PaymentID        *string
	Provider         string
	EventSource      string
	IdempotencyKey   string
	EventType        string
	ProviderStatus   string
	Payload          string // stored as JSONB
	ProcessingStatus string
	RejectionReason  *string
	ReceivedAt       time.Time
	ProcessedAt      *time.Time
}

const (
	PaymentEventProcessingStatusReceived  = "RECEIVED"
	PaymentEventProcessingStatusProcessed = "PROCESSED"
	PaymentEventProcessingStatusRejected  = "REJECTED"

	RejectionReasonInvalidStateTransition  = "INVALID_STATE_TRANSITION"
	RejectionReasonAmountMismatch          = "AMOUNT_MISMATCH"
	RejectionReasonPaymentNotFound         = "PAYMENT_NOT_FOUND"
	RejectionReasonInvalidEvent            = "INVALID_EVENT"
	RejectionReasonUnsupportedProviderStatus = "UNSUPPORTED_PROVIDER_STATUS"
)

type PaymentEventRepository interface {
	Create(ctx context.Context, event *PaymentEvent) error
	MarkProcessed(ctx context.Context, id string) error
	MarkRejected(ctx context.Context, id string, reason string) error
}
