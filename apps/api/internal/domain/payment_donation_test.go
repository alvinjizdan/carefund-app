package domain_test

import (
	"testing"
	"carefund-api/internal/domain"
)

func TestDonationValidation(t *testing.T) {
	d1 := &domain.Donation{Amount: 50000, CampaignID: "camp-123"}
	if err := d1.Validate(); err != nil {
		t.Errorf("expected valid donation, got %v", err)
	}

	d2 := &domain.Donation{Amount: 0, CampaignID: "camp-123"}
	if err := d2.Validate(); err == nil {
		t.Errorf("expected error for zero amount")
	}

	d3 := &domain.Donation{Amount: 50000, CampaignID: ""}
	if err := d3.Validate(); err == nil {
		t.Errorf("expected error for empty campaign")
	}
}

func TestPaymentStateMachine(t *testing.T) {
	p := &domain.Payment{Status: domain.PaymentStatusPending}
	
	// Valid transitions
	if !p.IsValidTransition(domain.PaymentStatusCaptured) {
		t.Errorf("expected PENDING -> CAPTURED to be valid")
	}
	
	// Invalid transition
	if p.IsValidTransition(domain.PaymentStatusSettled) {
		t.Errorf("expected PENDING -> SETTLED to be invalid directly")
	}

	p.Status = domain.PaymentStatusCaptured
	if !p.IsValidTransition(domain.PaymentStatusSettled) {
		t.Errorf("expected CAPTURED -> SETTLED to be valid")
	}
	if p.IsValidTransition(domain.PaymentStatusPending) {
		t.Errorf("expected CAPTURED -> PENDING to be invalid")
	}

	// Financial safety rule: successful does not imply settlement
	if p.Status == domain.PaymentStatusSettled {
		t.Errorf("payment capture should not auto-assign settlement")
	}
}
