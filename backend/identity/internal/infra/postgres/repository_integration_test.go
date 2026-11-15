//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"chimera/identity/internal/domain"
	"chimera/identity/internal/testkit"
	"chimera/identity/migrations"
)

const (
	missingUUID = "00000000-0000-0000-0000-000000000000"
	resetTables = `TRUNCATE outbox, users RESTART IDENTITY CASCADE`
)

type postgresFixture struct {
	ctx    context.Context
	pool   *pgxpool.Pool
	users *UserRepository
}

func newPostgresFixture(t *testing.T) *postgresFixture {
	t.Helper()
	pool := testkit.Open(t, migrations.FS, resetTables)
	return &postgresFixture{
		ctx:   context.Background(),
		pool:  pool,
		users: NewUserRepository(pool),
	}
}

func (f *postgresFixture) seedUser(t *testing.T) domain.User {
	t.Helper()
	var user domain.User
	err := f.pool.QueryRow(f.ctx,
		`INSERT INTO users (email, name, password_hash)
		 VALUES ('fixture@example.com', 'Fixture', 'hash')
		 RETURNING id::text, email, name`,
	).Scan(&user.ID, &user.Email, &user.Name)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return user
}

func runIntegration(t *testing.T, component string, s testkit.Spec, body func(*testing.T, testkit.Report)) {
	t.Helper()
	s.Layer = testkit.LayerData
	s.Component = component
	s.Kind = testkit.KindIntegration
	testkit.RunSpec(t, s, body)
}

func countRows(t *testing.T, f *postgresFixture, query string, args ...any) int {
	t.Helper()
	var count int
	if err := f.pool.QueryRow(f.ctx, query, args...).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return count
}

func TestUserRepositoryIntegration(t *testing.T) {
	const component = "UserRepository"

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-CREATE-01", Method: "Create",
		Title:     "create persists a user row",
		Given:     "an empty users table",
		When:      "Create is called with alice@example.com, name Alice, hash `hash`",
		Then:      "one users row exists with the returned id; the id is a server-generated UUID",
		Technique: testkit.TechniqueEquivalence,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"email": "alice@example.com", "name": "Alice"},
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var created domain.User
		var err error
		r.Arrange(func(t *testing.T) { f = newPostgresFixture(t) })
		r.Act(func(t *testing.T) {
			created, err = f.users.Create(f.ctx, domain.User{Email: "alice@example.com", Name: "Alice"}, "hash")
		})
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			assertEqual(t, len(created.ID), 36)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM users WHERE id = $1::uuid`, created.ID), 1)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-GET-01", Method: "GetByID",
		Title:     "get by id reads a seeded user",
		Given:     "one user inserted with plain SQL",
		When:      "GetByID is called with that id",
		Then:      "id, email, and name equal the seeded row",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var want, got domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			want = f.seedUser(t)
		})
		r.Act(func(t *testing.T) { got, err = f.users.GetByID(f.ctx, want.ID) })
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("GetByID() error = %v", err)
			}
			assertEqual(t, got, want)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-GET-02", Method: "GetByID",
		Title:     "get by id of an unknown uuid returns not found",
		Given:     "an empty users table",
		When:      "GetByID is called with the nil UUID",
		Then:      "error not_found `user not found`",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"id": missingUUID},
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var err error
		r.Arrange(func(t *testing.T) { f = newPostgresFixture(t) })
		r.Act(func(t *testing.T) { _, err = f.users.GetByID(f.ctx, missingUUID) })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeNotFound, "user not found") })
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-LIST-01", Method: "List",
		Title:     "list returns every stored user",
		Given:     "one seeded user",
		When:      "List is called",
		Then:      "exactly that user is returned",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var want domain.User
		var got []domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			want = f.seedUser(t)
		})
		r.Act(func(t *testing.T) { got, err = f.users.List(f.ctx) })
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			assertEqual(t, got, []domain.User{want})
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-LIST-02", Method: "List",
		Title:     "list of an empty table returns an empty slice",
		Given:     "an empty users table",
		When:      "List is called",
		Then:      "an empty, non-nil slice and no error",
		Technique: testkit.TechniqueBoundary,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var got []domain.User
		var err error
		r.Arrange(func(t *testing.T) { f = newPostgresFixture(t) })
		r.Act(func(t *testing.T) { got, err = f.users.List(f.ctx) })
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			assertEqual(t, got, []domain.User{})
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-DELETE-01", Method: "Delete",
		Title:     "delete removes the user row",
		Given:     "one seeded user",
		When:      "Delete is called with that id",
		Then:      "no error and the users row is gone",
		Technique: testkit.TechniqueEquivalence,
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
			if err != nil {
				t.Fatalf("Delete() error = %v", err)
			}
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM users WHERE id = $1::uuid`, user.ID), 0)
		})
	})
}
