package main

import (
	"encoding/json"
	"log"
	"net/http"

	"carefund-api/internal/api"
	"carefund-api/internal/config"
	"carefund-api/internal/database"
	"carefund-api/internal/service"
	"carefund-api/internal/infrastructure/payment/midtrans"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Printf("Warning: failed to connect to database: %v", err)
		// We log warning and continue so that /health can still work even if DB is down.
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

	// Services
	authSvc := service.NewAuthService(cfg.JWTSecret, cfg.JWTAccessTTL)
	userSvc := service.NewUserService(userRepo, roleRepo, authSvc, txManager)
	campSvc := service.NewCampaignService(campRepo, txManager)

	midtransGw := midtrans.NewGateway(cfg)
	donationSvc := service.NewDonationService(database.NewDonationRepository(db), database.NewPaymentRepository(db), campRepo, midtransGw, txManager)
	
	webhookSvc := service.NewWebhookService(database.NewPaymentRepository(db), database.NewDonationRepository(db), database.NewPaymentEventRepository(db), txManager)

	// Router
	mux := api.NewRouter(authSvc, userSvc, campSvc, donationSvc, webhookSvc, rtRepo, roleRepo, cfg)

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

	log.Printf("Server listening on port %s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
