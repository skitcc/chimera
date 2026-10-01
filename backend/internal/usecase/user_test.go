package usecase

import (
	"context"
	"testing"

	"chimera/internal/domain"
	"chimera/internal/testkit"
)

const userComponent = "UserService"

func seedOldUser(users *fakeUserRepository) domain.User {
	old := domain.User{ID: testUserID, Email: "old@example.com", Name: "Old"}
	users.users[old.ID] = old
	users.passwords[old.ID] = "hash:old-password"
	return old
}

func TestUserServiceList(t *testing.T) {
	const method = "List"

	runSpec(t, userComponent, testkit.Spec{
		ID: "UC-USR-LIST-01", Method: method,
		Title:     "returns stored users",
		Given:     "one stored user",
		When:      "List is called",
		Then:      "a slice with exactly that user is returned",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var service *UserService
		var got []domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			var users *fakeUserRepository
			service, users, _ = newUserFixture()
			users.users[testUserID] = validUser()
		})
		r.Act(func(t *testing.T) { got, err = service.List(context.Background()) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, []domain.User{validUser()}, got)
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "UC-USR-LIST-02", Method: method,
		Title:     "returns repository list error",
		Given:     "a user repository whose List fails",
		When:      "List is called",
		Then:      "the repository error is returned unchanged with a nil slice",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *UserService
		var got []domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			var users *fakeUserRepository
			service, users, _ = newUserFixture()
			users.listErr = errDependency
		})
		r.Act(func(t *testing.T) { got, err = service.List(context.Background()) })
		r.Assert(func(t *testing.T) {
			assertErrorIs(t, err, errDependency)
			assertEqual(t, 0, len(got))
		})
	})
}

func TestUserServiceGetByID(t *testing.T) {
	const method = "GetByID"

	runSpec(t, userComponent, testkit.Spec{
		ID: "UC-USR-GET-01", Method: method,
		Title:     "returns user by id",
		Given:     "a stored user user-1",
		When:      "GetByID is called with user-1",
		Then:      "the stored user is returned",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"id": testUserID},
	}, func(t *testing.T, r testkit.Report) {
		var service *UserService
		var got domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			var users *fakeUserRepository
			service, users, _ = newUserFixture()
			users.users[testUserID] = validUser()
		})
		r.Act(func(t *testing.T) { got, err = service.GetByID(context.Background(), testUserID) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, validUser(), got)
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "UC-USR-GET-02", Method: method,
		Title:     "rejects empty id before lookup",
		Given:     "a user repository whose lookup is set to fail",
		When:      "GetByID is called with an empty id",
		Then:      "invalid \"user id is required\" is returned instead of the repository error",
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"id": ""},
	}, func(t *testing.T, r testkit.Report) {
		var service *UserService
		var err error
		r.Arrange(func(t *testing.T) {
			var users *fakeUserRepository
			service, users, _ = newUserFixture()
			users.getErr = errDependency
		})
		r.Act(func(t *testing.T) { _, err = service.GetByID(context.Background(), "") })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeInvalid, "user id is required") })
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "UC-USR-GET-03", Method: method,
		Title:     "propagates not found for unknown id",
		Given:     "no stored users",
		When:      "GetByID is called with user-1",
		Then:      "the repository not_found \"user not found\" error is returned",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"id": testUserID},
	}, func(t *testing.T, r testkit.Report) {
		var service *UserService
		var err error
		r.Arrange(func(t *testing.T) { service, _, _ = newUserFixture() })
		r.Act(func(t *testing.T) { _, err = service.GetByID(context.Background(), testUserID) })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeNotFound, "user not found") })
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "UC-USR-GET-04", Method: method,
		Title:     "returns repository lookup error",
		Given:     "a user repository whose GetByID fails",
		When:      "GetByID is called with user-1",
		Then:      "the repository error is returned unchanged",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *UserService
		var err error
		r.Arrange(func(t *testing.T) {
			var users *fakeUserRepository
			service, users, _ = newUserFixture()
			users.getErr = errDependency
		})
		r.Act(func(t *testing.T) { _, err = service.GetByID(context.Background(), testUserID) })
		r.Assert(func(t *testing.T) { assertErrorIs(t, err, errDependency) })
	})
}

func TestUserServiceCreate(t *testing.T) {
	const method = "Create"

	runSpec(t, userComponent, testkit.Spec{
		ID: "UC-USR-CREATE-01", Method: method,
		Title:     "normalizes email, hashes password and stores user",
		Given:     "a user write with padded uppercase email and the 8-character password \"password\"",
		When:      "Create is called",
		Then:      "the user user-1 with lowercase email is returned and the stored hash is hash:password",
		Technique: testkit.TechniqueEquivalence,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"email": " LISTENER@EXAMPLE.COM ", "password length": "8"},
	}, func(t *testing.T, r testkit.Report) {
		var service *UserService
		var users *fakeUserRepository
		var in domain.UserWrite
		var got domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			service, users, _ = newUserFixture()
			in = validUserWrite()
			in.Email = " LISTENER@EXAMPLE.COM "
		})
		r.Act(func(t *testing.T) { got, err = service.Create(context.Background(), in) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, validUser(), got)
			assertEqual(t, validUser(), users.users[testUserID])
			assertEqual(t, "hash:password", users.passwords[testUserID])
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "UC-USR-CREATE-02", Method: method,
		Title:     "rejects password of length 7 before hashing",
		Given:     "a valid email, a 7-character password and a hasher set to fail",
		When:      "Create is called",
		Then:      "invalid \"password must be at least 8 characters\" is returned instead of the hashing error, and no user is stored",
		Technique: testkit.TechniqueBoundary,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"password length": "7"},
	}, func(t *testing.T, r testkit.Report) {
		var service *UserService
		var users *fakeUserRepository
		var in domain.UserWrite
		var err error
		r.Arrange(func(t *testing.T) {
			var hasher *fakePasswordHasher
			service, users, hasher = newUserFixture()
			hasher.hashErr = errDependency
			in = validUserWrite()
			in.Password = "1234567"
		})
		r.Act(func(t *testing.T) { _, err = service.Create(context.Background(), in) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInvalid, "password must be at least 8 characters")
			assertEqual(t, 0, len(users.users))
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "UC-USR-CREATE-03", Method: method,
		Title:     "reports invalid email before short password",
		Given:     "an email without @ and a 5-character password",
		When:      "Create is called",
		Then:      "invalid \"valid email is required\" is returned and no user is stored",
		Technique: testkit.TechniqueDecisionTable,
		Params:    map[string]string{"email": "invalid", "password length": "5"},
	}, func(t *testing.T, r testkit.Report) {
		var service *UserService
		var users *fakeUserRepository
		var in domain.UserWrite
		var err error
		r.Arrange(func(t *testing.T) {
			service, users, _ = newUserFixture()
			in = domain.UserWrite{Email: "invalid", Name: "Listener", Password: "short"}
		})
		r.Act(func(t *testing.T) { _, err = service.Create(context.Background(), in) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInvalid, "valid email is required")
			assertEqual(t, 0, len(users.users))
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "UC-USR-CREATE-04", Method: method,
		Title:     "returns hashing error without storing user",
		Given:     "a valid user write and a hasher that fails",
		When:      "Create is called",
		Then:      "the hasher error is returned unchanged and no user is stored",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *UserService
		var users *fakeUserRepository
		var err error
		r.Arrange(func(t *testing.T) {
			var hasher *fakePasswordHasher
			service, users, hasher = newUserFixture()
			hasher.hashErr = errDependency
		})
		r.Act(func(t *testing.T) { _, err = service.Create(context.Background(), validUserWrite()) })
		r.Assert(func(t *testing.T) {
			assertErrorIs(t, err, errDependency)
			assertEqual(t, 0, len(users.users))
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "UC-USR-CREATE-05", Method: method,
		Title:     "returns repository create error",
		Given:     "a valid user write and a user repository whose Create fails",
		When:      "Create is called",
		Then:      "the repository error is returned unchanged with an empty user",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *UserService
		var got domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			var users *fakeUserRepository
			service, users, _ = newUserFixture()
			users.createErr = errDependency
		})
		r.Act(func(t *testing.T) { got, err = service.Create(context.Background(), validUserWrite()) })
		r.Assert(func(t *testing.T) {
			assertErrorIs(t, err, errDependency)
			assertEqual(t, domain.User{}, got)
		})
	})
}

func TestUserServiceUpdate(t *testing.T) {
	const method = "Update"

	runSpec(t, userComponent, testkit.Spec{
		ID: "UC-USR-UPDATE-01", Method: method,
		Title:     "updates email and name but not password",
		Given:     "a stored user old@example.com with hash of \"old-password\"",
		When:      "Update is called for user-1 with new email, name and password \"password\"",
		Then:      "the updated user is returned and stored, and the stored password hash is unchanged",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"id": testUserID},
	}, func(t *testing.T, r testkit.Report) {
		var service *UserService
		var users *fakeUserRepository
		var got domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			service, users, _ = newUserFixture()
			seedOldUser(users)
		})
		r.Act(func(t *testing.T) { got, err = service.Update(context.Background(), testUserID, validUserWrite()) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, validUser(), got)
			assertEqual(t, validUser(), users.users[testUserID])
			assertEqual(t, "hash:old-password", users.passwords[testUserID])
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "UC-USR-UPDATE-02", Method: method,
		Title:     "reports empty id before invalid email",
		Given:     "a stored user and an input with an email without @",
		When:      "Update is called with an empty id",
		Then:      "invalid \"user id is required\" is returned and the stored user is unchanged",
		Technique: testkit.TechniqueDecisionTable,
		Params:    map[string]string{"id": "", "email": "invalid"},
	}, func(t *testing.T, r testkit.Report) {
		var service *UserService
		var users *fakeUserRepository
		var old domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			service, users, _ = newUserFixture()
			old = seedOldUser(users)
		})
		r.Act(func(t *testing.T) {
			_, err = service.Update(context.Background(), "", domain.UserWrite{Email: "invalid"})
		})
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInvalid, "user id is required")
			assertEqual(t, old, users.users[testUserID])
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "UC-USR-UPDATE-03", Method: method,
		Title:     "rejects invalid email without updating",
		Given:     "a stored user user-1",
		When:      "Update is called for user-1 with an email without @",
		Then:      "invalid \"valid email is required\" is returned and the stored user is unchanged",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"email": "invalid"},
	}, func(t *testing.T, r testkit.Report) {
		var service *UserService
		var users *fakeUserRepository
		var old domain.User
		var in domain.UserWrite
		var err error
		r.Arrange(func(t *testing.T) {
			service, users, _ = newUserFixture()
			old = seedOldUser(users)
			in = validUserWrite()
			in.Email = "invalid"
		})
		r.Act(func(t *testing.T) { _, err = service.Update(context.Background(), testUserID, in) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInvalid, "valid email is required")
			assertEqual(t, old, users.users[testUserID])
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "UC-USR-UPDATE-04", Method: method,
		Title:     "propagates not found for unknown user",
		Given:     "no stored users",
		When:      "Update is called for user-1 with a valid input",
		Then:      "not_found \"user not found\" is returned and no user is created",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var service *UserService
		var users *fakeUserRepository
		var err error
		r.Arrange(func(t *testing.T) { service, users, _ = newUserFixture() })
		r.Act(func(t *testing.T) { _, err = service.Update(context.Background(), testUserID, validUserWrite()) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeNotFound, "user not found")
			assertEqual(t, 0, len(users.users))
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "UC-USR-UPDATE-05", Method: method,
		Title:     "returns repository update error",
		Given:     "a user repository whose Update fails",
		When:      "Update is called for user-1 with a valid input",
		Then:      "the repository error is returned unchanged with an empty user",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *UserService
		var got domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			var users *fakeUserRepository
			service, users, _ = newUserFixture()
			users.updateErr = errDependency
		})
		r.Act(func(t *testing.T) { got, err = service.Update(context.Background(), testUserID, validUserWrite()) })
		r.Assert(func(t *testing.T) {
			assertErrorIs(t, err, errDependency)
			assertEqual(t, domain.User{}, got)
		})
	})
}

func TestUserServiceDelete(t *testing.T) {
	const method = "Delete"

	runSpec(t, userComponent, testkit.Spec{
		ID: "UC-USR-DELETE-01", Method: method,
		Title:     "deletes user and password hash",
		Given:     "a stored user user-1 with a password hash",
		When:      "Delete is called with user-1",
		Then:      "no error is returned and neither the user nor its hash remains",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"id": testUserID},
	}, func(t *testing.T, r testkit.Report) {
		var service *UserService
		var users *fakeUserRepository
		var err error
		r.Arrange(func(t *testing.T) {
			service, users, _ = newUserFixture()
			seedOldUser(users)
		})
		r.Act(func(t *testing.T) { err = service.Delete(context.Background(), testUserID) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, 0, len(users.users))
			assertEqual(t, 0, len(users.passwords))
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "UC-USR-DELETE-02", Method: method,
		Title:     "rejects empty id without deleting",
		Given:     "a stored user user-1",
		When:      "Delete is called with an empty id",
		Then:      "invalid \"user id is required\" is returned and user-1 remains stored",
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"id": ""},
	}, func(t *testing.T, r testkit.Report) {
		var service *UserService
		var users *fakeUserRepository
		var err error
		r.Arrange(func(t *testing.T) {
			service, users, _ = newUserFixture()
			seedOldUser(users)
		})
		r.Act(func(t *testing.T) { err = service.Delete(context.Background(), "") })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInvalid, "user id is required")
			assertEqual(t, 1, len(users.users))
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "UC-USR-DELETE-03", Method: method,
		Title:     "propagates not found for unknown user",
		Given:     "no stored users",
		When:      "Delete is called with user-1",
		Then:      "not_found \"user not found\" is returned",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var service *UserService
		var err error
		r.Arrange(func(t *testing.T) { service, _, _ = newUserFixture() })
		r.Act(func(t *testing.T) { err = service.Delete(context.Background(), testUserID) })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeNotFound, "user not found") })
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "UC-USR-DELETE-04", Method: method,
		Title:     "returns repository delete error and keeps user",
		Given:     "a stored user and a user repository whose Delete fails",
		When:      "Delete is called with user-1",
		Then:      "the repository error is returned unchanged and user-1 remains stored",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *UserService
		var users *fakeUserRepository
		var err error
		r.Arrange(func(t *testing.T) {
			service, users, _ = newUserFixture()
			seedOldUser(users)
			users.deleteErr = errDependency
		})
		r.Act(func(t *testing.T) { err = service.Delete(context.Background(), testUserID) })
		r.Assert(func(t *testing.T) {
			assertErrorIs(t, err, errDependency)
			assertEqual(t, 1, len(users.users))
		})
	})
}
