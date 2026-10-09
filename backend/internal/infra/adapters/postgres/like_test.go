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
			repo = NewTrackLikeRepository(stubDB{
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
			"DA-LIKE-ADD-03", "add maps unique violation on another constraint to email already exists",
			"23505", "track_likes_other_key", domain.CodeConflict, "email already exists",
		},
		{
			"DA-LIKE-ADD-04", "add maps foreign key violation to not found",
			"23503", "track_likes_track_id_fkey", domain.CodeNotFound, "not found",
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
				repo = NewTrackLikeRepository(stubDB{
					execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
						return pgconn.CommandTag{}, &pgconn.PgError{Code: tc.sqlstate, ConstraintName: tc.constraint}
					},
				})
			})
			r.Act(func(t *testing.T) { err = repo.Add(context.Background(), "user-1", "track-1") })
			r.Assert(func(t *testing.T) { assertDomainError(t, err, tc.code, tc.message) })
		})
	}
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
			repo = NewTrackLikeRepository(stubDB{
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
			repo = NewTrackLikeRepository(stubDB{
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
			repo = NewTrackLikeRepository(stubDB{
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
			repo = NewTrackLikeRepository(stubDB{
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

func TestTrackLikeRepositoryListReadyByUser(t *testing.T) {
	const method = "ListReadyByUser"
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

	runSpec(t, likeComponent, testkit.Spec{
		ID: "DA-LIKE-LISTREADY-01", Method: method,
		Title:     "list ready by user sends user id and ready status and returns liked tracks",
		Given:     "the database returns one liked ready track owned by another user",
		When:      "ListReadyByUser is called with user-1",
		Then:      "the query receives [user-1, ready], the liked track is returned without error, and the rows are closed",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"user id": "user-1"},
	}, func(t *testing.T, r testkit.Report) {
		var gotArgs []any
		var rows *stubRows
		var repo *TrackLikeRepository
		var got []domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			rows = newStubRows(trackValues(want[0]))
			repo = NewTrackLikeRepository(stubDB{
				queryFn: func(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
					gotArgs = args
					return rows, nil
				},
			})
		})
		r.Act(func(t *testing.T) { got, err = repo.ListReadyByUser(context.Background(), "user-1") })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, gotArgs, []any{"user-1", string(domain.TrackReady)})
			assertEqual(t, got, want)
			assertRowsClosed(t, rows)
		})
	})

	runSpec(t, likeComponent, testkit.Spec{
		ID: "DA-LIKE-LISTREADY-02", Method: method,
		Title:     "list ready by user wraps a query failure as internal list liked tracks error",
		Given:     "the database query fails with a driver error",
		When:      "ListReadyByUser is called",
		Then:      "an internal domain error with message \"list liked tracks\" wrapping the driver error and nil tracks are returned",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"driver error": "query failed"},
	}, func(t *testing.T, r testkit.Report) {
		dbErr := errors.New("query failed")
		var repo *TrackLikeRepository
		var got []domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewTrackLikeRepository(stubDB{
				queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return nil, dbErr },
			})
		})
		r.Act(func(t *testing.T) { got, err = repo.ListReadyByUser(context.Background(), "user-1") })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInternal, "list liked tracks")
			assertWraps(t, err, dbErr)
			assertEqual(t, got, []domain.Track(nil))
		})
	})

	runSpec(t, likeComponent, testkit.Spec{
		ID: "DA-LIKE-LISTREADY-03", Method: method,
		Title:     "list ready by user maps a rows iteration error to internal list tracks error",
		Given:     "the database returns no rows and rows.Err reports a driver error",
		When:      "ListReadyByUser is called",
		Then:      "an internal domain error with message \"list tracks\" wrapping the rows error is returned, and the rows are closed",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"rows error": "rows failed"},
	}, func(t *testing.T, r testkit.Report) {
		rowsErr := errors.New("rows failed")
		var rows *stubRows
		var repo *TrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			rows = newStubRows()
			rows.err = rowsErr
			repo = NewTrackLikeRepository(stubDB{
				queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return rows, nil },
			})
		})
		r.Act(func(t *testing.T) { _, err = repo.ListReadyByUser(context.Background(), "user-1") })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInternal, "list tracks")
			assertWraps(t, err, rowsErr)
			assertRowsClosed(t, rows)
		})
	})
}
