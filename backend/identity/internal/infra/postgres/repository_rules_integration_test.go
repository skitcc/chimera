//go:build integration

package postgres

import (
	"testing"

	"chimera/identity/internal/domain"
	"chimera/identity/internal/testkit"
)

func TestUserRepositoryRulesIntegration(t *testing.T) {
	const component = "UserRepository"

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-UPDATE-01", Method: "Update",
		Title:     "update persists a changed email and name",
		Given:     "one seeded user",
		When:      "Update is called with email updated@example.com and name Updated",
		Then:      "the returned user and a fresh GetByID both carry the new values",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"email": "updated@example.com", "name": "Updated"},
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var user, got domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			user = f.seedUser(t)
		})
		r.Act(func(t *testing.T) {
			got, err = f.users.Update(f.ctx, domain.User{ID: user.ID, Email: "updated@example.com", Name: "Updated"})
		})
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("Update() error = %v", err)
			}
			assertEqual(t, got, domain.User{ID: user.ID, Email: "updated@example.com", Name: "Updated"})
			stored, readErr := f.users.GetByID(f.ctx, user.ID)
			if readErr != nil {
				t.Fatalf("GetByID() error = %v", readErr)
			}
			assertEqual(t, stored, got)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-UPDATE-02", Method: "Update",
		Title:     "update of an unknown uuid returns not found",
		Given:     "an empty users table",
		When:      "Update is called with the nil UUID",
		Then:      "error not_found `user not found`; the table stays empty",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"id": missingUUID},
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var err error
		r.Arrange(func(t *testing.T) { f = newPostgresFixture(t) })
		r.Act(func(t *testing.T) {
			_, err = f.users.Update(f.ctx, domain.User{ID: missingUUID, Email: "missing@example.com", Name: "Missing"})
		})
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeNotFound, "user not found")
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM users`), 0)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-UPDATE-03", Method: "Update",
		Title:     "update to another user's email returns conflict",
		Given:     "two users, alice@example.com and bob@example.com",
		When:      "Bob is updated to alice@example.com",
		Then:      "error conflict `email already exists`; Bob keeps his email",
		Technique: testkit.TechniqueErrorGuessing,
		Severity:  testkit.SeverityCritical,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var bob domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			mustCreateUser(t, f, "alice@example.com", "hash")
			bob = mustCreateUser(t, f, "bob@example.com", "hash")
		})
		r.Act(func(t *testing.T) {
			_, err = f.users.Update(f.ctx, domain.User{ID: bob.ID, Email: "alice@example.com", Name: "Bob"})
		})
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeConflict, "email already exists")
			stored, readErr := f.users.GetByID(f.ctx, bob.ID)
			if readErr != nil {
				t.Fatalf("GetByID() error = %v", readErr)
			}
			assertEqual(t, stored.Email, "bob@example.com")
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-CREATE-02", Method: "Create",
		Title:     "duplicate email returns conflict",
		Given:     "a stored user alice@example.com",
		When:      "Create is called again with alice@example.com",
		Then:      "UNIQUE(users.email) fails with SQLSTATE 23505, mapped to conflict `email already exists`; one row remains",
		Technique: testkit.TechniqueErrorGuessing,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"email": "alice@example.com"},
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			mustCreateUser(t, f, "alice@example.com", "hash")
		})
		r.Act(func(t *testing.T) {
			_, err = f.users.Create(f.ctx, domain.User{Email: "alice@example.com", Name: "Other"}, "other-hash")
		})
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeConflict, "email already exists")
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM users`), 1)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-CREATE-03", Method: "Create",
		Title:     "email uniqueness in the database is case-sensitive",
		Given:     "a stored user alice@example.com",
		When:      "Create is called with Alice@Example.com, bypassing domain normalization",
		Then:      "the insert succeeds and two rows exist; lowercasing is enforced only by the domain layer",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"first": "alice@example.com", "second": "Alice@Example.com"},
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			mustCreateUser(t, f, "alice@example.com", "hash")
		})
		r.Act(func(t *testing.T) {
			_, err = f.users.Create(f.ctx, domain.User{Email: "Alice@Example.com", Name: "Alice"}, "hash")
		})
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM users`), 2)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-GETEMAIL-01", Method: "GetByEmail",
		Title:     "get by email returns the stored password hash",
		Given:     "a user created with hash stored-hash",
		When:      "GetByEmail is called with alice@example.com",
		Then:      "the auth user carries the email and the exact stored hash",
		Technique: testkit.TechniqueEquivalence,
		Severity:  testkit.SeverityCritical,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var got domain.AuthUser
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			mustCreateUser(t, f, "alice@example.com", "stored-hash")
		})
		r.Act(func(t *testing.T) { got, err = f.users.GetByEmail(f.ctx, "alice@example.com") })
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("GetByEmail() error = %v", err)
			}
			assertEqual(t, got.User.Email, "alice@example.com")
			assertEqual(t, got.PasswordHash, "stored-hash")
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-GETEMAIL-02", Method: "GetByEmail",
		Title:     "get by unknown email returns not found",
		Given:     "an empty users table",
		When:      "GetByEmail is called with missing@example.com",
		Then:      "error not_found `user not found`",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var err error
		r.Arrange(func(t *testing.T) { f = newPostgresFixture(t) })
		r.Act(func(t *testing.T) { _, err = f.users.GetByEmail(f.ctx, "missing@example.com") })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeNotFound, "user not found") })
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-GET-03", Method: "GetByID",
		Title:     "malformed uuid returns invalid id",
		Given:     "an empty users table",
		When:      "GetByID is called with not-a-uuid",
		Then:      "PostgreSQL rejects the cast with SQLSTATE 22P02, mapped to invalid `invalid id`",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"id": "not-a-uuid"},
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var err error
		r.Arrange(func(t *testing.T) { f = newPostgresFixture(t) })
		r.Act(func(t *testing.T) { _, err = f.users.GetByID(f.ctx, "not-a-uuid") })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeInvalid, "invalid id") })
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-DELETE-02", Method: "Delete",
		Title:     "delete records UserDeleted in the outbox in the same transaction",
		Given:     "one seeded user",
		When:      "Delete is called for that user",
		Then:      "the user row is gone and one unsent outbox row for identity.user.v1 keyed by the user id carries chimera.identity.user.deleted.v1",
		Technique: testkit.TechniqueEquivalence,
		Severity:  testkit.SeverityCritical,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var user domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			user = f.seedUser(t)
		})
		r.Act(func(t *testing.T) { err = f.users.Delete(f.ctx, user.ID) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM users WHERE id = $1::uuid`, user.ID), 0)
			assertEqual(t, countRows(t, f,
				`SELECT count(*) FROM outbox
				 WHERE topic = 'identity.user.v1' AND record_key = $1
				   AND event_type = 'chimera.identity.user.deleted.v1' AND sent_at IS NULL`, user.ID), 1)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-DELETE-03", Method: "Delete",
		Title:     "delete of an unknown uuid returns not found",
		Given:     "an empty users table",
		When:      "Delete is called with the nil UUID",
		Then:      "zero rows affected, mapped to not_found `user not found`",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var err error
		r.Arrange(func(t *testing.T) { f = newPostgresFixture(t) })
		r.Act(func(t *testing.T) { err = f.users.Delete(f.ctx, missingUUID) })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeNotFound, "user not found") })
	})
}

func mustCreateUser(t *testing.T, f *postgresFixture, email, hash string) domain.User {
	t.Helper()
	user, err := f.users.Create(f.ctx, domain.User{Email: email, Name: "Member"}, hash)
	if err != nil {
		t.Fatalf("create user %s: %v", email, err)
	}
	return user
}
