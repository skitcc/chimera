package usecase

import (
	"context"
	"testing"

	"chimera/internal/domain"
)

func TestUserServiceList(t *testing.T) {
	runCase(t, "returns users", func(t *testing.T) {
		// Arrange
		service, users, _ := newUserFixture()
		user := validUser()
		users.users[user.ID] = user

		// Act
		got, err := service.List(context.Background())

		// Assert
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		assertEqual(t, []domain.User{user}, got)
	})

	runCase(t, "returns repository error", func(t *testing.T) {
		// Arrange
		service, users, _ := newUserFixture()
		users.listErr = errDependency

		// Act
		_, err := service.List(context.Background())

		// Assert
		assertErrorIs(t, err, errDependency)
	})
}

func TestUserServiceGetByID(t *testing.T) {
	runCase(t, "returns user", func(t *testing.T) {
		// Arrange
		service, users, _ := newUserFixture()
		user := validUser()
		users.users[user.ID] = user

		// Act
		got, err := service.GetByID(context.Background(), user.ID)

		// Assert
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		assertEqual(t, user, got)
	})

	runCase(t, "rejects empty id", func(t *testing.T) {
		// Arrange
		service, _, _ := newUserFixture()

		// Act
		_, err := service.GetByID(context.Background(), "")

		// Assert
		assertErrorCode(t, err, domain.CodeInvalid)
	})

	runCase(t, "returns repository error", func(t *testing.T) {
		// Arrange
		service, users, _ := newUserFixture()
		users.getErr = errDependency

		// Act
		_, err := service.GetByID(context.Background(), testUserID)

		// Assert
		assertErrorIs(t, err, errDependency)
	})
}

func TestUserServiceCreate(t *testing.T) {
	runCase(t, "hashes password and creates user", func(t *testing.T) {
		// Arrange
		service, users, _ := newUserFixture()
		in := validUserWrite()

		// Act
		got, err := service.Create(context.Background(), in)

		// Assert
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		assertEqual(t, validUser(), got)
		assertEqual(t, "hash:password", users.passwords[got.ID])
	})

	runCase(t, "rejects invalid input", func(t *testing.T) {
		// Arrange
		service, users, _ := newUserFixture()
		in := validUserWrite()
		in.Password = "short"

		// Act
		_, err := service.Create(context.Background(), in)

		// Assert
		assertErrorCode(t, err, domain.CodeInvalid)
		assertEqual(t, 0, len(users.users))
	})

	runCase(t, "returns hashing error", func(t *testing.T) {
		// Arrange
		service, _, hasher := newUserFixture()
		hasher.hashErr = errDependency

		// Act
		_, err := service.Create(context.Background(), validUserWrite())

		// Assert
		assertErrorIs(t, err, errDependency)
	})

	runCase(t, "returns repository error", func(t *testing.T) {
		// Arrange
		service, users, _ := newUserFixture()
		users.createErr = errDependency

		// Act
		_, err := service.Create(context.Background(), validUserWrite())

		// Assert
		assertErrorIs(t, err, errDependency)
	})
}

func TestUserServiceUpdate(t *testing.T) {
	runCase(t, "updates mutable fields", func(t *testing.T) {
		// Arrange
		service, users, _ := newUserFixture()
		users.users[testUserID] = domain.User{
			ID: testUserID, Email: "old@example.com", Name: "Old",
		}
		in := validUserWrite()

		// Act
		got, err := service.Update(context.Background(), testUserID, in)

		// Assert
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		assertEqual(t, validUser(), got)
	})

	runCase(t, "rejects empty id", func(t *testing.T) {
		// Arrange
		service, _, _ := newUserFixture()

		// Act
		_, err := service.Update(context.Background(), "", validUserWrite())

		// Assert
		assertErrorCode(t, err, domain.CodeInvalid)
	})

	runCase(t, "rejects invalid input", func(t *testing.T) {
		// Arrange
		service, _, _ := newUserFixture()
		in := validUserWrite()
		in.Email = "invalid"

		// Act
		_, err := service.Update(context.Background(), testUserID, in)

		// Assert
		assertErrorCode(t, err, domain.CodeInvalid)
	})

	runCase(t, "returns repository error", func(t *testing.T) {
		// Arrange
		service, users, _ := newUserFixture()
		users.updateErr = errDependency

		// Act
		_, err := service.Update(context.Background(), testUserID, validUserWrite())

		// Assert
		assertErrorIs(t, err, errDependency)
	})
}

func TestUserServiceDelete(t *testing.T) {
	runCase(t, "deletes user", func(t *testing.T) {
		// Arrange
		service, users, _ := newUserFixture()
		users.users[testUserID] = validUser()

		// Act
		err := service.Delete(context.Background(), testUserID)

		// Assert
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if _, ok := users.users[testUserID]; ok {
			t.Fatal("deleted user remains in repository")
		}
	})

	runCase(t, "rejects empty id", func(t *testing.T) {
		// Arrange
		service, _, _ := newUserFixture()

		// Act
		err := service.Delete(context.Background(), "")

		// Assert
		assertErrorCode(t, err, domain.CodeInvalid)
	})

	runCase(t, "returns repository error", func(t *testing.T) {
		// Arrange
		service, users, _ := newUserFixture()
		users.deleteErr = errDependency

		// Act
		err := service.Delete(context.Background(), testUserID)

		// Assert
		assertErrorIs(t, err, errDependency)
	})
}
