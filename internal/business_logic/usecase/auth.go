package usecase

import (
	"context"

	"chimera/internal/business_logic/apperrors"
	"chimera/internal/business_logic/domain"
	"chimera/internal/business_logic/port"
)

var _ port.AuthService = (*AuthService)(nil)

type AuthService struct{}

func NewAuthService() *AuthService {
	return &AuthService{}
}

func (s *AuthService) Register(_ context.Context, in domain.RegisterInput) (domain.AuthResult, error) {
	if in.Email == "" {
		return domain.AuthResult{}, apperrors.Invalid("email is required")
	}
	if in.Password == "" {
		return domain.AuthResult{}, apperrors.Invalid("password is required")
	}
	return domain.AuthResult{
		Token: "stub-token",
		User: domain.User{
			ID:    "stub-user",
			Email: in.Email,
			Name:  in.Name,
		},
	}, nil
}

func (s *AuthService) Login(_ context.Context, in domain.LoginInput) (domain.AuthResult, error) {
	if in.Email == "" || in.Password == "" {
		return domain.AuthResult{}, apperrors.Unauthorized("invalid credentials")
	}
	return domain.AuthResult{
		Token: "stub-token",
		User: domain.User{
			ID:    "stub-user",
			Email: in.Email,
		},
	}, nil
}
