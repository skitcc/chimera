package domain

import "strings"

type User struct {
	ID    string
	Email string
	Name  string
}

type AuthUser struct {
	User         User
	PasswordHash string
}

type UserID string

func (id UserID) Validate() error {
	if id == "" {
		return Invalid("user id is required")
	}
	return nil
}

type UserWrite struct {
	Email    string
	Name     string
	Password string
}

func (w *UserWrite) Validate() error {
	w.Email = normalizeEmail(w.Email)
	if w.Email == "" || !strings.Contains(w.Email, "@") {
		return Invalid("valid email is required")
	}
	return nil
}

func (w *UserWrite) ValidateCreate() error {
	if err := w.Validate(); err != nil {
		return err
	}
	if len(w.Password) < minPasswordLen {
		return Invalid("password must be at least 8 characters")
	}
	return nil
}

func (w UserWrite) User(id string) User {
	return User{ID: id, Email: w.Email, Name: w.Name}
}
