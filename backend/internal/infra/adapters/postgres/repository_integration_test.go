//go:build integration

package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"chimera/internal/domain"
	"chimera/internal/testkit"
)

const missingUUID = "00000000-0000-0000-0000-000000000000"

type postgresFixture struct {
	ctx    context.Context
	pool   *pgxpool.Pool
	users  *UserRepository
	tracks *TrackRepository
	likes  *TrackLikeRepository
}

func newPostgresFixture(t *testing.T) *postgresFixture {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping test database: %v", err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE track_likes, tracks, users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("reset test database: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `TRUNCATE track_likes, tracks, users RESTART IDENTITY CASCADE`)
	})
	return &postgresFixture{
		ctx:    ctx,
		pool:   pool,
		users:  NewUserRepository(pool),
		tracks: NewTrackRepository(pool),
		likes:  NewTrackLikeRepository(pool),
	}
}

func (f *postgresFixture) seedUser(t *testing.T) domain.User {
	t.Helper()
	var user domain.User
	err := f.pool.QueryRow(f.ctx,
		`INSERT INTO users (email, name, password_hash)
		 VALUES ('fixture@example.com', 'Fixture', 'hash')
		 RETURNING id::text, email, name`,
	).Scan(&user.ID, &user.Email, &user.Name)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return user
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

func (f *postgresFixture) seedLike(t *testing.T, userID, trackID string) {
	t.Helper()
	if _, err := f.pool.Exec(f.ctx,
		`INSERT INTO track_likes (user_id, track_id) VALUES ($1::uuid, $2::uuid)`,
		userID, trackID,
	); err != nil {
		t.Fatalf("seed like: %v", err)
	}
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

func TestUserRepositoryIntegration(t *testing.T) {
	const component = "UserRepository"

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-CREATE-01", Method: "Create",
		Title:     "create persists a user row",
		Given:     "an empty users table",
		When:      "Create is called with alice@example.com, name Alice, hash `hash`",
		Then:      "one users row exists with the returned id; the id is a server-generated UUID",
		Technique: testkit.TechniqueEquivalence,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"email": "alice@example.com", "name": "Alice"},
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var created domain.User
		var err error
		r.Arrange(func(t *testing.T) { f = newPostgresFixture(t) })
		r.Act(func(t *testing.T) {
			created, err = f.users.Create(f.ctx, domain.User{Email: "alice@example.com", Name: "Alice"}, "hash")
		})
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			assertEqual(t, len(created.ID), 36)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM users WHERE id = $1::uuid`, created.ID), 1)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-GET-01", Method: "GetByID",
		Title:     "get by id reads a seeded user",
		Given:     "one user inserted with plain SQL",
		When:      "GetByID is called with that id",
		Then:      "id, email, and name equal the seeded row",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var want, got domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			want = f.seedUser(t)
		})
		r.Act(func(t *testing.T) { got, err = f.users.GetByID(f.ctx, want.ID) })
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("GetByID() error = %v", err)
			}
			assertEqual(t, got, want)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-GET-02", Method: "GetByID",
		Title:     "get by id of an unknown uuid returns not found",
		Given:     "an empty users table",
		When:      "GetByID is called with the nil UUID",
		Then:      "error not_found `user not found`",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"id": missingUUID},
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var err error
		r.Arrange(func(t *testing.T) { f = newPostgresFixture(t) })
		r.Act(func(t *testing.T) { _, err = f.users.GetByID(f.ctx, missingUUID) })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeNotFound, "user not found") })
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-LIST-01", Method: "List",
		Title:     "list returns every stored user",
		Given:     "one seeded user",
		When:      "List is called",
		Then:      "exactly that user is returned",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var want domain.User
		var got []domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			want = f.seedUser(t)
		})
		r.Act(func(t *testing.T) { got, err = f.users.List(f.ctx) })
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			assertEqual(t, got, []domain.User{want})
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-LIST-02", Method: "List",
		Title:     "list of an empty table returns an empty slice",
		Given:     "an empty users table",
		When:      "List is called",
		Then:      "an empty, non-nil slice and no error",
		Technique: testkit.TechniqueBoundary,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var got []domain.User
		var err error
		r.Arrange(func(t *testing.T) { f = newPostgresFixture(t) })
		r.Act(func(t *testing.T) { got, err = f.users.List(f.ctx) })
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			assertEqual(t, got, []domain.User{})
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-DELETE-01", Method: "Delete",
		Title:     "delete removes the user row",
		Given:     "one seeded user without tracks",
		When:      "Delete is called with that id",
		Then:      "no error and the users row is gone",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var user domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			user = f.seedUser(t)
		})
		r.Act(func(t *testing.T) { err = f.users.Delete(f.ctx, user.ID) })
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("Delete() error = %v", err)
			}
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM users WHERE id = $1::uuid`, user.ID), 0)
		})
	})
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
		var user domain.User
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
		Title:     "create for an unknown user returns not found",
		Given:     "an empty users table",
		When:      "Create is called with user id equal to the nil UUID",
		Then:      "FK tracks.user_id fails with SQLSTATE 23503, mapped to not_found `not found`; no tracks row is written",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"user_id": missingUUID},
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var err error
		r.Arrange(func(t *testing.T) { f = newPostgresFixture(t) })
		r.Act(func(t *testing.T) {
			_, err = f.tracks.Create(f.ctx, domain.Track{UserID: missingUUID, Title: "Orphan", SizeBytes: 1, Status: domain.TrackPending})
		})
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeNotFound, "not found")
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

func TestTrackLikeRepositoryIntegration(t *testing.T) {
	const component = "TrackLikeRepository"

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-LIKE-ADD-01", Method: "Add",
		Title:     "add persists a like row",
		Given:     "an existing user and ready track",
		When:      "Add is called for the pair",
		Then:      "one track_likes row exists for (user_id, track_id)",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var user domain.User
		var track domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			user = f.seedUser(t)
			track = f.seedTrack(t, user.ID)
		})
		r.Act(func(t *testing.T) { err = f.likes.Add(f.ctx, user.ID, track.ID) })
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("Add() error = %v", err)
			}
			assertEqual(t, countRows(t, f,
				`SELECT count(*) FROM track_likes WHERE user_id = $1::uuid AND track_id = $2::uuid`,
				user.ID, track.ID), 1)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-LIKE-LISTREADY-01", Method: "ListReadyByUser",
		Title:     "list ready likes returns the liked ready track",
		Given:     "a user who liked one ready track",
		When:      "ListReadyByUser is called for that user",
		Then:      "exactly that track is returned",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var user domain.User
		var want domain.Track
		var got []domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			user = f.seedUser(t)
			want = f.seedTrack(t, user.ID)
			f.seedLike(t, user.ID, want.ID)
		})
		r.Act(func(t *testing.T) { got, err = f.likes.ListReadyByUser(f.ctx, user.ID) })
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("ListReadyByUser() error = %v", err)
			}
			assertEqual(t, got, []domain.Track{want})
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-LIKE-REMOVE-01", Method: "Remove",
		Title:     "remove deletes the like and keeps user and track",
		Given:     "a user who liked one track",
		When:      "Remove is called for the pair",
		Then:      "the track_likes row is gone; the users and tracks rows remain",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var user domain.User
		var track domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			user = f.seedUser(t)
			track = f.seedTrack(t, user.ID)
			f.seedLike(t, user.ID, track.ID)
		})
		r.Act(func(t *testing.T) { err = f.likes.Remove(f.ctx, user.ID, track.ID) })
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("Remove() error = %v", err)
			}
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM track_likes`), 0)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM users`), 1)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM tracks`), 1)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-LIKE-REMOVE-02", Method: "Remove",
		Title:     "remove of a missing like succeeds without changes",
		Given:     "a user and a track without a like",
		When:      "Remove is called for the pair",
		Then:      "no error; track_likes stays empty (remove is idempotent)",
		Technique: testkit.TechniqueBoundary,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var user domain.User
		var track domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			user = f.seedUser(t)
			track = f.seedTrack(t, user.ID)
		})
		r.Act(func(t *testing.T) { err = f.likes.Remove(f.ctx, user.ID, track.ID) })
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("Remove() error = %v", err)
			}
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM track_likes`), 0)
		})
	})
}
