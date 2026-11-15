package usecase

import (
	"context"

	"chimera/identity/internal/domain"
)

type UserRepository interface {
	List(ctx context.Context) ([]domain.User, error)
	GetByID(ctx context.Context, id string) (domain.User, error)
	GetMany(ctx context.Context, ids []string) ([]domain.User, error)
	Create(ctx context.Context, u domain.User, passwordHash string) (domain.User, error)
	Update(ctx context.Context, u domain.User) (domain.User, error)
	// Delete removes the account and records UserDeleted for the other services in the same commit.
	Delete(ctx context.Context, id string) error
}

type ProfileRepository interface {
	UpdateProfile(ctx context.Context, user domain.User, passwordHash *string) (domain.User, error)
}

type AuthUserRepository interface {
	Create(ctx context.Context, u domain.User, passwordHash string) (domain.User, error)
	GetByEmail(ctx context.Context, email string) (domain.AuthUser, error)
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

type TokenIssuer interface {
	Issue(user domain.User) (string, error)
}
