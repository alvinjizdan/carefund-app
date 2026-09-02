package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type requestIDKeyType string

const RequestIDKey requestIDKeyType = "request_id"

func RequestIDAndSecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := r.Header.Get("X-Request-ID")
			if reqID == "" || len(reqID) > 64 {
				reqID = uuid.New().String()
			}

			w.Header().Set("X-Request-ID", reqID)
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			ctx := context.WithValue(r.Context(), RequestIDKey, reqID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
