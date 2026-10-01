//go:build integration

package postgres

import (
	"fmt"
	"testing"

	"chimera/internal/domain"
	"chimera/internal/testkit"
)

func TestUserRepositoryRulesIntegration(t *testing.T) {
	const component = "UserRepository"

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-UPDATE-01", Method: "Update",
		Title:     "update persists a changed email and name",
		Given:     "one seeded user",
		When:      "Update is called with email updated@example.com and name Updated",
		Then:      "the returned user and a fresh GetByID both carry the new values",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"email": "updated@example.com", "name": "Updated"},
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var user, got domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			user = f.seedUser(t)
		})
		r.Act(func(t *testing.T) {
			got, err = f.users.Update(f.ctx, domain.User{ID: user.ID, Email: "updated@example.com", Name: "Updated"})
		})
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("Update() error = %v", err)
			}
			assertEqual(t, got, domain.User{ID: user.ID, Email: "updated@example.com", Name: "Updated"})
			stored, readErr := f.users.GetByID(f.ctx, user.ID)
			if readErr != nil {
				t.Fatalf("GetByID() error = %v", readErr)
			}
			assertEqual(t, stored, got)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-UPDATE-02", Method: "Update",
		Title:     "update of an unknown uuid returns not found",
		Given:     "an empty users table",
		When:      "Update is called with the nil UUID",
		Then:      "error not_found `user not found`; the table stays empty",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"id": missingUUID},
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var err error
		r.Arrange(func(t *testing.T) { f = newPostgresFixture(t) })
		r.Act(func(t *testing.T) {
			_, err = f.users.Update(f.ctx, domain.User{ID: missingUUID, Email: "missing@example.com", Name: "Missing"})
		})
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeNotFound, "user not found")
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM users`), 0)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-UPDATE-03", Method: "Update",
		Title:     "update to another user's email returns conflict",
		Given:     "two users, alice@example.com and bob@example.com",
		When:      "Bob is updated to alice@example.com",
		Then:      "error conflict `email already exists`; Bob keeps his email",
		Technique: testkit.TechniqueErrorGuessing,
		Severity:  testkit.SeverityCritical,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var bob domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			mustCreateUser(t, f, "alice@example.com", "hash")
			bob = mustCreateUser(t, f, "bob@example.com", "hash")
		})
		r.Act(func(t *testing.T) {
			_, err = f.users.Update(f.ctx, domain.User{ID: bob.ID, Email: "alice@example.com", Name: "Bob"})
		})
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeConflict, "email already exists")
			stored, readErr := f.users.GetByID(f.ctx, bob.ID)
			if readErr != nil {
				t.Fatalf("GetByID() error = %v", readErr)
			}
			assertEqual(t, stored.Email, "bob@example.com")
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-CREATE-02", Method: "Create",
		Title:     "duplicate email returns conflict",
		Given:     "a stored user alice@example.com",
		When:      "Create is called again with alice@example.com",
		Then:      "UNIQUE(users.email) fails with SQLSTATE 23505, mapped to conflict `email already exists`; one row remains",
		Technique: testkit.TechniqueErrorGuessing,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"email": "alice@example.com"},
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			mustCreateUser(t, f, "alice@example.com", "hash")
		})
		r.Act(func(t *testing.T) {
			_, err = f.users.Create(f.ctx, domain.User{Email: "alice@example.com", Name: "Other"}, "other-hash")
		})
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeConflict, "email already exists")
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM users`), 1)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-CREATE-03", Method: "Create",
		Title:     "email uniqueness in the database is case-sensitive",
		Given:     "a stored user alice@example.com",
		When:      "Create is called with Alice@Example.com, bypassing domain normalization",
		Then:      "the insert succeeds and two rows exist; lowercasing is enforced only by the domain layer",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"first": "alice@example.com", "second": "Alice@Example.com"},
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			mustCreateUser(t, f, "alice@example.com", "hash")
		})
		r.Act(func(t *testing.T) {
			_, err = f.users.Create(f.ctx, domain.User{Email: "Alice@Example.com", Name: "Alice"}, "hash")
		})
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM users`), 2)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-GETEMAIL-01", Method: "GetByEmail",
		Title:     "get by email returns the stored password hash",
		Given:     "a user created with hash stored-hash",
		When:      "GetByEmail is called with alice@example.com",
		Then:      "the auth user carries the email and the exact stored hash",
		Technique: testkit.TechniqueEquivalence,
		Severity:  testkit.SeverityCritical,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var got domain.AuthUser
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			mustCreateUser(t, f, "alice@example.com", "stored-hash")
		})
		r.Act(func(t *testing.T) { got, err = f.users.GetByEmail(f.ctx, "alice@example.com") })
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("GetByEmail() error = %v", err)
			}
			assertEqual(t, got.User.Email, "alice@example.com")
			assertEqual(t, got.PasswordHash, "stored-hash")
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-GETEMAIL-02", Method: "GetByEmail",
		Title:     "get by unknown email returns not found",
		Given:     "an empty users table",
		When:      "GetByEmail is called with missing@example.com",
		Then:      "error not_found `user not found`",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var err error
		r.Arrange(func(t *testing.T) { f = newPostgresFixture(t) })
		r.Act(func(t *testing.T) { _, err = f.users.GetByEmail(f.ctx, "missing@example.com") })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeNotFound, "user not found") })
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-GET-03", Method: "GetByID",
		Title:     "malformed uuid returns invalid id",
		Given:     "an empty users table",
		When:      "GetByID is called with not-a-uuid",
		Then:      "PostgreSQL rejects the cast with SQLSTATE 22P02, mapped to invalid `invalid id`",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"id": "not-a-uuid"},
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var err error
		r.Arrange(func(t *testing.T) { f = newPostgresFixture(t) })
		r.Act(func(t *testing.T) { _, err = f.users.GetByID(f.ctx, "not-a-uuid") })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeInvalid, "invalid id") })
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-DELETE-02", Method: "Delete",
		Title:     "delete of a user who owns a track is refused",
		Given:     "a user who owns one track",
		When:      "Delete is called for that user",
		Then:      "FK tracks.user_id (no cascade) fails with SQLSTATE 23503, mapped to not_found `not found` (known issue KI-1); the user row stays",
		Technique: testkit.TechniqueErrorGuessing,
		Severity:  testkit.SeverityCritical,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var user domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			user = f.seedUser(t)
			f.seedTrack(t, user.ID)
		})
		r.Act(func(t *testing.T) { err = f.users.Delete(f.ctx, user.ID) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeNotFound, "not found")
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM users WHERE id = $1::uuid`, user.ID), 1)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-USR-DELETE-03", Method: "Delete",
		Title:     "delete of an unknown uuid returns not found",
		Given:     "an empty users table",
		When:      "Delete is called with the nil UUID",
		Then:      "zero rows affected, mapped to not_found `user not found`",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var err error
		r.Arrange(func(t *testing.T) { f = newPostgresFixture(t) })
		r.Act(func(t *testing.T) { err = f.users.Delete(f.ctx, missingUUID) })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeNotFound, "user not found") })
	})
}

func TestTrackRepositoryRulesIntegration(t *testing.T) {
	const component = "TrackRepository"

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-TRK-UPDATE-01", Method: "Update",
		Title:     "update persists title and status",
		Given:     "one ready-seeded track changed in memory to title Renamed and status processing",
		When:      "Update is called",
		Then:      "the returned and the re-read track both have title Renamed and status processing",
		Technique: testkit.TechniqueState,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var track, got domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			track = f.seedTrack(t, f.seedUser(t).ID)
			track.Title = "Renamed"
			track.Status = domain.TrackProcessing
		})
		r.Act(func(t *testing.T) { got, err = f.tracks.Update(f.ctx, track) })
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("Update() error = %v", err)
			}
			assertEqual(t, got, track)
			stored, readErr := f.tracks.GetByID(f.ctx, track.ID)
			if readErr != nil {
				t.Fatalf("GetByID() error = %v", readErr)
			}
			assertEqual(t, stored, track)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-TRK-UPDATE-02", Method: "Update",
		Title:     "update of an unknown uuid returns not found",
		Given:     "an existing user and no tracks",
		When:      "Update is called for the nil UUID",
		Then:      "error not_found `track not found`",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var user domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			user = f.seedUser(t)
		})
		r.Act(func(t *testing.T) {
			_, err = f.tracks.Update(f.ctx, domain.Track{ID: missingUUID, UserID: user.ID, Title: "x", Status: domain.TrackReady})
		})
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeNotFound, "track not found") })
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-TRK-LIST-02", Method: "List",
		Title:     "artist filter ignores case and surrounding spaces and keeps the status filter",
		Given:     "a ready and a pending track, both by Fixture Artist",
		When:      "List is called with artist `  FiXtUrE aRtIsT  ` and status ready",
		Then:      "only the ready track is returned",
		Technique: testkit.TechniqueDecisionTable,
		Params:    map[string]string{"artist": "  FiXtUrE aRtIsT  ", "status": "ready"},
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var ready domain.Track
		var got []domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			user := f.seedUser(t)
			ready = f.seedTrack(t, user.ID)
			f.insertTrack(t, user.ID, "Draft", "Fixture Artist", domain.TrackPending)
		})
		r.Act(func(t *testing.T) {
			got, err = f.tracks.List(f.ctx, domain.TrackFilter{Artist: "  FiXtUrE aRtIsT  ", Status: domain.TrackReady})
		})
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			assertEqual(t, got, []domain.Track{ready})
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-TRK-LIST-03", Method: "List",
		Title:     "owner filter without status returns every status",
		Given:     "a ready and a pending track of one user and a track of another user",
		When:      "List is called with only the first user's id",
		Then:      "both of that user's tracks are returned in created_at order; the other user's track is excluded",
		Technique: testkit.TechniqueDecisionTable,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var owner domain.User
		var ready, pending domain.Track
		var got []domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			owner = f.seedUser(t)
			other := mustCreateUser(t, f, "other@example.com", "hash")
			ready = f.seedTrack(t, owner.ID)
			pending = f.insertTrack(t, owner.ID, "Draft", "Fixture Artist", domain.TrackPending)
			f.seedTrack(t, other.ID)
		})
		r.Act(func(t *testing.T) { got, err = f.tracks.List(f.ctx, domain.TrackFilter{UserID: owner.ID}) })
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			assertEqual(t, got, []domain.Track{ready, pending})
		})
	})

	for i, tc := range []struct {
		title, stored, query string
		match                bool
	}{
		{"apostrophe is matched literally", "O'Brien", "o'brien", true},
		{"percent sign is not a wildcard", "100% Pure", "100%", false},
		{"underscore is not a wildcard", "AxB", "A_B", false},
		{"cyrillic artist is matched exactly", "Мумий Тролль", "Мумий Тролль", true},
		{"sql text in the filter is treated as data", "Fixture Artist", "x' OR '1'='1", false},
	} {
		then := "no track is returned"
		if tc.match {
			then = "the stored track is returned"
		}
		runIntegration(t, component, testkit.Spec{
			ID: fmt.Sprintf("IT-DA-TRK-LIST-%02d", 4+i), Method: "List",
			Title:     tc.title,
			Given:     "one ready track with artist `" + tc.stored + "`",
			When:      "List is called with artist `" + tc.query + "` and status ready",
			Then:      then + "; the filter is a bound parameter compared with lower(btrim()) equality",
			Technique: testkit.TechniqueErrorGuessing,
			Severity:  testkit.SeverityCritical,
			Params:    map[string]string{"stored artist": tc.stored, "query": tc.query},
		}, func(t *testing.T, r testkit.Report) {
			var f *postgresFixture
			var stored domain.Track
			var got []domain.Track
			var err error
			r.Arrange(func(t *testing.T) {
				f = newPostgresFixture(t)
				stored = f.insertTrack(t, f.seedUser(t).ID, "Song", tc.stored, domain.TrackReady)
			})
			r.Act(func(t *testing.T) {
				got, err = f.tracks.List(f.ctx, domain.TrackFilter{Artist: tc.query, Status: domain.TrackReady})
			})
			r.Assert(func(t *testing.T) {
				if err != nil {
					t.Fatalf("List() error = %v", err)
				}
				want := []domain.Track{}
				if tc.match {
					want = []domain.Track{stored}
				}
				assertEqual(t, got, want)
				assertEqual(t, countRows(t, f, `SELECT count(*) FROM tracks`), 1)
			})
		})
	}

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-TRK-DELETE-02", Method: "Delete",
		Title:     "delete of an unknown uuid returns not found",
		Given:     "one stored track",
		When:      "Delete is called for the nil UUID",
		Then:      "error not_found `track not found`; the stored track remains",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			f.seedTrack(t, f.seedUser(t).ID)
		})
		r.Act(func(t *testing.T) { err = f.tracks.Delete(f.ctx, missingUUID) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeNotFound, "track not found")
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM tracks`), 1)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-TRK-DELETE-03", Method: "Delete",
		Title:     "delete cascades the track's likes",
		Given:     "a track with one like",
		When:      "Delete is called for the track",
		Then:      "track_likes.track_id ON DELETE CASCADE removes the like; tracks and track_likes are empty",
		Technique: testkit.TechniqueState,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var track domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			user := f.seedUser(t)
			track = f.seedTrack(t, user.ID)
			f.seedLike(t, user.ID, track.ID)
		})
		r.Act(func(t *testing.T) { err = f.tracks.Delete(f.ctx, track.ID) })
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("Delete() error = %v", err)
			}
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM track_likes`), 0)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM tracks`), 0)
		})
	})
}

func TestTrackLikeRepositoryRulesIntegration(t *testing.T) {
	const component = "TrackLikeRepository"

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-LIKE-LISTREADY-02", Method: "ListReadyByUser",
		Title:     "list ready likes skips a liked pending track",
		Given:     "a user who liked one ready and one pending track",
		When:      "ListReadyByUser is called",
		Then:      "only the ready track is returned; both like rows still exist",
		Technique: testkit.TechniqueState,
		Severity:  testkit.SeverityCritical,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var user domain.User
		var ready domain.Track
		var got []domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			user = f.seedUser(t)
			ready = f.seedTrack(t, user.ID)
			pending := f.insertTrack(t, user.ID, "Draft", "Fixture Artist", domain.TrackPending)
			f.seedLike(t, user.ID, ready.ID)
			f.seedLike(t, user.ID, pending.ID)
		})
		r.Act(func(t *testing.T) { got, err = f.likes.ListReadyByUser(f.ctx, user.ID) })
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("ListReadyByUser() error = %v", err)
			}
			assertEqual(t, got, []domain.Track{ready})
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM track_likes`), 2)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-LIKE-LISTREADY-03", Method: "ListReadyByUser",
		Title:     "list ready likes returns only the requested user's likes",
		Given:     "two users who each liked a different ready track",
		When:      "ListReadyByUser is called for the first user",
		Then:      "only the first user's liked track is returned",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var alice domain.User
		var mine domain.Track
		var got []domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			alice = f.seedUser(t)
			bob := mustCreateUser(t, f, "bob@example.com", "hash")
			mine = f.seedTrack(t, alice.ID)
			theirs := f.insertTrack(t, bob.ID, "Other", "Other Artist", domain.TrackReady)
			f.seedLike(t, alice.ID, mine.ID)
			f.seedLike(t, bob.ID, theirs.ID)
		})
		r.Act(func(t *testing.T) { got, err = f.likes.ListReadyByUser(f.ctx, alice.ID) })
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("ListReadyByUser() error = %v", err)
			}
			assertEqual(t, got, []domain.Track{mine})
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
		var user domain.User
		var track domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			user = f.seedUser(t)
			track = f.seedTrack(t, user.ID)
			f.seedLike(t, user.ID, track.ID)
		})
		r.Act(func(t *testing.T) { err = f.likes.Add(f.ctx, user.ID, track.ID) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeConflict, "track already liked")
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM track_likes`), 1)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-LIKE-ADD-03", Method: "Add",
		Title:     "like of an unknown track returns not found",
		Given:     "an existing user and no tracks",
		When:      "Add is called with track id equal to the nil UUID",
		Then:      "FK track_likes.track_id fails with SQLSTATE 23503, mapped to not_found `not found`; no row is written",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"track_id": missingUUID},
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var user domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			user = f.seedUser(t)
		})
		r.Act(func(t *testing.T) { err = f.likes.Add(f.ctx, user.ID, missingUUID) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeNotFound, "not found")
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM track_likes`), 0)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-LIKE-ADD-04", Method: "Add",
		Title:     "malformed track id returns invalid id",
		Given:     "an existing user",
		When:      "Add is called with track id not-a-uuid",
		Then:      "SQLSTATE 22P02, mapped to invalid `invalid id`",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"track_id": "not-a-uuid"},
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var user domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			user = f.seedUser(t)
		})
		r.Act(func(t *testing.T) { err = f.likes.Add(f.ctx, user.ID, "not-a-uuid") })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeInvalid, "invalid id") })
	})
}

func mustCreateUser(t *testing.T, f *postgresFixture, email, hash string) domain.User {
	t.Helper()
	user, err := f.users.Create(f.ctx, domain.User{Email: email, Name: "Member"}, hash)
	if err != nil {
		t.Fatalf("create user %s: %v", email, err)
	}
	return user
}
