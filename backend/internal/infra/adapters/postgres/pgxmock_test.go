package postgres

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v5"

	"chimera/internal/domain"
	"chimera/internal/testkit"
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
			repo = NewUserRepository(db)
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
			repo = NewUserRepository(db)
		})
		r.Act(func(t *testing.T) { _, err = repo.GetByID(context.Background(), "missing") })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeNotFound, "user not found")
			assertExpectationsMet(t, db)
		})
	})
}

func TestTrackRepositoryPGXMock(t *testing.T) {
	runSpec(t, trackComponent, testkit.Spec{
		ID: "DA-PGX-TRK-CREATE-01", Method: "Create", Kind: testkit.KindPgxmock,
		Title:     "create sends exact insert arguments and reads back the generated id and object key",
		Given:     "pgxmock expects INSERT INTO tracks with [user-1, Track, Artist, 1024, pending] and returns the inserted row",
		When:      "Create is called with a pending track",
		Then:      "the returned track has id track-1, object key track-1.mp3 and the input fields, and all expectations are met",
		Technique: testkit.TechniqueEquivalence,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"user id": "user-1", "size bytes": "1024", "status": "pending"},
	}, func(t *testing.T, r testkit.Report) {
		input := domain.Track{
			UserID: "user-1", Title: "Track", Artist: "Artist",
			SizeBytes: 1024, Status: domain.TrackPending,
		}
		want := input
		want.ID = "track-1"
		want.ObjectKey = "track-1.mp3"
		var db pgxmock.PgxPoolIface
		var repo *TrackRepository
		var got domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			db = newMockPool(t)
			db.ExpectQuery(`INSERT INTO tracks`).
				WithArgs(input.UserID, input.Title, input.Artist, input.SizeBytes, string(input.Status)).
				WillReturnRows(pgxmock.NewRows([]string{
					"id", "user_id", "title", "artist", "object_key", "size_bytes", "status",
				}).AddRow(want.ID, want.UserID, want.Title, want.Artist, want.ObjectKey, want.SizeBytes, string(want.Status)))
			repo = NewTrackRepository(db)
		})
		r.Act(func(t *testing.T) { got, err = repo.Create(context.Background(), input) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, got, want)
			assertExpectationsMet(t, db)
		})
	})
}

func TestTrackLikeRepositoryPGXMock(t *testing.T) {
	runSpec(t, likeComponent, testkit.Spec{
		ID: "DA-PGX-LIKE-ADD-01", Method: "Add", Kind: testkit.KindPgxmock,
		Title:     "add wraps an unavailable database as internal like track error preserving the cause",
		Given:     "pgxmock expects INSERT INTO track_likes with [user-1, track-1] and returns a non-postgres error",
		When:      "Add is called",
		Then:      "an internal domain error with message \"like track\" wrapping the driver error is returned and all expectations are met",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"driver error": "database unavailable"},
	}, func(t *testing.T, r testkit.Report) {
		dbErr := errors.New("database unavailable")
		var db pgxmock.PgxPoolIface
		var repo *TrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			db = newMockPool(t)
			db.ExpectExec(`INSERT INTO track_likes`).
				WithArgs("user-1", "track-1").
				WillReturnError(dbErr)
			repo = NewTrackLikeRepository(db)
		})
		r.Act(func(t *testing.T) { err = repo.Add(context.Background(), "user-1", "track-1") })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInternal, "like track")
			assertWraps(t, err, dbErr)
			assertExpectationsMet(t, db)
		})
	})
}
