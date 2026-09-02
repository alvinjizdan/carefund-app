package middleware

import (
	"net/http"
	"strings"
)

func CORS(allowedOriginsStr string) func(http.Handler) http.Handler {
	origins := strings.Split(allowedOriginsStr, ",")
	allowedOriginsMap := make(map[string]bool)
	for _, o := range origins {
		trimmed := strings.TrimSpace(o)
		if trimmed != "" {
			allowedOriginsMap[trimmed] = true
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if allowedOriginsMap[origin] || allowedOriginsMap["*"] {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					if origin != "*" {
						w.Header().Set("Access-Control-Allow-Credentials", "true")
					}
				}
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
