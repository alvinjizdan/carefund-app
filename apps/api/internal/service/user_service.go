package service

import (
	"context"

	"carefund-api/internal/database"
	"carefund-api/internal/domain"
)

type UserService interface {
	RegisterUser(ctx context.Context, email, password, name string) (*domain.User, error)
	Login(ctx context.Context, email, password string) (*domain.User, []string, error)
	GetUser(ctx context.Context, id string) (*domain.User, error)
}

type userService struct {
	userRepo domain.UserRepository
	roleRepo domain.RoleRepository
	authSvc  AuthService
	tx       database.TransactionManager
}

func NewUserService(userRepo domain.UserRepository, roleRepo domain.RoleRepository, authSvc AuthService, tx database.TransactionManager) UserService {
	return &userService{
		userRepo: userRepo,
		roleRepo: roleRepo,
		authSvc:  authSvc,
		tx:       tx,
	}
}

func (s *userService) RegisterUser(ctx context.Context, email, password, name string) (*domain.User, error) {
	if email == "" || password == "" || name == "" {
		return nil, domain.ErrInvalidInput
	}

	hash, err := s.authSvc.HashPassword(password)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		Email:        email,
		PasswordHash: hash,
		Name:         name,
		IsActive:     true,
	}

	err = s.tx.Do(ctx, func(txCtx context.Context) error {
		if err := s.userRepo.Create(txCtx, user); err != nil {
			return err
		}

		// Assign DONOR role by default
		role, err := s.roleRepo.FindByName(txCtx, "DONOR")
		if err != nil {
			return err
		}

		return s.roleRepo.AssignRole(txCtx, user.ID, role.ID)
	})

	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *userService) Login(ctx context.Context, email, password string) (*domain.User, []string, error) {
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, nil, domain.ErrNotFound // Or generic unauthorized to prevent enumeration
	}

	if !s.authSvc.VerifyPassword(password, user.PasswordHash) {
		return nil, nil, domain.ErrNotFound // Treat as not found/unauthorized
	}

	roles, err := s.roleRepo.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}

	roleNames := make([]string, len(roles))
	for i, r := range roles {
		roleNames[i] = r.Name
	}

	return user, roleNames, nil
}

func (s *userService) GetUser(ctx context.Context, id string) (*domain.User, error) {
	return s.userRepo.FindByID(ctx, id)
}
