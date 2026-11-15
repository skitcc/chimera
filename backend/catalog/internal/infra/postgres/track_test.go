package postgres

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v5"

	"chimera/catalog/internal/domain"
	"chimera/catalog/internal/testkit"
)

const trackComponent = "TrackRepository"

func TestTrackRepositoryList(t *testing.T) {
	const method = "List"
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

	runSpec(t, trackComponent, testkit.Spec{
		ID: "DA-TRK-LIST-01", Method: method,
		Title:     "list with all filters sends status user and artist and returns scanned tracks",
		Given:     "a filter with status ready, user user-1, artist Artist and the database returns two track rows",
		When:      "List is called",
		Then:      "the query receives [ready, user-1, Artist], both tracks are returned in order, and the rows are closed",
		Technique: testkit.TechniqueDecisionTable,
		Params:    map[string]string{"status": "ready", "user id": "user-1", "artist": "Artist"},
	}, func(t *testing.T, r testkit.Report) {
		var gotArgs []any
		var rows *stubRows
		var repo *TrackRepository
		var got []domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			rows = newStubRows(trackValues(want[0]), trackValues(want[1]))
			repo = NewTrackRepository(stubDB{
				queryFn: func(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
					gotArgs = args
					return rows, nil
				},
			})
		})
		r.Act(func(t *testing.T) {
			got, err = repo.List(context.Background(), domain.TrackFilter{
				Status: domain.TrackReady,
				UserID: "user-1",
				Artist: "Artist",
			})
		})
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, gotArgs, []any{string(domain.TrackReady), "user-1", "Artist", nil})
			assertEqual(t, got, want)
			assertRowsClosed(t, rows)
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "DA-TRK-LIST-02", Method: method,
		Title:     "list with empty filter passes nil for every filter argument",
		Given:     "an empty filter and the database returns zero rows",
		When:      "List is called",
		Then:      "the query receives [nil, nil, nil, nil], an empty non-nil slice is returned without error, and the rows are closed",
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"filter": "empty"},
	}, func(t *testing.T, r testkit.Report) {
		var gotArgs []any
		var rows *stubRows
		var repo *TrackRepository
		var got []domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			rows = newStubRows()
			repo = NewTrackRepository(stubDB{
				queryFn: func(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
					gotArgs = args
					return rows, nil
				},
			})
		})
		r.Act(func(t *testing.T) { got, err = repo.List(context.Background(), domain.TrackFilter{}) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, gotArgs, []any{nil, nil, nil, nil})
			assertEqual(t, got, []domain.Track{})
			assertRowsClosed(t, rows)
		})
	})

	cases := []struct {
		id, title string
		filter    domain.TrackFilter
		wantArgs  []any
	}{
		{
			"DA-TRK-LIST-03", "list with only status filter passes nil user and artist",
			domain.TrackFilter{Status: domain.TrackProcessing},
			[]any{string(domain.TrackProcessing), nil, nil, nil},
		},
		{
			"DA-TRK-LIST-04", "list with only user filter passes nil status and artist",
			domain.TrackFilter{UserID: "user-1"},
			[]any{nil, "user-1", nil, nil},
		},
		{
			"DA-TRK-LIST-05", "list with only artist filter passes nil status and user",
			domain.TrackFilter{Artist: "Artist"},
			[]any{nil, nil, "Artist", nil},
		},
	}

	for _, tc := range cases {
		runSpec(t, trackComponent, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "a filter with exactly one non-empty field",
			When:      "List is called",
			Then:      "only the set field is sent as a query argument and the other filters are nil",
			Technique: testkit.TechniqueDecisionTable,
			Params: map[string]string{
				"status":  string(tc.filter.Status),
				"user id": tc.filter.UserID,
				"artist":  tc.filter.Artist,
			},
		}, func(t *testing.T, r testkit.Report) {
			var gotArgs []any
			var repo *TrackRepository
			var err error
			r.Arrange(func(t *testing.T) {
				repo = NewTrackRepository(stubDB{
					queryFn: func(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
						gotArgs = args
						return newStubRows(), nil
					},
				})
			})
			r.Act(func(t *testing.T) { _, err = repo.List(context.Background(), tc.filter) })
			r.Assert(func(t *testing.T) {
				assertNoError(t, err)
				assertEqual(t, gotArgs, tc.wantArgs)
			})
		})
	}

	runSpec(t, trackComponent, testkit.Spec{
		ID: "DA-TRK-LIST-06", Method: method,
		Title:     "list wraps a query failure as internal list tracks error",
		Given:     "the database query fails with a driver error",
		When:      "List is called",
		Then:      "an internal domain error with message \"list tracks\" wrapping the driver error and nil tracks are returned",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"driver error": "query failed"},
	}, func(t *testing.T, r testkit.Report) {
		dbErr := errors.New("query failed")
		var repo *TrackRepository
		var got []domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewTrackRepository(stubDB{
				queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return nil, dbErr },
			})
		})
		r.Act(func(t *testing.T) { got, err = repo.List(context.Background(), domain.TrackFilter{}) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInternal, "list tracks")
			assertWraps(t, err, dbErr)
			assertEqual(t, got, []domain.Track(nil))
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "DA-TRK-LIST-07", Method: method,
		Title:     "list maps a row scan failure to internal scan track error and closes rows",
		Given:     "the database returns one track row whose scan fails",
		When:      "List is called",
		Then:      "an internal domain error with message \"scan track\" and nil tracks are returned, and the rows are closed",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"scan error at row": "0"},
	}, func(t *testing.T, r testkit.Report) {
		var rows *stubRows
		var repo *TrackRepository
		var got []domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			rows = newStubRows(trackValues(want[0]))
			rows.scanErrAt = 0
			repo = NewTrackRepository(stubDB{
				queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return rows, nil },
			})
		})
		r.Act(func(t *testing.T) { got, err = repo.List(context.Background(), domain.TrackFilter{}) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInternal, "scan track")
			assertEqual(t, got, []domain.Track(nil))
			assertRowsClosed(t, rows)
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "DA-TRK-LIST-08", Method: method,
		Title:     "list maps a rows iteration error to internal list tracks error",
		Given:     "the database returns no rows and rows.Err reports a driver error",
		When:      "List is called",
		Then:      "an internal domain error with message \"list tracks\" wrapping the rows error is returned, and the rows are closed",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"rows error": "rows failed"},
	}, func(t *testing.T, r testkit.Report) {
		rowsErr := errors.New("rows failed")
		var rows *stubRows
		var repo *TrackRepository
		var err error
		r.Arrange(func(t *testing.T) {
			rows = newStubRows()
			rows.err = rowsErr
			repo = NewTrackRepository(stubDB{
				queryFn: func(context.Context, string, ...any) (pgx.Rows, error) { return rows, nil },
			})
		})
		r.Act(func(t *testing.T) { _, err = repo.List(context.Background(), domain.TrackFilter{}) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInternal, "list tracks")
			assertWraps(t, err, rowsErr)
			assertRowsClosed(t, rows)
		})
	})
}

func TestTrackRepositoryGetByID(t *testing.T) {
	const method = "GetByID"
	want := domain.Track{
		ID:        "track-1",
		UserID:    "user-1",
		Title:     "Track",
		Artist:    "Artist",
		ObjectKey: "track-1.mp3",
		SizeBytes: 42,
		Status:    domain.TrackReady,
	}

	runSpec(t, trackComponent, testkit.Spec{
		ID: "DA-TRK-GET-01", Method: method,
		Title:     "get by id sends the id and returns the scanned track with status",
		Given:     "the database returns a ready track row for track-1",
		When:      "GetByID is called with track-1",
		Then:      "the query receives [track-1] and the track including status ready is returned without error",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"id": want.ID},
	}, func(t *testing.T, r testkit.Report) {
		var gotArgs []any
		var repo *TrackRepository
		var got domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewTrackRepository(stubDB{
				queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
					gotArgs = args
					return stubRow{values: trackValues(want)}
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

	runSpec(t, trackComponent, testkit.Spec{
		ID: "DA-TRK-GET-02", Method: method,
		Title:     "get by id maps pgx no rows to track not found",
		Given:     "the database row scan returns pgx.ErrNoRows",
		When:      "GetByID is called with a missing id",
		Then:      "a not_found domain error with message \"track not found\" and a zero track are returned",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"id": "missing", "driver error": "pgx.ErrNoRows"},
	}, func(t *testing.T, r testkit.Report) {
		var repo *TrackRepository
		var got domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewTrackRepository(stubDB{
				queryRowFn: func(context.Context, string, ...any) pgx.Row { return stubRow{err: pgx.ErrNoRows} },
			})
		})
		r.Act(func(t *testing.T) { got, err = repo.GetByID(context.Background(), "missing") })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeNotFound, "track not found")
			assertEqual(t, got, domain.Track{})
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "DA-TRK-GET-03", Method: method,
		Title:     "get by id maps sqlstate 22P02 to invalid id",
		Given:     "the database rejects the id with SQLSTATE 22P02 (invalid text representation)",
		When:      "GetByID is called with a non-uuid id",
		Then:      "an invalid domain error with message \"invalid id\" is returned",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"id": "invalid", "sqlstate": "22P02"},
	}, func(t *testing.T, r testkit.Report) {
		var repo *TrackRepository
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewTrackRepository(stubDB{
				queryRowFn: func(context.Context, string, ...any) pgx.Row {
					return stubRow{err: &pgconn.PgError{Code: "22P02"}}
				},
			})
		})
		r.Act(func(t *testing.T) { _, err = repo.GetByID(context.Background(), "invalid") })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeInvalid, "invalid id") })
	})
}

func TestTrackRepositoryCreate(t *testing.T) {
	const method = "Create"
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

	runSpec(t, trackComponent, testkit.Spec{
		ID: "DA-TRK-CREATE-01", Method: method,
		Title:     "create sends owner title artist size and status and returns generated id and object key",
		Given:     "a pending track without id and the database returns the inserted row with id track-1",
		When:      "Create is called",
		Then:      "the insert receives [user-1, Track, Artist, 42, pending] and the track with id track-1 and object key track-1.mp3 is returned",
		Technique: testkit.TechniqueEquivalence,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"user id": input.UserID, "size bytes": "42", "status": string(input.Status)},
	}, func(t *testing.T, r testkit.Report) {
		var gotArgs []any
		var repo *TrackRepository
		var got domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewTrackRepository(stubDB{
				queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
					gotArgs = args
					return stubRow{values: trackValues(want)}
				},
			})
		})
		r.Act(func(t *testing.T) { got, err = repo.Create(context.Background(), input) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, gotArgs, []any{input.UserID, input.Title, input.Artist, input.SizeBytes, string(input.Status)})
			assertEqual(t, got, want)
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "DA-TRK-CREATE-02", Method: method,
		Title:     "create refuses an owner recorded in deleted_users",
		Given:     "the guarded insert returns no row because the owner is in deleted_users",
		When:      "Create is called for a deleted user",
		Then:      "a not_found domain error with message \"user not found\" and a zero track are returned",
		Technique: testkit.TechniqueDecisionTable,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"result": "pgx.ErrNoRows"},
	}, func(t *testing.T, r testkit.Report) {
		var repo *TrackRepository
		var got domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewTrackRepository(stubDB{
				queryRowFn: func(context.Context, string, ...any) pgx.Row {
					return stubRow{err: pgx.ErrNoRows}
				},
			})
		})
		r.Act(func(t *testing.T) { got, err = repo.Create(context.Background(), input) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeNotFound, "user not found")
			assertEqual(t, got, domain.Track{})
		})
	})
}

func TestTrackRepositoryUpdate(t *testing.T) {
	const method = "Update"
	want := domain.Track{
		ID:        "track-1",
		UserID:    "user-1",
		Title:     "Updated",
		Artist:    "Artist",
		ObjectKey: "custom.mp3",
		SizeBytes: 84,
		Status:    domain.TrackReady,
	}

	runSpec(t, trackComponent, testkit.Spec{
		ID: "DA-TRK-UPDATE-01", Method: method,
		Title:     "update sends every column including the explicit object key and returns the updated track",
		Given:     "a track with object key custom.mp3 and the database returns the updated row",
		When:      "Update is called",
		Then:      "the update receives [track-1, user-1, Updated, Artist, custom.mp3, 84, ready] and the updated track is returned",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"id": want.ID, "object key": want.ObjectKey, "status": string(want.Status)},
	}, func(t *testing.T, r testkit.Report) {
		var gotArgs []any
		var repo *TrackRepository
		var got domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewTrackRepository(stubDB{
				queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
					gotArgs = args
					return stubRow{values: trackValues(want)}
				},
			})
		})
		r.Act(func(t *testing.T) { got, err = repo.Update(context.Background(), want) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, gotArgs, []any{
				want.ID, want.UserID, want.Title, want.Artist, want.ObjectKey, want.SizeBytes, string(want.Status),
			})
			assertEqual(t, got, want)
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "DA-TRK-UPDATE-02", Method: method,
		Title:     "update with empty object key sends the id based default key",
		Given:     "a track track-1 with an empty object key",
		When:      "Update is called",
		Then:      "the object key argument sent to the database is track-1.mp3",
		Technique: testkit.TechniqueBoundary,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"id": want.ID, "object key": ""},
	}, func(t *testing.T, r testkit.Report) {
		in := want
		in.ObjectKey = ""
		var gotArgs []any
		var repo *TrackRepository
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewTrackRepository(stubDB{
				queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
					gotArgs = args
					return stubRow{values: trackValues(want)}
				},
			})
		})
		r.Act(func(t *testing.T) { _, err = repo.Update(context.Background(), in) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, len(gotArgs), 7)
			assertEqual(t, gotArgs[4], any("track-1.mp3"))
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "DA-TRK-UPDATE-03", Method: method,
		Title:     "update maps pgx no rows to track not found",
		Given:     "the update returns no row because the id does not exist",
		When:      "Update is called",
		Then:      "a not_found domain error with message \"track not found\" and a zero track are returned",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"driver error": "pgx.ErrNoRows"},
	}, func(t *testing.T, r testkit.Report) {
		var repo *TrackRepository
		var got domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewTrackRepository(stubDB{
				queryRowFn: func(context.Context, string, ...any) pgx.Row { return stubRow{err: pgx.ErrNoRows} },
			})
		})
		r.Act(func(t *testing.T) { got, err = repo.Update(context.Background(), want) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeNotFound, "track not found")
			assertEqual(t, got, domain.Track{})
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "DA-TRK-UPDATE-04", Method: method,
		Title:     "update maps foreign key violation on tracks_user_id_fkey to not found",
		Given:     "the update fails with SQLSTATE 23503 on constraint tracks_user_id_fkey",
		When:      "Update is called with an owner that does not exist",
		Then:      "a not_found domain error with message \"not found\" is returned",
		Technique: testkit.TechniqueDecisionTable,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"sqlstate": "23503", "constraint": "tracks_user_id_fkey"},
	}, func(t *testing.T, r testkit.Report) {
		var repo *TrackRepository
		var err error
		r.Arrange(func(t *testing.T) {
			repo = NewTrackRepository(stubDB{
				queryRowFn: func(context.Context, string, ...any) pgx.Row {
					return stubRow{err: &pgconn.PgError{Code: "23503", ConstraintName: "tracks_user_id_fkey"}}
				},
			})
		})
		r.Act(func(t *testing.T) { _, err = repo.Update(context.Background(), want) })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeNotFound, "not found") })
	})
}

func TestTrackRepositoryDelete(t *testing.T) {
	const method = "Delete"
	const deleteSQL = `DELETE FROM tracks WHERE id = $1::uuid RETURNING id::text, user_id::text, object_key`
	const outboxSQL = `INSERT INTO outbox`

	runSpec(t, trackComponent, testkit.Spec{
		ID: "DA-TRK-DELETE-01", Method: method, Kind: testkit.KindPgxmock,
		Title:     "delete removes the row and writes TrackDeleted to the outbox in one transaction",
		Given:     "the delete returns track-1 of user-1 with object key track-1.mp3",
		When:      "Delete is called with track-1",
		Then:      "an outbox row for catalog.track.v1 keyed by track-1 is inserted and the transaction commits",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"id": "track-1"},
	}, func(t *testing.T, r testkit.Report) {
		var db pgxmock.PgxPoolIface
		var repo *TrackRepository
		var err error
		r.Arrange(func(t *testing.T) {
			db = newMockPool(t)
			db.ExpectBegin()
			db.ExpectQuery(regexp.QuoteMeta(deleteSQL)).WithArgs("track-1").
				WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "object_key"}).
					AddRow("track-1", "user-1", "track-1.mp3"))
			db.ExpectExec(regexp.QuoteMeta(outboxSQL)).
				WithArgs("catalog.track.v1", "track-1", "chimera.catalog.track.deleted.v1", pgxmock.AnyArg(), pgxmock.AnyArg()).
				WillReturnResult(pgxmock.NewResult("INSERT", 1))
			db.ExpectCommit()
			db.ExpectRollback()
			repo = NewTrackRepository(db)
		})
		r.Act(func(t *testing.T) { err = repo.Delete(context.Background(), "track-1") })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertExpectationsMet(t, db)
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "DA-TRK-DELETE-02", Method: method, Kind: testkit.KindPgxmock,
		Title:     "delete maps a missing row to track not found and writes no event",
		Given:     "the delete returns no row",
		When:      "Delete is called with a missing id",
		Then:      "a not_found domain error with message \"track not found\" is returned and the transaction rolls back",
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"id": "missing"},
	}, func(t *testing.T, r testkit.Report) {
		var db pgxmock.PgxPoolIface
		var repo *TrackRepository
		var err error
		r.Arrange(func(t *testing.T) {
			db = newMockPool(t)
			db.ExpectBegin()
			db.ExpectQuery(regexp.QuoteMeta(deleteSQL)).WithArgs("missing").WillReturnError(pgx.ErrNoRows)
			db.ExpectRollback()
			repo = NewTrackRepository(db)
		})
		r.Act(func(t *testing.T) { err = repo.Delete(context.Background(), "missing") })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeNotFound, "track not found")
			assertExpectationsMet(t, db)
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "DA-TRK-DELETE-03", Method: method, Kind: testkit.KindPgxmock,
		Title:     "delete maps sqlstate 22P02 to invalid id",
		Given:     "the delete fails with SQLSTATE 22P02 (invalid text representation)",
		When:      "Delete is called with a non-uuid id",
		Then:      "an invalid domain error with message \"invalid id\" is returned",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"id": "invalid", "sqlstate": "22P02"},
	}, func(t *testing.T, r testkit.Report) {
		var db pgxmock.PgxPoolIface
		var repo *TrackRepository
		var err error
		r.Arrange(func(t *testing.T) {
			db = newMockPool(t)
			db.ExpectBegin()
			db.ExpectQuery(regexp.QuoteMeta(deleteSQL)).WithArgs("invalid").
				WillReturnError(&pgconn.PgError{Code: "22P02"})
			db.ExpectRollback()
			repo = NewTrackRepository(db)
		})
		r.Act(func(t *testing.T) { err = repo.Delete(context.Background(), "invalid") })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInvalid, "invalid id")
			assertExpectationsMet(t, db)
		})
	})
}
