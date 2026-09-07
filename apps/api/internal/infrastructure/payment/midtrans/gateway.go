package midtrans

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"carefund-api/internal/config"
	"carefund-api/internal/domain"
	"carefund-api/internal/logger"
	"carefund-api/internal/metrics"

	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
	"github.com/midtrans/midtrans-go/snap"
)

type Gateway struct {
	snapClient snap.Client
	coreClient coreapi.Client
	env        midtrans.EnvironmentType
}

func NewGateway(cfg *config.Config) *Gateway {
	var env midtrans.EnvironmentType
	if cfg.MidtransEnvironment == "production" {
		env = midtrans.Production
	} else {
		env = midtrans.Sandbox
	}

	sClient := snap.Client{}
	sClient.New(cfg.MidtransServerKey, env)

	cClient := coreapi.Client{}
	cClient.New(cfg.MidtransServerKey, env)

	return &Gateway{
		snapClient: sClient,
		coreClient: cClient,
		env:        env,
	}
}

func classifyError(err error) string {
	if err == nil {
		return "success"
	}
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		return "timeout"
	}
	if strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "no such host") || strings.Contains(errStr, "network") {
		return "network_error"
	}
	if strings.Contains(errStr, "412") || strings.Contains(errStr, "400") || strings.Contains(errStr, "deny") || strings.Contains(errStr, "cannot be refunded") || strings.Contains(errStr, "invalid transaction status") {
		return "rejected"
	}
	return "provider_error"
}

func (g *Gateway) CreatePayment(ctx context.Context, p *domain.Payment, d *domain.Donation, customerEmail string, customerName string) (*domain.PaymentCreationResult, error) {
	// Map to Midtrans Snap request
	req := &snap.Request{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  p.OrderID,
			GrossAmt: p.GrossAmount,
		},
		CustomerDetail: &midtrans.CustomerDetails{
			FName: customerName,
			Email: customerEmail,
		},
	}

	// Call Midtrans SDK
	start := time.Now()
	snapResp, err := g.snapClient.CreateTransaction(req)
	duration := time.Since(start)
	metrics.RecordProviderRequest("MIDTRANS", "create_transaction", classifyError(err), duration)

	if err != nil {
		logger.Error(ctx, "Failed to create Snap transaction", err,
			logger.F("component", "MidtransGateway"),
			logger.F("order_id", p.OrderID),
		)
		return nil, errors.New("failed to initialize payment gateway")
	}

	if snapResp == nil || snapResp.Token == "" {
		return nil, errors.New("invalid response from payment gateway")
	}

	return &domain.PaymentCreationResult{
		ProviderReference: p.OrderID, 
		PaymentToken:      snapResp.Token,
		RedirectURL:       snapResp.RedirectURL,
	}, nil
}

func (g *Gateway) GetPaymentStatus(ctx context.Context, orderID string) (*domain.PaymentStatusResult, error) {
	start := time.Now()
	resp, err := g.coreClient.CheckTransaction(orderID)
	duration := time.Since(start)
	metrics.RecordProviderRequest("MIDTRANS", "check_transaction", classifyError(err), duration)

	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return nil, errors.New("transaction not found")
		}
		logger.Error(ctx, "CheckTransaction failed", err,
			logger.F("component", "MidtransGateway"),
			logger.F("order_id", orderID),
		)
		return nil, errors.New("failed to retrieve payment status from provider")
	}

	if resp == nil {
		return nil, errors.New("empty response from provider")
	}

	if resp.StatusCode == "404" {
		return nil, errors.New("transaction not found")
	}

	grossAmountStr := strings.Split(resp.GrossAmount, ".")[0]
	grossAmount, _ := strconv.ParseInt(grossAmountStr, 10, 64)

	rawPayloadBytes, _ := json.Marshal(resp)

	return &domain.PaymentStatusResult{
		OrderID:        resp.OrderID,
		TransactionID:  resp.TransactionID,
		GrossAmount:    grossAmount,
		ProviderStatus: resp.TransactionStatus,
		FraudStatus:    resp.FraudStatus,
		RawPayload:     string(rawPayloadBytes),
	}, nil
}

func (g *Gateway) RefundPayment(ctx context.Context, req *domain.RefundRequest) (*domain.RefundResult, error) {
	midtransReq := &coreapi.RefundReq{
		RefundKey: req.IdempotencyKey,
		Amount:    req.Amount,
		Reason:    req.Reason,
	}

	start := time.Now()
	resp, err := g.coreClient.DirectRefundTransaction(req.OrderID, midtransReq)
	duration := time.Since(start)
	metrics.RecordProviderRequest("MIDTRANS", "direct_refund", classifyError(err), duration)
	if err != nil {
		errStr := strings.ToLower(err.Error())
		logger.Warn(ctx, "RefundTransaction provider error",
			logger.F("component", "MidtransGateway"),
			logger.F("order_id", req.OrderID),
			logger.F("refund_id", req.RefundID),
			logger.F("idempotency_key", req.IdempotencyKey),
			logger.F("err", err.Message),
		)

		if strings.Contains(errStr, "412") || strings.Contains(errStr, "400") ||
			strings.Contains(errStr, "cannot be refunded") || strings.Contains(errStr, "invalid transaction status") ||
			strings.Contains(errStr, "deny") {
			return nil, &domain.ProviderRejectionError{Message: "provider rejected refund request"}
		}

		return nil, errors.New("failed to process refund with payment gateway")
	}

	if resp == nil {
		return nil, errors.New("empty response from payment gateway")
	}

	rawPayloadBytes, _ := json.Marshal(resp)

	providerRefundID := resp.RefundChargebackUUID
	if providerRefundID == "" && resp.ID != "" {
		providerRefundID = resp.ID
	}
	if providerRefundID == "" {
		providerRefundID = resp.RefundKey
	}

	isCompleted := (resp.StatusCode == "200" || resp.StatusCode == "201") &&
		(resp.TransactionStatus == "refund" || resp.TransactionStatus == "partial_refund")

	return &domain.RefundResult{
		RefundID:         req.RefundID,
		ProviderRefundID: providerRefundID,
		ProviderStatus:   resp.TransactionStatus,
		IsAccepted:       true,
		IsCompleted:      isCompleted,
		RawPayload:       string(rawPayloadBytes),
	}, nil
}
