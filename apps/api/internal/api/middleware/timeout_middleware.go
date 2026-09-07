package middleware

import (
	"context"
	"net/http"
	"time"
)

// Timeout wraps the HTTP request with a bounded context timeout.
//
// Rationale:
//   - Protects the system against resource exhaustion from hanging database locks or sluggish networks.
//   - Inherited by downstream database queries (via database/sql ExecContext/QueryRowContext)
//     and transactional blocks (tx.Do).
//   - If the timeout is exceeded, the context is cancelled, automatically prompting
//     database/sql to roll back uncommitted transactions and release connection pool slots.
//   - The timeout value is chosen to be generous enough for legitimate database mutations and
//     external payment provider round-trips (typically 1-5s), while ensuring execution does
//     not outlast the server's TCP WriteTimeout.
func Timeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
