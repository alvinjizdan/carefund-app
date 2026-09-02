package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"carefund-api/internal/api/middleware"
	"carefund-api/internal/domain"
	"carefund-api/internal/service"
)

type DonationHandler struct {
	donationSvc service.DonationService
}

func NewDonationHandler(donationSvc service.DonationService) *DonationHandler {
	return &DonationHandler{donationSvc: donationSvc}
}

func (h *DonationHandler) CreateDonation(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(middleware.UserKey).(*middleware.AuthenticatedUser)
	if !ok {
		RespondError(w, r, domain.ErrUnauthorized)
		return
	}

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

	donation, payment, res, err := h.donationSvc.CreateDonation(r.Context(), user.ID, user.Email, "", req.CampaignID, req.Amount, req.IsAnonymous, req.Message)
	if err != nil {
		RespondError(w, r, err)
		return
	}

	RespondJSON(w, http.StatusCreated, SuccessResponse{
		Data: map[string]interface{}{
			"donation_id":  donation.ID,
			"payment_id":   payment.ID,
			"order_id":     payment.OrderID,
			"amount":       donation.Amount,
			"status":       donation.Status,
			"payment_token": res.PaymentToken,
			"redirect_url":  res.RedirectURL,
		},
	})
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
