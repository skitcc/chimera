package domain

import "chimera/internal/business_logic/apperrors"

type User struct {
	ID    string
	Email string
	Name  string
}

type UserID string

func (id UserID) Validate() error {
	if id == "" {
		return apperrors.Invalid("user id is required")
	}
	return nil
}

type UserWrite struct {
	Email string
	Name  string
}

func (w UserWrite) Validate() error {
	if w.Email == "" {
		return apperrors.Invalid("email is required")
	}
	return nil
}

func (w UserWrite) User(id string) User {
	return User{ID: id, Email: w.Email, Name: w.Name}
}
