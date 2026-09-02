package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"carefund-api/internal/domain"
)

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error     ErrorDetail `json:"error"`
	RequestID string      `json:"request_id"`
}

type SuccessResponse struct {
	Data interface{} `json:"data"`
	Meta interface{} `json:"meta,omitempty"`
}

func RespondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload != nil {
		json.NewEncoder(w).Encode(payload)
	}
}

func RespondError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	code := "INTERNAL_ERROR"
	msg := "An internal server error occurred"

	if errors.Is(err, domain.ErrNotFound) {
		status = http.StatusNotFound
		code = "NOT_FOUND"
		msg = err.Error()
	} else if errors.Is(err, domain.ErrDuplicate) {
		status = http.StatusConflict
		code = "DUPLICATE"
		msg = err.Error()
	} else if errors.Is(err, domain.ErrInvalidInput) {
		status = http.StatusBadRequest
		code = "INVALID_REQUEST"
		msg = err.Error()
	} else if errors.Is(err, domain.ErrUnauthorized) {
		status = http.StatusUnauthorized
		code = "UNAUTHORIZED"
		msg = err.Error()
	} else if errors.Is(err, domain.ErrForbidden) {
		status = http.StatusForbidden
		code = "FORBIDDEN"
		msg = err.Error()
	} else if errors.Is(err, domain.ErrInvalidStateTransition) {
		status = http.StatusConflict
		code = "INVALID_STATE_TRANSITION"
		msg = err.Error()
	}

	RespondJSON(w, status, ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: msg,
		},
		RequestID: r.Header.Get("X-Request-ID"), // Basic handling
	})
}
