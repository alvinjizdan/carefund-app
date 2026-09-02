package api

import (
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"carefund-api/internal/config"
	"carefund-api/internal/domain"
)

type WebhookHandler struct {
	webhookSvc domain.WebhookService
	cfg        *config.Config
}

func NewWebhookHandler(webhookSvc domain.WebhookService, cfg *config.Config) *WebhookHandler {
	return &WebhookHandler{
		webhookSvc: webhookSvc,
		cfg:        cfg,
	}
}

func (h *WebhookHandler) MidtransNotification(w http.ResponseWriter, r *http.Request) {
	if h.cfg == nil || (h.cfg.Env != "test" && h.cfg.MidtransServerKey == "") {
		http.Error(w, "Webhook Security Misconfigured", http.StatusInternalServerError)
		return
	}

	// Read raw body for persistence (capped at 1MB)
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Extract fields securely
	orderID, _ := payload["order_id"].(string)
	statusCode, _ := payload["status_code"].(string)
	grossAmountStr, _ := payload["gross_amount"].(string)
	signatureKey, _ := payload["signature_key"].(string)
	transactionID, _ := payload["transaction_id"].(string)
	transactionStatus, _ := payload["transaction_status"].(string)
	fraudStatus, _ := payload["fraud_status"].(string)

	// Validate Signature
	if orderID == "" || statusCode == "" || grossAmountStr == "" || signatureKey == "" {
		http.Error(w, "Invalid Payload", http.StatusBadRequest)
		return
	}

	hashInput := orderID + statusCode + grossAmountStr + h.cfg.MidtransServerKey
	hasher := sha512.New()
	hasher.Write([]byte(hashInput))
	expectedSignature := hex.EncodeToString(hasher.Sum(nil))

	if subtle.ConstantTimeCompare([]byte(expectedSignature), []byte(signatureKey)) != 1 {
		// Constant-time signature comparison failed
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse gross amount correctly (Midtrans sends like "50000.00")
	// We only deal in integers. So split by "." and parse.
	grossAmountStr = strings.Split(grossAmountStr, ".")[0]
	grossAmount, err := strconv.ParseInt(grossAmountStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid Amount", http.StatusBadRequest)
		return
	}

	// Generate Idempotency Key - Midtrans can send multiple hooks. 
	// The transaction ID + status + status code represents a unique state progression.
	idempotencyKey := "midtrans_" + transactionID + "_" + transactionStatus + "_" + statusCode

	notif := &domain.WebhookNotification{
		Provider:        "MIDTRANS",
		EventSource:     "WEBHOOK",
		ProviderEventID: transactionID,
		OrderID:         orderID,
		TransactionID:   transactionID,
		GrossAmount:     grossAmount,
		ProviderStatus:  transactionStatus,
		FraudStatus:     fraudStatus,
		RawPayload:      string(body),
		IdempotencyKey:  idempotencyKey,
	}

	err = h.webhookSvc.ProcessNotification(r.Context(), notif)
	if err != nil {
		// If it's a domain validation error (e.g. invalid state transition, mismatch amount), 
		// we should still return 200 to Midtrans so it stops retrying, or maybe 400 depending on strategy.
		// Usually returning 200 is best so provider doesn't spam us for an unfixable error like amount mismatch.
		if err == domain.ErrInvalidStateTransition || err == domain.ErrDuplicate {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
			return
		}
		
		// For transient errors like DB timeouts, let provider retry
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
