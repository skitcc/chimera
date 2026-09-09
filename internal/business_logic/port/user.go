package port

import (
	"context"

	"chimera/internal/business_logic/domain"
)

type UserService interface {
	List(ctx context.Context) ([]domain.User, error)
	GetByID(ctx context.Context, id string) (domain.User, error)
	Create(ctx context.Context, in domain.UserWrite) (domain.User, error)
	Update(ctx context.Context, id string, in domain.UserWrite) (domain.User, error)
	Delete(ctx context.Context, id string) error
}
