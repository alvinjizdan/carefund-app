package main

import (
	"context"
	"log"

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
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()

	txManager := database.NewTransactionManager(db)
	donRepo := database.NewDonationRepository(db)
	payRepo := database.NewPaymentRepository(db)
	eventRepo := database.NewPaymentEventRepository(db)

	midtransGw := midtrans.NewGateway(cfg)
	idempotencyRepo := database.NewIdempotencyRepository(db)
	webhookSvc := service.NewWebhookService(payRepo, donRepo, eventRepo, txManager, service.WithWebhookIdempotencyRepository(idempotencyRepo))
	reconSvc := service.NewReconciliationService(payRepo, webhookSvc, midtransGw, txManager)

	ctx := context.Background()
	batchSize := 50
	
	log.Printf("[Worker] Starting payment reconciliation worker...")
	processed, err := reconSvc.ReconcilePendingPayments(ctx, batchSize, cfg.PaymentPendingTTL)
	if err != nil {
		log.Fatalf("[Worker] Reconciliation failed: %v", err)
	}
	log.Printf("[Worker] Reconciliation completed. Processed: %d", processed)
}
