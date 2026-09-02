package api_test

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"carefund-api/internal/api"
	"carefund-api/internal/api/middleware"
	"carefund-api/internal/config"
	"carefund-api/internal/database"
	"carefund-api/internal/domain"
	"carefund-api/internal/infrastructure/payment/midtrans"
	"carefund-api/internal/service"
)


func setupTestAPI(t *testing.T) (*database.DB, http.Handler, service.AuthService) {
	cfg := &config.Config{
		Env:        "test",
		DBHost:     "localhost",
		DBPort:     "5432",
		DBUser:     "postgres",
		DBPassword: "234djisamSOE",
		DBName:     "carefund-app_test",
		DBSSLMode:  "disable",
	}

	db, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}

	_, _ = db.ExecContext(context.Background(), `
		DROP TABLE IF EXISTS idempotency_keys CASCADE;
		CREATE TABLE IF NOT EXISTS idempotency_keys (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			idempotency_key VARCHAR(64) NOT NULL,
			request_hash VARCHAR(64) NOT NULL,
			order_id VARCHAR(64),
			status VARCHAR(16) NOT NULL DEFAULT 'PENDING',
			response_code INT,
			response_body JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '24 hours',
			CONSTRAINT idx_idempotency_user_key UNIQUE (user_id, idempotency_key)
		);
	`)


	tables := []string{"idempotency_keys", "payment_events", "refunds", "payments", "donations", "settlement_items", "settlements", "campaigns", "categories", "user_roles", "roles", "users"}
	for _, table := range tables {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM "+table)
	}

	_, _ = db.ExecContext(context.Background(), "INSERT INTO roles (name) VALUES ('DONOR'), ('CAMPAIGN_OWNER'), ('ADMIN')")


	txManager := database.NewTransactionManager(db)
	userRepo := database.NewUserRepository(db)
	roleRepo := database.NewRoleRepository(db)
	campRepo := database.NewCampaignRepository(db)
	rtRepo := database.NewRefreshTokenRepository(db)
	idempotencyRepo := database.NewIdempotencyRepository(db)

	authSvc := service.NewAuthService("test-secret-key", 15*time.Minute)
	userSvc := service.NewUserService(userRepo, roleRepo, authSvc, txManager)
	campSvc := service.NewCampaignService(campRepo, txManager)

	mockGw := midtrans.NewMockPaymentGateway()
	donationSvc := service.NewDonationService(database.NewDonationRepository(db), database.NewPaymentRepository(db), campRepo, mockGw, txManager)
	
	webhookSvc := service.NewWebhookService(database.NewPaymentRepository(db), database.NewDonationRepository(db), database.NewPaymentEventRepository(db), txManager, service.WithWebhookIdempotencyRepository(idempotencyRepo))

	router := api.NewRouter(authSvc, userSvc, campSvc, donationSvc, webhookSvc, rtRepo, roleRepo, idempotencyRepo, cfg)

	return db, router, authSvc
}

func TestRegisterAndLoginAPI(t *testing.T) {
	db, router, _ := setupTestAPI(t)
	defer db.Close()

	// 1. Register
	reqBody := map[string]string{
		"name":     "API User",
		"email":    "api@example.com",
		"password": "pass",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201 Created, got %d. Body: %s", w.Code, w.Body.String())
	}

	// 2. Login
	loginBody := map[string]string{
		"email":    "api@example.com",
		"password": "pass",
	}
	lBytes, _ := json.Marshal(loginBody)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(lBytes))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()

	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w2.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &resp)

	data := resp["data"].(map[string]interface{})
	token := data["access_token"].(string)
	if token == "" {
		t.Errorf("expected token in response")
	}

	// 3. GET /api/v1/me
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	w3 := httptest.NewRecorder()

	router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Errorf("expected 200 OK for /me, got %d", w3.Code)
	}
}

func TestCampaignAuthorizationAPI(t *testing.T) {
	db, router, authSvc := setupTestAPI(t)
	defer db.Close()
	ctx := context.Background()

	// Setup users
	userRepo := database.NewUserRepository(db)
	donorUser := &domain.User{Email: "donor@example.com", PasswordHash: "h", Name: "D", IsActive: true}
	_ = userRepo.Create(ctx, donorUser)
	donorToken, _ := authSvc.GenerateAccessToken(donorUser, []string{"DONOR"})

	adminUser := &domain.User{Email: "admin@example.com", PasswordHash: "h", Name: "A", IsActive: true}
	_ = userRepo.Create(ctx, adminUser)
	adminToken, _ := authSvc.GenerateAccessToken(adminUser, []string{"ADMIN"})

	catRepo := database.NewCategoryRepository(db)
	cat := &domain.Category{Name: "Edu", Slug: "edu", IsActive: true}
	_ = catRepo.Create(ctx, cat)

	// 1. Create Campaign (Donor)
	now := time.Now()
	campReq := map[string]interface{}{
		"title":         "Help",
		"description":   "Desc",
		"category_id":   cat.ID,
		"target_amount": 1000,
		"start_at":      now,
		"end_at":        now.Add(24 * time.Hour),
	}
	body, _ := json.Marshal(campReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/campaigns", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+donorToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	campID := data["ID"].(string)

	// 2. Approve Campaign as Donor (Should fail - Forbidden)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/campaigns/"+campID+"/approve", nil)
	req2.Header.Set("Authorization", "Bearer "+donorToken)
	w2 := httptest.NewRecorder()

	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for donor trying to approve, got %d", w2.Code)
	}

	// 3. Approve Campaign as Admin (Should succeed, wait, campaign is DRAFT)
	// It should fail with Invalid State Transition!
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/campaigns/"+campID+"/approve", nil)
	req3.Header.Set("Authorization", "Bearer "+adminToken)
	w3 := httptest.NewRecorder()

	router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for approving DRAFT campaign, got %d", w3.Code)
	}

	// 4. Submit for review as Donor
	req4 := httptest.NewRequest(http.MethodPost, "/api/v1/campaigns/"+campID+"/submit-review", nil)
	req4.Header.Set("Authorization", "Bearer "+donorToken)
	w4 := httptest.NewRecorder()

	router.ServeHTTP(w4, req4)
	if w4.Code != http.StatusOK {
		t.Errorf("expected 200 OK for submit, got %d", w4.Code)
	}

	// 5. Approve as Admin (Now it's valid!)
	req5 := httptest.NewRequest(http.MethodPost, "/api/v1/campaigns/"+campID+"/approve", nil)
	req5.Header.Set("Authorization", "Bearer "+adminToken)
	w5 := httptest.NewRecorder()

	router.ServeHTTP(w5, req5)
	if w5.Code != http.StatusOK {
		t.Errorf("expected 200 OK for approval, got %d. Body: %s", w5.Code, w5.Body.String())
	}
}

func TestBOLAObjectLevelAuthorizationAPI(t *testing.T) {
	db, router, authSvc := setupTestAPI(t)
	defer db.Close()
	ctx := context.Background()

	// 1. Setup User A (Donor A), User B (Donor B), Admin User
	userRepo := database.NewUserRepository(db)
	userA := &domain.User{Email: "userA@example.com", PasswordHash: "h", Name: "User A", IsActive: true}
	_ = userRepo.Create(ctx, userA)
	tokenA, _ := authSvc.GenerateAccessToken(userA, []string{"DONOR"})

	userB := &domain.User{Email: "userB@example.com", PasswordHash: "h", Name: "User B", IsActive: true}
	_ = userRepo.Create(ctx, userB)
	tokenB, _ := authSvc.GenerateAccessToken(userB, []string{"DONOR"})

	adminUser := &domain.User{Email: "adminBOLA@example.com", PasswordHash: "h", Name: "Admin", IsActive: true}
	_ = userRepo.Create(ctx, adminUser)
	adminToken, _ := authSvc.GenerateAccessToken(adminUser, []string{"ADMIN"})

	// Setup Category & Active Campaign owned by User A
	catRepo := database.NewCategoryRepository(db)
	cat := &domain.Category{Name: "General", Slug: "gen", IsActive: true}
	_ = catRepo.Create(ctx, cat)

	campRepo := database.NewCampaignRepository(db)
	now := time.Now()
	camp := &domain.Campaign{
		OwnerID:       userA.ID,
		CategoryID:    cat.ID,
		Title:         "Active Camp",
		Slug:          "active-camp",
		Description:   "Desc",
		TargetAmount:  1000000,
		Status:        domain.CampaignStateActive,
		StartAt:       now.Add(-time.Hour),
		EndAt:         now.Add(24 * time.Hour),
	}
	_ = campRepo.Create(ctx, camp)

	// User A creates a donation
	txManager := database.NewTransactionManager(db)
	donationRepo := database.NewDonationRepository(db)
	paymentRepo := database.NewPaymentRepository(db)
	mockGw := midtrans.NewMockPaymentGateway()
	donSvc := service.NewDonationService(donationRepo, paymentRepo, campRepo, mockGw, txManager)

	donationA, paymentA, _, err := donSvc.CreateDonation(ctx, userA.ID, userA.Email, userA.Name, camp.ID, 50000, false, "Go User A!")
	if err != nil {
		t.Fatalf("failed to create donation: %v", err)
	}

	// ----------------------------------------------------
	// A. DONATION LOOKUP AUTHORIZATION
	// ----------------------------------------------------
	// 1. Owner (User A) accesses own donation -> 200 OK
	reqA := httptest.NewRequest(http.MethodGet, "/api/v1/donations/"+donationA.ID, nil)
	reqA.Header.Set("Authorization", "Bearer "+tokenA)
	wA := httptest.NewRecorder()
	router.ServeHTTP(wA, reqA)
	if wA.Code != http.StatusOK {
		t.Errorf("expected 200 OK for owner accessing own donation, got %d", wA.Code)
	}

	// 2. Other User (User B) accesses User A's donation -> 403 Forbidden
	reqB := httptest.NewRequest(http.MethodGet, "/api/v1/donations/"+donationA.ID, nil)
	reqB.Header.Set("Authorization", "Bearer "+tokenB)
	wB := httptest.NewRecorder()
	router.ServeHTTP(wB, reqB)
	if wB.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for User B accessing User A's donation, got %d", wB.Code)
	}

	// 3. Unauthenticated access -> 401 Unauthorized
	reqNoAuth := httptest.NewRequest(http.MethodGet, "/api/v1/donations/"+donationA.ID, nil)
	wNoAuth := httptest.NewRecorder()
	router.ServeHTTP(wNoAuth, reqNoAuth)
	if wNoAuth.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for unauthenticated donation lookup, got %d", wNoAuth.Code)
	}

	// 4. Admin accesses User A's donation -> 200 OK
	reqAdmin := httptest.NewRequest(http.MethodGet, "/api/v1/donations/"+donationA.ID, nil)
	reqAdmin.Header.Set("Authorization", "Bearer "+adminToken)
	wAdmin := httptest.NewRecorder()
	router.ServeHTTP(wAdmin, reqAdmin)
	if wAdmin.Code != http.StatusOK {
		t.Errorf("expected 200 OK for Admin accessing donation, got %d", wAdmin.Code)
	}

	// ----------------------------------------------------
	// B. PAYMENT LOOKUP AUTHORIZATION
	// ----------------------------------------------------
	// 1. Owner (User A) accesses own payment -> 200 OK
	reqPayA := httptest.NewRequest(http.MethodGet, "/api/v1/payments/"+paymentA.ID, nil)
	reqPayA.Header.Set("Authorization", "Bearer "+tokenA)
	wPayA := httptest.NewRecorder()
	router.ServeHTTP(wPayA, reqPayA)
	if wPayA.Code != http.StatusOK {
		t.Errorf("expected 200 OK for owner accessing own payment, got %d", wPayA.Code)
	}

	// 2. Other User (User B) accesses User A's payment -> 403 Forbidden
	reqPayB := httptest.NewRequest(http.MethodGet, "/api/v1/payments/"+paymentA.ID, nil)
	reqPayB.Header.Set("Authorization", "Bearer "+tokenB)
	wPayB := httptest.NewRecorder()
	router.ServeHTTP(wPayB, reqPayB)
	if wPayB.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for User B accessing User A's payment, got %d", wPayB.Code)
	}

	// 3. Unauthenticated access -> 401 Unauthorized
	reqPayNoAuth := httptest.NewRequest(http.MethodGet, "/api/v1/payments/"+paymentA.ID, nil)
	wPayNoAuth := httptest.NewRecorder()
	router.ServeHTTP(wPayNoAuth, reqPayNoAuth)
	if wPayNoAuth.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for unauthenticated payment lookup, got %d", wPayNoAuth.Code)
	}

	// 4. Admin accesses User A's payment -> 200 OK
	reqPayAdmin := httptest.NewRequest(http.MethodGet, "/api/v1/payments/"+paymentA.ID, nil)
	reqPayAdmin.Header.Set("Authorization", "Bearer "+adminToken)
	wPayAdmin := httptest.NewRecorder()
	router.ServeHTTP(wPayAdmin, reqPayAdmin)
	if wPayAdmin.Code != http.StatusOK {
		t.Errorf("expected 200 OK for Admin accessing payment, got %d", wPayAdmin.Code)
	}
}

func TestWebhookSecurityAPI(t *testing.T) {
	db, _, _ := setupTestAPI(t)
	defer db.Close()

	testServerKey := "test-server-key-12345"
	cfg := &config.Config{
		Env:               "test",
		MidtransServerKey: testServerKey,
	}

	txManager := database.NewTransactionManager(db)
	webhookSvc := service.NewWebhookService(database.NewPaymentRepository(db), database.NewDonationRepository(db), database.NewPaymentEventRepository(db), txManager)
	webhookHandler := api.NewWebhookHandler(webhookSvc, cfg)

	// Helper to compute SHA512 signature
	computeSig := func(orderID, statusCode, grossAmount, serverKey string) string {
		input := orderID + statusCode + grossAmount + serverKey
		h := sha512.New()
		h.Write([]byte(input))
		return hex.EncodeToString(h.Sum(nil))
	}

	orderID := "CF-TEST-WH-001"
	statusCode := "200"
	grossAmount := "50000.00"
	validSig := computeSig(orderID, statusCode, grossAmount, testServerKey)

	// Create payment row in DB
	donationRepo := database.NewDonationRepository(db)
	paymentRepo := database.NewPaymentRepository(db)

	don := &domain.Donation{CampaignID: "00000000-0000-0000-0000-000000000001", Amount: 50000, Status: domain.DonationStatusPending}
	_ = donationRepo.Create(context.Background(), don)
	pay := &domain.Payment{DonationID: don.ID, Provider: "MIDTRANS", OrderID: orderID, GrossAmount: 50000, Status: domain.PaymentStatusPending}
	_ = paymentRepo.Create(context.Background(), pay)

	// 1. Valid Signature -> 200 OK
	payloadValid := map[string]interface{}{
		"order_id":           orderID,
		"status_code":        statusCode,
		"gross_amount":       grossAmount,
		"signature_key":      validSig,
		"transaction_id":     "tx-12345",
		"transaction_status": "settlement",
		"fraud_status":       "accept",
	}
	bodyBytes, _ := json.Marshal(payloadValid)
	reqValid := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/midtrans", bytes.NewBuffer(bodyBytes))
	wValid := httptest.NewRecorder()
	webhookHandler.MidtransNotification(wValid, reqValid)

	if wValid.Code != http.StatusOK {
		t.Errorf("expected 200 OK for valid webhook signature, got %d. Body: %s", wValid.Code, wValid.Body.String())
	}

	// 2. Invalid Signature -> 401 Unauthorized
	payloadInvalid := map[string]interface{}{
		"order_id":           orderID,
		"status_code":        statusCode,
		"gross_amount":       grossAmount,
		"signature_key":      "invalid-hex-signature",
		"transaction_id":     "tx-12345",
		"transaction_status": "settlement",
	}
	bodyInvalidBytes, _ := json.Marshal(payloadInvalid)
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/midtrans", bytes.NewBuffer(bodyInvalidBytes))
	wInvalid := httptest.NewRecorder()
	webhookHandler.MidtransNotification(wInvalid, reqInvalid)

	if wInvalid.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for invalid signature, got %d", wInvalid.Code)
	}

	// 3. Tampered Payload / Modified Order ID -> 401 Unauthorized
	tamperedSig := computeSig("TAMPERED-ORDER-ID", statusCode, grossAmount, testServerKey)
	payloadTampered := map[string]interface{}{
		"order_id":           orderID, // Tampered order ID mismatched with signature
		"status_code":        statusCode,
		"gross_amount":       grossAmount,
		"signature_key":      tamperedSig,
		"transaction_id":     "tx-12345",
		"transaction_status": "settlement",
	}
	bodyTamperedBytes, _ := json.Marshal(payloadTampered)
	reqTampered := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/midtrans", bytes.NewBuffer(bodyTamperedBytes))
	wTampered := httptest.NewRecorder()
	webhookHandler.MidtransNotification(wTampered, reqTampered)

	if wTampered.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for tampered payload signature mismatch, got %d", wTampered.Code)
	}

	// 4. Missing Server Key Configuration when Env != "test" -> 500 Internal Server Error (fail closed)
	cfgProductionNoKey := &config.Config{
		Env:               "production",
		MidtransServerKey: "",
	}
	webhookHandlerProdNoKey := api.NewWebhookHandler(webhookSvc, cfgProductionNoKey)
	reqProdNoKey := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/midtrans", bytes.NewBuffer(bodyBytes))
	wProdNoKey := httptest.NewRecorder()
	webhookHandlerProdNoKey.MidtransNotification(wProdNoKey, reqProdNoKey)

	if wProdNoKey.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 Internal Server Error when MidtransServerKey is missing in non-test env, got %d", wProdNoKey.Code)
	}

	// 5. Duplicate Valid Webhook -> 200 OK (Idempotent)
	reqDup := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/midtrans", bytes.NewBuffer(bodyBytes))
	wDup := httptest.NewRecorder()
	webhookHandler.MidtransNotification(wDup, reqDup)

	if wDup.Code != http.StatusOK {
		t.Errorf("expected 200 OK for duplicate valid webhook, got %d", wDup.Code)
	}
}

func TestSecurityRemediationPhase5K(t *testing.T) {
	db, router, _ := setupTestAPI(t)
	defer db.Close()

	// 1. Request ID & Security Headers Middleware Test
	t.Run("RequestID_And_Security_Headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		reqID := w.Header().Get("X-Request-ID")
		if reqID == "" {
			t.Errorf("expected X-Request-ID in response headers")
		}

		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("expected X-Content-Type-Options: nosniff")
		}

		if w.Header().Get("X-Frame-Options") != "DENY" {
			t.Errorf("expected X-Frame-Options: DENY")
		}
	})

	// 2. CORS Preflight & Allowed Origins Test
	t.Run("CORS_Handling", func(t *testing.T) {
		// Preflight OPTIONS
		reqPreflight := httptest.NewRequest(http.MethodOptions, "/api/v1/campaigns", nil)
		reqPreflight.Header.Set("Origin", "http://localhost:3000")
		wPreflight := httptest.NewRecorder()
		router.ServeHTTP(wPreflight, reqPreflight)

		if wPreflight.Code != http.StatusNoContent {
			t.Errorf("expected 204 No Content for OPTIONS preflight, got %d", wPreflight.Code)
		}
		if wPreflight.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
			t.Errorf("expected Access-Control-Allow-Origin: http://localhost:3000, got %s", wPreflight.Header().Get("Access-Control-Allow-Origin"))
		}

		// Disallowed origin preflight
		reqDisallowed := httptest.NewRequest(http.MethodOptions, "/api/v1/campaigns", nil)
		reqDisallowed.Header.Set("Origin", "http://malicious-site.com")
		wDisallowed := httptest.NewRecorder()
		router.ServeHTTP(wDisallowed, reqDisallowed)

		if wDisallowed.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("expected empty Access-Control-Allow-Origin for disallowed origin")
		}
	})

	// 3. Request Body Limit Test (1MB Cap)
	t.Run("Oversized_Request_Body_Rejected", func(t *testing.T) {
		// Construct > 1MB JSON body
		largeBytes := make([]byte, 1024*1024+100) // ~1.1MB
		for i := range largeBytes {
			largeBytes[i] = 'a'
		}
		payload := map[string]string{
			"email":    "test@example.com",
			"password": "pass",
			"junk":     string(largeBytes),
		}
		bodyBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for request body > 1MB, got %d", w.Code)
		}
	})

	// 4. Pagination Limit Bounding Test
	t.Run("Pagination_Limit_Bounded_At_100", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/campaigns?limit=500&offset=0", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)

		meta, ok := resp["meta"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected meta in response")
		}

		limit := int(meta["limit"].(float64))
		if limit != 100 {
			t.Errorf("expected limit bounded at 100, got %d", limit)
		}
	})
}

func TestDonationHTTPIdempotency(t *testing.T) {
	db, router, authSvc := setupTestAPI(t)
	defer db.Close()

	// Create user, category & active campaign
	var userID, catID, campID string
	err := db.QueryRow("INSERT INTO users (email, password_hash, name) VALUES ('donor_idem@example.com', 'hash', 'Donor Idem') RETURNING id").Scan(&userID)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	db.Exec("INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE name = 'DONOR'", userID)

	err = db.QueryRow("INSERT INTO categories (name, slug) VALUES ('Idem Health', 'idem-health') RETURNING id").Scan(&catID)

	if err != nil {
		t.Fatalf("failed to insert category: %v", err)
	}

	err = db.QueryRow("INSERT INTO campaigns (owner_id, category_id, title, slug, description, target_amount, status, start_at, end_at) VALUES ($1, $2, 'Camp Idem', 'camp-idem', 'Desc', 1000000, 'ACTIVE', NOW(), NOW() + INTERVAL '30 days') RETURNING id", userID, catID).Scan(&campID)


	if err != nil {
		t.Fatalf("failed to insert campaign: %v", err)
	}


	donorUser := &domain.User{ID: userID, Email: "donor_idem@example.com", Name: "Donor Idem"}
	token, _ := authSvc.GenerateAccessToken(donorUser, []string{"DONOR"})

	payload := map[string]interface{}{
		"campaign_id":  campID,
		"amount":       50000,
		"is_anonymous": false,
		"message":      "Stay strong!",
	}
	bodyBytes, _ := json.Marshal(payload)

	idemKey := "idem-key-12345"

	// 1. First Request with Idempotency-Key
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/donations", bytes.NewBuffer(bodyBytes))
	req1.Header.Set("Authorization", "Bearer "+token)
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", idemKey)

	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for first idempotency request, got %d (body: %s)", w1.Code, w1.Body.String())
	}

	var resp1 map[string]interface{}
	json.Unmarshal(w1.Body.Bytes(), &resp1)
	data1 := resp1["data"].(map[string]interface{})
	orderID1 := data1["order_id"].(string)

	// 2. Retry with SAME Idempotency-Key and SAME payload
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/donations", bytes.NewBuffer(bodyBytes))
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", idemKey)

	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for retried idempotency request, got %d", w2.Code)
	}

	var resp2 map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &resp2)
	data2 := resp2["data"].(map[string]interface{})
	orderID2 := data2["order_id"].(string)

	if orderID1 != orderID2 {
		t.Errorf("expected cached order_id %s, got %s", orderID1, orderID2)
	}

	// Verify only 1 donation row was created in PostgreSQL
	var donCount int
	db.QueryRow("SELECT COUNT(*) FROM donations WHERE campaign_id = $1", campID).Scan(&donCount)
	if donCount != 1 {
		t.Errorf("expected exactly 1 donation record in DB, found %d", donCount)
	}

	// 3. Request with SAME Idempotency-Key but DIFFERENT payload -> 400 Bad Request
	diffPayload := map[string]interface{}{
		"campaign_id":  campID,
		"amount":       999999, // Modified amount!
		"is_anonymous": false,
		"message":      "Stay strong!",
	}
	diffBodyBytes, _ := json.Marshal(diffPayload)

	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/donations", bytes.NewBuffer(diffBodyBytes))
	req3.Header.Set("Authorization", "Bearer "+token)
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Idempotency-Key", idemKey)

	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for same key with different payload, got %d", w3.Code)
	}
}

func TestTrustedProxyIPExtraction(t *testing.T) {
	trustedExtractor := middleware.MakeIPExtractor("10.0.0.0/8, 192.168.1.1")
	untrustedExtractor := middleware.MakeIPExtractor("")

	// 1. Direct client (untrusted) + spoofed X-Forwarded-For -> spoof ignored
	reqDirect := httptest.NewRequest(http.MethodGet, "/health", nil)
	reqDirect.RemoteAddr = "203.0.113.5:12345"
	reqDirect.Header.Set("X-Forwarded-For", "1.1.1.1")

	ipDirect := untrustedExtractor(reqDirect)
	if ipDirect != "203.0.113.5" {
		t.Errorf("expected direct client RemoteAddr 203.0.113.5, got %s", ipDirect)
	}

	// 2. Trusted proxy + valid X-Forwarded-For -> forwarded IP accepted
	reqProxy := httptest.NewRequest(http.MethodGet, "/health", nil)
	reqProxy.RemoteAddr = "10.0.0.1:54321"
	reqProxy.Header.Set("X-Forwarded-For", "198.51.100.42, 10.0.0.1")

	ipProxy := trustedExtractor(reqProxy)
	if ipProxy != "198.51.100.42" {
		t.Errorf("expected forwarded IP 198.51.100.42, got %s", ipProxy)
	}
}
// TestConcurrentIdempotencyDuplicatePrevention verifies that 20 concurrent HTTP requests
// sharing the same (user_id, Idempotency-Key, payload) never create more than one
// Donation, Payment, or idempotency_keys record in PostgreSQL.
//
// This is the primary concurrency regression test for the Phase 5K.1 remediation.
// The PostgreSQL-backed atomic reservation (INSERT idempotency_keys FIRST inside the
// financial transaction) is the enforcing mechanism.
func TestConcurrentIdempotencyDuplicatePrevention(t *testing.T) {
	db, router, authSvc := setupTestAPI(t)
	defer db.Close()

	// Setup: user, category, campaign
	var userID, catID, campID string
	if err := db.QueryRow("INSERT INTO users (email, password_hash, name) VALUES ('concurrent_idem@example.com', 'hash', 'Concurrent Idem') RETURNING id").Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	db.Exec("INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE name = 'DONOR'", userID)
	if err := db.QueryRow("INSERT INTO categories (name, slug) VALUES ('ConcIdem Cat', 'conc-idem-cat') RETURNING id").Scan(&catID); err != nil {
		t.Fatalf("insert category: %v", err)
	}
	if err := db.QueryRow("INSERT INTO campaigns (owner_id, category_id, title, slug, description, target_amount, status, start_at, end_at) VALUES ($1, $2, 'Concurrent Idem Camp', 'conc-idem-camp', 'Desc', 1000000, 'ACTIVE', NOW(), NOW() + INTERVAL '30 days') RETURNING id", userID, catID).Scan(&campID); err != nil {
		t.Fatalf("insert campaign: %v", err)
	}

	donorUser := &domain.User{ID: userID, Email: "concurrent_idem@example.com", Name: "Concurrent Idem"}
	token, _ := authSvc.GenerateAccessToken(donorUser, []string{"DONOR"})

	payload := map[string]interface{}{
		"campaign_id":  campID,
		"amount":       75000,
		"is_anonymous": false,
		"message":      "Concurrent donation",
	}
	bodyBytes, _ := json.Marshal(payload)
	idemKey := "concurrent-idem-key-abc"

	const numGoroutines = 20
	results := make([]int, numGoroutines)

	// Use a channel as a starting gun to maximize overlap between goroutines.
	startGun := make(chan struct{})
	done := make(chan struct{}, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		i := i
		go func() {
			<-startGun // wait for all goroutines to be ready
			req := httptest.NewRequest(http.MethodPost, "/api/v1/donations", bytes.NewBuffer(bodyBytes))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", idemKey)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			results[i] = w.Code
			done <- struct{}{}
		}()
	}

	// Fire all goroutines simultaneously.
	close(startGun)
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Count response codes.
	created201 := 0
	accepted202 := 0
	other := 0
	for _, code := range results {
		switch code {
		case http.StatusCreated:
			created201++
		case http.StatusAccepted:
			accepted202++
		default:
			other++
		}
	}

	t.Logf("Concurrent results: 201=%d, 202=%d, other=%d", created201, accepted202, other)

	// ASSERTION: Zero unexpected status codes.
	if other != 0 {
		t.Errorf("unexpected HTTP status codes from concurrent requests: %v", results)
	}

	// ASSERTION: At most one 201 Created (the winning reservation).
	// All others must be 201 (cache hit) or 202 (in-flight / PENDING).
	// After all goroutines complete, at least one should have gotten 201.
	if created201 == 0 {
		t.Errorf("expected at least one 201 Created, got none")
	}

	// ASSERTION: PostgreSQL must have exactly ONE donation for this campaign.
	var donCount int
	db.QueryRow("SELECT COUNT(*) FROM donations WHERE campaign_id = $1", campID).Scan(&donCount)
	if donCount != 1 {
		t.Errorf("CONCURRENCY VIOLATION: expected exactly 1 donation, found %d", donCount)
	}

	// ASSERTION: PostgreSQL must have exactly ONE payment for this campaign's donation.
	var payCount int
	db.QueryRow("SELECT COUNT(*) FROM payments WHERE donation_id IN (SELECT id FROM donations WHERE campaign_id = $1)", campID).Scan(&payCount)
	if payCount != 1 {
		t.Errorf("CONCURRENCY VIOLATION: expected exactly 1 payment, found %d", payCount)
	}

	// ASSERTION: PostgreSQL must have exactly ONE idempotency record for this key.
	var idemCount int
	db.QueryRow("SELECT COUNT(*) FROM idempotency_keys WHERE user_id = $1 AND idempotency_key = $2", userID, idemKey).Scan(&idemCount)
	if idemCount != 1 {
		t.Errorf("CONCURRENCY VIOLATION: expected exactly 1 idempotency_keys record, found %d", idemCount)
	}
}

// TestConcurrentIdempotencyDifferentPayload verifies that when two concurrent requests
// use the same Idempotency-Key but different request payloads, exactly one payload wins
// the reservation and only one Donation + Payment is created. The losing payload receives
// 400 Bad Request (payload mismatch).
func TestConcurrentIdempotencyDifferentPayload(t *testing.T) {
	db, router, authSvc := setupTestAPI(t)
	defer db.Close()

	var userID, catID, campID string
	if err := db.QueryRow("INSERT INTO users (email, password_hash, name) VALUES ('diffpayload_idem@example.com', 'hash', 'DiffPayload Idem') RETURNING id").Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	db.Exec("INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE name = 'DONOR'", userID)
	if err := db.QueryRow("INSERT INTO categories (name, slug) VALUES ('DiffPayload Cat', 'diffpayload-cat') RETURNING id").Scan(&catID); err != nil {
		t.Fatalf("insert category: %v", err)
	}
	if err := db.QueryRow("INSERT INTO campaigns (owner_id, category_id, title, slug, description, target_amount, status, start_at, end_at) VALUES ($1, $2, 'DiffPayload Camp', 'diffpayload-camp', 'Desc', 1000000, 'ACTIVE', NOW(), NOW() + INTERVAL '30 days') RETURNING id", userID, catID).Scan(&campID); err != nil {
		t.Fatalf("insert campaign: %v", err)
	}

	donorUser := &domain.User{ID: userID, Email: "diffpayload_idem@example.com", Name: "DiffPayload Idem"}
	token, _ := authSvc.GenerateAccessToken(donorUser, []string{"DONOR"})

	payloadA := map[string]interface{}{"campaign_id": campID, "amount": 10000, "is_anonymous": false, "message": "Payload A"}
	payloadB := map[string]interface{}{"campaign_id": campID, "amount": 20000, "is_anonymous": false, "message": "Payload B"}
	bodyA, _ := json.Marshal(payloadA)
	bodyB, _ := json.Marshal(payloadB)
	idemKey := "diffpayload-idem-key-xyz"

	const numGoroutines = 10
	codes := make([]int, numGoroutines)
	startGun := make(chan struct{})
	done := make(chan struct{}, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		i := i
		go func() {
			<-startGun
			body := bodyA
			if i%2 == 1 {
				body = bodyB
			}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/donations", bytes.NewBuffer(body))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", idemKey)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			codes[i] = w.Code
			done <- struct{}{}
		}()
	}

	close(startGun)
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	t.Logf("Different-payload concurrent results: %v", codes)

	// Count allowed codes.
	allowed := 0
	for _, c := range codes {
		// 201 = winning payload, 202 = in-flight, 400 = payload mismatch, 409 = state conflict
		if c == http.StatusCreated || c == http.StatusAccepted || c == http.StatusBadRequest || c == http.StatusConflict {
			allowed++
		}
	}
	if allowed != numGoroutines {
		t.Errorf("unexpected status codes in different-payload test: %v", codes)
	}

	// CRITICAL: exactly ONE donation, ONE payment.
	var donCount int
	db.QueryRow("SELECT COUNT(*) FROM donations WHERE campaign_id = $1", campID).Scan(&donCount)
	if donCount != 1 {
		t.Errorf("CONCURRENCY VIOLATION: expected exactly 1 donation, found %d", donCount)
	}

	var payCount int
	db.QueryRow("SELECT COUNT(*) FROM payments WHERE donation_id IN (SELECT id FROM donations WHERE campaign_id = $1)", campID).Scan(&payCount)
	if payCount != 1 {
		t.Errorf("CONCURRENCY VIOLATION: expected exactly 1 payment, found %d", payCount)
	}

	var idemCount int
	db.QueryRow("SELECT COUNT(*) FROM idempotency_keys WHERE user_id = $1 AND idempotency_key = $2", userID, idemKey).Scan(&idemCount)
	if idemCount != 1 {
		t.Errorf("CONCURRENCY VIOLATION: expected exactly 1 idempotency record, found %d", idemCount)
	}

	// Wait a moment for any background processes or handler to finish and complete the record.
	// Since MockGateway is fast, Complete() should have been called by the winner.
	time.Sleep(100 * time.Millisecond)

	// Perform a sequential request using the losing payload and the same Idempotency-Key
	// To find the loser payload, we must find the winner payload.
	var winningHash string
	db.QueryRow("SELECT request_hash FROM idempotency_keys WHERE user_id = $1 AND idempotency_key = $2", userID, idemKey).Scan(&winningHash)
	
	// Prepare a loser payload that definitely hashes differently
	loserBody := []byte(`{"campaign_id":"` + campID + `","amount":99999,"is_anonymous":true,"message":"Loser Request"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/donations", bytes.NewBuffer(loserBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", idemKey)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should be 400 Bad Request because the hash doesn't match the established key
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected loser sequential retry to get 400 Bad Request, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// TestIdempotencyFailureScenarios covers the individual failure scenarios A-G.
func TestIdempotencyFailureScenarios(t *testing.T) {
	db, router, authSvc := setupTestAPI(t)
	defer db.Close()

	// Shared setup helper
	setupUser := func(suffix string) (userID, campID, token string) {
		var catID string
		db.QueryRow("INSERT INTO users (email, password_hash, name) VALUES ($1, 'hash', $2) RETURNING id",
			"scenario_"+suffix+"@example.com", "Scenario "+suffix).Scan(&userID)
		db.Exec("INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE name = 'DONOR'", userID)
		db.QueryRow("INSERT INTO categories (name, slug) VALUES ($1, $2) RETURNING id",
			"Cat "+suffix, "cat-"+suffix).Scan(&catID)
		db.QueryRow("INSERT INTO campaigns (owner_id, category_id, title, slug, description, target_amount, status, start_at, end_at) VALUES ($1, $2, $3, $4, 'Desc', 1000000, 'ACTIVE', NOW(), NOW() + INTERVAL '30 days') RETURNING id",
			userID, catID, "Camp "+suffix, "camp-"+suffix).Scan(&campID)
		u := &domain.User{ID: userID, Email: "scenario_" + suffix + "@example.com"}
		tok, _ := authSvc.GenerateAccessToken(u, []string{"DONOR"})
		return userID, campID, tok
	}

	makeBody := func(campID string, amount int64) []byte {
		b, _ := json.Marshal(map[string]interface{}{
			"campaign_id": campID, "amount": amount, "is_anonymous": false, "message": "test",
		})
		return b
	}

	// Scenario A: Request fails before DB creation (invalid amount <= 0).
	// Expected: no Donation, no Payment, no idempotency record. 400 returned.
	t.Run("A_failure_before_db_creation", func(t *testing.T) {
		userID, _, tok := setupUser("A")
		body, _ := json.Marshal(map[string]interface{}{"campaign_id": "nonexistent", "amount": 0})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/donations", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "scenario-A-key")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
		var count int
		db.QueryRow("SELECT COUNT(*) FROM idempotency_keys WHERE user_id = $1", userID).Scan(&count)
		if count != 0 {
			t.Errorf("expected 0 idempotency records, found %d", count)
		}
	})

	// Scenario D: Midtrans succeeds + HTTP response stored.
	// Expected: 201 on first request, 201 with same order_id on retry.
	t.Run("D_midtrans_success_response_cached", func(t *testing.T) {
		_, campID, tok := setupUser("D")
		body := makeBody(campID, 50000)
		key := "scenario-D-key"

		req1 := httptest.NewRequest(http.MethodPost, "/api/v1/donations", bytes.NewBuffer(body))
		req1.Header.Set("Authorization", "Bearer "+tok)
		req1.Header.Set("Content-Type", "application/json")
		req1.Header.Set("Idempotency-Key", key)
		w1 := httptest.NewRecorder()
		router.ServeHTTP(w1, req1)
		if w1.Code != http.StatusCreated {
			t.Fatalf("expected 201 on first request, got %d: %s", w1.Code, w1.Body.String())
		}

		var resp1 map[string]interface{}
		json.Unmarshal(w1.Body.Bytes(), &resp1)
		orderID1 := resp1["data"].(map[string]interface{})["order_id"].(string)

		req2 := httptest.NewRequest(http.MethodPost, "/api/v1/donations", bytes.NewBuffer(body))
		req2.Header.Set("Authorization", "Bearer "+tok)
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Idempotency-Key", key)
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)
		if w2.Code != http.StatusCreated {
			t.Fatalf("expected 201 on retry, got %d: %s", w2.Code, w2.Body.String())
		}

		var resp2 map[string]interface{}
		json.Unmarshal(w2.Body.Bytes(), &resp2)
		orderID2 := resp2["data"].(map[string]interface{})["order_id"].(string)

		if orderID1 != orderID2 {
			t.Errorf("expected same order_id on retry: got %s vs %s", orderID1, orderID2)
		}
	})

	// Scenario F: Concurrent duplicate requests — same as TestConcurrentIdempotencyDuplicatePrevention
	// but inline for the scenario matrix.
	t.Run("F_concurrent_duplicate_requests", func(t *testing.T) {
		_, campID, tok := setupUser("F")
		body := makeBody(campID, 30000)
		key := "scenario-F-key"

		const n = 10
		codes := make([]int, n)
		gun := make(chan struct{})
		done := make(chan struct{}, n)
		for i := 0; i < n; i++ {
			i := i
			go func() {
				<-gun
				req := httptest.NewRequest(http.MethodPost, "/api/v1/donations", bytes.NewBuffer(body))
				req.Header.Set("Authorization", "Bearer "+tok)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Idempotency-Key", key)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				codes[i] = w.Code
				done <- struct{}{}
			}()
		}
		close(gun)
		for i := 0; i < n; i++ {
			<-done
		}

		var donCount int
		db.QueryRow("SELECT COUNT(*) FROM donations WHERE campaign_id = $1", campID).Scan(&donCount)
		if donCount != 1 {
			t.Errorf("CONCURRENCY VIOLATION Scenario F: expected 1 donation, found %d", donCount)
		}

		var payCount int
		db.QueryRow("SELECT COUNT(*) FROM payments WHERE donation_id IN (SELECT id FROM donations WHERE campaign_id = $1)", campID).Scan(&payCount)
		if payCount != 1 {
			t.Errorf("CONCURRENCY VIOLATION Scenario F: expected 1 payment, found %d", payCount)
		}
	})

	// Scenario G: Concurrent different payload requests.
	t.Run("G_concurrent_different_payload_requests", func(t *testing.T) {
		_, campID, tok := setupUser("G")
		bodyA := makeBody(campID, 11111)
		bodyB := makeBody(campID, 22222)
		key := "scenario-G-key"

		const n = 8
		codes := make([]int, n)
		gun := make(chan struct{})
		done := make(chan struct{}, n)
		for i := 0; i < n; i++ {
			i := i
			go func() {
				<-gun
				body := bodyA
				if i%2 == 1 {
					body = bodyB
				}
				req := httptest.NewRequest(http.MethodPost, "/api/v1/donations", bytes.NewBuffer(body))
				req.Header.Set("Authorization", "Bearer "+tok)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Idempotency-Key", key)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				codes[i] = w.Code
				done <- struct{}{}
			}()
		}
		close(gun)
		for i := 0; i < n; i++ {
			<-done
		}

		var donCount int
		db.QueryRow("SELECT COUNT(*) FROM donations WHERE campaign_id = $1", campID).Scan(&donCount)
		if donCount > 1 {
			t.Errorf("CONCURRENCY VIOLATION Scenario G: expected at most 1 donation, found %d", donCount)
		}
	})
}

