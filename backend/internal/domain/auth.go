package domain

import (
	"net/mail"
	"strings"
)

const minPasswordLen = 8

type RegisterInput struct {
	Email    string
	Password string
	Name     string
}

func (in *RegisterInput) Validate() error {
	in.Email = normalizeEmail(in.Email)
	if !validEmail(in.Email) {
		return Invalid("valid email is required")
	}
	if len(in.Password) < minPasswordLen {
		return Invalid("password must be at least 8 characters")
	}
	return nil
}

func (in RegisterInput) User() User {
	return User{Email: in.Email, Name: in.Name}
}

type LoginInput struct {
	Email    string
	Password string
}

func (in *LoginInput) Validate() error {
	in.Email = normalizeEmail(in.Email)
	if !validEmail(in.Email) {
		return Invalid("valid email is required")
	}
	if in.Password == "" {
		return Invalid("password is required")
	}
	return nil
}

type AuthResult struct {
	Token string
	User  User
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validEmail(email string) bool {
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email
}
