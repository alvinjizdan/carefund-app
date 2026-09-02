package service_test

import (
	"testing"
	"time"

	"carefund-api/internal/domain"
	"carefund-api/internal/service"
	"github.com/golang-jwt/jwt/v5"
)

func TestJWTValidation(t *testing.T) {
	authSvc := service.NewAuthService("secret", 10*time.Minute)
	user := &domain.User{ID: "uuid-123", Email: "test@example.com"}

	// 1. Valid signature
	token, err := authSvc.GenerateAccessToken(user, []string{"DONOR"})
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}

	parsed, err := authSvc.ValidateToken(token)
	if err != nil || !parsed.Valid {
		t.Fatalf("expected valid token")
	}

	claims := parsed.Claims.(jwt.MapClaims)
	if claims["sub"] != "uuid-123" {
		t.Errorf("missing or invalid sub")
	}
	if claims["exp"] == nil {
		t.Errorf("missing exp")
	}

	// 2. Invalid signature
	authSvcBad := service.NewAuthService("wrong-secret", 10*time.Minute)
	_, err = authSvcBad.ValidateToken(token)
	if err == nil {
		t.Errorf("expected invalid signature to fail")
	}

	// 3. Expired token
	authSvcFast := service.NewAuthService("secret", -1*time.Minute) // instantly expires
	expiredToken, _ := authSvcFast.GenerateAccessToken(user, []string{"DONOR"})

	_, err = authSvc.ValidateToken(expiredToken)
	if err == nil {
		t.Errorf("expected expired token to fail")
	}

	// 4. Malformed token
	_, err = authSvc.ValidateToken("header.payload")
	if err == nil {
		t.Errorf("expected malformed token to fail")
	}
}
