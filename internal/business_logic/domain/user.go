package domain

type User struct {
	ID    string
	Email string
	Name  string
}

type UserWrite struct {
	Email string
	Name  string
}
