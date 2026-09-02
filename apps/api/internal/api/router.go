package api

import (
	"net/http"

	"carefund-api/internal/api/middleware"
	"carefund-api/internal/config"
	"carefund-api/internal/domain"
	"carefund-api/internal/service"
)

func NewRouter(
	authSvc service.AuthService,
	userSvc service.UserService,
	campSvc service.CampaignService,
	donationSvc service.DonationService,
	webhookSvc domain.WebhookService,
	rtRepo domain.RefreshTokenRepository,
	roleRepo domain.RoleRepository,
	cfg *config.Config,
) *http.ServeMux {
	mux := http.NewServeMux()

	authHandler := NewAuthHandler(userSvc, authSvc, rtRepo, roleRepo)
	campHandler := NewCampaignHandler(campSvc)
	donHandler := NewDonationHandler(donationSvc)
	webhookHandler := NewWebhookHandler(webhookSvc, cfg)

	// Webhook Route
	mux.HandleFunc("POST /api/v1/webhooks/midtrans", webhookHandler.MidtransNotification)

	// Auth Middleware
	authenticate := middleware.Auth(authSvc)
	requireAdmin := middleware.RequireRole("ADMIN")

	// Public Auth Routes
	mux.HandleFunc("POST /api/v1/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/v1/auth/refresh", authHandler.Refresh)

	// Protected Auth Routes
	mux.Handle("POST /api/v1/auth/logout", authenticate(http.HandlerFunc(authHandler.Logout)))
	mux.Handle("GET /api/v1/me", authenticate(http.HandlerFunc(authHandler.Me)))

	// Public Campaign Routes
	mux.HandleFunc("GET /api/v1/campaigns", campHandler.List)
	mux.HandleFunc("GET /api/v1/campaigns/{campaign_id}", campHandler.Get)

	// Protected Campaign Routes (Owner/Authenticated)
	mux.Handle("POST /api/v1/campaigns", authenticate(http.HandlerFunc(campHandler.Create)))
	mux.Handle("PATCH /api/v1/campaigns/{campaign_id}", authenticate(http.HandlerFunc(campHandler.Update)))
	mux.Handle("POST /api/v1/campaigns/{campaign_id}/submit-review", authenticate(http.HandlerFunc(campHandler.SubmitReview)))

	// Admin Campaign Routes
	mux.Handle("POST /api/v1/campaigns/{campaign_id}/approve", authenticate(requireAdmin(http.HandlerFunc(campHandler.Approve))))
	mux.Handle("POST /api/v1/campaigns/{campaign_id}/reject", authenticate(requireAdmin(http.HandlerFunc(campHandler.Reject))))
	mux.Handle("POST /api/v1/campaigns/{campaign_id}/suspend", authenticate(requireAdmin(http.HandlerFunc(campHandler.Suspend))))

	// Donation & Payment Routes
	mux.Handle("POST /api/v1/donations", authenticate(http.HandlerFunc(donHandler.CreateDonation)))
	mux.Handle("GET /api/v1/donations/{donation_id}", authenticate(http.HandlerFunc(donHandler.GetDonation)))
	mux.Handle("GET /api/v1/me/donations", authenticate(http.HandlerFunc(donHandler.ListMyDonations)))
	mux.Handle("GET /api/v1/payments/{payment_id}", authenticate(http.HandlerFunc(donHandler.GetPayment)))

	return mux
}
