package domain_test

import (
	"testing"

	"chimera/internal/domain"
	"chimera/internal/testkit"
)

func TestRegisterInputValidate(t *testing.T) {
	runCase(t, "accepts valid input and normalizes email", func(t *testing.T) {
		// Arrange
		in := testkit.AuthMother().
			WithEmail("  Listener@Example.COM  ").
			BuildRegisterInput()

		// Act
		err := in.Validate()

		// Assert
		assertNoError(t, err)
		assertEqual(t, "listener@example.com", in.Email)
	})

	runCase(t, "accepts password at minimum boundary", func(t *testing.T) {
		// Arrange
		in := testkit.AuthMother().
			WithPassword("12345678").
			BuildRegisterInput()

		// Act
		err := in.Validate()

		// Assert
		assertNoError(t, err)
	})

	runCase(t, "rejects malformed email", func(t *testing.T) {
		// Arrange
		in := testkit.AuthMother().
			WithEmail("listener.example.com").
			BuildRegisterInput()

		// Act
		err := in.Validate()

		// Assert
		assertErrorCode(t, domain.CodeInvalid, err)
	})

	runCase(t, "rejects password below minimum boundary", func(t *testing.T) {
		// Arrange
		in := testkit.AuthMother().
			WithPassword("1234567").
			BuildRegisterInput()

		// Act
		err := in.Validate()

		// Assert
		assertErrorCode(t, domain.CodeInvalid, err)
	})
}

func TestRegisterInputUser(t *testing.T) {
	runCase(t, "maps email and name", func(t *testing.T) {
		// Arrange
		in := testkit.Auth.ValidRegister()

		// Act
		got := in.User()

		// Assert
		assertEqual(t, in.Email, got.Email)
		assertEqual(t, in.Name, got.Name)
	})

	runCase(t, "does not populate an id or password", func(t *testing.T) {
		// Arrange
		in := testkit.AuthMother().
			WithPassword("secret-value").
			BuildRegisterInput()

		// Act
		got := in.User()

		// Assert
		assertEqual(t, "", got.ID)
		assertEqual(t, false, got.Email == in.Password)
	})
}

func TestLoginInputValidate(t *testing.T) {
	runCase(t, "accepts credentials and normalizes email", func(t *testing.T) {
		// Arrange
		in := testkit.AuthMother().
			WithEmail("  Listener@Example.COM  ").
			BuildLoginInput()

		// Act
		err := in.Validate()

		// Assert
		assertNoError(t, err)
		assertEqual(t, "listener@example.com", in.Email)
	})

	runCase(t, "accepts any non-empty password", func(t *testing.T) {
		// Arrange
		in := testkit.AuthMother().
			WithPassword("x").
			BuildLoginInput()

		// Act
		err := in.Validate()

		// Assert
		assertNoError(t, err)
	})

	runCase(t, "rejects blank email after normalization", func(t *testing.T) {
		// Arrange
		in := testkit.AuthMother().
			WithEmail("   ").
			BuildLoginInput()

		// Act
		err := in.Validate()

		// Assert
		assertErrorCode(t, domain.CodeInvalid, err)
	})

	runCase(t, "rejects empty password", func(t *testing.T) {
		// Arrange
		in := testkit.AuthMother().
			WithPassword("").
			BuildLoginInput()

		// Act
		err := in.Validate()

		// Assert
		assertErrorCode(t, domain.CodeInvalid, err)
	})
}
