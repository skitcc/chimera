package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"chimera/internal/domain"
)

func TestTrackLikeRepositoryAdd(t *testing.T) {
	runCase(t, "success", func(t *testing.T) {
		repo := NewTrackLikeRepository(stubDB{
			execFn: func(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
				assertEqual(t, args, []any{"user-1", "track-1"})
				return pgconn.NewCommandTag("INSERT 0 1"), nil
			},
		})

		if err := repo.Add(context.Background(), "user-1", "track-1"); err != nil {
			t.Fatalf("Add returned error: %v", err)
		}
	})

	runCase(t, "duplicate like constraint", func(t *testing.T) {
		repo := NewTrackLikeRepository(stubDB{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
				return pgconn.CommandTag{}, &pgconn.PgError{
					Code:           "23505",
					ConstraintName: "track_likes_pkey",
				}
			},
		})

		err := repo.Add(context.Background(), "user-1", "track-1")
		assertDomainError(t, err, domain.CodeConflict, "track already liked")
	})

	runCase(t, "missing foreign key", func(t *testing.T) {
		repo := NewTrackLikeRepository(stubDB{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
				return pgconn.CommandTag{}, &pgconn.PgError{Code: "23503"}
			},
		})

		err := repo.Add(context.Background(), "missing", "track-1")
		assertDomainError(t, err, domain.CodeNotFound, "not found")
	})
}

func TestTrackLikeRepositoryRemove(t *testing.T) {
	runCase(t, "success", func(t *testing.T) {
		repo := NewTrackLikeRepository(stubDB{
			execFn: func(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
				assertEqual(t, args, []any{"user-1", "track-1"})
				return pgconn.NewCommandTag("DELETE 1"), nil
			},
		})

		if err := repo.Remove(context.Background(), "user-1", "track-1"); err != nil {
			t.Fatalf("Remove returned error: %v", err)
		}
	})

	runCase(t, "invalid id", func(t *testing.T) {
		repo := NewTrackLikeRepository(stubDB{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
				return pgconn.CommandTag{}, &pgconn.PgError{Code: "22P02"}
			},
		})

		err := repo.Remove(context.Background(), "invalid", "track-1")
		assertDomainError(t, err, domain.CodeInvalid, "invalid id")
	})

	runCase(t, "database error", func(t *testing.T) {
		dbErr := errors.New("delete failed")
		repo := NewTrackLikeRepository(stubDB{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
				return pgconn.CommandTag{}, dbErr
			},
		})

		err := repo.Remove(context.Background(), "user-1", "track-1")
		assertDomainError(t, err, domain.CodeInternal, "unlike track")
		if !errors.Is(err, dbErr) {
			t.Fatalf("error does not wrap database error: %v", err)
		}
	})
}

func TestTrackLikeRepositoryListReadyByUser(t *testing.T) {
	want := []domain.Track{
		{
			ID:        "track-1",
			UserID:    "owner-1",
			Title:     "Liked",
			Artist:    "Artist",
			ObjectKey: "track-1.mp3",
			SizeBytes: 42,
			Status:    domain.TrackReady,
		},
	}

	runCase(t, "success", func(t *testing.T) {
		rows := newStubRows(trackValues(want[0]))
		repo := NewTrackLikeRepository(stubDB{
			queryFn: func(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
				assertEqual(t, args, []any{"user-1", string(domain.TrackReady)})
				return rows, nil
			},
		})

		got, err := repo.ListReadyByUser(context.Background(), "user-1")
		if err != nil {
			t.Fatalf("ListReadyByUser returned error: %v", err)
		}
		assertEqual(t, got, want)
	})

	runCase(t, "query error", func(t *testing.T) {
		dbErr := errors.New("query failed")
		repo := NewTrackLikeRepository(stubDB{
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) {
				return nil, dbErr
			},
		})

		_, err := repo.ListReadyByUser(context.Background(), "user-1")
		assertDomainError(t, err, domain.CodeInternal, "list liked tracks")
		if !errors.Is(err, dbErr) {
			t.Fatalf("error does not wrap database error: %v", err)
		}
	})

	runCase(t, "rows error", func(t *testing.T) {
		rows := newStubRows()
		rows.err = errors.New("rows failed")
		repo := NewTrackLikeRepository(stubDB{
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) {
				return rows, nil
			},
		})

		_, err := repo.ListReadyByUser(context.Background(), "user-1")
		assertDomainError(t, err, domain.CodeInternal, "list tracks")
	})
}
