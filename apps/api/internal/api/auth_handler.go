package api

import (
	"encoding/json"
	"net/http"
	"time"

	"carefund-api/internal/api/middleware"
	"carefund-api/internal/domain"
	"carefund-api/internal/service"
)

type AuthHandler struct {
	userSvc     service.UserService
	authSvc     service.AuthService
	rtRepo      domain.RefreshTokenRepository
	roleRepo    domain.RoleRepository
}

func NewAuthHandler(userSvc service.UserService, authSvc service.AuthService, rtRepo domain.RefreshTokenRepository, roleRepo domain.RoleRepository) *AuthHandler {
	return &AuthHandler{userSvc: userSvc, authSvc: authSvc, rtRepo: rtRepo, roleRepo: roleRepo}
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, r, domain.ErrInvalidInput)
		return
	}

	user, err := h.userSvc.RegisterUser(r.Context(), req.Email, req.Password, req.Name)
	if err != nil {
		RespondError(w, r, err)
		return
	}

	// Default role: DONOR
	roles := []string{"DONOR"}

	accessToken, err := h.authSvc.GenerateAccessToken(user, roles)
	if err != nil {
		RespondError(w, r, err)
		return
	}

	rawRefresh, hashRefresh := h.authSvc.GenerateRefreshToken()
	rt := &domain.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashRefresh,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}

	if err := h.rtRepo.Create(r.Context(), rt); err != nil {
		RespondError(w, r, err)
		return
	}

	RespondJSON(w, http.StatusCreated, SuccessResponse{
		Data: map[string]interface{}{
			"user":          user,
			"access_token":  accessToken,
			"refresh_token": rawRefresh,
		},
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, r, domain.ErrInvalidInput)
		return
	}

	user, roles, err := h.userSvc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		RespondError(w, r, err)
		return
	}

	accessToken, err := h.authSvc.GenerateAccessToken(user, roles)
	if err != nil {
		RespondError(w, r, err)
		return
	}

	rawRefresh, hashRefresh := h.authSvc.GenerateRefreshToken()
	rt := &domain.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashRefresh,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour), // 7 days
	}
	if err := h.rtRepo.Create(r.Context(), rt); err != nil {
		RespondError(w, r, err)
		return
	}

	RespondJSON(w, http.StatusOK, SuccessResponse{
		Data: map[string]interface{}{
			"user": map[string]interface{}{
				"id":    user.ID,
				"name":  user.Name,
				"email": user.Email,
				"roles": roles,
			},
			"access_token": accessToken,
			"refresh_token": rawRefresh,
		},
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, r, domain.ErrInvalidInput)
		return
	}


	if req.RefreshToken == "" {
		RespondError(w, r, domain.ErrInvalidInput)
		return
	}

	hash := h.authSvc.HashRefreshToken(req.RefreshToken)
	rt, err := h.rtRepo.FindByTokenHash(r.Context(), hash)
	if err != nil {
		// Treat not found as unauthorized
		RespondError(w, r, domain.ErrUnauthorized)
		return
	}

	if !rt.IsValid() {
		// If revoked or expired, we might want to also revoke all other tokens for safety, 
		// but simple revocation check is fine for now
		RespondError(w, r, domain.ErrUnauthorized)
		return
	}

	user, err := h.userSvc.GetUser(r.Context(), rt.UserID)
	if err != nil {
		RespondError(w, r, domain.ErrUnauthorized)
		return
	}

	roles, err := h.roleRepo.GetUserRoles(r.Context(), user.ID)
	if err != nil {
		RespondError(w, r, domain.ErrInternalError)
		return
	}
	
	roleNames := make([]string, len(roles))
	for i, role := range roles {
		roleNames[i] = role.Name
	}

	// Revoke old token (rotation)
	now := time.Now()
	rt.RevokedAt = &now
	rt.LastUsedAt = &now
	if err := h.rtRepo.Update(r.Context(), rt); err != nil {
		RespondError(w, r, domain.ErrInternalError)
		return
	}

	// Generate new access and refresh tokens
	accessToken, err := h.authSvc.GenerateAccessToken(user, roleNames)
	if err != nil {
		RespondError(w, r, err)
		return
	}

	newRawRefresh, newHashRefresh := h.authSvc.GenerateRefreshToken()
	newRt := &domain.RefreshToken{
		UserID:    user.ID,
		TokenHash: newHashRefresh,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	if err := h.rtRepo.Create(r.Context(), newRt); err != nil {
		RespondError(w, r, err)
		return
	}

	RespondJSON(w, http.StatusOK, SuccessResponse{
		Data: map[string]interface{}{
			"access_token": accessToken,
			"refresh_token": newRawRefresh,
		},
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// The client might provide the refresh token to revoke it, or we revoke all based on the authenticated user.
	// We'll revoke all for the authenticated user, or expect the refresh_token in body.
	
	authUser, ok := r.Context().Value(middleware.UserKey).(*middleware.AuthenticatedUser)
	if ok && authUser != nil {
		// Logged in user revokes all their refresh tokens
		if err := h.rtRepo.RevokeByUserID(r.Context(), authUser.ID); err != nil {
			// ignore or log
		}
	} else {
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.RefreshToken != "" {
			hash := h.authSvc.HashRefreshToken(req.RefreshToken)
			if rt, err := h.rtRepo.FindByTokenHash(r.Context(), hash); err == nil {
				now := time.Now()
				rt.RevokedAt = &now
				_ = h.rtRepo.Update(r.Context(), rt)
			}
		}
	}

	RespondJSON(w, http.StatusOK, SuccessResponse{Data: "logged-out"})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	authUser, ok := r.Context().Value(middleware.UserKey).(*middleware.AuthenticatedUser)
	if !ok {
		RespondError(w, r, domain.ErrUnauthorized)
		return
	}

	user, err := h.userSvc.GetUser(r.Context(), authUser.ID)
	if err != nil {
		RespondError(w, r, err)
		return
	}

	RespondJSON(w, http.StatusOK, SuccessResponse{
		Data: map[string]interface{}{
			"id":    user.ID,
			"name":  user.Name,
			"email": user.Email,
			"roles": authUser.Roles,
		},
	})
}
