package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"carefund-api/internal/api/middleware"
	"carefund-api/internal/domain"
	"carefund-api/internal/service"
)

type DonationHandler struct {
	donationSvc     service.DonationService
	idempotencyRepo domain.IdempotencyRepository
}

func NewDonationHandler(donationSvc service.DonationService, idempotencyRepo domain.IdempotencyRepository) *DonationHandler {
	return &DonationHandler{
		donationSvc:     donationSvc,
		idempotencyRepo: idempotencyRepo,
	}
}

// CreateDonation implements concurrency-safe HTTP idempotency using a PostgreSQL-backed
// reservation pattern.
//
// Flow when an Idempotency-Key is present:
//
//  1. Decode and validate the request body; compute SHA256(requestHash).
//  2. Look up the idempotency_keys table for (user_id, key, expires_at > NOW()).
//     a. COMPLETED record + matching hash  → replay cached response.
//     b. COMPLETED record + mismatched hash → 400 Bad Request.
//     c. PENDING record + matching hash  → 202 Accepted (in-flight; client should retry).
//     d. PENDING record + mismatched hash  → 400 Bad Request.
//     e. FAILED record                     → 422 Unprocessable Entity (definitive rejection).
//     f. Not found / expired               → proceed to creation.
//
//  3. Call donationSvc.CreateDonationIdempotent which, inside a single DB transaction:
//     a. Atomically INSERT the idempotency reservation (status=PENDING).
//     b. If UNIQUE conflict → ErrIdempotencyConflict → caller waits / re-reads.
//     c. INSERT Donation.
//     d. INSERT Payment.
//     e. Commit.
//
//  4. Call Midtrans outside the transaction.
//     a. Success  → idempotencyRepo.Complete (UPDATE status=COMPLETED, store response).
//     b. Definitive rejection → idempotencyRepo.Fail (UPDATE status=FAILED).
//     c. Ambiguous timeout  → leave PENDING; reconciliation handles it.
//
//  5. Return the response to the client.
func (h *DonationHandler) CreateDonation(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(middleware.UserKey).(*middleware.AuthenticatedUser)
	if !ok {
		RespondError(w, r, domain.ErrUnauthorized)
		return
	}

	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey != "" && len(idempotencyKey) > 64 {
		RespondError(w, r, domain.ErrInvalidInput)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		CampaignID  string `json:"campaign_id"`
		Amount      int64  `json:"amount"`
		IsAnonymous bool   `json:"is_anonymous"`
		Message     string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, r, domain.ErrInvalidInput)
		return
	}

	// Compute deterministic SHA256 hash of the logical request payload.
	payloadRaw := fmt.Sprintf("%s:%d:%t:%s", req.CampaignID, req.Amount, req.IsAnonymous, req.Message)
	hashBytes := sha256.Sum256([]byte(payloadRaw))
	requestHash := hex.EncodeToString(hashBytes[:])

	// --- Idempotency-Key present: use the concurrency-safe path ---
	if idempotencyKey != "" && h.idempotencyRepo != nil {
		h.createDonationIdempotent(w, r, user, req.CampaignID, req.Amount, req.IsAnonymous, req.Message, idempotencyKey, requestHash)
		return
	}

	// --- No Idempotency-Key: standard (non-idempotent) creation path ---
	donation, payment, res, err := h.donationSvc.CreateDonation(r.Context(), user.ID, user.Email, "", req.CampaignID, req.Amount, req.IsAnonymous, req.Message)
	if err != nil {
		RespondError(w, r, err)
		return
	}

	respondDonation(w, donation, payment, res)
}

// createDonationIdempotent is the concurrency-safe sub-handler.
func (h *DonationHandler) createDonationIdempotent(
	w http.ResponseWriter,
	r *http.Request,
	user *middleware.AuthenticatedUser,
	campaignID string,
	amount int64,
	isAnonymous bool,
	message string,
	idempotencyKey string,
	requestHash string,
) {
	// PHASE 1: Look up any existing record BEFORE attempting creation.
	existing, err := h.idempotencyRepo.Get(r.Context(), user.ID, idempotencyKey)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		// Unexpected DB error on lookup.
		RespondError(w, r, fmt.Errorf("idempotency lookup error: %w", err))
		return
	}

	if existing != nil {
		h.handleExistingIdempotencyRecord(w, r, existing, requestHash)
		return
	}

	// PHASE 2: No existing record — attempt atomic creation.
	expiresAt := time.Now().Add(24 * time.Hour)
	donation, payment, res, err := h.donationSvc.CreateDonationIdempotent(
		r.Context(),
		user.ID, user.Email, "",
		campaignID, amount, isAnonymous, message,
		h.idempotencyRepo,
		idempotencyKey,
		requestHash,
		expiresAt,
	)

	if err != nil {
		if errors.Is(err, domain.ErrIdempotencyConflict) {
			// A concurrent request won the reservation race.
			// Re-read the record to return the appropriate response.
			// The winning request may still be in-flight (PENDING) or may have completed.
			existing2, readErr := h.idempotencyRepo.Get(r.Context(), user.ID, idempotencyKey)
			if readErr != nil || existing2 == nil {
				// Record not yet visible (e.g., winner's tx not committed) — treat as in-flight.
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				RespondJSON(w, http.StatusAccepted, ErrorResponse{Error: ErrorDetail{Code: "request_in_flight", Message: "A concurrent request with this Idempotency-Key is in progress. Please retry after a moment."}})
				return
			}
			h.handleExistingIdempotencyRecord(w, r, existing2, requestHash)
			return
		}

		// Other errors (campaign not found, invalid state, etc.)
		RespondError(w, r, err)
		return
	}

	// PHASE 3: Financial records committed; now call Midtrans (outside DB transaction).
	responseData := buildDonationResponse(donation, payment, res)
	respObj := SuccessResponse{Data: responseData}
	respBytes, marshalErr := json.Marshal(respObj)
	if marshalErr != nil {
		// Extremely unlikely; do not leave idempotency record dangling as PENDING.
		_ = h.idempotencyRepo.Fail(r.Context(), user.ID, idempotencyKey)
		RespondError(w, r, fmt.Errorf("internal error marshalling response"))
		return
	}

	// PHASE 4: Persist the completed idempotency record.
	// INVARIANT: idempotency_keys.Complete must succeed before returning success to the client.
	// If Complete fails, we do NOT return a successful response — the idempotency record remains
	// PENDING and the next retry will follow the PENDING re-read path (Phase 1 / handleExisting).
	completeErr := h.idempotencyRepo.Complete(r.Context(), user.ID, idempotencyKey, http.StatusCreated, respBytes)
	if completeErr != nil {
		// Do not return 201 with a false idempotency promise.
		// The financial records exist in PENDING state. Reconciliation will handle payment state.
		// The client should retry; the retry will find the record PENDING and get 202.
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "1")
		RespondJSON(w, http.StatusAccepted, ErrorResponse{Error: ErrorDetail{Code: "idempotency_persistence_failure", Message: "The operation completed but idempotency state could not be persisted. Please retry to retrieve the result."}})
		return
	}


	// PHASE 5: Return the response.
	RespondJSON(w, http.StatusCreated, respObj)
}

// handleExistingIdempotencyRecord handles lookup of an existing record (PENDING/COMPLETED/FAILED).
func (h *DonationHandler) handleExistingIdempotencyRecord(
	w http.ResponseWriter,
	r *http.Request,
	record *domain.IdempotencyRecord,
	requestHash string,
) {
	// Payload hash mismatch: same key, different request body.
	if record.RequestHash != requestHash {
		RespondError(w, r, domain.ErrInvalidInput)
		return
	}

	switch record.Status {
	case domain.IdempotencyStatusCompleted:
		// Replay the cached successful response.
		var cachedPayload interface{}
		_ = json.Unmarshal(record.ResponseBody, &cachedPayload)
		RespondJSON(w, *record.ResponseCode, cachedPayload)

	case domain.IdempotencyStatusPending:
		// The original request is still in-flight (either Midtrans call or idempotency
		// persistence is incomplete). Instruct the client to retry.
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "1")
		RespondJSON(w, http.StatusAccepted, ErrorResponse{Error: ErrorDetail{Code: "request_in_flight", Message: "This operation is still being processed. Please retry after a moment."}})

	case domain.IdempotencyStatusFailed:
		// Definitive Midtrans rejection. Inform the client; do not replay.
		RespondJSON(w, http.StatusUnprocessableEntity, ErrorResponse{Error: ErrorDetail{Code: "payment_failed", Message: "The original payment attempt was definitively rejected by the payment provider. Please initiate a new request with a new Idempotency-Key."}})


	default:
		RespondError(w, r, fmt.Errorf("unexpected idempotency status: %s", record.Status))
	}
}

// buildDonationResponse constructs the standard donation response map.
func buildDonationResponse(donation *domain.Donation, payment *domain.Payment, res *domain.PaymentCreationResult) map[string]interface{} {
	return map[string]interface{}{
		"donation_id":   donation.ID,
		"payment_id":    payment.ID,
		"order_id":      payment.OrderID,
		"amount":        donation.Amount,
		"status":        donation.Status,
		"payment_token": res.PaymentToken,
		"redirect_url":  res.RedirectURL,
	}
}

// respondDonation is the standard (non-idempotent) response helper.
func respondDonation(w http.ResponseWriter, donation *domain.Donation, payment *domain.Payment, res *domain.PaymentCreationResult) {
	RespondJSON(w, http.StatusCreated, SuccessResponse{Data: buildDonationResponse(donation, payment, res)})
}

func (h *DonationHandler) GetDonation(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(middleware.UserKey).(*middleware.AuthenticatedUser)
	if !ok {
		RespondError(w, r, domain.ErrUnauthorized)
		return
	}

	id := r.PathValue("donation_id")
	if id == "" {
		RespondError(w, r, domain.ErrInvalidInput)
		return
	}

	donation, err := h.donationSvc.GetDonationForUser(r.Context(), user.ID, user.Roles, id)
	if err != nil {
		RespondError(w, r, err)
		return
	}

	RespondJSON(w, http.StatusOK, SuccessResponse{
		Data: donation,
	})
}

func (h *DonationHandler) GetPayment(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(middleware.UserKey).(*middleware.AuthenticatedUser)
	if !ok {
		RespondError(w, r, domain.ErrUnauthorized)
		return
	}

	id := r.PathValue("payment_id")
	if id == "" {
		RespondError(w, r, domain.ErrInvalidInput)
		return
	}

	payment, err := h.donationSvc.GetPaymentForUser(r.Context(), user.ID, user.Roles, id)
	if err != nil {
		RespondError(w, r, err)
		return
	}

	RespondJSON(w, http.StatusOK, SuccessResponse{
		Data: payment,
	})
}

func (h *DonationHandler) ListMyDonations(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(middleware.UserKey).(*middleware.AuthenticatedUser)
	if !ok {
		RespondError(w, r, domain.ErrUnauthorized)
		return
	}

	limit := 10
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 100 {
		limit = 100
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	donations, err := h.donationSvc.ListUserDonations(r.Context(), user.ID, limit, offset)
	if err != nil {
		RespondError(w, r, err)
		return
	}

	if donations == nil {
		donations = []*domain.Donation{}
	}

	RespondJSON(w, http.StatusOK, SuccessResponse{
		Data: donations,
	})
}
