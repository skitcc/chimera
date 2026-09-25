package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"chimera/internal/domain"
)

func TestTrackRepositoryList(t *testing.T) {
	want := []domain.Track{
		{
			ID:        "track-1",
			UserID:    "user-1",
			Title:     "First",
			Artist:    "Artist",
			ObjectKey: "track-1.mp3",
			SizeBytes: 10,
			Status:    domain.TrackReady,
		},
		{
			ID:        "track-2",
			UserID:    "user-1",
			Title:     "Second",
			Artist:    "Artist",
			ObjectKey: "track-2.mp3",
			SizeBytes: 20,
			Status:    domain.TrackReady,
		},
	}

	runCase(t, "success with filters", func(t *testing.T) {
		rows := newStubRows(trackValues(want[0]), trackValues(want[1]))
		repo := NewTrackRepository(stubDB{
			queryFn: func(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
				assertEqual(t, args, []any{string(domain.TrackReady), "user-1", "Artist"})
				return rows, nil
			},
		})

		got, err := repo.List(context.Background(), domain.TrackFilter{
			Status: domain.TrackReady,
			UserID: "user-1",
			Artist: "Artist",
		})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		assertEqual(t, got, want)
		if !rows.closed {
			t.Fatal("List did not close rows")
		}
	})

	runCase(t, "success without filters", func(t *testing.T) {
		repo := NewTrackRepository(stubDB{
			queryFn: func(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
				assertEqual(t, args, []any{nil, nil, nil})
				return newStubRows(), nil
			},
		})

		got, err := repo.List(context.Background(), domain.TrackFilter{})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		assertEqual(t, got, []domain.Track{})
	})

	runCase(t, "query error", func(t *testing.T) {
		dbErr := errors.New("query failed")
		repo := NewTrackRepository(stubDB{
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) {
				return nil, dbErr
			},
		})

		_, err := repo.List(context.Background(), domain.TrackFilter{})
		assertDomainError(t, err, domain.CodeInternal, "list tracks")
		if !errors.Is(err, dbErr) {
			t.Fatalf("error does not wrap database error: %v", err)
		}
	})

	runCase(t, "scan error", func(t *testing.T) {
		rows := newStubRows(trackValues(want[0]))
		rows.scanErrAt = 0
		repo := NewTrackRepository(stubDB{
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) {
				return rows, nil
			},
		})

		_, err := repo.List(context.Background(), domain.TrackFilter{})
		assertDomainError(t, err, domain.CodeInternal, "scan track")
	})
}

func TestTrackRepositoryGetByID(t *testing.T) {
	want := domain.Track{
		ID:        "track-1",
		UserID:    "user-1",
		Title:     "Track",
		Artist:    "Artist",
		ObjectKey: "track-1.mp3",
		SizeBytes: 42,
		Status:    domain.TrackReady,
	}

	runCase(t, "success", func(t *testing.T) {
		repo := NewTrackRepository(stubDB{
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return stubRow{values: trackValues(want)}
			},
		})

		got, err := repo.GetByID(context.Background(), want.ID)
		if err != nil {
			t.Fatalf("GetByID returned error: %v", err)
		}
		assertEqual(t, got, want)
	})

	runCase(t, "not found", func(t *testing.T) {
		repo := NewTrackRepository(stubDB{
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return stubRow{err: pgx.ErrNoRows}
			},
		})

		_, err := repo.GetByID(context.Background(), "missing")
		assertDomainError(t, err, domain.CodeNotFound, "track not found")
	})

	runCase(t, "invalid id", func(t *testing.T) {
		repo := NewTrackRepository(stubDB{
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return stubRow{err: &pgconn.PgError{Code: "22P02"}}
			},
		})

		_, err := repo.GetByID(context.Background(), "invalid")
		assertDomainError(t, err, domain.CodeInvalid, "invalid id")
	})
}

func TestTrackRepositoryCreate(t *testing.T) {
	input := domain.Track{
		UserID:    "user-1",
		Title:     "Track",
		Artist:    "Artist",
		SizeBytes: 42,
		Status:    domain.TrackPending,
	}
	want := input
	want.ID = "track-1"
	want.ObjectKey = "track-1.mp3"

	runCase(t, "success", func(t *testing.T) {
		repo := NewTrackRepository(stubDB{
			queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
				assertEqual(t, args, []any{
					input.UserID,
					input.Title,
					input.Artist,
					input.SizeBytes,
					string(input.Status),
				})
				return stubRow{values: trackValues(want)}
			},
		})

		got, err := repo.Create(context.Background(), input)
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		assertEqual(t, got, want)
	})

	runCase(t, "missing user constraint", func(t *testing.T) {
		repo := NewTrackRepository(stubDB{
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return stubRow{err: &pgconn.PgError{Code: "23503", ConstraintName: "tracks_user_id_fkey"}}
			},
		})

		_, err := repo.Create(context.Background(), input)
		assertDomainError(t, err, domain.CodeNotFound, "not found")
	})
}

func TestTrackRepositoryUpdate(t *testing.T) {
	want := domain.Track{
		ID:        "track-1",
		UserID:    "user-1",
		Title:     "Updated",
		Artist:    "Artist",
		ObjectKey: "custom.mp3",
		SizeBytes: 84,
		Status:    domain.TrackReady,
	}

	runCase(t, "success", func(t *testing.T) {
		repo := NewTrackRepository(stubDB{
			queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
				assertEqual(t, args, []any{
					want.ID,
					want.UserID,
					want.Title,
					want.Artist,
					want.ObjectKey,
					want.SizeBytes,
					string(want.Status),
				})
				return stubRow{values: trackValues(want)}
			},
		})

		got, err := repo.Update(context.Background(), want)
		if err != nil {
			t.Fatalf("Update returned error: %v", err)
		}
		assertEqual(t, got, want)
	})

	runCase(t, "not found", func(t *testing.T) {
		repo := NewTrackRepository(stubDB{
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return stubRow{err: pgx.ErrNoRows}
			},
		})

		_, err := repo.Update(context.Background(), want)
		assertDomainError(t, err, domain.CodeNotFound, "track not found")
	})

	runCase(t, "missing user constraint", func(t *testing.T) {
		repo := NewTrackRepository(stubDB{
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return stubRow{err: &pgconn.PgError{Code: "23503", ConstraintName: "tracks_user_id_fkey"}}
			},
		})

		_, err := repo.Update(context.Background(), want)
		assertDomainError(t, err, domain.CodeNotFound, "not found")
	})
}

func TestTrackRepositoryDelete(t *testing.T) {
	runCase(t, "success", func(t *testing.T) {
		repo := NewTrackRepository(stubDB{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("DELETE 1"), nil
			},
		})

		if err := repo.Delete(context.Background(), "track-1"); err != nil {
			t.Fatalf("Delete returned error: %v", err)
		}
	})

	runCase(t, "not found", func(t *testing.T) {
		repo := NewTrackRepository(stubDB{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("DELETE 0"), nil
			},
		})

		err := repo.Delete(context.Background(), "missing")
		assertDomainError(t, err, domain.CodeNotFound, "track not found")
	})

	runCase(t, "invalid id", func(t *testing.T) {
		repo := NewTrackRepository(stubDB{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
				return pgconn.CommandTag{}, &pgconn.PgError{Code: "22P02"}
			},
		})

		err := repo.Delete(context.Background(), "invalid")
		assertDomainError(t, err, domain.CodeInvalid, "invalid id")
	})
}
