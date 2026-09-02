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
	"carefund-api/internal/config"
	"carefund-api/internal/database"
	"carefund-api/internal/domain"
	"carefund-api/internal/infrastructure/payment/midtrans"
	"carefund-api/internal/service"
)

func setupTestAPI(t *testing.T) (*database.DB, *http.ServeMux, service.AuthService) {
	cfg := &config.Config{
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

	tables := []string{"payment_events", "refunds", "payments", "donations", "settlement_items", "settlements", "campaigns", "categories", "user_roles", "roles", "users"}
	for _, table := range tables {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM "+table)
	}

	_, _ = db.ExecContext(context.Background(), "INSERT INTO roles (name) VALUES ('DONOR'), ('CAMPAIGN_OWNER'), ('ADMIN')")

	txManager := database.NewTransactionManager(db)
	userRepo := database.NewUserRepository(db)
	roleRepo := database.NewRoleRepository(db)
	campRepo := database.NewCampaignRepository(db)

	rtRepo := database.NewRefreshTokenRepository(db)

	authSvc := service.NewAuthService("test-secret-key", 15*time.Minute)
	userSvc := service.NewUserService(userRepo, roleRepo, authSvc, txManager)
	campSvc := service.NewCampaignService(campRepo, txManager)

	mockGw := midtrans.NewMockPaymentGateway()
	donationSvc := service.NewDonationService(database.NewDonationRepository(db), database.NewPaymentRepository(db), campRepo, mockGw, txManager)
	
	webhookSvc := service.NewWebhookService(database.NewPaymentRepository(db), database.NewDonationRepository(db), database.NewPaymentEventRepository(db), txManager)

	router := api.NewRouter(authSvc, userSvc, campSvc, donationSvc, webhookSvc, rtRepo, roleRepo, cfg)

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

