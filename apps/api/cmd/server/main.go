package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"carefund-api/internal/api"
	"carefund-api/internal/config"
	"carefund-api/internal/database"
	"carefund-api/internal/infrastructure/payment/midtrans"
	"carefund-api/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Printf("Warning: failed to connect to database: %v", err)
		// Log warning and continue so that /health can still respond even if DB is down.
	} else {
		defer db.Close()
		log.Println("Successfully connected to the database")
	}

	// Repositories
	txManager := database.NewTransactionManager(db)
	userRepo := database.NewUserRepository(db)
	roleRepo := database.NewRoleRepository(db)
	campRepo := database.NewCampaignRepository(db)
	rtRepo := database.NewRefreshTokenRepository(db)
	idempotencyRepo := database.NewIdempotencyRepository(db)

	// Services
	authSvc := service.NewAuthService(cfg.JWTSecret, cfg.JWTAccessTTL)
	userSvc := service.NewUserService(userRepo, roleRepo, authSvc, txManager)
	campSvc := service.NewCampaignService(campRepo, txManager)

	midtransGw := midtrans.NewGateway(cfg)
	donationSvc := service.NewDonationService(database.NewDonationRepository(db), database.NewPaymentRepository(db), campRepo, midtransGw, txManager)
	webhookSvc := service.NewWebhookService(database.NewPaymentRepository(db), database.NewDonationRepository(db), database.NewPaymentEventRepository(db), txManager, service.WithWebhookIdempotencyRepository(idempotencyRepo))

	// Router
	handler := api.NewRouter(authSvc, userSvc, campSvc, donationSvc, webhookSvc, rtRepo, roleRepo, idempotencyRepo, cfg)


	// Custom ServeMux for Health & Readiness (or registered directly in router)
	mux := http.NewServeMux()
	mux.Handle("/", handler)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if db == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "database connection not established"})
			return
		}

		if err := db.Ping(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "database ping failed"})
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	})

	// Configure Hardened HTTP Server with Explicit Timeouts
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start Server in Background Goroutine
	go func() {
		log.Printf("Server listening on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server failed: %v", err)
		}
	}()

	// Listen for OS Signals for Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}

