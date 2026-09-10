package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"carefund-api/internal/api"
	"carefund-api/internal/config"
	"carefund-api/internal/database"
	"carefund-api/internal/infrastructure/payment/midtrans"
	"carefund-api/internal/logger"
	"carefund-api/internal/metrics"
	"carefund-api/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal(context.Background(), "Failed to load config", err, logger.F("component", "Server"))
	}

	db, err := database.Connect(cfg)
	if err != nil {
		logger.Warn(context.Background(), "Failed to connect to database on startup", logger.F("component", "Server"), logger.F("err", err.Error()))
		// Log warning and continue so that /health can still respond even if DB is down.
	} else {
		defer db.Close()
		logger.Info(context.Background(), "Successfully connected to database", logger.F("component", "Server"))
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

	// Configure Hardened Public HTTP Server with Explicit Timeouts
	// ReadTimeout: 15s (allows full body reading)
	// WriteTimeout: 30s (headroom above the 20s request context timeout)
	// IdleTimeout: 60s (recycles idle keepalive TCP connections)
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Internal Metrics Server (dedicated listener)
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", metrics.Handler())

	metricsSrv := &http.Server{
		Addr:              cfg.MetricsAddress(),
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	// Start Public API Server in Background Goroutine
	go func() {
		logger.Info(context.Background(), "Server listening", logger.F("component", "Server"), logger.F("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal(context.Background(), "Server failed unexpectedly", err, logger.F("component", "Server"))
		}
	}()

	// Start Internal Metrics Server in Background Goroutine
	go func() {
		logger.Info(context.Background(), "Internal metrics server listening", logger.F("component", "MetricsServer"), logger.F("addr", cfg.MetricsAddress()))
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(context.Background(), "Metrics server failed unexpectedly", err, logger.F("component", "MetricsServer"))
		}
	}()

	// Listen for OS Signals for Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logger.Info(context.Background(), "Shutting down servers gracefully...", logger.F("component", "Server"))

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error(context.Background(), "Public server forced to shutdown", err, logger.F("component", "Server"))
	}
	if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
		logger.Error(context.Background(), "Metrics server forced to shutdown", err, logger.F("component", "MetricsServer"))
	}

	logger.Info(context.Background(), "Server exited cleanly", logger.F("component", "Server"))
}
