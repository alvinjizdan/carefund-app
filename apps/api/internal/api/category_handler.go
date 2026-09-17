package api

import (
	"net/http"

	"carefund-api/internal/domain"
)

type CategoryHandler struct {
	catRepo domain.CategoryRepository
}

func NewCategoryHandler(catRepo domain.CategoryRepository) *CategoryHandler {
	return &CategoryHandler{catRepo: catRepo}
}

func (h *CategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	categories, err := h.catRepo.ListAllActive(r.Context())
	if err != nil {
		RespondError(w, r, err)
		return
	}

	if categories == nil {
		categories = []*domain.Category{}
	}

	RespondJSON(w, http.StatusOK, SuccessResponse{Data: categories})
}
