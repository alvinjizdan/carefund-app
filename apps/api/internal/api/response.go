package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"carefund-api/internal/domain"
	"carefund-api/internal/logger"
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
	} else if errors.Is(err, context.DeadlineExceeded) {
		status = http.StatusGatewayTimeout
		code = "TIMEOUT"
		msg = "The request timed out while waiting for a response"
	} else if errors.Is(err, context.Canceled) {
		status = 499 // Client Closed Request
		code = "CLIENT_CLOSED_REQUEST"
		msg = "The request was cancelled before completion"
	}

	reqID := logger.GetRequestID(r.Context())
	if reqID == "" {
		reqID = r.Header.Get("X-Request-ID")
	}

	if status >= 500 {
		logger.Error(r.Context(), "HTTP request failed", err,
			logger.F("component", "HTTP"),
			logger.F("status", status),
			logger.F("code", code),
			logger.F("method", r.Method),
			logger.F("path", r.URL.Path),
		)
	} else {
		logger.Warn(r.Context(), "HTTP request rejected",
			logger.F("component", "HTTP"),
			logger.F("status", status),
			logger.F("code", code),
			logger.F("method", r.Method),
			logger.F("path", r.URL.Path),
		)
	}

	RespondJSON(w, status, ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: msg,
		},
		RequestID: reqID,
	})
}
