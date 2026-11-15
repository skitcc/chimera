package domain

// User is the Identity profile shown next to a follow. Social does not store it.
type User struct {
	ID    string
	Email string
	Name  string
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
