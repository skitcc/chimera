package usecase

import (
	"context"

	"chimera/internal/business_logic/apperrors"
	"chimera/internal/business_logic/domain"
	"chimera/internal/business_logic/port"
)

var _ port.UserService = (*UserService)(nil)

type UserService struct{}

func NewUserService() *UserService {
	return &UserService{}
}

func (s *UserService) List(_ context.Context) ([]domain.User, error) {
	return []domain.User{}, nil
}

func (s *UserService) GetByID(_ context.Context, id string) (domain.User, error) {
	if id == "" {
		return domain.User{}, apperrors.Invalid("user id is required")
	}
	return domain.User{ID: id}, nil
}

func (s *UserService) Create(_ context.Context, in domain.UserWrite) (domain.User, error) {
	if in.Email == "" {
		return domain.User{}, apperrors.Invalid("email is required")
	}
	return domain.User{ID: "stub-user", Email: in.Email, Name: in.Name}, nil
}

func (s *UserService) Update(_ context.Context, id string, in domain.UserWrite) (domain.User, error) {
	if id == "" {
		return domain.User{}, apperrors.Invalid("user id is required")
	}
	return domain.User{ID: id, Email: in.Email, Name: in.Name}, nil
}

func (s *UserService) Delete(_ context.Context, id string) error {
	if id == "" {
		return apperrors.Invalid("user id is required")
	}
	return nil
}
