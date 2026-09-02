package api

import (
	"encoding/json"
	"net/http"
	"time"

	"carefund-api/internal/api/middleware"
	"carefund-api/internal/domain"
	"carefund-api/internal/service"
)

type CampaignHandler struct {
	campSvc service.CampaignService
}

func NewCampaignHandler(campSvc service.CampaignService) *CampaignHandler {
	return &CampaignHandler{campSvc: campSvc}
}

type createCampaignReq struct {
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	CategoryID   string    `json:"category_id"`
	TargetAmount int64     `json:"target_amount"`
	StartAt      time.Time `json:"start_at"`
	EndAt        time.Time `json:"end_at"`
}

func (h *CampaignHandler) Create(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value(middleware.UserKey).(*middleware.AuthenticatedUser)

	var req createCampaignReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, r, domain.ErrInvalidInput)
		return
	}

	campaign, err := h.campSvc.CreateCampaign(
		r.Context(),
		authUser.ID,
		req.CategoryID,
		req.Title,
		req.Description,
		req.TargetAmount,
		req.StartAt,
		req.EndAt,
	)
	if err != nil {
		RespondError(w, r, err)
		return
	}

	RespondJSON(w, http.StatusCreated, SuccessResponse{Data: campaign})
}

func (h *CampaignHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("campaign_id") // Go 1.22 routing
	campaign, err := h.campSvc.GetCampaign(r.Context(), id)
	if err != nil {
		RespondError(w, r, err)
		return
	}

	RespondJSON(w, http.StatusOK, SuccessResponse{Data: campaign})
}

func (h *CampaignHandler) List(w http.ResponseWriter, r *http.Request) {
	campaigns, err := h.campSvc.ListCampaigns(r.Context(), 10, 0)
	if err != nil {
		RespondError(w, r, err)
		return
	}

	RespondJSON(w, http.StatusOK, SuccessResponse{Data: campaigns, Meta: map[string]int{"page": 1, "page_size": 10}})
}

func (h *CampaignHandler) Update(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value(middleware.UserKey).(*middleware.AuthenticatedUser)
	id := r.PathValue("campaign_id")

	var req createCampaignReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, r, domain.ErrInvalidInput)
		return
	}

	campaign, err := h.campSvc.UpdateCampaign(
		r.Context(),
		authUser.ID,
		id,
		req.Title,
		req.Description,
		req.CategoryID,
		req.TargetAmount,
		req.StartAt,
		req.EndAt,
	)
	if err != nil {
		RespondError(w, r, err)
		return
	}

	RespondJSON(w, http.StatusOK, SuccessResponse{Data: campaign})
}

func (h *CampaignHandler) SubmitReview(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value(middleware.UserKey).(*middleware.AuthenticatedUser)
	id := r.PathValue("campaign_id")

	if err := h.campSvc.SubmitForReview(r.Context(), authUser.ID, id); err != nil {
		RespondError(w, r, err)
		return
	}
	RespondJSON(w, http.StatusOK, SuccessResponse{Data: "submitted"})
}

func (h *CampaignHandler) Approve(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value(middleware.UserKey).(*middleware.AuthenticatedUser)
	id := r.PathValue("campaign_id")

	if err := h.campSvc.ApproveCampaign(r.Context(), authUser.ID, id); err != nil {
		RespondError(w, r, err)
		return
	}
	RespondJSON(w, http.StatusOK, SuccessResponse{Data: "approved"})
}

func (h *CampaignHandler) Reject(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value(middleware.UserKey).(*middleware.AuthenticatedUser)
	id := r.PathValue("campaign_id")

	var req struct{ Reason string `json:"reason"` }
	json.NewDecoder(r.Body).Decode(&req)

	if err := h.campSvc.RejectCampaign(r.Context(), authUser.ID, id, req.Reason); err != nil {
		RespondError(w, r, err)
		return
	}
	RespondJSON(w, http.StatusOK, SuccessResponse{Data: "rejected"})
}

func (h *CampaignHandler) Suspend(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value(middleware.UserKey).(*middleware.AuthenticatedUser)
	id := r.PathValue("campaign_id")

	if err := h.campSvc.SuspendCampaign(r.Context(), authUser.ID, id); err != nil {
		RespondError(w, r, err)
		return
	}
	RespondJSON(w, http.StatusOK, SuccessResponse{Data: "suspended"})
}
