//go:build integration

package usecase

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"chimera/catalog/internal/domain"
	"chimera/catalog/internal/infra/postgres"
	"chimera/catalog/internal/testkit"
	"chimera/catalog/migrations"
)

const (
	integrationMaxBytes = int64(1024)
	integrationSize     = int64(128)
	nilUUID             = "00000000-0000-0000-0000-000000000000"
	resetTables         = `TRUNCATE outbox, processed_events, deleted_users, tracks RESTART IDENTITY CASCADE`
)

type seededUser struct {
	ID string
}

var errPresign = errors.New("presign unavailable")

// storedObject stands in for S3. These cases use PostgreSQL and do not call RustFS.
type storedObject struct {
	size       int64
	presignErr error
}

func (s storedObject) PresignPut(context.Context, string) (string, error) {
	if s.presignErr != nil {
		return "", s.presignErr
	}
	return "http://objects/upload", nil
}

func (s storedObject) PresignGet(context.Context, string) (string, error) {
	return "http://objects/stream", nil
}

func (s storedObject) Stat(context.Context, string) (int64, error) {
	return s.size, nil
}

type appFixture struct {
	ctx     context.Context
	pool    *pgxpool.Pool
	tracks  *postgres.TrackRepository
	catalog *TrackService
}

func newAppFixture(t *testing.T, objects ObjectStorage) *appFixture {
	t.Helper()
	ctx := context.Background()
	pool := testkit.Open(t, migrations.FS, resetTables)

	tracks := postgres.NewTrackRepository(pool)
	if objects == nil {
		objects = storedObject{size: integrationSize}
	}
	return &appFixture{
		ctx:     ctx,
		pool:    pool,
		tracks:  tracks,
		catalog: NewTrackService(tracks, objects, integrationMaxBytes),
	}
}

func (f *appFixture) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(f.ctx, query, args...).Scan(&n); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return n
}

func (f *appFixture) status(t *testing.T, trackID string) string {
	t.Helper()
	var status string
	if err := f.pool.QueryRow(f.ctx, `SELECT status FROM tracks WHERE id = $1::uuid`, trackID).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	return status
}

// mustUser returns a fresh owner id; users themselves live in Identity.
func (f *appFixture) mustUser(t *testing.T, _ string) seededUser {
	t.Helper()
	var user seededUser
	if err := f.pool.QueryRow(f.ctx, `SELECT gen_random_uuid()::text`).Scan(&user.ID); err != nil {
		t.Fatalf("seed user id: %v", err)
	}
	return user
}

func (f *appFixture) insertTrack(t *testing.T, userID, title, artist string, status domain.TrackStatus, day int) domain.Track {
	t.Helper()
	created, err := f.tracks.Create(f.ctx, domain.Track{
		UserID: userID, Title: title, Artist: artist, SizeBytes: integrationSize, Status: status,
	})
	if err != nil {
		t.Fatalf("insert track: %v", err)
	}
	at := time.Date(2026, 1, day, 0, 0, 0, 0, time.UTC)
	if _, err := f.pool.Exec(f.ctx, `UPDATE tracks SET created_at = $2 WHERE id = $1::uuid`, created.ID, at); err != nil {
		t.Fatalf("set created_at: %v", err)
	}
	return created
}

func runService(t *testing.T, component string, s testkit.Spec, body func(*testing.T, testkit.Report)) {
	t.Helper()
	s.Layer = testkit.LayerUsecase
	s.Component = component
	s.Kind = testkit.KindIntegration
	testkit.RunSpec(t, s, body)
}

func assertCode(t *testing.T, err error, code domain.Code, message string) {
	t.Helper()
	app, ok := domain.As(err)
	if !ok {
		t.Fatalf("got error %v, want domain error", err)
	}
	if app.Code != code || app.Message != message {
		t.Fatalf("got domain error (%s, %q), want (%s, %q)", app.Code, app.Message, code, message)
	}
}

func requireEqual(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestTrackServiceIntegration(t *testing.T) {
	const component = "TrackService"

	runService(t, component, testkit.Spec{
		ID: "IT-UC-TRK-LIST-01", Method: "List",
		Title:     "feed returns only ready tracks for the artist",
		Given:     "a ready and a pending track by Fixture Artist",
		When:      "List is called with artist `  fixture artist  `",
		Then:      "only the ready track is returned; matching ignores case and surrounding spaces",
		Technique: testkit.TechniqueDecisionTable,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"artist": "  fixture artist  "},
	}, func(t *testing.T, r testkit.Report) {
		var f *appFixture
		var ready domain.Track
		var page domain.TrackPage
		var err error
		r.Arrange(func(t *testing.T) {
			f = newAppFixture(t, nil)
			user := f.mustUser(t, "alice@example.com")
			ready = f.insertTrack(t, user.ID, "Live", "Fixture Artist", domain.TrackReady, 1)
			f.insertTrack(t, user.ID, "Draft", "Fixture Artist", domain.TrackPending, 2)
		})
		r.Act(func(t *testing.T) {
			page, err = f.catalog.List(f.ctx, domain.TrackFeedQuery{Artist: "  fixture artist  "})
		})
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			requireEqual(t, page.Items, []domain.Track{ready})
			requireEqual(t, page.Limit, 20)
		})
	})

	runService(t, component, testkit.Spec{
		ID: "IT-UC-TRK-LIST-02", Method: "List",
		Title:     "cursor walks the feed one page at a time",
		Given:     "two ready tracks created on 2026-01-01 and 2026-01-02",
		When:      "List is called with limit 1, then with the returned cursor",
		Then:      "page 1 has the older track and cursor `1`; page 2 has the newer track and no cursor",
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"limit": "1"},
	}, func(t *testing.T, r testkit.Report) {
		var f *appFixture
		var first, second domain.Track
		var page, next domain.TrackPage
		var err, nextErr error
		r.Arrange(func(t *testing.T) {
			f = newAppFixture(t, nil)
			user := f.mustUser(t, "alice@example.com")
			first = f.insertTrack(t, user.ID, "First", "Artist", domain.TrackReady, 1)
			second = f.insertTrack(t, user.ID, "Second", "Artist", domain.TrackReady, 2)
		})
		r.Act(func(t *testing.T) {
			page, err = f.catalog.List(f.ctx, domain.TrackFeedQuery{PageQuery: domain.PageQuery{Limit: 1}})
			next, nextErr = f.catalog.List(f.ctx, domain.TrackFeedQuery{PageQuery: domain.PageQuery{Limit: 1, Cursor: page.NextCursor}})
		})
		r.Assert(func(t *testing.T) {
			if err != nil || nextErr != nil {
				t.Fatalf("List() errors = %v, %v", err, nextErr)
			}
			requireEqual(t, page.Items, []domain.Track{first})
			requireEqual(t, page.NextCursor, "1")
			requireEqual(t, next.Items, []domain.Track{second})
			requireEqual(t, next.NextCursor, "")
		})
	})

	runService(t, component, testkit.Spec{
		ID: "IT-UC-TRK-LIST-03", Method: "List",
		Title:     "limit above 100 is rejected",
		Given:     "one ready track",
		When:      "List is called with limit 101",
		Then:      "invalid `limit exceeded`",
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"limit": "101"},
	}, func(t *testing.T, r testkit.Report) {
		var f *appFixture
		var err error
		r.Arrange(func(t *testing.T) {
			f = newAppFixture(t, nil)
			f.insertTrack(t, f.mustUser(t, "alice@example.com").ID, "Live", "Artist", domain.TrackReady, 1)
		})
		r.Act(func(t *testing.T) {
			_, err = f.catalog.List(f.ctx, domain.TrackFeedQuery{PageQuery: domain.PageQuery{Limit: 101}})
		})
		r.Assert(func(t *testing.T) { assertCode(t, err, domain.CodeInvalid, "limit exceeded") })
	})

	runService(t, component, testkit.Spec{
		ID: "IT-UC-TRK-LISTUP-01", Method: "ListByUploader",
		Title:     "uploader list includes a pending track",
		Given:     "a user with one pending upload",
		When:      "ListByUploader is called for that user without a status",
		Then:      "the pending track is returned",
		Technique: testkit.TechniqueState,
	}, func(t *testing.T, r testkit.Report) {
		var f *appFixture
		var pending domain.Track
		var page domain.TrackPage
		var err error
		r.Arrange(func(t *testing.T) {
			f = newAppFixture(t, nil)
			pending = f.insertTrack(t, f.mustUser(t, "alice@example.com").ID, "Draft", "Artist", domain.TrackPending, 1)
		})
		r.Act(func(t *testing.T) {
			page, err = f.catalog.ListByUploader(f.ctx, domain.TrackOwnerQuery{UserID: pending.UserID})
		})
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("ListByUploader() error = %v", err)
			}
			requireEqual(t, page.Items, []domain.Track{pending})
		})
	})

	runService(t, component, testkit.Spec{
		ID: "IT-UC-TRK-GET-01", Method: "GetByID",
		Title:     "get of an unknown track returns not found",
		Given:     "no tracks",
		When:      "GetByID is called with the nil UUID",
		Then:      "not_found `track not found`",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var f *appFixture
		var err error
		r.Arrange(func(t *testing.T) { f = newAppFixture(t, nil) })
		r.Act(func(t *testing.T) { _, err = f.catalog.GetByID(f.ctx, nilUUID, "") })
		r.Assert(func(t *testing.T) { assertCode(t, err, domain.CodeNotFound, "track not found") })
	})

	runService(t, component, testkit.Spec{
		ID: "IT-UC-TRK-UPDATE-01", Method: "Update",
		Title:     "update persists the title and keeps the status",
		Given:     "a ready track titled Old",
		When:      "Update is called with title New",
		Then:      "the stored title is New and status stays ready",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var f *appFixture
		var track, updated domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			f = newAppFixture(t, nil)
			track = f.insertTrack(t, f.mustUser(t, "alice@example.com").ID, "Old", "Artist", domain.TrackReady, 1)
		})
		r.Act(func(t *testing.T) {
			updated, err = f.catalog.Update(f.ctx, track.UserID, track.ID, domain.TrackWrite{Title: "New", Artist: "Artist"})
		})
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("Update() error = %v", err)
			}
			requireEqual(t, updated.Title, "New")
			requireEqual(t, updated.Status, domain.TrackReady)
			stored, readErr := f.tracks.GetByID(f.ctx, track.ID)
			if readErr != nil {
				t.Fatalf("GetByID() error = %v", readErr)
			}
			requireEqual(t, stored, updated)
		})
	})

	runService(t, component, testkit.Spec{
		ID: "IT-UC-TRK-DELETE-01", Method: "Delete",
		Title:     "delete removes the track and queues TrackDeleted",
		Given:     "a ready track",
		When:      "Delete is called by its owner",
		Then:      "tracks is empty and one catalog.track.v1 outbox row keyed by the track id exists",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var f *appFixture
		var track domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			f = newAppFixture(t, nil)
			user := f.mustUser(t, "alice@example.com")
			track = f.insertTrack(t, user.ID, "Live", "Artist", domain.TrackReady, 1)
		})
		r.Act(func(t *testing.T) { err = f.catalog.Delete(f.ctx, track.UserID, track.ID) })
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("Delete() error = %v", err)
			}
			requireEqual(t, f.count(t, `SELECT count(*) FROM tracks`), 0)
			requireEqual(t, f.count(t, `SELECT count(*) FROM outbox WHERE topic = 'catalog.track.v1' AND record_key = $1`, track.ID), 1)
		})
	})

	for i, tc := range []struct {
		size int64
		ok   bool
	}{{1, true}, {integrationMaxBytes, true}, {integrationMaxBytes + 1, false}} {
		then := "a pending track row is created with object key `<id>.mp3` and an upload URL is returned"
		if !tc.ok {
			then = "invalid `file too large`; no tracks row is written"
		}
		runService(t, component, testkit.Spec{
			ID: []string{"IT-UC-TRK-INIT-01", "IT-UC-TRK-INIT-02", "IT-UC-TRK-INIT-03"}[i], Method: "InitUpload",
			Title:     "init upload with size " + itoa64(tc.size) + " of max " + itoa64(integrationMaxBytes),
			Given:     "an existing user; TRACK_MAX_BYTES is " + itoa64(integrationMaxBytes),
			When:      "InitUpload is called with size " + itoa64(tc.size),
			Then:      then,
			Technique: testkit.TechniqueBoundary,
			Severity:  testkit.SeverityCritical,
			Params:    map[string]string{"size": itoa64(tc.size), "max": itoa64(integrationMaxBytes)},
		}, func(t *testing.T, r testkit.Report) {
			var f *appFixture
			var user seededUser
			var session domain.TrackUploadSession
			var err error
			r.Arrange(func(t *testing.T) {
				f = newAppFixture(t, nil)
				user = f.mustUser(t, "alice@example.com")
			})
			r.Act(func(t *testing.T) {
				session, err = f.catalog.InitUpload(f.ctx, domain.TrackUploadInit{UserID: user.ID, Title: "Song", Artist: "Artist", SizeBytes: tc.size})
			})
			r.Assert(func(t *testing.T) {
				if !tc.ok {
					assertCode(t, err, domain.CodeInvalid, "file too large")
					requireEqual(t, f.count(t, `SELECT count(*) FROM tracks`), 0)
					return
				}
				if err != nil {
					t.Fatalf("InitUpload() error = %v", err)
				}
				requireEqual(t, session.UploadURL, "http://objects/upload")
				stored, readErr := f.tracks.GetByID(f.ctx, session.Track.ID)
				if readErr != nil {
					t.Fatalf("GetByID() error = %v", readErr)
				}
				requireEqual(t, stored.Status, domain.TrackPending)
				requireEqual(t, stored.SizeBytes, tc.size)
				requireEqual(t, stored.ObjectKey, stored.ID+".mp3")
			})
		})
	}

	runService(t, component, testkit.Spec{
		ID: "IT-UC-TRK-INIT-04", Method: "InitUpload",
		Title:     "presign failure rolls back the created track",
		Given:     "object storage that fails to presign uploads",
		When:      "InitUpload is called with a valid input",
		Then:      "the presign error is returned and the pending row created before it is deleted",
		Technique: testkit.TechniqueErrorGuessing,
		Severity:  testkit.SeverityCritical,
	}, func(t *testing.T, r testkit.Report) {
		var f *appFixture
		var user seededUser
		var err error
		r.Arrange(func(t *testing.T) {
			f = newAppFixture(t, storedObject{presignErr: errPresign})
			user = f.mustUser(t, "alice@example.com")
		})
		r.Act(func(t *testing.T) {
			_, err = f.catalog.InitUpload(f.ctx, domain.TrackUploadInit{UserID: user.ID, Title: "Song", SizeBytes: 10})
		})
		r.Assert(func(t *testing.T) {
			if !errors.Is(err, errPresign) {
				t.Fatalf("InitUpload() error = %v, want %v", err, errPresign)
			}
			requireEqual(t, f.count(t, `SELECT count(*) FROM tracks`), 0)
		})
	})

	runService(t, component, testkit.Spec{
		ID: "IT-UC-TRK-COMPLETE-01", Method: "CompleteUpload",
		Title:     "complete upload moves pending to ready",
		Given:     "a pending track of size 128 and a stored object of size 128",
		When:      "CompleteUpload is called by the owner",
		Then:      "the returned and stored status is ready",
		Technique: testkit.TechniqueState,
		Severity:  testkit.SeverityBlocker,
	}, func(t *testing.T, r testkit.Report) {
		var f *appFixture
		var user seededUser
		var track, got domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			f = newAppFixture(t, storedObject{size: integrationSize})
			user = f.mustUser(t, "alice@example.com")
			track = f.insertTrack(t, user.ID, "Song", "Artist", domain.TrackPending, 1)
		})
		r.Act(func(t *testing.T) {
			got, err = f.catalog.CompleteUpload(f.ctx, domain.TrackUploadComplete{TrackID: track.ID, UserID: user.ID})
		})
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("CompleteUpload() error = %v", err)
			}
			requireEqual(t, got.Status, domain.TrackReady)
			requireEqual(t, f.status(t, track.ID), "ready")
		})
	})

	runService(t, component, testkit.Spec{
		ID: "IT-UC-TRK-COMPLETE-02", Method: "CompleteUpload",
		Title:     "complete upload by another user returns forbidden",
		Given:     "a pending track owned by Alice",
		When:      "CompleteUpload is called by Bob",
		Then:      "forbidden `not allowed`; the row stays pending",
		Technique: testkit.TechniqueDecisionTable,
		Severity:  testkit.SeverityBlocker,
	}, func(t *testing.T, r testkit.Report) {
		var f *appFixture
		var other seededUser
		var track domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			f = newAppFixture(t, storedObject{size: integrationSize})
			owner := f.mustUser(t, "alice@example.com")
			other = f.mustUser(t, "bob@example.com")
			track = f.insertTrack(t, owner.ID, "Song", "Artist", domain.TrackPending, 1)
		})
		r.Act(func(t *testing.T) {
			_, err = f.catalog.CompleteUpload(f.ctx, domain.TrackUploadComplete{TrackID: track.ID, UserID: other.ID})
		})
		r.Assert(func(t *testing.T) {
			assertCode(t, err, domain.CodeForbidden, "not allowed")
			requireEqual(t, f.status(t, track.ID), "pending")
		})
	})

	runService(t, component, testkit.Spec{
		ID: "IT-UC-TRK-COMPLETE-03", Method: "CompleteUpload",
		Title:     "uploaded size mismatch keeps the track pending",
		Given:     "a pending track declared as 128 bytes and a stored object of 64 bytes",
		When:      "CompleteUpload is called by the owner",
		Then:      "invalid `upload size mismatch: expected 128, got 64`; the row stays pending",
		Technique: testkit.TechniqueBoundary,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"declared": "128", "stored": "64"},
	}, func(t *testing.T, r testkit.Report) {
		var f *appFixture
		var user seededUser
		var track domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			f = newAppFixture(t, storedObject{size: 64})
			user = f.mustUser(t, "alice@example.com")
			track = f.insertTrack(t, user.ID, "Song", "Artist", domain.TrackPending, 1)
		})
		r.Act(func(t *testing.T) {
			_, err = f.catalog.CompleteUpload(f.ctx, domain.TrackUploadComplete{TrackID: track.ID, UserID: user.ID})
		})
		r.Assert(func(t *testing.T) {
			assertCode(t, err, domain.CodeInvalid, "upload size mismatch: expected 128, got 64")
			requireEqual(t, f.status(t, track.ID), "pending")
		})
	})

	runService(t, component, testkit.Spec{
		ID: "IT-UC-TRK-COMPLETE-04", Method: "CompleteUpload",
		Title:     "complete upload of a ready track returns conflict",
		Given:     "a track that is already ready",
		When:      "CompleteUpload is called again by the owner",
		Then:      "MarkProcessing refuses: conflict `track is not awaiting upload`; status stays ready",
		Technique: testkit.TechniqueState,
	}, func(t *testing.T, r testkit.Report) {
		var f *appFixture
		var user seededUser
		var track domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			f = newAppFixture(t, storedObject{size: integrationSize})
			user = f.mustUser(t, "alice@example.com")
			track = f.insertTrack(t, user.ID, "Song", "Artist", domain.TrackReady, 1)
		})
		r.Act(func(t *testing.T) {
			_, err = f.catalog.CompleteUpload(f.ctx, domain.TrackUploadComplete{TrackID: track.ID, UserID: user.ID})
		})
		r.Assert(func(t *testing.T) {
			assertCode(t, err, domain.CodeConflict, "track is not awaiting upload")
			requireEqual(t, f.status(t, track.ID), "ready")
		})
	})

	runService(t, component, testkit.Spec{
		ID: "IT-UC-TRK-STREAM-01", Method: "StreamURL",
		Title:     "stream url is returned for a ready track",
		Given:     "a ready track",
		When:      "StreamURL is called",
		Then:      "the presigned read link from object storage is returned",
		Technique: testkit.TechniqueState,
	}, func(t *testing.T, r testkit.Report) {
		var f *appFixture
		var track domain.Track
		var url string
		var err error
		r.Arrange(func(t *testing.T) {
			f = newAppFixture(t, nil)
			track = f.insertTrack(t, f.mustUser(t, "alice@example.com").ID, "Live", "Artist", domain.TrackReady, 1)
		})
		r.Act(func(t *testing.T) { url, err = f.catalog.StreamURL(f.ctx, track.ID) })
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("StreamURL() error = %v", err)
			}
			requireEqual(t, url, "http://objects/stream")
		})
	})

	runService(t, component, testkit.Spec{
		ID: "IT-UC-TRK-STREAM-02", Method: "StreamURL",
		Title:     "stream url of a pending track returns conflict",
		Given:     "a pending track",
		When:      "StreamURL is called",
		Then:      "conflict `track is not ready`; no link is returned",
		Technique: testkit.TechniqueState,
		Severity:  testkit.SeverityCritical,
	}, func(t *testing.T, r testkit.Report) {
		var f *appFixture
		var track domain.Track
		var url string
		var err error
		r.Arrange(func(t *testing.T) {
			f = newAppFixture(t, nil)
			track = f.insertTrack(t, f.mustUser(t, "alice@example.com").ID, "Draft", "Artist", domain.TrackPending, 1)
		})
		r.Act(func(t *testing.T) { url, err = f.catalog.StreamURL(f.ctx, track.ID) })
		r.Assert(func(t *testing.T) {
			assertCode(t, err, domain.CodeConflict, "track is not ready")
			requireEqual(t, url, "")
		})
	})
}

func itoa64(n int64) string {
	return strconv.FormatInt(n, 10)
}
