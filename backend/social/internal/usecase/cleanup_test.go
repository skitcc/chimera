package usecase

import (
	"context"
	"testing"

	"chimera/social/internal/domain"
	"chimera/social/internal/testkit"
)

const cleanupComponent = "CleanupService"

func TestCleanupServiceRemoveUser(t *testing.T) {
	const method = "RemoveUser"

	runSpec(t, cleanupComponent, testkit.Spec{
		ID: "UC-CLN-USER-01", Method: method,
		Title:     "passes a valid deletion to the repository",
		Given:     "a UserDeleted event for user-1",
		When:      "RemoveUser is called",
		Then:      "the repository receives the same deletion once",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var store *fakeCleanupRepository
		var err error
		in := domain.UserDeletion{EventID: testEventID, UserID: testUserID}
		r.Arrange(func(t *testing.T) { store = &fakeCleanupRepository{} })
		r.Act(func(t *testing.T) { err = NewCleanupService(store).RemoveUser(context.Background(), in) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, []domain.UserDeletion{in}, store.users)
		})
	})

	cases := []struct {
		id, title, eventID, userID, message string
	}{
		{"UC-CLN-USER-02", "rejects missing event id", "", testUserID, "event id is required"},
		{"UC-CLN-USER-03", "rejects malformed event id", "event-1", testUserID, "invalid event id"},
		{"UC-CLN-USER-04", "rejects missing user id", testEventID, "", "user id is required"},
	}
	for _, tc := range cases {
		runSpec(t, cleanupComponent, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "a UserDeleted event with event id \"" + tc.eventID + "\" and user id \"" + tc.userID + "\"",
			When:      "RemoveUser is called",
			Then:      "invalid \"" + tc.message + "\" is returned and the repository is not called",
			Technique: testkit.TechniqueEquivalence,
			Params:    map[string]string{"event id": tc.eventID, "user id": tc.userID},
		}, func(t *testing.T, r testkit.Report) {
			var store *fakeCleanupRepository
			var err error
			r.Arrange(func(t *testing.T) { store = &fakeCleanupRepository{} })
			r.Act(func(t *testing.T) {
				err = NewCleanupService(store).RemoveUser(context.Background(), domain.UserDeletion{EventID: tc.eventID, UserID: tc.userID})
			})
			r.Assert(func(t *testing.T) {
				assertDomainError(t, err, domain.CodeInvalid, tc.message)
				assertEqual(t, 0, len(store.users))
			})
		})
	}
}

func TestCleanupServiceRemoveTrack(t *testing.T) {
	const method = "RemoveTrack"

	runSpec(t, cleanupComponent, testkit.Spec{
		ID: "UC-CLN-TRACK-01", Method: method,
		Title:     "passes a valid deletion to the repository",
		Given:     "a TrackDeleted event for track-1",
		When:      "RemoveTrack is called",
		Then:      "the repository receives the same deletion once",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var store *fakeCleanupRepository
		var err error
		in := domain.TrackDeletion{EventID: testEventID, TrackID: testTrackID}
		r.Arrange(func(t *testing.T) { store = &fakeCleanupRepository{} })
		r.Act(func(t *testing.T) { err = NewCleanupService(store).RemoveTrack(context.Background(), in) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, []domain.TrackDeletion{in}, store.tracks)
		})
	})

	runSpec(t, cleanupComponent, testkit.Spec{
		ID: "UC-CLN-TRACK-02", Method: method,
		Title:     "rejects missing track id",
		Given:     "a TrackDeleted event without track id",
		When:      "RemoveTrack is called",
		Then:      "invalid \"track id is required\" is returned and the repository is not called",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var store *fakeCleanupRepository
		var err error
		r.Arrange(func(t *testing.T) { store = &fakeCleanupRepository{} })
		r.Act(func(t *testing.T) {
			err = NewCleanupService(store).RemoveTrack(context.Background(), domain.TrackDeletion{EventID: testEventID})
		})
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInvalid, "track id is required")
			assertEqual(t, 0, len(store.tracks))
		})
	})

	runSpec(t, cleanupComponent, testkit.Spec{
		ID: "UC-CLN-TRACK-03", Method: method,
		Title:     "returns repository error for a retry",
		Given:     "a cleanup repository whose DeleteTrack fails",
		When:      "RemoveTrack is called with a valid deletion",
		Then:      "the repository error is returned unchanged",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var store *fakeCleanupRepository
		var err error
		r.Arrange(func(t *testing.T) { store = &fakeCleanupRepository{err: errDependency} })
		r.Act(func(t *testing.T) {
			err = NewCleanupService(store).RemoveTrack(context.Background(), domain.TrackDeletion{EventID: testEventID, TrackID: testTrackID})
		})
		r.Assert(func(t *testing.T) { assertErrorIs(t, err, errDependency) })
	})
}
