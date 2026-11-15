//go:build integration

package postgres

import (
	"fmt"
	"testing"

	"chimera/catalog/internal/domain"
	"chimera/catalog/internal/testkit"
)

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
		var user seededUser
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
		var owner seededUser
		var ready, pending domain.Track
		var got []domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			owner = f.seedUser(t)
			other := f.seedUser(t)
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
		Title:     "delete records TrackDeleted with the object key in the outbox",
		Given:     "one ready track",
		When:      "Delete is called for the track",
		Then:      "tracks is empty and one unsent catalog.track.v1 outbox row keyed by the track id is written",
		Technique: testkit.TechniqueState,
		Severity:  testkit.SeverityCritical,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var track domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			user := f.seedUser(t)
			track = f.seedTrack(t, user.ID)
		})
		r.Act(func(t *testing.T) { err = f.tracks.Delete(f.ctx, track.ID) })
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("Delete() error = %v", err)
			}
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM tracks`), 0)
			assertEqual(t, countRows(t, f,
				`SELECT count(*) FROM outbox
				 WHERE topic = 'catalog.track.v1' AND record_key = $1
				   AND event_type = 'chimera.catalog.track.deleted.v1' AND sent_at IS NULL`, track.ID), 1)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-TRK-LIST-30", Method: "List",
		Title:     "list by ids returns only the listed tracks and an empty id list matches nothing",
		Given:     "two ready tracks",
		When:      "List is called with ByIDs and only the second id, then with ByIDs and no ids",
		Then:      "the first call returns the second track only and the second call returns an empty list",
		Technique: testkit.TechniqueBoundary,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var second domain.Track
		var got, none []domain.Track
		var err, noneErr error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			user := f.seedUser(t)
			f.seedTrack(t, user.ID)
			second = f.insertTrack(t, user.ID, "Second", "Fixture Artist", domain.TrackReady)
		})
		r.Act(func(t *testing.T) {
			got, err = f.tracks.List(f.ctx, domain.TrackFilter{ByIDs: true, IDs: []string{second.ID}})
			none, noneErr = f.tracks.List(f.ctx, domain.TrackFilter{ByIDs: true})
		})
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertNoError(t, noneErr)
			assertEqual(t, got, []domain.Track{second})
			assertEqual(t, none, []domain.Track{})
		})
	})
}

func TestTrackRepositoryDeleteByUserIntegration(t *testing.T) {
	const component = "TrackRepository"
	const eventID = "11111111-1111-4111-8111-111111111111"

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-TRK-DELUSER-01", Method: "DeleteByUser",
		Title:     "user deletion removes only that user's tracks, records the tombstone and one event per track",
		Given:     "two tracks of the deleted user and one track of another user",
		When:      "DeleteByUser is called with a fresh event id",
		Then:      "only the other user's track remains, deleted_users holds the user and two TrackDeleted rows are in the outbox",
		Technique: testkit.TechniqueEquivalence,
		Severity:  testkit.SeverityCritical,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var user, other seededUser
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			user, other = f.seedUser(t), f.seedUser(t)
			f.seedTrack(t, user.ID)
			f.seedTrack(t, user.ID)
			f.seedTrack(t, other.ID)
		})
		r.Act(func(t *testing.T) {
			err = f.tracks.DeleteByUser(f.ctx, domain.UserDeletion{EventID: eventID, UserID: user.ID})
		})
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM tracks WHERE user_id = $1::uuid`, user.ID), 0)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM tracks WHERE user_id = $1::uuid`, other.ID), 1)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM deleted_users WHERE user_id = $1::uuid`, user.ID), 1)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM outbox WHERE event_type = 'chimera.catalog.track.deleted.v1'`), 2)
		})
	})

	runIntegration(t, component, testkit.Spec{
		ID: "IT-DA-TRK-DELUSER-02", Method: "DeleteByUser",
		Title:     "a redelivered user deletion changes nothing",
		Given:     "the event was already applied and the user then got a track through a direct insert",
		When:      "DeleteByUser is called again with the same event id",
		Then:      "the track stays and no further outbox rows are written",
		Technique: testkit.TechniqueState,
	}, func(t *testing.T, r testkit.Report) {
		var f *postgresFixture
		var user seededUser
		var err error
		r.Arrange(func(t *testing.T) {
			f = newPostgresFixture(t)
			user = f.seedUser(t)
			if err := f.tracks.DeleteByUser(f.ctx, domain.UserDeletion{EventID: eventID, UserID: user.ID}); err != nil {
				t.Fatalf("first DeleteByUser() error = %v", err)
			}
			f.seedTrack(t, user.ID)
		})
		r.Act(func(t *testing.T) {
			err = f.tracks.DeleteByUser(f.ctx, domain.UserDeletion{EventID: eventID, UserID: user.ID})
		})
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM tracks`), 1)
			assertEqual(t, countRows(t, f, `SELECT count(*) FROM outbox`), 0)
		})
	})

}
