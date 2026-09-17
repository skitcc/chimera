package postgres

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v5"

	"chimera/internal/domain"
)

func TestRepositoriesLondonPGXMock(t *testing.T) {
	runCase(t, "user lookup follows the expected query protocol", func(t *testing.T) {
		// Arrange
		db, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("create pgxmock: %v", err)
		}
		t.Cleanup(func() { db.Close() })
		db.ExpectQuery(regexp.QuoteMeta(`SELECT id::text, email, name FROM users WHERE id = $1::uuid`)).
			WithArgs("user-1").
			WillReturnRows(pgxmock.NewRows([]string{"id", "email", "name"}).
				AddRow("user-1", "listener@example.com", "Listener"))
		repo := NewUserRepository(db)

		// Act
		got, err := repo.GetByID(context.Background(), "user-1")

		// Assert
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		assertEqual(t, got, domain.User{ID: "user-1", Email: "listener@example.com", Name: "Listener"})
		if err := db.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet database expectations: %v", err)
		}
	})

	runCase(t, "missing user maps pgx no rows to domain not found", func(t *testing.T) {
		// Arrange
		db, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("create pgxmock: %v", err)
		}
		t.Cleanup(func() { db.Close() })
		db.ExpectQuery(regexp.QuoteMeta(`SELECT id::text, email, name FROM users WHERE id = $1::uuid`)).
			WithArgs("missing").
			WillReturnError(pgx.ErrNoRows)
		repo := NewUserRepository(db)

		// Act
		_, err = repo.GetByID(context.Background(), "missing")

		// Assert
		assertDomainError(t, err, domain.CodeNotFound, "user not found")
		if err := db.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet database expectations: %v", err)
		}
	})

	runCase(t, "track creation sends exact values and reads returned state", func(t *testing.T) {
		// Arrange
		db, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("create pgxmock: %v", err)
		}
		t.Cleanup(func() { db.Close() })
		input := domain.Track{
			UserID: "user-1", Title: "Track", Artist: "Artist",
			SizeBytes: 1024, Status: domain.TrackPending,
		}
		db.ExpectQuery(`INSERT INTO tracks`).
			WithArgs(input.UserID, input.Title, input.Artist, input.SizeBytes, string(input.Status)).
			WillReturnRows(pgxmock.NewRows([]string{
				"id", "user_id", "title", "artist", "object_key", "size_bytes", "status",
			}).AddRow("track-1", input.UserID, input.Title, input.Artist, "track-1.mp3", input.SizeBytes, string(input.Status)))
		repo := NewTrackRepository(db)

		// Act
		got, err := repo.Create(context.Background(), input)

		// Assert
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		assertEqual(t, got.ID, "track-1")
		assertEqual(t, got.ObjectKey, "track-1.mp3")
		if err := db.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet database expectations: %v", err)
		}
	})

	runCase(t, "like failure preserves database error as a domain error", func(t *testing.T) {
		// Arrange
		db, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("create pgxmock: %v", err)
		}
		t.Cleanup(func() { db.Close() })
		dbErr := errors.New("database unavailable")
		db.ExpectExec(`INSERT INTO track_likes`).
			WithArgs("user-1", "track-1").
			WillReturnError(dbErr)
		repo := NewTrackLikeRepository(db)

		// Act
		err = repo.Add(context.Background(), "user-1", "track-1")

		// Assert
		assertDomainError(t, err, domain.CodeInternal, "like track")
		if err := db.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet database expectations: %v", err)
		}
	})
}
