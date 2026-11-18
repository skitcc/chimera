package postgres

import (
	"context"
	"testing"

	"github.com/pashagolub/pgxmock/v5"

	"chimera/catalog/internal/domain"
	"chimera/catalog/internal/testkit"
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
			repo = newTrackRepository(db)
		})
		r.Act(func(t *testing.T) { got, err = repo.Create(context.Background(), input) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, got, want)
			assertExpectationsMet(t, db)
		})
	})
}
