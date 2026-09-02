package domain

import (
	"context"
)

// PaymentCreationResult represents the application-level DTO for a payment creation intent.
type PaymentCreationResult struct {
	ProviderReference string
	PaymentToken      string // e.g. Midtrans Snap token
	RedirectURL       string
}

// PaymentStatusResult holds the provider status information
type PaymentStatusResult struct {
	OrderID        string
	TransactionID  string
	GrossAmount    int64
	ProviderStatus string
	FraudStatus    string
	RawPayload     string
}

// RefundRequest represents neutral refund input for PaymentGateway.
type RefundRequest struct {
	OrderID        string
	RefundID       string
	IdempotencyKey string
	Amount         int64
	Reason         string
}

// RefundResult represents neutral provider refund response.
type RefundResult struct {
	RefundID         string
	ProviderRefundID string
	ProviderStatus   string
	IsAccepted       bool
	IsCompleted      bool
	RawPayload       string
}

// PaymentGateway is the provider-agnostic interface for interacting with payment gateways.
type PaymentGateway interface {
	// CreatePayment creates a payment intent with the provider.
	CreatePayment(ctx context.Context, p *Payment, d *Donation, customerEmail string, customerName string) (*PaymentCreationResult, error)

	// GetPaymentStatus retrieves the current status from the provider.
	GetPaymentStatus(ctx context.Context, orderID string) (*PaymentStatusResult, error)

	// RefundPayment executes a refund request with the provider.
	RefundPayment(ctx context.Context, req *RefundRequest) (*RefundResult, error)
}
