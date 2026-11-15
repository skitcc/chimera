package postgres

import (
	"context"
	"regexp"
	"testing"

	"github.com/pashagolub/pgxmock/v5"

	"chimera/identity/internal/domain"
	"chimera/identity/internal/testkit"
	"chimera/platform/idempotency"
)

func TestUserRepositoryCreateClaimsTheIdempotencyKey(t *testing.T) {
	runSpec(t, userComponent, testkit.Spec{
		ID: "DA-USR-CREATE-IDEM-01", Method: "Create", Kind: testkit.KindPgxmock,
		Title:     "create with an idempotency key claims the key, inserts the user and stores the id in one transaction",
		Given:     "the key insert reports one new row and the user insert returns id user-1",
		When:      "Create is called with the key in the context",
		Then:      "the returned user is user-1 and the transaction commits",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"key": "key-1", "email": "a@example.com"},
	}, func(t *testing.T, r testkit.Report) {
		var db pgxmock.PgxPoolIface
		var repo *UserRepository
		var got domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			db = newMockPool(t)
			db.ExpectBegin()
			db.ExpectExec(`INSERT INTO idempotency_keys`).
				WithArgs(opCreateUser, "", "key-1", pgxmock.AnyArg()).
				WillReturnResult(pgxmock.NewResult("INSERT", 1))
			db.ExpectQuery(regexp.QuoteMeta(`INSERT INTO users`)).
				WithArgs("a@example.com", "Ann", "hash").
				WillReturnRows(pgxmock.NewRows([]string{"id", "email", "name"}).
					AddRow("user-1", "a@example.com", "Ann"))
			db.ExpectExec(`UPDATE idempotency_keys`).
				WithArgs(opCreateUser, "", "key-1", "user-1").
				WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			db.ExpectCommit()
			db.ExpectRollback()
			repo = newUserRepository(db)
		})
		r.Act(func(t *testing.T) {
			ctx, keyErr := idempotency.With(context.Background(), "key-1")
			if keyErr != nil {
				t.Fatal(keyErr)
			}
			got, err = repo.Create(ctx, domain.User{Email: "a@example.com", Name: "Ann"}, "hash")
		})
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, got.ID, "user-1")
			assertExpectationsMet(t, db)
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "DA-USR-CREATE-IDEM-02", Method: "Create", Kind: testkit.KindPgxmock,
		Title:     "the same idempotency key with another request is a conflict and inserts nothing",
		Given:     "the key already exists with a different request hash",
		When:      "Create is called with that key",
		Then:      "a conflict error is returned and the transaction rolls back",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"key": "key-1"},
	}, func(t *testing.T, r testkit.Report) {
		var db pgxmock.PgxPoolIface
		var repo *UserRepository
		var err error
		r.Arrange(func(t *testing.T) {
			db = newMockPool(t)
			db.ExpectBegin()
			db.ExpectExec(`INSERT INTO idempotency_keys`).
				WithArgs(opCreateUser, "", "key-1", pgxmock.AnyArg()).
				WillReturnResult(pgxmock.NewResult("INSERT", 0))
			db.ExpectQuery(`SELECT request_hash, COALESCE`).
				WithArgs(opCreateUser, "", "key-1").
				WillReturnRows(pgxmock.NewRows([]string{"request_hash", "resource_id"}).
					AddRow([]byte("other"), "user-1"))
			db.ExpectRollback()
			repo = newUserRepository(db)
		})
		r.Act(func(t *testing.T) {
			ctx, keyErr := idempotency.With(context.Background(), "key-1")
			if keyErr != nil {
				t.Fatal(keyErr)
			}
			_, err = repo.Create(ctx, domain.User{Email: "a@example.com", Name: "Ann"}, "hash")
		})
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeConflict, "idempotency key reused with another request")
			assertExpectationsMet(t, db)
		})
	})
}
