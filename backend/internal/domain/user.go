package domain

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

func (id UserID) RequireActor(actorID string) error {
	if actorID == "" || string(id) != actorID {
		return Forbidden("not allowed")
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
	if !validEmail(w.Email) {
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

type UserReplace struct {
	Email string
	Name  string
}

func (in *UserReplace) Validate() error {
	in.Email = normalizeEmail(in.Email)
	if !validEmail(in.Email) {
		return Invalid("valid email is required")
	}
	return nil
}

type UserPatch struct {
	Email    *string
	Name     *string
	Password *string
}

func (in *UserPatch) Validate() error {
	if in.Email == nil && in.Name == nil && in.Password == nil {
		return Invalid("at least one field is required")
	}
	if in.Email != nil {
		email := normalizeEmail(*in.Email)
		if !validEmail(email) {
			return Invalid("valid email is required")
		}
		in.Email = &email
	}
	if in.Password != nil && len(*in.Password) < minPasswordLen {
		return Invalid("password must be at least 8 characters")
	}
	return nil
}
