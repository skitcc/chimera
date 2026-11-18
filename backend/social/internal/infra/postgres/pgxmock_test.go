package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/pashagolub/pgxmock/v5"

	"chimera/social/internal/domain"
	"chimera/social/internal/testkit"
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
			repo = newTrackLikeRepository(db)
		})
		r.Act(func(t *testing.T) { err = repo.Add(context.Background(), "user-1", "track-1") })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInternal, "like track")
			assertWraps(t, err, dbErr)
			assertExpectationsMet(t, db)
		})
	})
}
