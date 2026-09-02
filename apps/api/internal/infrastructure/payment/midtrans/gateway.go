package midtrans

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"

	"carefund-api/internal/config"
	"carefund-api/internal/domain"

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
	snapResp, err := g.snapClient.CreateTransaction(req)
	if err != nil {
		log.Printf("[Midtrans Error] Failed to create Snap transaction: %v", err)
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
	resp, err := g.coreClient.CheckTransaction(orderID)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return nil, errors.New("transaction not found")
		}
		log.Printf("[Midtrans Error] CheckTransaction failed for OrderID %s: %v", orderID, err)
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

	resp, err := g.coreClient.DirectRefundTransaction(req.OrderID, midtransReq)
	if err != nil {
		errStr := strings.ToLower(err.Error())
		log.Printf("[Midtrans Error] RefundTransaction failed for OrderID %s, RefundKey %s: %s", req.OrderID, req.IdempotencyKey, err.Message)

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
