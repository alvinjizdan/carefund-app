package midtrans_test

import (
	"context"
	"testing"
	"time"

	"carefund-api/internal/config"
	"carefund-api/internal/domain"
	"carefund-api/internal/infrastructure/payment/midtrans"
)

func TestMockGateway(t *testing.T) {
	mock := midtrans.NewMockPaymentGateway()
	
	p := &domain.Payment{OrderID: "ORDER-123", GrossAmount: 50000}
	d := &domain.Donation{Amount: 50000}
	
	res, err := mock.CreatePayment(context.Background(), p, d, "test@test.com", "Tester")
	if err != nil {
		t.Fatalf("expected success from mock, got %v", err)
	}
	if res.PaymentToken != "mock-snap-token-123" {
		t.Errorf("unexpected token: %s", res.PaymentToken)
	}

	if res.RedirectURL != "https://app.sandbox.midtrans.com/snap/v2/vtweb/mock-snap-token-123" {
		t.Errorf("unexpected redirect url: %s", res.RedirectURL)
	}

	mock.ShouldFail = true
	_, err = mock.CreatePayment(context.Background(), p, d, "test@test.com", "Tester")
	if err == nil {
		t.Errorf("expected failure when ShouldFail=true")
	}
}

func TestMidtransGatewayArchitecture(t *testing.T) {
	// Simple init test, do not execute real network call
	cfg := &config.Config{
		MidtransServerKey: "test-server-key",
		MidtransEnvironment: "sandbox",
	}
	
	gw := midtrans.NewGateway(cfg)
	if gw == nil {
		t.Fatalf("expected gateway to initialize")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// If we attempt a call with expired context or fake key, it should fail gracefully
	p := &domain.Payment{OrderID: "ORDER-TEST-001", GrossAmount: 50000}
	d := &domain.Donation{Amount: 50000}

	_, err := gw.CreatePayment(ctx, p, d, "test@test.com", "Tester")
	if err == nil {
		t.Errorf("expected error from Midtrans due to fake key/timeout")
	}
}
