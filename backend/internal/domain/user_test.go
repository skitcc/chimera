package domain_test

import (
	"testing"

	"chimera/internal/domain"
	"chimera/internal/testkit"
)

func TestUserIDValidate(t *testing.T) {
	runCase(t, "accepts non-empty id", func(t *testing.T) {
		// Arrange
		id := domain.UserID("user-123")

		// Act
		err := id.Validate()

		// Assert
		assertNoError(t, err)
	})

	runCase(t, "rejects empty id", func(t *testing.T) {
		// Arrange
		id := domain.UserID("")

		// Act
		err := id.Validate()

		// Assert
		assertErrorCode(t, domain.CodeInvalid, err)
	})
}

func TestUserWriteValidate(t *testing.T) {
	runCase(t, "accepts valid write and normalizes email", func(t *testing.T) {
		// Arrange
		write := testkit.UserMother().
			WithEmail("  Listener@Example.COM  ").
			BuildWrite()

		// Act
		err := write.Validate()

		// Assert
		assertNoError(t, err)
		assertEqual(t, "listener@example.com", write.Email)
	})

	runCase(t, "does not require password for update", func(t *testing.T) {
		// Arrange
		write := testkit.UserMother().
			WithPassword("").
			BuildWrite()

		// Act
		err := write.Validate()

		// Assert
		assertNoError(t, err)
	})

	runCase(t, "rejects malformed email", func(t *testing.T) {
		// Arrange
		write := testkit.UserMother().
			WithEmail("listener.example.com").
			BuildWrite()

		// Act
		err := write.Validate()

		// Assert
		assertErrorCode(t, domain.CodeInvalid, err)
	})
}

func TestUserWriteValidateCreate(t *testing.T) {
	runCase(t, "accepts password at minimum boundary", func(t *testing.T) {
		// Arrange
		write := testkit.UserMother().
			WithPassword("12345678").
			BuildWrite()

		// Act
		err := write.ValidateCreate()

		// Assert
		assertNoError(t, err)
	})

	runCase(t, "normalizes valid email", func(t *testing.T) {
		// Arrange
		write := testkit.UserMother().
			WithEmail("  Listener@Example.COM  ").
			BuildWrite()

		// Act
		err := write.ValidateCreate()

		// Assert
		assertNoError(t, err)
		assertEqual(t, "listener@example.com", write.Email)
	})

	runCase(t, "rejects invalid email before password", func(t *testing.T) {
		// Arrange
		write := testkit.UserMother().
			WithEmail("").
			WithPassword("").
			BuildWrite()

		// Act
		err := write.ValidateCreate()

		// Assert
		assertErrorCode(t, domain.CodeInvalid, err)
		assertEqual(t, "invalid: valid email is required", err.Error())
	})

	runCase(t, "rejects password below minimum boundary", func(t *testing.T) {
		// Arrange
		write := testkit.UserMother().
			WithPassword("1234567").
			BuildWrite()

		// Act
		err := write.ValidateCreate()

		// Assert
		assertErrorCode(t, domain.CodeInvalid, err)
	})
}

func TestUserWriteUser(t *testing.T) {
	runCase(t, "maps write fields and supplied id", func(t *testing.T) {
		// Arrange
		write := testkit.UserMother().BuildWrite()

		// Act
		got := write.User("new-user")

		// Assert
		assertEqual(t, "new-user", got.ID)
		assertEqual(t, write.Email, got.Email)
		assertEqual(t, write.Name, got.Name)
	})

	runCase(t, "allows an empty supplied id without leaking password", func(t *testing.T) {
		// Arrange
		write := testkit.UserMother().
			WithPassword("secret-value").
			BuildWrite()

		// Act
		got := write.User("")

		// Assert
		assertEqual(t, "", got.ID)
		assertEqual(t, false, got.Email == write.Password)
	})
}
