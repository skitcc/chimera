package testkit

import "chimera/identity/internal/domain"

const (
	defaultUserID       = "user-123"
	defaultEmail        = "listener@example.com"
	defaultPassword     = "password123"
	defaultPasswordHash = "hashed-password"
	defaultName         = "Test Listener"
	defaultToken        = "test-token"
)

type UserBuilder struct {
	user         domain.User
	password     string
	passwordHash string
}

func UserMother() UserBuilder {
	return UserBuilder{
		user: domain.User{
			ID:    defaultUserID,
			Email: defaultEmail,
			Name:  defaultName,
		},
		password:     defaultPassword,
		passwordHash: defaultPasswordHash,
	}
}

func (b UserBuilder) WithID(id string) UserBuilder {
	b.user.ID = id
	return b
}

func (b UserBuilder) WithEmail(email string) UserBuilder {
	b.user.Email = email
	return b
}

func (b UserBuilder) WithName(name string) UserBuilder {
	b.user.Name = name
	return b
}

func (b UserBuilder) WithPassword(password string) UserBuilder {
	b.password = password
	return b
}

func (b UserBuilder) WithPasswordHash(passwordHash string) UserBuilder {
	b.passwordHash = passwordHash
	return b
}

func (b UserBuilder) Build() domain.User {
	return b.user
}

func (b UserBuilder) BuildAuthUser() domain.AuthUser {
	return domain.AuthUser{User: b.user, PasswordHash: b.passwordHash}
}

func (b UserBuilder) BuildWrite() domain.UserWrite {
	return domain.UserWrite{
		Email:    b.user.Email,
		Name:     b.user.Name,
		Password: b.password,
	}
}

type AuthBuilder struct {
	email    string
	password string
	name     string
	token    string
	user     domain.User
}

func AuthMother() AuthBuilder {
	return AuthBuilder{
		email:    defaultEmail,
		password: defaultPassword,
		name:     defaultName,
		token:    defaultToken,
		user:     UserMother().Build(),
	}
}

func (b AuthBuilder) WithEmail(email string) AuthBuilder {
	b.email = email
	return b
}

func (b AuthBuilder) WithPassword(password string) AuthBuilder {
	b.password = password
	return b
}

func (b AuthBuilder) WithName(name string) AuthBuilder {
	b.name = name
	return b
}

func (b AuthBuilder) WithToken(token string) AuthBuilder {
	b.token = token
	return b
}

func (b AuthBuilder) WithUser(user domain.User) AuthBuilder {
	b.user = user
	return b
}

func (b AuthBuilder) BuildRegisterInput() domain.RegisterInput {
	return domain.RegisterInput{
		Email:    b.email,
		Password: b.password,
		Name:     b.name,
	}
}

func (b AuthBuilder) BuildLoginInput() domain.LoginInput {
	return domain.LoginInput{
		Email:    b.email,
		Password: b.password,
	}
}

func (b AuthBuilder) BuildResult() domain.AuthResult {
	return domain.AuthResult{Token: b.token, User: b.user}
}
