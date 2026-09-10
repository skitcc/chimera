package usecase

import (
	"context"

	"chimera/internal/business_logic/domain"
)

type AuthService struct{}

func NewAuthService() *AuthService {
	return &AuthService{}
}

func (s *AuthService) Register(_ context.Context, in domain.RegisterInput) (domain.AuthResult, error) {
	if err := in.Validate(); err != nil {
		return domain.AuthResult{}, err
	}
	return domain.AuthResult{
		Token: "stub-token",
		User:  in.User("stub-user"),
	}, nil
}

func (s *AuthService) Login(_ context.Context, in domain.LoginInput) (domain.AuthResult, error) {
	if err := in.Validate(); err != nil {
		return domain.AuthResult{}, err
	}
	return domain.AuthResult{
		Token: "stub-token",
		User:  in.User("stub-user"),
	}, nil
}
