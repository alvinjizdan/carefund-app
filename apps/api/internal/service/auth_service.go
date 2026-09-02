package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"

	"carefund-api/internal/domain"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	HashPassword(password string) (string, error)
	VerifyPassword(password, hash string) bool
	GenerateAccessToken(user *domain.User, roles []string) (string, error)
	ValidateToken(tokenStr string) (*jwt.Token, error)
	GenerateRefreshToken() (string, string)
	HashRefreshToken(token string) string
}

type authService struct {
	secret []byte
	ttl    time.Duration
}

func NewAuthService(secret string, ttl time.Duration) AuthService {
	return &authService{secret: []byte(secret), ttl: ttl}
}

func (s *authService) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", domain.ErrInternalError
	}
	return string(bytes), nil
}

func (s *authService) VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func (s *authService) GenerateAccessToken(user *domain.User, roles []string) (string, error) {
	claims := jwt.MapClaims{
		"sub":   user.ID,
		"roles": roles,
		"jti":   generateJTI(),
		"exp":   time.Now().Add(s.ttl).Unix(),
		"iat":   time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *authService) ValidateToken(tokenStr string) (*jwt.Token, error) {
	return jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, domain.ErrInvalidInput
		}
		return s.secret, nil
	})
}

func (s *authService) GenerateRefreshToken() (string, string) {
	b := make([]byte, 32)
	rand.Read(b)
	token := base64.URLEncoding.EncodeToString(b)
	return token, s.HashRefreshToken(token)
}

func (s *authService) HashRefreshToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func generateJTI() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
