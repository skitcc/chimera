package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"chimera/social/internal/domain"
	"chimera/social/internal/testkit"
)

const likeComponent = "TrackLikeRepository"

func TestTrackLikeRepositoryAdd(t *testing.T) {
	const method = "Add"

	runSpec(t, likeComponent, testkit.Spec{
		ID: "DA-LIKE-ADD-01", Method: method,
		Title:     "add sends user and track ids and succeeds on insert",
		Given:     "the insert reports command tag INSERT 0 1",
		When:      "Add is called with user-1 and track-1",
		Then:      "the exec receives [user-1, track-1] and no error is returned",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"user id": "user-1", "track id": "track-1"},
	}, func(t *testing.T, r testkit.Report) {
		var gotArgs []any
		var repo *TrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			repo = newTrackLikeRepository(stubDB{
				execFn: func(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
					gotArgs = args
					return pgconn.NewCommandTag("INSERT 0 1"), nil
				},
			})
		})
		r.Act(func(t *testing.T) { err = repo.Add(context.Background(), "user-1", "track-1") })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, gotArgs, []any{"user-1", "track-1"})
		})
	})

	cases := []struct {
		id, title, sqlstate, constraint string
		code                            domain.Code
		message                         string
	}{
		{
			"DA-LIKE-ADD-02", "add maps unique violation on track_likes_pkey to track already liked",
			"23505", "track_likes_pkey", domain.CodeConflict, "track already liked",
		},
		{
			"DA-LIKE-ADD-03", "add maps unique violation on another constraint to unique constraint violated",
			"23505", "track_likes_other_key", domain.CodeConflict, "unique constraint violated",
		},
	}

	for _, tc := range cases {
		runSpec(t, likeComponent, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "the insert fails with SQLSTATE " + tc.sqlstate + " on constraint " + tc.constraint,
			When:      "Add is called",
			Then:      "a " + string(tc.code) + " domain error with message \"" + tc.message + "\" is returned",
			Technique: testkit.TechniqueDecisionTable,
			Severity:  testkit.SeverityCritical,
			Params:    map[string]string{"sqlstate": tc.sqlstate, "constraint": tc.constraint},
		}, func(t *testing.T, r testkit.Report) {
			var repo *TrackLikeRepository
			var err error
			r.Arrange(func(t *testing.T) {
				repo = newTrackLikeRepository(stubDB{
					execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
						return pgconn.CommandTag{}, &pgconn.PgError{Code: tc.sqlstate, ConstraintName: tc.constraint}
					},
				})
			})
			r.Act(func(t *testing.T) { err = repo.Add(context.Background(), "user-1", "track-1") })
			r.Assert(func(t *testing.T) { assertDomainError(t, err, tc.code, tc.message) })
		})
	}

	runSpec(t, likeComponent, testkit.Spec{
		ID: "DA-LIKE-ADD-04", Method: method,
		Title:     "add reports not found when a tombstone blocks the insert",
		Given:     "the guarded insert reports command tag INSERT 0 0 because the user or track is deleted",
		When:      "Add is called with user-1 and track-1",
		Then:      "a not_found domain error with message \"not found\" is returned",
		Technique: testkit.TechniqueState,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"rows affected": "0"},
	}, func(t *testing.T, r testkit.Report) {
		var repo *TrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			repo = newTrackLikeRepository(stubDB{
				execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
					return pgconn.NewCommandTag("INSERT 0 0"), nil
				},
			})
		})
		r.Act(func(t *testing.T) { err = repo.Add(context.Background(), "user-1", "track-1") })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeNotFound, "not found") })
	})
}

func TestTrackLikeRepositoryRemove(t *testing.T) {
	const method = "Remove"

	runSpec(t, likeComponent, testkit.Spec{
		ID: "DA-LIKE-REMOVE-01", Method: method,
		Title:     "remove sends user and track ids and succeeds when one row is deleted",
		Given:     "the delete reports command tag DELETE 1",
		When:      "Remove is called with user-1 and track-1",
		Then:      "the exec receives [user-1, track-1] and no error is returned",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"user id": "user-1", "track id": "track-1", "rows affected": "1"},
	}, func(t *testing.T, r testkit.Report) {
		var gotArgs []any
		var repo *TrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			repo = newTrackLikeRepository(stubDB{
				execFn: func(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
					gotArgs = args
					return pgconn.NewCommandTag("DELETE 1"), nil
				},
			})
		})
		r.Act(func(t *testing.T) { err = repo.Remove(context.Background(), "user-1", "track-1") })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, gotArgs, []any{"user-1", "track-1"})
		})
	})

	runSpec(t, likeComponent, testkit.Spec{
		ID: "DA-LIKE-REMOVE-02", Method: method,
		Title:     "remove of a missing like succeeds without error",
		Given:     "the delete reports command tag DELETE 0",
		When:      "Remove is called for a like that does not exist",
		Then:      "no error is returned because Remove ignores the affected row count",
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"rows affected": "0"},
	}, func(t *testing.T, r testkit.Report) {
		var repo *TrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			repo = newTrackLikeRepository(stubDB{
				execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
					return pgconn.NewCommandTag("DELETE 0"), nil
				},
			})
		})
		r.Act(func(t *testing.T) { err = repo.Remove(context.Background(), "user-1", "track-1") })
		r.Assert(func(t *testing.T) { assertNoError(t, err) })
	})

	runSpec(t, likeComponent, testkit.Spec{
		ID: "DA-LIKE-REMOVE-03", Method: method,
		Title:     "remove maps sqlstate 22P02 to invalid id",
		Given:     "the delete fails with SQLSTATE 22P02 (invalid text representation)",
		When:      "Remove is called with a non-uuid user id",
		Then:      "an invalid domain error with message \"invalid id\" is returned",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"user id": "invalid", "sqlstate": "22P02"},
	}, func(t *testing.T, r testkit.Report) {
		var repo *TrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			repo = newTrackLikeRepository(stubDB{
				execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
					return pgconn.CommandTag{}, &pgconn.PgError{Code: "22P02"}
				},
			})
		})
		r.Act(func(t *testing.T) { err = repo.Remove(context.Background(), "invalid", "track-1") })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeInvalid, "invalid id") })
	})

	runSpec(t, likeComponent, testkit.Spec{
		ID: "DA-LIKE-REMOVE-04", Method: method,
		Title:     "remove wraps a driver failure as internal unlike track error",
		Given:     "the delete fails with a non-postgres driver error",
		When:      "Remove is called",
		Then:      "an internal domain error with message \"unlike track\" wrapping the driver error is returned",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"driver error": "delete failed"},
	}, func(t *testing.T, r testkit.Report) {
		dbErr := errors.New("delete failed")
		var repo *TrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			repo = newTrackLikeRepository(stubDB{
				execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
					return pgconn.CommandTag{}, dbErr
				},
			})
		})
		r.Act(func(t *testing.T) { err = repo.Remove(context.Background(), "user-1", "track-1") })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInternal, "unlike track")
			assertWraps(t, err, dbErr)
		})
	})
}

func TestTrackLikeRepositoryListTrackIDs(t *testing.T) {
	const method = "ListTrackIDs"

	runSpec(t, likeComponent, testkit.Spec{
		ID: "DA-LIKE-IDS-01", Method: method,
		Title:     "list returns track ids in row order and closes rows",
		Given:     "the query returns rows track-2 and track-1",
		When:      "ListTrackIDs is called with user-1",
		Then:      "the query receives [user-1] and [track-2, track-1] is returned",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var gotArgs []any
		var rows *stubRows
		var repo *TrackLikeRepository
		var got []string
		var err error
		r.Arrange(func(t *testing.T) {
			rows = newStubRows([]any{"track-2"}, []any{"track-1"})
			repo = newTrackLikeRepository(stubDB{
				queryFn: func(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
					gotArgs = args
					return rows, nil
				},
			})
		})
		r.Act(func(t *testing.T) { got, err = repo.ListTrackIDs(context.Background(), "user-1") })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, gotArgs, []any{"user-1"})
			assertEqual(t, got, []string{"track-2", "track-1"})
			assertRowsClosed(t, rows)
		})
	})

	runSpec(t, likeComponent, testkit.Spec{
		ID: "DA-LIKE-IDS-02", Method: method,
		Title:     "list returns an empty slice when the user has no likes",
		Given:     "the query returns no rows",
		When:      "ListTrackIDs is called with user-1",
		Then:      "an empty non-nil slice is returned",
		Technique: testkit.TechniqueBoundary,
	}, func(t *testing.T, r testkit.Report) {
		var repo *TrackLikeRepository
		var got []string
		var err error
		r.Arrange(func(t *testing.T) {
			repo = newTrackLikeRepository(stubDB{
				queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return newStubRows(), nil },
			})
		})
		r.Act(func(t *testing.T) { got, err = repo.ListTrackIDs(context.Background(), "user-1") })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, got, []string{})
		})
	})

	runSpec(t, likeComponent, testkit.Spec{
		ID: "DA-LIKE-IDS-03", Method: method,
		Title:     "list wraps a scan failure as internal scan liked track error",
		Given:     "the first row fails to scan",
		When:      "ListTrackIDs is called",
		Then:      "an internal domain error with message \"scan liked track\" is returned and rows are closed",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var rows *stubRows
		var repo *TrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			rows = newStubRows([]any{"track-1"})
			rows.scanErrAt = 0
			repo = newTrackLikeRepository(stubDB{
				queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return rows, nil },
			})
		})
		r.Act(func(t *testing.T) { _, err = repo.ListTrackIDs(context.Background(), "user-1") })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInternal, "scan liked track")
			assertRowsClosed(t, rows)
		})
	})
}
