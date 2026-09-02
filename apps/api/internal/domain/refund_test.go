package domain_test

import (
	"testing"

	"carefund-api/internal/domain"
)

func TestRefundValidation(t *testing.T) {
	refund := &domain.Refund{
		PaymentID: "test-payment", IdempotencyKey: "test-idem",
		Amount: 100,
		Reason: "Customer request",
	}
	if err := refund.Validate(); err != nil {
		t.Errorf("Expected valid, got %v", err)
	}

	invalidRefund := &domain.Refund{
		PaymentID: "test",
		Amount:    -10, // Invalid
		Reason:    "test",
	}
	if err := invalidRefund.Validate(); err == nil {
		t.Errorf("Expected error for negative amount")
	}

	invalidRefund2 := &domain.Refund{
		PaymentID: "test",
		Amount:    0, // Invalid
		Reason:    "test",
	}
	if err := invalidRefund2.Validate(); err == nil {
		t.Errorf("Expected error for zero amount")
	}
}

func TestRefundStateTransitions(t *testing.T) {
	r := &domain.Refund{Status: domain.RefundStatusPending}
	if !r.IsValidTransition(domain.RefundStatusCompleted) {
		t.Error("PENDING -> COMPLETED should be valid")
	}
	if !r.IsValidTransition(domain.RefundStatusFailed) {
		t.Error("PENDING -> FAILED should be valid")
	}

	r.Status = domain.RefundStatusCompleted
	if r.IsValidTransition(domain.RefundStatusPending) {
		t.Error("COMPLETED -> PENDING should be invalid")
	}
}

func TestRefundEligibility(t *testing.T) {
	validStates := []string{
		domain.PaymentStatusCaptured,
		domain.PaymentStatusSettled,
		domain.PaymentStatusPartiallyRefunded,
	}
	for _, st := range validStates {
		p := &domain.Payment{Status: st}
		if !domain.IsPaymentEligibleForRefund(p) {
			t.Errorf("State %s should be eligible", st)
		}
	}

	invalidStates := []string{
		domain.PaymentStatusPending,
		domain.PaymentStatusAuthorized,
		domain.PaymentStatusFailed,
		domain.PaymentStatusExpired,
		domain.PaymentStatusCancelled,
		domain.PaymentStatusRefunded,
	}
	for _, st := range invalidStates {
		p := &domain.Payment{Status: st}
		if domain.IsPaymentEligibleForRefund(p) {
			t.Errorf("State %s should NOT be eligible", st)
		}
	}
}

func TestCalculateRefundableAmount(t *testing.T) {
	p := &domain.Payment{GrossAmount: 100000}

	if amt := domain.CalculateRefundableAmount(p, 0); amt != 100000 {
		t.Errorf("Expected 100000, got %d", amt)
	}

	if amt := domain.CalculateRefundableAmount(p, 30000); amt != 70000 {
		t.Errorf("Expected 70000, got %d", amt)
	}

	if amt := domain.CalculateRefundableAmount(p, 100000); amt != 0 {
		t.Errorf("Expected 0, got %d", amt)
	}

	if amt := domain.CalculateRefundableAmount(p, 120000); amt != 0 {
		t.Errorf("Expected 0, got %d (should not be negative)", amt)
	}
}

func TestMapMidtransRefundStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"refund", domain.RefundStatusCompleted},
		{"partial_refund", domain.RefundStatusCompleted},
		{"success", domain.RefundStatusCompleted},
		{"settlement", domain.RefundStatusCompleted},
		{"deny", domain.RefundStatusFailed},
		{"rejected", domain.RefundStatusFailed},
		{"failure", domain.RefundStatusFailed},
		{"cancel", domain.RefundStatusCancelled},
		{"pending", domain.RefundStatusPending},
		{"unknown", domain.RefundStatusPending},
	}

	for _, tt := range tests {
		got := domain.MapMidtransRefundStatus(tt.input)
		if got != tt.expected {
			t.Errorf("MapMidtransRefundStatus(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}
