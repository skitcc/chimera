package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"chimera/internal/domain"
)

func TestUserRepositoryList(t *testing.T) {
	runCase(t, "success", func(t *testing.T) {
		want := []domain.User{
			{ID: "user-1", Email: "one@example.com", Name: "One"},
			{ID: "user-2", Email: "two@example.com", Name: "Two"},
		}
		rows := newStubRows(userValues(want[0]), userValues(want[1]))
		repo := NewUserRepository(stubDB{
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) {
				return rows, nil
			},
		})

		got, err := repo.List(context.Background())
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		assertEqual(t, got, want)
		if !rows.closed {
			t.Fatal("List did not close rows")
		}
	})

	runCase(t, "query error", func(t *testing.T) {
		dbErr := errors.New("query failed")
		repo := NewUserRepository(stubDB{
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) {
				return nil, dbErr
			},
		})

		_, err := repo.List(context.Background())
		assertDomainError(t, err, domain.CodeInternal, "list users")
		if !errors.Is(err, dbErr) {
			t.Fatalf("error does not wrap database error: %v", err)
		}
	})

	runCase(t, "row error", func(t *testing.T) {
		rows := newStubRows()
		rows.err = errors.New("rows failed")
		repo := NewUserRepository(stubDB{
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) {
				return rows, nil
			},
		})

		_, err := repo.List(context.Background())
		assertDomainError(t, err, domain.CodeInternal, "list users")
	})
}

func TestUserRepositoryGetByID(t *testing.T) {
	want := domain.User{ID: "user-1", Email: "one@example.com", Name: "One"}

	runCase(t, "success", func(t *testing.T) {
		repo := NewUserRepository(stubDB{
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return stubRow{values: userValues(want)}
			},
		})

		got, err := repo.GetByID(context.Background(), want.ID)
		if err != nil {
			t.Fatalf("GetByID returned error: %v", err)
		}
		assertEqual(t, got, want)
	})

	runCase(t, "not found", func(t *testing.T) {
		repo := NewUserRepository(stubDB{
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return stubRow{err: pgx.ErrNoRows}
			},
		})

		_, err := repo.GetByID(context.Background(), "missing")
		assertDomainError(t, err, domain.CodeNotFound, "user not found")
	})

	runCase(t, "invalid id", func(t *testing.T) {
		repo := NewUserRepository(stubDB{
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return stubRow{err: &pgconn.PgError{Code: "22P02"}}
			},
		})

		_, err := repo.GetByID(context.Background(), "invalid")
		assertDomainError(t, err, domain.CodeInvalid, "invalid id")
	})
}

func TestUserRepositoryGetByEmail(t *testing.T) {
	want := domain.AuthUser{
		User:         domain.User{ID: "user-1", Email: "one@example.com", Name: "One"},
		PasswordHash: "hash",
	}

	runCase(t, "success", func(t *testing.T) {
		repo := NewUserRepository(stubDB{
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				values := append(userValues(want.User), want.PasswordHash)
				return stubRow{values: values}
			},
		})

		got, err := repo.GetByEmail(context.Background(), want.User.Email)
		if err != nil {
			t.Fatalf("GetByEmail returned error: %v", err)
		}
		assertEqual(t, got, want)
	})

	runCase(t, "not found", func(t *testing.T) {
		repo := NewUserRepository(stubDB{
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return stubRow{err: pgx.ErrNoRows}
			},
		})

		_, err := repo.GetByEmail(context.Background(), "missing@example.com")
		assertDomainError(t, err, domain.CodeNotFound, "user not found")
	})

	runCase(t, "database error", func(t *testing.T) {
		repo := NewUserRepository(stubDB{
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return stubRow{err: errors.New("query failed")}
			},
		})

		_, err := repo.GetByEmail(context.Background(), want.User.Email)
		assertDomainError(t, err, domain.CodeInternal, "get user by email")
	})
}

func TestUserRepositoryCreate(t *testing.T) {
	input := domain.User{Email: "one@example.com", Name: "One"}
	want := domain.User{ID: "user-1", Email: input.Email, Name: input.Name}

	runCase(t, "success", func(t *testing.T) {
		repo := NewUserRepository(stubDB{
			queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
				assertEqual(t, args, []any{input.Email, input.Name, "hash"})
				return stubRow{values: userValues(want)}
			},
		})

		got, err := repo.Create(context.Background(), input, "hash")
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		assertEqual(t, got, want)
	})

	runCase(t, "duplicate email", func(t *testing.T) {
		repo := NewUserRepository(stubDB{
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return stubRow{err: &pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"}}
			},
		})

		_, err := repo.Create(context.Background(), input, "hash")
		assertDomainError(t, err, domain.CodeConflict, "email already exists")
	})
}

func TestUserRepositoryUpdate(t *testing.T) {
	want := domain.User{ID: "user-1", Email: "updated@example.com", Name: "Updated"}

	runCase(t, "success", func(t *testing.T) {
		repo := NewUserRepository(stubDB{
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return stubRow{values: userValues(want)}
			},
		})

		got, err := repo.Update(context.Background(), want)
		if err != nil {
			t.Fatalf("Update returned error: %v", err)
		}
		assertEqual(t, got, want)
	})

	runCase(t, "not found", func(t *testing.T) {
		repo := NewUserRepository(stubDB{
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return stubRow{err: pgx.ErrNoRows}
			},
		})

		_, err := repo.Update(context.Background(), want)
		assertDomainError(t, err, domain.CodeNotFound, "user not found")
	})

	runCase(t, "duplicate email", func(t *testing.T) {
		repo := NewUserRepository(stubDB{
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return stubRow{err: &pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"}}
			},
		})

		_, err := repo.Update(context.Background(), want)
		assertDomainError(t, err, domain.CodeConflict, "email already exists")
	})
}

func TestUserRepositoryDelete(t *testing.T) {
	runCase(t, "success", func(t *testing.T) {
		repo := NewUserRepository(stubDB{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("DELETE 1"), nil
			},
		})

		if err := repo.Delete(context.Background(), "user-1"); err != nil {
			t.Fatalf("Delete returned error: %v", err)
		}
	})

	runCase(t, "not found", func(t *testing.T) {
		repo := NewUserRepository(stubDB{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("DELETE 0"), nil
			},
		})

		err := repo.Delete(context.Background(), "missing")
		assertDomainError(t, err, domain.CodeNotFound, "user not found")
	})

	runCase(t, "invalid id", func(t *testing.T) {
		repo := NewUserRepository(stubDB{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
				return pgconn.CommandTag{}, &pgconn.PgError{Code: "22P02"}
			},
		})

		err := repo.Delete(context.Background(), "invalid")
		assertDomainError(t, err, domain.CodeInvalid, "invalid id")
	})
}
