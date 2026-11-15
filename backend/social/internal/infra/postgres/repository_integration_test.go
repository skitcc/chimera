//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"chimera/social/internal/domain"
	"chimera/social/internal/testkit"
	"chimera/social/migrations"
)

const resetSQL = `TRUNCATE processed_events, deleted_users, deleted_tracks, playlist_tracks, playlists, user_follows, track_likes`

type postgresFixture struct {
	ctx     context.Context
	pool    *pgxpool.Pool
	likes   *TrackLikeRepository
	cleanup *CleanupRepository
}

func newPostgresFixture(t *testing.T) *postgresFixture {
	t.Helper()
	pool := testkit.Open(t, migrations.FS, resetSQL)
	return &postgresFixture{
		ctx:     context.Background(),
		pool:    pool,
		likes:   NewTrackLikeRepository(pool),
		cleanup: NewCleanupRepository(pool),
	}
}

// newID returns a fresh uuid. Users and tracks live in other services, so
// Social only needs their ids.
func (f *postgresFixture) newID(t *testing.T) string {
	t.Helper()
	var id string
	if err := f.pool.QueryRow(f.ctx, `SELECT gen_random_uuid()::text`).Scan(&id); err != nil {
		t.Fatalf("generate uuid: %v", err)
	}
	return id
}

func (f *postgresFixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.pool.Exec(f.ctx, sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

func (f *postgresFixture) seedLike(t *testing.T, userID, trackID string) {
	t.Helper()
	f.exec(t, `INSERT INTO track_likes (user_id, track_id) VALUES ($1::uuid, $2::uuid)`, userID, trackID)
}

func (f *postgresFixture) seedPlaylist(t *testing.T, ownerID string, trackIDs ...string) string {
	t.Helper()
	var id string
	if err := f.pool.QueryRow(f.ctx,
		`INSERT INTO playlists (owner_id, title) VALUES ($1::uuid, 'Fixture') RETURNING id::text`, ownerID,
	).Scan(&id); err != nil {
		t.Fatalf("seed playlist: %v", err)
	}
	for i, trackID := range trackIDs {
		f.exec(t, `INSERT INTO playlist_tracks (playlist_id, track_id, position) VALUES ($1::uuid, $2::uuid, $3)`, id, trackID, i)
	}
	return id
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

func TestTrackLikeRepositoryIntegration(t *testing.T) {
	const component = "TrackLikeRepository"

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-LIKE-ADD-01", Method: "Add",
		Title:     "add persists a like row",
		Given:     "a user id and a track id without tombstones",
		When:      "Add is called for the pair",
		Then:      "one track_likes row exists for (user_id, track_id)",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var userID, trackID string
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			userID, trackID = f.newID(t), f.newID(t)
		})
		r.Act(func(t *testing.T) { err = f.likes.Add(f.ctx, userID, trackID) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, countRows(t, f,
				`SELECT count(*) FROM track_likes WHERE user_id = $1::uuid AND track_id = $2::uuid`,
				userID, trackID), 1)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-LIKE-ADD-02", Method: "Add",
		Title:     "duplicate like returns conflict",
		Given:     "a user who already liked the track",
		When:      "Add is called again for the same pair",
		Then:      "PRIMARY KEY (user_id, track_id) fails with SQLSTATE 23505, mapped to conflict `track already liked`; one row remains",
		Technique: testkit.TechniqueErrorGuessing,
		Severity:  testkit.SeverityCritical,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var userID, trackID string
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			userID, trackID = f.newID(t), f.newID(t)
			f.seedLike(t, userID, trackID)
		})
		r.Act(func(t *testing.T) { err = f.likes.Add(f.ctx, userID, trackID) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeConflict, "track already liked")
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM track_likes`), 1)
		})
	})

	tombstones := []struct {
		id, title, given, table string
	}{
		{"IT-DA-LIKE-ADD-03", "like of a deleted track returns not found", "a track id recorded in deleted_tracks", "deleted_tracks"},
		{"IT-DA-LIKE-ADD-05", "like by a deleted user returns not found", "a user id recorded in deleted_users", "deleted_users"},
	}
	for _, tc := range tombstones {
		runIntegration(t, component, testkit.Spec{
			ID: tc.id, Method: "Add", Title: tc.title,
			Given:     tc.given,
			When:      "Add is called for the pair",
			Then:      "the guarded insert writes nothing and not_found `not found` is returned",
			Technique: testkit.TechniqueState,
			Severity:  testkit.SeverityCritical,
			Params:    map[string]string{"tombstone": tc.table},
		}, func(t *testing.T, r testkit.Report) {
			var f *postgresFixture
			var userID, trackID string
			var err error
			r.Arrange(func(t *testing.T) {
				f = newPostgresFixture(t)
				userID, trackID = f.newID(t), f.newID(t)
				if tc.table == "deleted_tracks" {
					f.exec(t, `INSERT INTO deleted_tracks (track_id) VALUES ($1::uuid)`, trackID)
					return
				}
				f.exec(t, `INSERT INTO deleted_users (user_id) VALUES ($1::uuid)`, userID)
			})
			r.Act(func(t *testing.T) { err = f.likes.Add(f.ctx, userID, trackID) })
			r.Assert(func(t *testing.T) {
				assertDomainError(t, err, domain.CodeNotFound, "not found")
				assertEqual(t, countRows(t, f, `SELECT count(*) FROM track_likes`), 0)
			})
		})
	}

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-LIKE-ADD-04", Method: "Add",
		Title:     "malformed track id returns invalid id",
		Given:     "a valid user id",
		When:      "Add is called with track id not-a-uuid",
		Then:      "SQLSTATE 22P02, mapped to invalid `invalid id`",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"track_id": "not-a-uuid"},
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var userID string
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			userID = f.newID(t)
		})
		r.Act(func(t *testing.T) { err = f.likes.Add(f.ctx, userID, "not-a-uuid") })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeInvalid, "invalid id") })
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-LIKE-IDS-01", Method: "ListTrackIDs",
		Title:     "list returns the user's liked track ids newest first",
		Given:     "alice liked track A then track B, bob liked track C",
		When:      "ListTrackIDs is called for alice",
		Then:      "[B, A] is returned and bob's like is not included",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var alice, a, b string
		var got []string
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			alice, a, b = f.newID(t), f.newID(t), f.newID(t)
			f.exec(t, `INSERT INTO track_likes (user_id, track_id, created_at) VALUES ($1::uuid, $2::uuid, now() - interval '1 minute')`, alice, a)
			f.seedLike(t, alice, b)
			f.seedLike(t, f.newID(t), f.newID(t))
		})
		r.Act(func(t *testing.T) { got, err = f.likes.ListTrackIDs(f.ctx, alice) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, got, []string{b, a})
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-LIKE-REMOVE-01", Method: "Remove",
		Title:     "remove deletes only the requested like",
		Given:     "a user who liked two tracks",
		When:      "Remove is called for the first pair",
		Then:      "the first like is gone and the second remains",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var userID, trackID, other string
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			userID, trackID, other = f.newID(t), f.newID(t), f.newID(t)
			f.seedLike(t, userID, trackID)
			f.seedLike(t, userID, other)
		})
		r.Act(func(t *testing.T) { err = f.likes.Remove(f.ctx, userID, trackID) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM track_likes WHERE track_id = $1::uuid`, other), 1)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM track_likes`), 1)
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
		var err error
		r.Arrange(func(t *testing.T) { f = newPostgresFixture(t) })
		r.Act(func(t *testing.T) { err = f.likes.Remove(f.ctx, f.newID(t), f.newID(t)) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM track_likes`), 0)
		})
	})
}

func TestCleanupRepositoryIntegration(t *testing.T) {
	const component = "CleanupRepository"

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-CLN-USER-01", Method: "DeleteUser",
		Title:     "user deletion removes all social data of the user",
		Given:     "alice has a like, follows bob, is followed by carol and owns a playlist with a track; bob has his own like",
		When:      "DeleteUser is applied for alice",
		Then:      "alice's like, both follows, her playlist and its entries are gone, bob's like remains and alice is tombstoned",
		Technique: testkit.TechniqueState,
		Severity:  testkit.SeverityCritical,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var alice, bob string
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			alice, bob = f.newID(t), f.newID(t)
			carol, track := f.newID(t), f.newID(t)
			f.seedLike(t, alice, track)
			f.seedLike(t, bob, track)
			f.exec(t, `INSERT INTO user_follows (follower_id, following_id) VALUES ($1::uuid, $2::uuid), ($3::uuid, $1::uuid)`, alice, bob, carol)
			f.seedPlaylist(t, alice, track)
		})
		r.Act(func(t *testing.T) {
			err = f.cleanup.DeleteUser(f.ctx, domain.UserDeletion{EventID: f.newID(t), UserID: alice})
		})
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM track_likes WHERE user_id = $1::uuid`, alice), 0)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM track_likes WHERE user_id = $1::uuid`, bob), 1)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM user_follows`), 0)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM playlists`), 0)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM playlist_tracks`), 0)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM deleted_users WHERE user_id = $1::uuid`, alice), 1)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-CLN-USER-02", Method: "DeleteUser",
		Title:     "redelivered user deletion changes nothing",
		Given:     "a UserDeleted event that was already applied, and a like written for the user afterwards",
		When:      "DeleteUser is applied again with the same event id",
		Then:      "no error and the later like is kept because the event is recorded in processed_events",
		Technique: testkit.TechniqueState,
		Severity:  testkit.SeverityCritical,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var event domain.UserDeletion
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			event = domain.UserDeletion{EventID: f.newID(t), UserID: f.newID(t)}
			assertNoError(t, f.cleanup.DeleteUser(f.ctx, event))
			f.seedLike(t, event.UserID, f.newID(t))
		})
		r.Act(func(t *testing.T) { err = f.cleanup.DeleteUser(f.ctx, event) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM track_likes`), 1)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM processed_events`), 1)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-CLN-TRACK-01", Method: "DeleteTrack",
		Title:     "track deletion removes likes and playlist entries and closes the position gap",
		Given:     "a playlist [A, B, C] and two likes of B",
		When:      "DeleteTrack is applied for B",
		Then:      "both likes and the B entry are gone, C moves to position 1 and B is tombstoned",
		Technique: testkit.TechniqueState,
		Severity:  testkit.SeverityCritical,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var playlistID, b, c string
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			a := f.newID(t)
			b, c = f.newID(t), f.newID(t)
			playlistID = f.seedPlaylist(t, f.newID(t), a, b, c)
			f.seedLike(t, f.newID(t), b)
			f.seedLike(t, f.newID(t), b)
		})
		r.Act(func(t *testing.T) {
			err = f.cleanup.DeleteTrack(f.ctx, domain.TrackDeletion{EventID: f.newID(t), TrackID: b})
		})
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM track_likes`), 0)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM playlist_tracks WHERE playlist_id = $1::uuid`, playlistID), 2)
			assertEqual(t, countRows(t, f,
				`SELECT position FROM playlist_tracks WHERE playlist_id = $1::uuid AND track_id = $2::uuid`, playlistID, c), 1)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM deleted_tracks WHERE track_id = $1::uuid`, b), 1)
		})
	})
}
