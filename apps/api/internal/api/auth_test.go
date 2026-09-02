package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRefreshTokenFlow(t *testing.T) {
	db, router, _ := setupTestAPI(t)
	defer db.Close()

	// 1. Register
	reqBody := map[string]string{
		"name":     "API User 2",
		"email":    "api2@example.com",
		"password": "pass",
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	
	oldAccess := data["access_token"].(string)
	oldRefresh := data["refresh_token"].(string)

	if oldRefresh == "" {
		t.Fatalf("expected refresh token")
	}

	// 2. Refresh
	refBody := map[string]string{
		"refresh_token": oldRefresh,
	}
	refBytes, _ := json.Marshal(refBody)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBuffer(refBytes))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for refresh, got %d", w2.Code)
	}

	var refResp map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &refResp)
	refData := refResp["data"].(map[string]interface{})
	
	newAccess := refData["access_token"].(string)
	newRefresh := refData["refresh_token"].(string)

	if newAccess == oldAccess {
		t.Errorf("expected new access token")
	}
	if newRefresh == oldRefresh {
		t.Errorf("expected new refresh token (rotation)")
	}

	// 3. Try to use old refresh token again (should fail)
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBuffer(refBytes))
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for reused refresh token, got %d", w3.Code)
	}

	// 4. Logout
	req4 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req4.Header.Set("Authorization", "Bearer "+newAccess)
	w4 := httptest.NewRecorder()
	router.ServeHTTP(w4, req4)
	
	if w4.Code != http.StatusOK {
		t.Errorf("expected 200 OK for logout, got %d", w4.Code)
	}

	// 5. Try to refresh with newRefresh (should fail because logged out)
	newRefBody := map[string]string{
		"refresh_token": newRefresh,
	}
	newRefBytes, _ := json.Marshal(newRefBody)
	req5 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBuffer(newRefBytes))
	req5.Header.Set("Content-Type", "application/json")
	w5 := httptest.NewRecorder()
	router.ServeHTTP(w5, req5)

	if w5.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized after logout, got %d", w5.Code)
	}
}
