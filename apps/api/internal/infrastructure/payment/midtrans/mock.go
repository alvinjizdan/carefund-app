package midtrans

import (
	"context"
	"errors"
	"strings"

	"carefund-api/internal/domain"
)

// MockPaymentGateway implements a fake payment gateway for testing.
type MockPaymentGateway struct {
	ShouldFail        bool
	ShouldTimeout     bool
	ShouldReject      bool
	ShouldStayPending bool
	MockToken         string
	MockRedirectURL   string
	RefundCallCount   int
	LastRefundRequest *domain.RefundRequest
}

func NewMockPaymentGateway() *MockPaymentGateway {
	return &MockPaymentGateway{
		MockToken:       "mock-snap-token-123",
		MockRedirectURL: "https://app.sandbox.midtrans.com/snap/v2/vtweb/mock-snap-token-123",
	}
}

func (m *MockPaymentGateway) CreatePayment(ctx context.Context, p *domain.Payment, d *domain.Donation, customerEmail string, customerName string) (*domain.PaymentCreationResult, error) {
	if m.ShouldFail {
		return nil, errors.New("mock gateway error")
	}

	token := "mock_token_" + p.OrderID
	if m.MockToken != "" {
		token = m.MockToken
	}

	url := "https://mock.gateway/pay/" + p.OrderID
	if m.MockRedirectURL != "" {
		url = m.MockRedirectURL
	}

	return &domain.PaymentCreationResult{
		ProviderReference: p.OrderID,
		PaymentToken:      token,
		RedirectURL:       url,
	}, nil
}

func (m *MockPaymentGateway) GetPaymentStatus(ctx context.Context, orderID string) (*domain.PaymentStatusResult, error) {
	if m.ShouldFail {
		return nil, errors.New("mock gateway network error")
	}

	if strings.HasPrefix(orderID, "ORDER-NOTFOUND") {
		return nil, errors.New("transaction not found")
	}

	return &domain.PaymentStatusResult{
		OrderID:        orderID,
		TransactionID:  "mock_tx_" + orderID,
		GrossAmount:    10000,
		ProviderStatus: "settlement",
		FraudStatus:    "accept",
		RawPayload:     `{"status":"settlement"}`,
	}, nil
}

func (m *MockPaymentGateway) RefundPayment(ctx context.Context, req *domain.RefundRequest) (*domain.RefundResult, error) {
	m.RefundCallCount++
	m.LastRefundRequest = req

	if m.ShouldTimeout {
		return nil, errors.New("network timeout")
	}
	if m.ShouldFail {
		return nil, errors.New("mock gateway network error")
	}
	if m.ShouldReject {
		return nil, &domain.ProviderRejectionError{Message: "provider rejected refund request"}
	}

	if m.ShouldStayPending {
		return &domain.RefundResult{
			RefundID:         req.RefundID,
			ProviderRefundID: "mock_provider_pending_" + req.RefundID,
			ProviderStatus:   "pending",
			IsAccepted:       true,
			IsCompleted:      false,
			RawPayload:       `{"status_code":"200","transaction_status":"pending"}`,
		}, nil
	}

	return &domain.RefundResult{
		RefundID:         req.RefundID,
		ProviderRefundID: "mock_provider_refund_" + req.RefundID,
		ProviderStatus:   "refund",
		IsAccepted:       true,
		IsCompleted:      true,
		RawPayload:       `{"status_code":"200","transaction_status":"refund"}`,
	}, nil
}
