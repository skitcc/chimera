package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"chimera/internal/domain"
	"chimera/internal/testkit"
)

const userComponent = "UserRepository"

func TestUserRepositoryList(t *testing.T) {
	const method = "List"

	runSpec(t, userComponent, testkit.Spec{
		ID: "DA-USR-LIST-01", Method: method,
		Title:     "list returns every scanned user in row order and closes rows",
		Given:     "the database returns two user rows",
		When:      "List is called",
		Then:      "both users are returned in the same order, no error is returned, and the rows are closed",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"rows": "2"},
	}, func(t *testing.T, r testkit.Report) {
		want := []domain.User{
			{ID: "user-1", Email: "one@example.com", Name: "One"},
			{ID: "user-2", Email: "two@example.com", Name: "Two"},
		}
		var rows *stubRows
		var repo *UserRepository
		var got []domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			rows = newStubRows(userValues(want[0]), userValues(want[1]))
			repo = NewUserRepository(stubDB{
				queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return rows, nil },
			})
		})
		r.Act(func(t *testing.T) { got, err = repo.List(context.Background()) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, got, want)
			assertRowsClosed(t, rows)
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "DA-USR-LIST-02", Method: method,
		Title:     "list returns an empty non-nil slice when there are no users",
		Given:     "the database returns zero user rows",
		When:      "List is called",
		Then:      "an empty non-nil slice and no error are returned, and the rows are closed",
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"rows": "0"},
	}, func(t *testing.T, r testkit.Report) {
		var rows *stubRows
		var repo *UserRepository
		var got []domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			rows = newStubRows()
			repo = NewUserRepository(stubDB{
				queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return rows, nil },
			})
		})
		r.Act(func(t *testing.T) { got, err = repo.List(context.Background()) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, got, []domain.User{})
			assertRowsClosed(t, rows)
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "DA-USR-LIST-03", Method: method,
		Title:     "list wraps a query failure as internal list users error",
		Given:     "the database query fails with a driver error",
		When:      "List is called",
		Then:      "an internal domain error with message \"list users\" is returned and it wraps the driver error",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"driver error": "query failed"},
	}, func(t *testing.T, r testkit.Report) {
		dbErr := errors.New("query failed")
		var repo *UserRepository
		var got []domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewUserRepository(stubDB{
				queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return nil, dbErr },
			})
		})
		r.Act(func(t *testing.T) { got, err = repo.List(context.Background()) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInternal, "list users")
			assertWraps(t, err, dbErr)
			assertEqual(t, got, []domain.User(nil))
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "DA-USR-LIST-04", Method: method,
		Title:     "list maps a row scan failure to internal scan user error and closes rows",
		Given:     "the database returns one user row whose scan fails",
		When:      "List is called",
		Then:      "an internal domain error with message \"scan user\" and nil users are returned, and the rows are closed",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"scan error at row": "0"},
	}, func(t *testing.T, r testkit.Report) {
		var rows *stubRows
		var repo *UserRepository
		var got []domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			rows = newStubRows(userValues(domain.User{ID: "user-1", Email: "one@example.com", Name: "One"}))
			rows.scanErrAt = 0
			repo = NewUserRepository(stubDB{
				queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return rows, nil },
			})
		})
		r.Act(func(t *testing.T) { got, err = repo.List(context.Background()) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInternal, "scan user")
			assertEqual(t, got, []domain.User(nil))
			assertRowsClosed(t, rows)
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "DA-USR-LIST-05", Method: method,
		Title:     "list maps a rows iteration error to internal list users error",
		Given:     "the database returns no rows and rows.Err reports a driver error",
		When:      "List is called",
		Then:      "an internal domain error with message \"list users\" wrapping the rows error is returned, and the rows are closed",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"rows error": "rows failed"},
	}, func(t *testing.T, r testkit.Report) {
		rowsErr := errors.New("rows failed")
		var rows *stubRows
		var repo *UserRepository
		var err error
		r.Arrange(func(t *testing.T) {
			rows = newStubRows()
			rows.err = rowsErr
			repo = NewUserRepository(stubDB{
				queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return rows, nil },
			})
		})
		r.Act(func(t *testing.T) { _, err = repo.List(context.Background()) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInternal, "list users")
			assertWraps(t, err, rowsErr)
			assertRowsClosed(t, rows)
		})
	})
}

func TestUserRepositoryGetByID(t *testing.T) {
	const method = "GetByID"
	want := domain.User{ID: "user-1", Email: "one@example.com", Name: "One"}

	runSpec(t, userComponent, testkit.Spec{
		ID: "DA-USR-GET-01", Method: method,
		Title:     "get by id sends the id and returns the scanned user",
		Given:     "the database returns a row for user-1",
		When:      "GetByID is called with user-1",
		Then:      "the query receives [user-1] and the scanned user is returned without error",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"id": want.ID},
	}, func(t *testing.T, r testkit.Report) {
		var gotArgs []any
		var repo *UserRepository
		var got domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewUserRepository(stubDB{
				queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
					gotArgs = args
					return stubRow{values: userValues(want)}
				},
			})
		})
		r.Act(func(t *testing.T) { got, err = repo.GetByID(context.Background(), want.ID) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, gotArgs, []any{want.ID})
			assertEqual(t, got, want)
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "DA-USR-GET-02", Method: method,
		Title:     "get by id maps pgx no rows to user not found",
		Given:     "the database row scan returns pgx.ErrNoRows",
		When:      "GetByID is called with a missing id",
		Then:      "a not_found domain error with message \"user not found\" and a zero user are returned",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"id": "missing", "driver error": "pgx.ErrNoRows"},
	}, func(t *testing.T, r testkit.Report) {
		var repo *UserRepository
		var got domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewUserRepository(stubDB{
				queryRowFn: func(context.Context, string, ...any) pgx.Row { return stubRow{err: pgx.ErrNoRows} },
			})
		})
		r.Act(func(t *testing.T) { got, err = repo.GetByID(context.Background(), "missing") })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeNotFound, "user not found")
			assertEqual(t, got, domain.User{})
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "DA-USR-GET-03", Method: method,
		Title:     "get by id maps sqlstate 22P02 to invalid id",
		Given:     "the database rejects the id with SQLSTATE 22P02 (invalid text representation)",
		When:      "GetByID is called with a non-uuid id",
		Then:      "an invalid domain error with message \"invalid id\" is returned",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"id": "invalid", "sqlstate": "22P02"},
	}, func(t *testing.T, r testkit.Report) {
		var repo *UserRepository
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewUserRepository(stubDB{
				queryRowFn: func(context.Context, string, ...any) pgx.Row {
					return stubRow{err: &pgconn.PgError{Code: "22P02"}}
				},
			})
		})
		r.Act(func(t *testing.T) { _, err = repo.GetByID(context.Background(), "invalid") })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeInvalid, "invalid id") })
	})
}

func TestUserRepositoryGetByEmail(t *testing.T) {
	const method = "GetByEmail"
	want := domain.AuthUser{
		User:         domain.User{ID: "user-1", Email: "one@example.com", Name: "One"},
		PasswordHash: "hash",
	}

	runSpec(t, userComponent, testkit.Spec{
		ID: "DA-USR-GETEMAIL-01", Method: method,
		Title:     "get by email sends the email and returns the user with password hash",
		Given:     "the database returns a user row with password hash \"hash\"",
		When:      "GetByEmail is called with one@example.com",
		Then:      "the query receives [one@example.com] and the auth user including the password hash is returned",
		Technique: testkit.TechniqueEquivalence,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"email": want.User.Email},
	}, func(t *testing.T, r testkit.Report) {
		var gotArgs []any
		var repo *UserRepository
		var got domain.AuthUser
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewUserRepository(stubDB{
				queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
					gotArgs = args
					return stubRow{values: append(userValues(want.User), want.PasswordHash)}
				},
			})
		})
		r.Act(func(t *testing.T) { got, err = repo.GetByEmail(context.Background(), want.User.Email) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, gotArgs, []any{want.User.Email})
			assertEqual(t, got, want)
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "DA-USR-GETEMAIL-02", Method: method,
		Title:     "get by email maps pgx no rows to user not found",
		Given:     "the database row scan returns pgx.ErrNoRows",
		When:      "GetByEmail is called with an unknown email",
		Then:      "a not_found domain error with message \"user not found\" and a zero auth user are returned",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"email": "missing@example.com", "driver error": "pgx.ErrNoRows"},
	}, func(t *testing.T, r testkit.Report) {
		var repo *UserRepository
		var got domain.AuthUser
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewUserRepository(stubDB{
				queryRowFn: func(context.Context, string, ...any) pgx.Row { return stubRow{err: pgx.ErrNoRows} },
			})
		})
		r.Act(func(t *testing.T) { got, err = repo.GetByEmail(context.Background(), "missing@example.com") })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeNotFound, "user not found")
			assertEqual(t, got, domain.AuthUser{})
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "DA-USR-GETEMAIL-03", Method: method,
		Title:     "get by email wraps a driver failure as internal get user by email error",
		Given:     "the database row scan fails with a non-postgres driver error",
		When:      "GetByEmail is called",
		Then:      "an internal domain error with message \"get user by email\" wrapping the driver error is returned",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"driver error": "query failed"},
	}, func(t *testing.T, r testkit.Report) {
		dbErr := errors.New("query failed")
		var repo *UserRepository
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewUserRepository(stubDB{
				queryRowFn: func(context.Context, string, ...any) pgx.Row { return stubRow{err: dbErr} },
			})
		})
		r.Act(func(t *testing.T) { _, err = repo.GetByEmail(context.Background(), want.User.Email) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInternal, "get user by email")
			assertWraps(t, err, dbErr)
		})
	})
}

func TestUserRepositoryCreate(t *testing.T) {
	const method = "Create"
	input := domain.User{Email: "one@example.com", Name: "One"}
	want := domain.User{ID: "user-1", Email: input.Email, Name: input.Name}

	runSpec(t, userComponent, testkit.Spec{
		ID: "DA-USR-CREATE-01", Method: method,
		Title:     "create sends email name and password hash and returns the generated id",
		Given:     "a user without id and the database returns the inserted row with id user-1",
		When:      "Create is called with password hash \"hash\"",
		Then:      "the insert receives [one@example.com, One, hash] and the user with id user-1 is returned",
		Technique: testkit.TechniqueEquivalence,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"email": input.Email, "name": input.Name, "password hash": "hash"},
	}, func(t *testing.T, r testkit.Report) {
		var gotArgs []any
		var repo *UserRepository
		var got domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewUserRepository(stubDB{
				queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
					gotArgs = args
					return stubRow{values: userValues(want)}
				},
			})
		})
		r.Act(func(t *testing.T) { got, err = repo.Create(context.Background(), input, "hash") })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, gotArgs, []any{input.Email, input.Name, "hash"})
			assertEqual(t, got, want)
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "DA-USR-CREATE-02", Method: method,
		Title:     "create maps unique violation on users_email_key to email already exists",
		Given:     "the insert fails with SQLSTATE 23505 on constraint users_email_key",
		When:      "Create is called",
		Then:      "a conflict domain error with message \"email already exists\" and a zero user are returned",
		Technique: testkit.TechniqueDecisionTable,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"sqlstate": "23505", "constraint": "users_email_key"},
	}, func(t *testing.T, r testkit.Report) {
		var repo *UserRepository
		var got domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewUserRepository(stubDB{
				queryRowFn: func(context.Context, string, ...any) pgx.Row {
					return stubRow{err: &pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"}}
				},
			})
		})
		r.Act(func(t *testing.T) { got, err = repo.Create(context.Background(), input, "hash") })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeConflict, "email already exists")
			assertEqual(t, got, domain.User{})
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "DA-USR-CREATE-03", Method: method,
		Title:     "create wraps an unmapped sqlstate as internal create user error preserving the cause",
		Given:     "the insert fails with SQLSTATE 40001 (serialization failure), which mapError does not map",
		When:      "Create is called",
		Then:      "an internal domain error with message \"create user\" is returned whose Unwrap is the original PgError",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"sqlstate": "40001"},
	}, func(t *testing.T, r testkit.Report) {
		pgErr := &pgconn.PgError{Code: "40001"}
		var repo *UserRepository
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewUserRepository(stubDB{
				queryRowFn: func(context.Context, string, ...any) pgx.Row { return stubRow{err: pgErr} },
			})
		})
		r.Act(func(t *testing.T) { _, err = repo.Create(context.Background(), input, "hash") })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInternal, "create user")
			assertWraps(t, err, pgErr)
			app, _ := domain.As(err)
			assertEqual(t, app.Unwrap(), error(pgErr))
		})
	})
}

func TestUserRepositoryUpdate(t *testing.T) {
	const method = "Update"
	want := domain.User{ID: "user-1", Email: "updated@example.com", Name: "Updated"}

	runSpec(t, userComponent, testkit.Spec{
		ID: "DA-USR-UPDATE-01", Method: method,
		Title:     "update sends id email and name and returns the updated user",
		Given:     "the database returns the updated row for user-1",
		When:      "Update is called with the new email and name",
		Then:      "the update receives [user-1, updated@example.com, Updated] and the updated user is returned",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"id": want.ID, "email": want.Email, "name": want.Name},
	}, func(t *testing.T, r testkit.Report) {
		var gotArgs []any
		var repo *UserRepository
		var got domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewUserRepository(stubDB{
				queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
					gotArgs = args
					return stubRow{values: userValues(want)}
				},
			})
		})
		r.Act(func(t *testing.T) { got, err = repo.Update(context.Background(), want) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, gotArgs, []any{want.ID, want.Email, want.Name})
			assertEqual(t, got, want)
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "DA-USR-UPDATE-02", Method: method,
		Title:     "update maps pgx no rows to user not found",
		Given:     "the update returns no row because the id does not exist",
		When:      "Update is called",
		Then:      "a not_found domain error with message \"user not found\" and a zero user are returned",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"driver error": "pgx.ErrNoRows"},
	}, func(t *testing.T, r testkit.Report) {
		var repo *UserRepository
		var got domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewUserRepository(stubDB{
				queryRowFn: func(context.Context, string, ...any) pgx.Row { return stubRow{err: pgx.ErrNoRows} },
			})
		})
		r.Act(func(t *testing.T) { got, err = repo.Update(context.Background(), want) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeNotFound, "user not found")
			assertEqual(t, got, domain.User{})
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "DA-USR-UPDATE-03", Method: method,
		Title:     "update maps unique violation on users_email_key to email already exists",
		Given:     "the update fails with SQLSTATE 23505 on constraint users_email_key",
		When:      "Update is called with an email owned by another user",
		Then:      "a conflict domain error with message \"email already exists\" is returned",
		Technique: testkit.TechniqueDecisionTable,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"sqlstate": "23505", "constraint": "users_email_key"},
	}, func(t *testing.T, r testkit.Report) {
		var repo *UserRepository
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewUserRepository(stubDB{
				queryRowFn: func(context.Context, string, ...any) pgx.Row {
					return stubRow{err: &pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"}}
				},
			})
		})
		r.Act(func(t *testing.T) { _, err = repo.Update(context.Background(), want) })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeConflict, "email already exists") })
	})
}

func TestUserRepositoryDelete(t *testing.T) {
	const method = "Delete"

	runSpec(t, userComponent, testkit.Spec{
		ID: "DA-USR-DELETE-01", Method: method,
		Title:     "delete sends the id and succeeds when one row is affected",
		Given:     "the delete reports command tag DELETE 1",
		When:      "Delete is called with user-1",
		Then:      "the exec receives [user-1] and no error is returned",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"id": "user-1", "rows affected": "1"},
	}, func(t *testing.T, r testkit.Report) {
		var gotArgs []any
		var repo *UserRepository
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewUserRepository(stubDB{
				execFn: func(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
					gotArgs = args
					return pgconn.NewCommandTag("DELETE 1"), nil
				},
			})
		})
		r.Act(func(t *testing.T) { err = repo.Delete(context.Background(), "user-1") })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, gotArgs, []any{"user-1"})
		})
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "DA-USR-DELETE-02", Method: method,
		Title:     "delete maps zero affected rows to user not found",
		Given:     "the delete reports command tag DELETE 0",
		When:      "Delete is called with a missing id",
		Then:      "a not_found domain error with message \"user not found\" is returned",
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"id": "missing", "rows affected": "0"},
	}, func(t *testing.T, r testkit.Report) {
		var repo *UserRepository
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewUserRepository(stubDB{
				execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
					return pgconn.NewCommandTag("DELETE 0"), nil
				},
			})
		})
		r.Act(func(t *testing.T) { err = repo.Delete(context.Background(), "missing") })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeNotFound, "user not found") })
	})

	runSpec(t, userComponent, testkit.Spec{
		ID: "DA-USR-DELETE-03", Method: method,
		Title:     "delete maps sqlstate 22P02 to invalid id",
		Given:     "the delete fails with SQLSTATE 22P02 (invalid text representation)",
		When:      "Delete is called with a non-uuid id",
		Then:      "an invalid domain error with message \"invalid id\" is returned",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"id": "invalid", "sqlstate": "22P02"},
	}, func(t *testing.T, r testkit.Report) {
		var repo *UserRepository
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewUserRepository(stubDB{
				execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
					return pgconn.CommandTag{}, &pgconn.PgError{Code: "22P02"}
				},
			})
		})
		r.Act(func(t *testing.T) { err = repo.Delete(context.Background(), "invalid") })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeInvalid, "invalid id") })
	})
}
