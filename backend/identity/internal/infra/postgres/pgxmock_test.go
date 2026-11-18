package postgres

import (
	"context"
	"regexp"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v5"

	"chimera/identity/internal/domain"
	"chimera/identity/internal/testkit"
)

func newMockPool(t *testing.T) pgxmock.PgxPoolIface {
	t.Helper()
	db, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create pgxmock: %v", err)
	}
	t.Cleanup(db.Close)
	return db
}

func assertExpectationsMet(t *testing.T, db pgxmock.PgxPoolIface) {
	t.Helper()
	if err := db.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet database expectations: %v", err)
	}
}

const userByIDSQL = `SELECT id::text, email, name FROM users WHERE id = $1::uuid`

func TestUserRepositoryPGXMock(t *testing.T) {
	runSpec(t, userComponent, testkit.Spec{
		ID: "DA-PGX-USR-GET-01", Method: "GetByID", Kind: testkit.KindPgxmock,
		Title:     "get by id issues the exact user lookup query and returns the row",
		Given:     "pgxmock expects the users-by-id SELECT with argument user-1 and returns one row",
		When:      "GetByID is called with user-1",
		Then:      "the user {user-1, listener@example.com, Listener} is returned without error and all expectations are met",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"id": "user-1"},
	}, func(t *testing.T, r testkit.Report) {
		var db pgxmock.PgxPoolIface
		var repo *UserRepository
		var got domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			db = newMockPool(t)
			db.ExpectQuery(regexp.QuoteMeta(userByIDSQL)).
				WithArgs("user-1").
				WillReturnRows(pgxmock.NewRows([]string{"id", "email", "name"}).
					AddRow("user-1", "listener@example.com", "Listener"))
			repo = newUserRepository(db)
		})
		r.Act(func(t *testing.T) { got, err = repo.GetByID(context.Background(), "user-1") })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, got, domain.User{ID: "user-1", Email: "listener@example.com", Name: "Listener"})
			assertExpectationsMet(t, db)
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "DA-PGX-USR-GET-02", Method: "GetByID", Kind: testkit.KindPgxmock,
		Title:     "get by id maps pgx no rows from the driver to user not found",
		Given:     "pgxmock expects the users-by-id SELECT with argument missing and returns pgx.ErrNoRows",
		When:      "GetByID is called with missing",
		Then:      "a not_found domain error with message \"user not found\" is returned and all expectations are met",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"id": "missing", "driver error": "pgx.ErrNoRows"},
	}, func(t *testing.T, r testkit.Report) {
		var db pgxmock.PgxPoolIface
		var repo *UserRepository
		var err error
		r.Arrange(func(t *testing.T) {
			db = newMockPool(t)
			db.ExpectQuery(regexp.QuoteMeta(userByIDSQL)).
				WithArgs("missing").
				WillReturnError(pgx.ErrNoRows)
			repo = newUserRepository(db)
		})
		r.Act(func(t *testing.T) { _, err = repo.GetByID(context.Background(), "missing") })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeNotFound, "user not found")
			assertExpectationsMet(t, db)
		})
	})
}
