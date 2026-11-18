//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"chimera/catalog/internal/domain"
	"chimera/catalog/internal/testkit"
	"chimera/catalog/migrations"
)

const (
	missingUUID = "00000000-0000-0000-0000-000000000000"
	resetTables = `TRUNCATE idempotency_keys, outbox, processed_events, deleted_users, tracks RESTART IDENTITY CASCADE`
)

type postgresFixture struct {
	ctx    context.Context
	pool   *pgxpool.Pool
	tracks *TrackRepository
}

// seededUser stands in for an Identity user: Catalog stores only the owner id.
type seededUser struct {
	ID string
}

func newPostgresFixture(t *testing.T) *postgresFixture {
	t.Helper()
	pool := testkit.Open(t, migrations.FS, resetTables)
	return &postgresFixture{
		ctx:    context.Background(),
		pool:   pool,
		tracks: newTrackRepository(pool),
	}
}

func (f *postgresFixture) seedUser(t *testing.T) seededUser {
	t.Helper()
	var user seededUser
	if err := f.pool.QueryRow(f.ctx, `SELECT gen_random_uuid()::text`).Scan(&user.ID); err != nil {
		t.Fatalf("seed user id: %v", err)
	}
	return user
}

func (f *postgresFixture) tombstone(t *testing.T, userID string) {
	t.Helper()
	if _, err := f.pool.Exec(f.ctx, `INSERT INTO deleted_users (user_id) VALUES ($1::uuid)`, userID); err != nil {
		t.Fatalf("seed deleted user: %v", err)
	}
}

func (f *postgresFixture) seedTrack(t *testing.T, userID string) domain.Track {
	t.Helper()
	return f.insertTrack(t, userID, "Fixture Track", "Fixture Artist", domain.TrackReady)
}

func (f *postgresFixture) insertTrack(t *testing.T, userID, title, artist string, status domain.TrackStatus) domain.Track {
	t.Helper()
	var track domain.Track
	var storedStatus string
	err := f.pool.QueryRow(f.ctx,
		`INSERT INTO tracks (user_id, title, artist, object_key, size_bytes, status)
		 VALUES ($1::uuid, $2, $3, 'fixture.mp3', 128, $4)
		 RETURNING id::text, user_id::text, title, artist, object_key, size_bytes, status`,
		userID, title, artist, string(status),
	).Scan(&track.ID, &track.UserID, &track.Title, &track.Artist, &track.ObjectKey, &track.SizeBytes, &storedStatus)
	if err != nil {
		t.Fatalf("insert track: %v", err)
	}
	track.Status = domain.TrackStatus(storedStatus)
	return track
}

func runIntegration(t *testing.T, component string, s testkit.Spec, body func(*testing.T, testkit.Report)) {
	t.Helper()
	s.Layer = testkit.LayerData
	s.Component = component
	s.Kind = testkit.KindIntegration
	testkit.RunSpec(t, s, body)
}

func countRows(t *testing.T, f *postgresFixture, query string, args ...any) int {
	t.Helper()
	var count int
	if err := f.pool.QueryRow(f.ctx, query, args...).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return count
}

func TestTrackRepositoryIntegration(t *testing.T) {
	const component = "TrackRepository"

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-TRK-CREATE-01", Method: "Create",
		Title:     "create persists a pending track and derives the object key",
		Given:     "an existing user",
		When:      "Create is called with title Created, size 256, status pending",
		Then:      "one tracks row exists; status is pending and object_key is `<id>.mp3`",
		Technique: testkit.TechniqueEquivalence,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"size_bytes": "256", "status": "pending"},
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var user seededUser
		var created domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			user = f.seedUser(t)
		})
		r.Act(func(t *testing.T) {
			created, err = f.tracks.Create(f.ctx, domain.Track{
				UserID: user.ID, Title: "Created", Artist: "Artist",
				SizeBytes: 256, Status: domain.TrackPending,
			})
		})
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			assertEqual(t, created.Status, domain.TrackPending)
			assertEqual(t, created.ObjectKey, created.ID+".mp3")
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM tracks WHERE id = $1::uuid`, created.ID), 1)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-TRK-CREATE-02", Method: "Create",
		Title:     "create for a deleted user returns not found",
		Given:     "the owner id is recorded in deleted_users",
		When:      "Create is called for that owner",
		Then:      "the guarded insert writes nothing and returns not_found `user not found`",
		Technique: testkit.TechniqueErrorGuessing,
		Severity:  testkit.SeverityCritical,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var user seededUser
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			user = f.seedUser(t)
			f.tombstone(t, user.ID)
		})
		r.Act(func(t *testing.T) {
			_, err = f.tracks.Create(f.ctx, domain.Track{UserID: user.ID, Title: "Orphan", SizeBytes: 1, Status: domain.TrackPending})
		})
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeNotFound, "user not found")
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM tracks`), 0)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-TRK-GET-01", Method: "GetByID",
		Title:     "get by id reads a seeded track",
		Given:     "one ready track inserted with plain SQL",
		When:      "GetByID is called with that id",
		Then:      "every column, including status and object key, equals the seeded row",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var want, got domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			want = f.seedTrack(t, f.seedUser(t).ID)
		})
		r.Act(func(t *testing.T) { got, err = f.tracks.GetByID(f.ctx, want.ID) })
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("GetByID() error = %v", err)
			}
			assertEqual(t, got, want)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-TRK-LIST-01", Method: "List",
		Title:     "list by artist and ready status returns the matching track",
		Given:     "one ready track by Fixture Artist",
		When:      "List is called with artist Fixture Artist and status ready",
		Then:      "exactly that track is returned",
		Technique: testkit.TechniqueDecisionTable,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var want domain.Track
		var got []domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			want = f.seedTrack(t, f.seedUser(t).ID)
		})
		r.Act(func(t *testing.T) {
			got, err = f.tracks.List(f.ctx, domain.TrackFilter{Artist: want.Artist, Status: domain.TrackReady})
		})
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			assertEqual(t, got, []domain.Track{want})
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-TRK-DELETE-01", Method: "Delete",
		Title:     "delete removes the track row",
		Given:     "one seeded track",
		When:      "Delete is called with that id",
		Then:      "no error and the tracks row is gone",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var track domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			track = f.seedTrack(t, f.seedUser(t).ID)
		})
		r.Act(func(t *testing.T) { err = f.tracks.Delete(f.ctx, track.ID) })
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("Delete() error = %v", err)
			}
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM tracks WHERE id = $1::uuid`, track.ID), 0)
		})
	})
}
