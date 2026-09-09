package port

import (
	"context"

	"chimera/internal/business_logic/domain"
)

type AuthService interface {
	Register(ctx context.Context, in domain.RegisterInput) (domain.AuthResult, error)
	Login(ctx context.Context, in domain.LoginInput) (domain.AuthResult, error)
}
