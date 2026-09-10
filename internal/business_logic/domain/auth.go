package domain

import "chimera/internal/business_logic/apperrors"

type RegisterInput struct {
	Email    string
	Password string
	Name     string
}

func (in RegisterInput) Validate() error {
	if in.Email == "" {
		return apperrors.Invalid("email is required")
	}
	if in.Password == "" {
		return apperrors.Invalid("password is required")
	}
	return nil
}

func (in RegisterInput) User(id string) User {
	return User{ID: id, Email: in.Email, Name: in.Name}
}

type LoginInput struct {
	Email    string
	Password string
}

func (in LoginInput) Validate() error {
	if in.Email == "" {
		return apperrors.Invalid("email is required")
	}
	if in.Password == "" {
		return apperrors.Invalid("password is required")
	}
	return nil
}

func (in LoginInput) User(id string) User {
	return User{ID: id, Email: in.Email}
}

type AuthResult struct {
	Token string
	User  User
}
