package usecase

import (
	"context"

	"chimera/internal/domain"
)

type UserService struct {
	users  UserRepository
	hasher PasswordHasher
}

func NewUserService(users UserRepository, hasher PasswordHasher) *UserService {
	return &UserService{users: users, hasher: hasher}
}

func (s *UserService) List(ctx context.Context) ([]domain.User, error) {
	return s.users.List(ctx)
}

func (s *UserService) GetByID(ctx context.Context, id string) (domain.User, error) {
	if err := domain.UserID(id).Validate(); err != nil {
		return domain.User{}, err
	}
	return s.users.GetByID(ctx, id)
}

func (s *UserService) Create(ctx context.Context, in domain.UserWrite) (domain.User, error) {
	if err := in.ValidateCreate(); err != nil {
		return domain.User{}, err
	}
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return domain.User{}, err
	}
	return s.users.Create(ctx, in.User(""), hash)
}

func (s *UserService) Update(ctx context.Context, id string, in domain.UserWrite) (domain.User, error) {
	if err := domain.UserID(id).Validate(); err != nil {
		return domain.User{}, err
	}
	if err := in.Validate(); err != nil {
		return domain.User{}, err
	}
	return s.users.Update(ctx, in.User(id))
}

func (s *UserService) Delete(ctx context.Context, id string) error {
	if err := domain.UserID(id).Validate(); err != nil {
		return err
	}
	return s.users.Delete(ctx, id)
}
