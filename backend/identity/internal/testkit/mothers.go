package testkit

import "chimera/identity/internal/domain"

var (
	Users UserObjectMother
	Auth  AuthObjectMother
)

type UserObjectMother struct{}

func (UserObjectMother) Alice() domain.User {
	return UserMother().
		WithID("user-alice").
		WithEmail("alice@example.com").
		WithName("Alice").
		Build()
}

func (UserObjectMother) Bob() domain.User {
	return UserMother().
		WithID("user-bob").
		WithEmail("bob@example.com").
		WithName("Bob").
		Build()
}

type AuthObjectMother struct{}

func (AuthObjectMother) ValidRegister() domain.RegisterInput {
	return AuthMother().BuildRegisterInput()
}

func (AuthObjectMother) ValidLogin() domain.LoginInput {
	return AuthMother().BuildLoginInput()
}
