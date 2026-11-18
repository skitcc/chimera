package usecase

import (
	"context"
	"testing"

	"chimera/catalog/internal/domain"
	"chimera/catalog/internal/testkit"
)

const cleanupComponent = "CleanupService"

type fakeCleanupStore struct {
	deletions []domain.UserDeletion
	removed   []string
	err       error
}

func (f *fakeCleanupStore) DeleteByUser(_ context.Context, d domain.UserDeletion) error {
	f.deletions = append(f.deletions, d)
	return f.err
}

func (f *fakeCleanupStore) Remove(_ context.Context, key string) error {
	f.removed = append(f.removed, key)
	return f.err
}

func TestCleanupServiceRemoveUserTracks(t *testing.T) {
	const method = "RemoveUserTracks"

	runSpec(t, cleanupComponent, testkit.Spec{
		ID: "UC-CLN-DRAFT-01", Method: method, Kind: testkit.KindLondon,
		Title:     "passes a valid deletion to the repository",
		Given:     "a deletion with event id evt-1 and user id user-1",
		When:      "RemoveUserTracks is called",
		Then:      "the repository receives exactly that deletion and no error is returned",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var store *fakeCleanupStore
		var service *CleanupService
		var err error
		in := domain.UserDeletion{EventID: "11111111-1111-4111-8111-111111111111", UserID: testUserID}
		r.Arrange(func(t *testing.T) {
			store = &fakeCleanupStore{}
			service = NewCleanupService(store, store)
		})
		r.Act(func(t *testing.T) { err = service.RemoveUserTracks(context.Background(), in) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, []domain.UserDeletion{in}, store.deletions)
		})
	})

	for _, tc := range []struct {
		id, title, message string
		in                 domain.UserDeletion
	}{
		{"UC-CLN-DRAFT-02", "rejects a deletion without event id", "event id is required", domain.UserDeletion{UserID: testUserID}},
		{"UC-CLN-DRAFT-03", "rejects a deletion without user id", "user id is required", domain.UserDeletion{EventID: "11111111-1111-4111-8111-111111111111"}},
	} {
		runSpec(t, cleanupComponent, testkit.Spec{
			ID: tc.id, Method: method, Kind: testkit.KindLondon,
			Title:     tc.title,
			Given:     "a deletion with a missing field",
			When:      "RemoveUserTracks is called",
			Then:      `an invalid error "` + tc.message + `" is returned and the repository is not called`,
			Technique: testkit.TechniqueBoundary,
		}, func(t *testing.T, r testkit.Report) {
			var store *fakeCleanupStore
			var err error
			r.Arrange(func(t *testing.T) { store = &fakeCleanupStore{} })
			r.Act(func(t *testing.T) {
				err = NewCleanupService(store, store).RemoveUserTracks(context.Background(), tc.in)
			})
			r.Assert(func(t *testing.T) {
				assertDomainError(t, err, domain.CodeInvalid, tc.message)
				assertEqual(t, 0, len(store.deletions))
			})
		})
	}
}

func TestCleanupServiceRemoveObject(t *testing.T) {
	const method = "RemoveObject"

	runSpec(t, cleanupComponent, testkit.Spec{
		ID: "UC-CLN-OBJ-01", Method: method, Kind: testkit.KindLondon,
		Title:     "removes the object by key",
		Given:     "the object key track-1.mp3",
		When:      "RemoveObject is called",
		Then:      "the storage removes track-1.mp3",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var store *fakeCleanupStore
		var err error
		r.Arrange(func(t *testing.T) { store = &fakeCleanupStore{} })
		r.Act(func(t *testing.T) {
			err = NewCleanupService(store, store).RemoveObject(context.Background(), "track-1.mp3")
		})
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, []string{"track-1.mp3"}, store.removed)
		})
	})

	runSpec(t, cleanupComponent, testkit.Spec{
		ID: "UC-CLN-OBJ-02", Method: method, Kind: testkit.KindLondon,
		Title:     "rejects an empty key",
		Given:     "an empty object key",
		When:      "RemoveObject is called",
		Then:      `an invalid error "object key is required" is returned and the storage is not called`,
		Technique: testkit.TechniqueBoundary,
	}, func(t *testing.T, r testkit.Report) {
		var store *fakeCleanupStore
		var err error
		r.Arrange(func(t *testing.T) { store = &fakeCleanupStore{} })
		r.Act(func(t *testing.T) { err = NewCleanupService(store, store).RemoveObject(context.Background(), "") })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInvalid, "object key is required")
			assertEqual(t, 0, len(store.removed))
		})
	})
}

func TestTrackServiceBatchGet(t *testing.T) {
	const method = "BatchGet"

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-BATCH-01", Method: method,
		Title:     "returns existing tracks of any status and skips missing ids",
		Given:     "a ready and a pending track",
		When:      "BatchGet is called with both ids and a missing one",
		Then:      "both stored tracks are returned without visibility filtering",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var ready, pending domain.Track
		var got []domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, _ = newTrackFixture()
			ready = readyTrackWithID("track-ready")
			pending = pendingTrack()
			pending.ID = "track-pending"
			tracks.seed(ready, pending)
		})
		r.Act(func(t *testing.T) {
			got, err = service.BatchGet(context.Background(), domain.TrackIDs{ready.ID, "missing", pending.ID})
		})
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, []domain.Track{ready, pending}, got)
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-BATCH-02", Method: method,
		Title:     "rejects an empty id in the batch",
		Given:     "a batch with an empty id",
		When:      "BatchGet is called",
		Then:      `an invalid error "track id is required" is returned`,
		Technique: testkit.TechniqueBoundary,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var err error
		r.Arrange(func(t *testing.T) { service, _, _ = newTrackFixture() })
		r.Act(func(t *testing.T) { _, err = service.BatchGet(context.Background(), domain.TrackIDs{""}) })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeInvalid, "track id is required") })
	})
}
