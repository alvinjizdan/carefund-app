package middleware

import (
	"net/http"
	"time"

	"carefund-api/internal/metrics"
)

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

func (w *statusResponseWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.statusCode = code
		w.wroteHeader = true
		w.ResponseWriter.WriteHeader(code)
	}
}

func (w *statusResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// Metrics creates an HTTP middleware that instruments request count and latency histograms.
func Metrics() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			srw := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(srw, r)

			duration := time.Since(start)
			normalizedRoute := metrics.NormalizeRoute(r.URL.Path)
			metrics.RecordHTTPRequest(r.Method, normalizedRoute, srw.statusCode, duration)
		})
	}
}
