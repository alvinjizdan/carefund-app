package api

import (
	"net/http"

	"carefund-api/internal/api/middleware"
	"carefund-api/internal/config"
	"carefund-api/internal/domain"
	"carefund-api/internal/service"
	"golang.org/x/time/rate"
)

func NewRouter(
	authSvc service.AuthService,
	userSvc service.UserService,
	campSvc service.CampaignService,
	donationSvc service.DonationService,
	webhookSvc domain.WebhookService,
	rtRepo domain.RefreshTokenRepository,
	roleRepo domain.RoleRepository,
	idempotencyRepo domain.IdempotencyRepository,
	cfg *config.Config,
) http.Handler {
	mux := http.NewServeMux()

	authHandler := NewAuthHandler(userSvc, authSvc, rtRepo, roleRepo)
	campHandler := NewCampaignHandler(campSvc)
	donHandler := NewDonationHandler(donationSvc, idempotencyRepo)
	webhookHandler := NewWebhookHandler(webhookSvc, cfg)


	// Auth Middleware
	authenticate := middleware.Auth(authSvc)
	requireAdmin := middleware.RequireRole("ADMIN")

	// Rate Limiters
	var authRateLimit, donRateLimit, webhookRateLimit, generalRateLimit func(http.Handler) http.Handler

	trustedProxies := ""
	if cfg != nil {
		trustedProxies = cfg.TrustedProxyCIDRs
	}
	ipExtractor := middleware.MakeIPExtractor(trustedProxies)

	if cfg != nil && cfg.Env == "test" {
		// High capacity for unit/integration tests
		authRateLimit = middleware.RateLimit(middleware.NewRateLimiter(rate.Inf, 10000), ipExtractor)
		donRateLimit = middleware.RateLimit(middleware.NewRateLimiter(rate.Inf, 10000), ipExtractor)
		webhookRateLimit = middleware.RateLimit(middleware.NewRateLimiter(rate.Inf, 10000), ipExtractor)
		generalRateLimit = middleware.RateLimit(middleware.NewRateLimiter(rate.Inf, 10000), ipExtractor)
	} else {
		// Production / Staging Rate Limits
		// Auth: 5 reqs/min per IP
		authLimiter := middleware.NewRateLimiter(rate.Limit(5.0/60.0), 5)
		authRateLimit = middleware.RateLimit(authLimiter, ipExtractor)

		// Donations: 10 reqs/min per User ID (or IP)
		donLimiter := middleware.NewRateLimiter(rate.Limit(10.0/60.0), 10)
		donRateLimit = middleware.RateLimit(donLimiter, func(r *http.Request) string {
			if user, ok := r.Context().Value(middleware.UserKey).(*middleware.AuthenticatedUser); ok && user.ID != "" {
				return user.ID
			}
			return ipExtractor(r)
		})

		// Webhook: 100 reqs/min per IP
		webhookLimiter := middleware.NewRateLimiter(rate.Limit(100.0/60.0), 100)
		webhookRateLimit = middleware.RateLimit(webhookLimiter, ipExtractor)

		// General: 60 reqs/min per IP
		generalLimiter := middleware.NewRateLimiter(rate.Limit(60.0/60.0), 60)
		generalRateLimit = middleware.RateLimit(generalLimiter, ipExtractor)
	}


	// Webhook Route
	mux.Handle("POST /api/v1/webhooks/midtrans", webhookRateLimit(http.HandlerFunc(webhookHandler.MidtransNotification)))

	// Public Auth Routes
	mux.Handle("POST /api/v1/auth/register", authRateLimit(http.HandlerFunc(authHandler.Register)))
	mux.Handle("POST /api/v1/auth/login", authRateLimit(http.HandlerFunc(authHandler.Login)))
	mux.Handle("POST /api/v1/auth/refresh", authRateLimit(http.HandlerFunc(authHandler.Refresh)))

	// Protected Auth Routes
	mux.Handle("POST /api/v1/auth/logout", generalRateLimit(authenticate(http.HandlerFunc(authHandler.Logout))))
	mux.Handle("GET /api/v1/me", generalRateLimit(authenticate(http.HandlerFunc(authHandler.Me))))

	// Public Campaign Routes
	mux.Handle("GET /api/v1/campaigns", generalRateLimit(http.HandlerFunc(campHandler.List)))
	mux.Handle("GET /api/v1/campaigns/{campaign_id}", generalRateLimit(http.HandlerFunc(campHandler.Get)))

	// Protected Campaign Routes (Owner/Authenticated)
	mux.Handle("POST /api/v1/campaigns", generalRateLimit(authenticate(http.HandlerFunc(campHandler.Create))))
	mux.Handle("PATCH /api/v1/campaigns/{campaign_id}", generalRateLimit(authenticate(http.HandlerFunc(campHandler.Update))))
	mux.Handle("POST /api/v1/campaigns/{campaign_id}/submit-review", generalRateLimit(authenticate(http.HandlerFunc(campHandler.SubmitReview))))

	// Admin Campaign Routes
	mux.Handle("POST /api/v1/campaigns/{campaign_id}/approve", generalRateLimit(authenticate(requireAdmin(http.HandlerFunc(campHandler.Approve)))))
	mux.Handle("POST /api/v1/campaigns/{campaign_id}/reject", generalRateLimit(authenticate(requireAdmin(http.HandlerFunc(campHandler.Reject)))))
	mux.Handle("POST /api/v1/campaigns/{campaign_id}/suspend", generalRateLimit(authenticate(requireAdmin(http.HandlerFunc(campHandler.Suspend)))))

	// Donation & Payment Routes
	mux.Handle("POST /api/v1/donations", authenticate(donRateLimit(http.HandlerFunc(donHandler.CreateDonation))))
	mux.Handle("GET /api/v1/donations/{donation_id}", generalRateLimit(authenticate(http.HandlerFunc(donHandler.GetDonation))))
	mux.Handle("GET /api/v1/me/donations", generalRateLimit(authenticate(http.HandlerFunc(donHandler.ListMyDonations))))
	mux.Handle("GET /api/v1/payments/{payment_id}", generalRateLimit(authenticate(http.HandlerFunc(donHandler.GetPayment))))

	// Apply Global Middleware: CORS and RequestID/Security Headers
	allowedOrigins := "http://localhost:3000"
	if cfg != nil && cfg.CORSAllowedOrigins != "" {
		allowedOrigins = cfg.CORSAllowedOrigins
	}

	handler := middleware.CORS(allowedOrigins)(mux)
	handler = middleware.RequestIDAndSecurityHeaders()(handler)

	return handler
}

