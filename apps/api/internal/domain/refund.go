package domain

import (
	"context"
	"time"
)

const (
	RefundStatusPending   = "PENDING"
	RefundStatusCompleted = "COMPLETED"
	RefundStatusFailed    = "FAILED"
	RefundStatusCancelled = "CANCELLED"
)

type Refund struct {
	ID               string
	PaymentID        string
	IdempotencyKey   string
	Amount           int64
	ProviderRefundID *string
	Status           string
	Reason           string
	RequestedAt      time.Time
	CompletedAt      *time.Time
}

// Validate checks basic structure invariants.
func (r *Refund) Validate() error {
	if r.Amount <= 0 {
		return ErrInvalidInput
	}
	if r.IdempotencyKey == "" {
		return ErrInvalidInput
	}
	if r.PaymentID == "" {
		return ErrInvalidInput
	}
	if r.Reason == "" {
		return ErrInvalidInput
	}
	return nil
}

// IsValidTransition checks refund state machine.
func (r *Refund) IsValidTransition(nextState string) bool {
	if r.Status == nextState {
		return true // Idempotency
	}
	switch r.Status {
	case RefundStatusPending:
		return nextState == RefundStatusCompleted ||
			nextState == RefundStatusFailed ||
			nextState == RefundStatusCancelled
	case RefundStatusCompleted, RefundStatusFailed, RefundStatusCancelled:
		// Terminal states
		return false
	default:
		return false
	}
}

// IsPaymentEligibleForRefund validates if payment state permits refund operations.
func IsPaymentEligibleForRefund(p *Payment) bool {
	return p.Status == PaymentStatusCaptured ||
		p.Status == PaymentStatusSettled ||
		p.Status == PaymentStatusPartiallyRefunded
}

// CalculateRefundableAmount determines how much can still be refunded.
func CalculateRefundableAmount(p *Payment, requestedOrCompletedRefundsAmount int64) int64 {
	refundable := p.GrossAmount - requestedOrCompletedRefundsAmount
	if refundable < 0 {
		return 0
	}
	return refundable
}

// MapMidtransRefundStatus maps Midtrans-specific refund/transaction status to CareFund Refund domain state.
func MapMidtransRefundStatus(status string) string {
	switch status {
	case "refund", "partial_refund", "success", "settlement":
		return RefundStatusCompleted
	case "deny", "rejected", "failure":
		return RefundStatusFailed
	case "cancel":
		return RefundStatusCancelled
	case "pending":
		return RefundStatusPending
	default:
		return RefundStatusPending
	}
}

type RefundRepository interface {
	Create(ctx context.Context, refund *Refund) error
	FindByID(ctx context.Context, id string) (*Refund, error)
	FindByIDForUpdate(ctx context.Context, id string) (*Refund, error)
	ListByPayment(ctx context.Context, paymentID string) ([]*Refund, error)
	SumActiveRefunds(ctx context.Context, paymentID string) (int64, error)
	SumCompletedRefunds(ctx context.Context, paymentID string) (int64, error)
	Update(ctx context.Context, refund *Refund) error
}
