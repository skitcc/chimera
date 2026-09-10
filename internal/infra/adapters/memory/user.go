package memory

import (
	"context"
	"strconv"
	"sync"

	"chimera/internal/business_logic/apperrors"
	"chimera/internal/business_logic/domain"
)

type UserRepository struct {
	mu   sync.RWMutex
	byID map[string]domain.User
	seq  int
}

func NewUserRepository() *UserRepository {
	return &UserRepository{byID: map[string]domain.User{}}
}

func (r *UserRepository) List(_ context.Context) ([]domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]domain.User, 0, len(r.byID))
	for _, u := range r.byID {
		out = append(out, u)
	}
	return out, nil
}

func (r *UserRepository) GetByID(_ context.Context, id string) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	u, ok := r.byID[id]
	if !ok {
		return domain.User{}, apperrors.NotFound("user not found")
	}
	return u, nil
}

func (r *UserRepository) Create(_ context.Context, u domain.User) (domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.seq++
	u.ID = strconv.Itoa(r.seq)
	r.byID[u.ID] = u
	return u, nil
}

func (r *UserRepository) Update(_ context.Context, u domain.User) (domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byID[u.ID]; !ok {
		return domain.User{}, apperrors.NotFound("user not found")
	}
	r.byID[u.ID] = u
	return u, nil
}

func (r *UserRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byID[id]; !ok {
		return apperrors.NotFound("user not found")
	}
	delete(r.byID, id)
	return nil
}
