package usecase

import (
	"context"
	"errors"

	"chimera/identity/internal/domain"
)

type fakeUserRepository struct {
	users      map[string]domain.User
	passwords  map[string]string
	listErr    error
	getErr     error
	createErr  error
	updateErr  error
	deleteErr  error
	nextUserID string
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{
		users:      make(map[string]domain.User),
		passwords:  make(map[string]string),
		nextUserID: testUserID,
	}
}

func (f *fakeUserRepository) List(context.Context) ([]domain.User, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	users := make([]domain.User, 0, len(f.users))
	for _, user := range f.users {
		users = append(users, user)
	}
	return users, nil
}

func (f *fakeUserRepository) GetByID(_ context.Context, id string) (domain.User, error) {
	if f.getErr != nil {
		return domain.User{}, f.getErr
	}
	user, ok := f.users[id]
	if !ok {
		return domain.User{}, domain.NotFound("user not found")
	}
	return user, nil
}

func (f *fakeUserRepository) Create(_ context.Context, user domain.User, passwordHash string) (domain.User, error) {
	if f.createErr != nil {
		return domain.User{}, f.createErr
	}
	if user.ID == "" {
		user.ID = f.nextUserID
	}
	f.users[user.ID] = user
	f.passwords[user.ID] = passwordHash
	return user, nil
}

func (f *fakeUserRepository) Update(_ context.Context, user domain.User) (domain.User, error) {
	if f.updateErr != nil {
		return domain.User{}, f.updateErr
	}
	if _, ok := f.users[user.ID]; !ok {
		return domain.User{}, domain.NotFound("user not found")
	}
	f.users[user.ID] = user
	return user, nil
}

func (f *fakeUserRepository) Delete(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.users[id]; !ok {
		return domain.NotFound("user not found")
	}
	delete(f.users, id)
	delete(f.passwords, id)
	return nil
}

func (f *fakeUserRepository) GetMany(_ context.Context, ids []string) ([]domain.User, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	users := make([]domain.User, 0, len(ids))
	for _, id := range ids {
		if user, ok := f.users[id]; ok {
			users = append(users, user)
		}
	}
	return users, nil
}

func (f *fakeUserRepository) UpdateProfile(_ context.Context, user domain.User, passwordHash *string) (domain.User, error) {
	if f.updateErr != nil {
		return domain.User{}, f.updateErr
	}
	if _, ok := f.users[user.ID]; !ok {
		return domain.User{}, domain.NotFound("user not found")
	}
	f.users[user.ID] = user
	if passwordHash != nil {
		f.passwords[user.ID] = *passwordHash
	}
	return user, nil
}

func (f *fakeUserRepository) GetByEmail(_ context.Context, email string) (domain.AuthUser, error) {
	if f.getErr != nil {
		return domain.AuthUser{}, f.getErr
	}
	for id, user := range f.users {
		if user.Email == email {
			return domain.AuthUser{User: user, PasswordHash: f.passwords[id]}, nil
		}
	}
	return domain.AuthUser{}, domain.NotFound("user not found")
}

type fakePasswordHasher struct {
	hashErr    error
	compareErr error
}

func (f *fakePasswordHasher) Hash(password string) (string, error) {
	if f.hashErr != nil {
		return "", f.hashErr
	}
	return "hash:" + password, nil
}

func (f *fakePasswordHasher) Compare(hash, password string) error {
	if f.compareErr != nil {
		return f.compareErr
	}
	if hash != "hash:"+password {
		return errors.New("password mismatch")
	}
	return nil
}

type fakeTokenIssuer struct {
	token string
	err   error
}

func (f *fakeTokenIssuer) Issue(domain.User) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.token, nil
}
