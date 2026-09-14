package usecase

import (
	"context"

	"chimera/internal/domain"
)

type AuthService struct {
	users  AuthUserRepository
	hasher PasswordHasher
	tokens TokenIssuer
}

func NewAuthService(users AuthUserRepository, hasher PasswordHasher, tokens TokenIssuer) *AuthService {
	return &AuthService{users: users, hasher: hasher, tokens: tokens}
}

func (s *AuthService) Register(ctx context.Context, in domain.RegisterInput) (domain.AuthResult, error) {
	if err := in.Validate(); err != nil {
		return domain.AuthResult{}, err
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return domain.AuthResult{}, err
	}

	user, err := s.users.Create(ctx, in.User(), hash)
	if err != nil {
		return domain.AuthResult{}, err
	}

	token, err := s.tokens.Issue(user)
	if err != nil {
		return domain.AuthResult{}, err
	}

	return domain.AuthResult{Token: token, User: user}, nil
}

func (s *AuthService) Login(ctx context.Context, in domain.LoginInput) (domain.AuthResult, error) {
	if err := in.Validate(); err != nil {
		return domain.AuthResult{}, err
	}

	acc, err := s.users.GetByEmail(ctx, in.Email)
	if err != nil {
		if domain.Is(err, domain.CodeNotFound) {
			return domain.AuthResult{}, domain.Unauthorized("invalid credentials")
		}
		return domain.AuthResult{}, err
	}

	if err := s.hasher.Compare(acc.PasswordHash, in.Password); err != nil {
		return domain.AuthResult{}, err
	}

	token, err := s.tokens.Issue(acc.User)
	if err != nil {
		return domain.AuthResult{}, err
	}

	return domain.AuthResult{Token: token, User: acc.User}, nil
}
