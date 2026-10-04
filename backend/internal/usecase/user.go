package usecase

import (
	"context"

	"chimera/internal/domain"
)

type UserService struct {
	users    UserRepository
	profiles ProfileRepository
	hasher   PasswordHasher
}

func NewUserService(users UserRepository, hasher PasswordHasher) *UserService {
	profiles, _ := users.(ProfileRepository)
	return &UserService{users: users, profiles: profiles, hasher: hasher}
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

func (s *UserService) ReplaceCurrent(ctx context.Context, userID string, in domain.UserReplace) (domain.User, error) {
	if err := domain.UserID(userID).Validate(); err != nil {
		return domain.User{}, err
	}
	if err := in.Validate(); err != nil {
		return domain.User{}, err
	}
	if s.profiles == nil {
		return domain.User{}, domain.Internal("profile repository is unavailable")
	}
	user, err := s.profiles.UpdateProfile(ctx, domain.User{ID: userID, Email: in.Email, Name: in.Name}, nil)
	return user, currentUserError(err)
}

func (s *UserService) PatchCurrent(ctx context.Context, userID string, in domain.UserPatch) (domain.User, error) {
	if err := domain.UserID(userID).Validate(); err != nil {
		return domain.User{}, err
	}
	if err := in.Validate(); err != nil {
		return domain.User{}, err
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return domain.User{}, currentUserError(err)
	}
	if in.Email != nil {
		user.Email = *in.Email
	}
	if in.Name != nil {
		user.Name = *in.Name
	}
	var hash *string
	if in.Password != nil {
		value, err := s.hasher.Hash(*in.Password)
		if err != nil {
			return domain.User{}, err
		}
		hash = &value
	}
	if s.profiles == nil {
		return domain.User{}, domain.Internal("profile repository is unavailable")
	}
	user, err = s.profiles.UpdateProfile(ctx, user, hash)
	return user, currentUserError(err)
}

func (s *UserService) DeleteCurrent(ctx context.Context, userID string) error {
	if err := domain.UserID(userID).Validate(); err != nil {
		return err
	}
	if err := s.users.Delete(ctx, userID); err != nil {
		return currentUserError(err)
	}
	return nil
}

func currentUserError(err error) error {
	if domain.Is(err, domain.CodeNotFound) {
		return domain.Unauthorized("invalid token")
	}
	return err
}
