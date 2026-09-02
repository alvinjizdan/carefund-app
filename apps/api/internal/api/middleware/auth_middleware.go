package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"carefund-api/internal/service"
	"github.com/golang-jwt/jwt/v5"
)

type userContextKey string

const UserKey userContextKey = "user"

type AuthenticatedUser struct {
	ID    string
	Email string
	Roles []string
}

func respondError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": msg,
		},
	})
}

func Auth(authSvc service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			token, err := authSvc.ValidateToken(tokenStr)
			if err != nil || !token.Valid {
				respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
				return
			}

			userID, _ := claims["sub"].(string)
			email, _ := claims["email"].(string)
			
			// Parse roles safely
			var roles []string
			if rawRoles, ok := claims["roles"].([]interface{}); ok {
				for _, r := range rawRoles {
					if strRole, ok := r.(string); ok {
						roles = append(roles, strRole)
					}
				}
			}

			authUser := &AuthenticatedUser{
				ID:    userID,
				Email: email,
				Roles: roles,
			}

			ctx := context.WithValue(r.Context(), UserKey, authUser)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := r.Context().Value(UserKey).(*AuthenticatedUser)
			if !ok {
				respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
				return
			}

			hasRole := false
			for _, r := range user.Roles {
				if r == role {
					hasRole = true
					break
				}
			}

			if !hasRole {
				respondError(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
