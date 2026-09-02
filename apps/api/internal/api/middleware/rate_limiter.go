package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*clientLimiter
	r        rate.Limit
	b        int
}

func NewRateLimiter(r rate.Limit, b int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*clientLimiter),
		r:        r,
		b:        b,
	}

	// Periodic cleanup worker to prevent unbounded memory growth
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			rl.cleanup()
		}
	}()

	return rl
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for key, cl := range rl.limiters {
		if time.Since(cl.lastSeen) > 10*time.Minute {
			delete(rl.limiters, key)
		}
	}
}

func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cl, exists := rl.limiters[key]
	if !exists {
		limiter := rate.NewLimiter(rl.r, rl.b)
		rl.limiters[key] = &clientLimiter{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}

	cl.lastSeen = time.Now()
	return cl.limiter
}

func ParseTrustedProxies(trustedCIDRsStr string) []*net.IPNet {
	if strings.TrimSpace(trustedCIDRsStr) == "" {
		return nil
	}

	var nets []*net.IPNet
	for _, cidr := range strings.Split(trustedCIDRsStr, ",") {
		trimmed := strings.TrimSpace(cidr)
		if trimmed == "" {
			continue
		}
		// If single IP without subnet mask, append /32 or /128
		if !strings.Contains(trimmed, "/") {
			if strings.Contains(trimmed, ":") {
				trimmed += "/128"
			} else {
				trimmed += "/32"
			}
		}
		_, ipNet, err := net.ParseCIDR(trimmed)
		if err == nil {
			nets = append(nets, ipNet)
		}
	}
	return nets
}

func MakeIPExtractor(trustedCIDRsStr string) func(r *http.Request) string {
	trustedNets := ParseTrustedProxies(trustedCIDRsStr)

	return func(r *http.Request) string {
		remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			remoteHost = r.RemoteAddr
		}

		remoteIP := net.ParseIP(remoteHost)
		isTrusted := false
		if remoteIP != nil {
			for _, net := range trustedNets {
				if net.Contains(remoteIP) {
					isTrusted = true
					break
				}
			}
		}

		// Only trust forwarded headers if request came from a trusted proxy subnet
		if isTrusted {
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				parts := strings.Split(xff, ",")
				for _, p := range parts {
					trimmed := strings.TrimSpace(p)
					if parsed := net.ParseIP(trimmed); parsed != nil {
						return trimmed
					}
				}
			}
			if xreal := r.Header.Get("X-Real-IP"); xreal != "" {
				trimmed := strings.TrimSpace(xreal)
				if parsed := net.ParseIP(trimmed); parsed != nil {
					return trimmed
				}
			}
		}

		// Fallback to direct connection IP
		return remoteHost
	}
}

func GetIP(r *http.Request) string {
	return MakeIPExtractor("")(r)
}


func RateLimit(rl *RateLimiter, keyExtractor func(r *http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyExtractor(r)
			limiter := rl.getLimiter(key)

			if !limiter.Allow() {
				respondError(w, http.StatusTooManyRequests, "TOO_MANY_REQUESTS", "rate limit exceeded, please try again later")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
