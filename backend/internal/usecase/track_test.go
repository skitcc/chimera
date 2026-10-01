package usecase

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"chimera/internal/domain"
	"chimera/internal/testkit"
)

const trackComponent = "TrackService"

func readyTrackWithID(id string) domain.Track {
	track := readyTrack()
	track.ID = id
	track.ObjectKey = "audio/" + id + ".mp3"
	return track
}

func TestTrackServiceList(t *testing.T) {
	const method = "List"

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-LIST-01", Method: method,
		Title:     "returns ready tracks of trimmed artist",
		Given:     "a ready and a pending track by Artist and a ready track by Other",
		When:      "List is called with artist \" Artist \" and limit 1",
		Then:      "the page holds only the ready Artist track, limit 1 and no next cursor",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"artist": " Artist ", "limit": "1"},
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var match domain.Track
		var got domain.TrackPage
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, _, _ = newTrackFixture()
			match = readyTrack()
			pending := pendingTrack()
			pending.ID = "track-2"
			other := readyTrackWithID("track-3")
			other.Artist = "Other"
			tracks.seed(match, pending, other)
		})
		r.Act(func(t *testing.T) {
			got, err = service.List(context.Background(), domain.TrackFeedQuery{
				PageQuery: domain.PageQuery{Limit: 1},
				Artist:    " Artist ",
			})
		})
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, domain.TrackPage{Items: []domain.Track{match}, Limit: 1}, got)
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-LIST-02", Method: method,
		Title:     "applies cursor offset and returns next cursor",
		Given:     "three ready tracks track-1, track-2, track-3",
		When:      "List is called with limit 1 and cursor \"1\"",
		Then:      "the page holds track-2 with next cursor \"2\"",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"limit": "1", "cursor": "1"},
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var second domain.Track
		var got domain.TrackPage
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, _, _ = newTrackFixture()
			second = readyTrackWithID("track-2")
			tracks.seed(readyTrack(), second, readyTrackWithID("track-3"))
		})
		r.Act(func(t *testing.T) {
			got, err = service.List(context.Background(), domain.TrackFeedQuery{
				PageQuery: domain.PageQuery{Limit: 1, Cursor: "1"},
			})
		})
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, domain.TrackPage{Items: []domain.Track{second}, NextCursor: "2", Limit: 1}, got)
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-LIST-03", Method: method,
		Title:     "defaults zero limit to 20",
		Given:     "one ready track",
		When:      "List is called with limit 0",
		Then:      "the page holds the track and reports limit 20",
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"limit": "0"},
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var got domain.TrackPage
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, _, _ = newTrackFixture()
			tracks.seed(readyTrack())
		})
		r.Act(func(t *testing.T) { got, err = service.List(context.Background(), domain.TrackFeedQuery{}) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, domain.TrackPage{Items: []domain.Track{readyTrack()}, Limit: 20}, got)
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-LIST-04", Method: method,
		Title:     "accepts maximum limit 100",
		Given:     "one ready track",
		When:      "List is called with limit 100",
		Then:      "the page holds the track and reports limit 100",
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"limit": "100"},
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var got domain.TrackPage
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, _, _ = newTrackFixture()
			tracks.seed(readyTrack())
		})
		r.Act(func(t *testing.T) {
			got, err = service.List(context.Background(), domain.TrackFeedQuery{PageQuery: domain.PageQuery{Limit: 100}})
		})
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, domain.TrackPage{Items: []domain.Track{readyTrack()}, Limit: 100}, got)
		})
	})

	for _, tc := range []struct {
		id, title, cursor, message, technique string
		limit                                 int
	}{
		{"UC-TRK-LIST-05", "rejects limit 101 before listing", "", "limit exceeded", testkit.TechniqueBoundary, 101},
		{"UC-TRK-LIST-06", "rejects non-numeric cursor abc before listing", "abc", "invalid cursor", testkit.TechniqueEquivalence, 10},
		{"UC-TRK-LIST-07", "rejects negative cursor -1 before listing", "-1", "invalid cursor", testkit.TechniqueBoundary, 10},
	} {
		runSpec(t, trackComponent, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "a track repository whose List is set to fail",
			When:      "List is called with limit " + strconv.Itoa(tc.limit) + " and cursor \"" + tc.cursor + "\"",
			Then:      "invalid \"" + tc.message + "\" is returned instead of the repository error",
			Technique: tc.technique,
			Params:    map[string]string{"limit": strconv.Itoa(tc.limit), "cursor": tc.cursor},
		}, func(t *testing.T, r testkit.Report) {
			var service *TrackService
			var err error
			r.Arrange(func(t *testing.T) {
				var tracks *fakeTrackRepository
				service, tracks, _, _ = newTrackFixture()
				tracks.listErr = errDependency
			})
			r.Act(func(t *testing.T) {
				_, err = service.List(context.Background(), domain.TrackFeedQuery{
					PageQuery: domain.PageQuery{Limit: tc.limit, Cursor: tc.cursor},
				})
			})
			r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeInvalid, tc.message) })
		})
	}

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-LIST-08", Method: method,
		Title:     "returns repository list error",
		Given:     "a track repository whose List fails",
		When:      "List is called with an empty query",
		Then:      "the repository error is returned unchanged with an empty page",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var got domain.TrackPage
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, _, _ = newTrackFixture()
			tracks.listErr = errDependency
		})
		r.Act(func(t *testing.T) { got, err = service.List(context.Background(), domain.TrackFeedQuery{}) })
		r.Assert(func(t *testing.T) {
			assertErrorIs(t, err, errDependency)
			assertEqual(t, domain.TrackPage{}, got)
		})
	})
}

func TestTrackServiceListByUploader(t *testing.T) {
	const method = "ListByUploader"

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-LISTUP-01", Method: method,
		Title:     "returns uploader tracks with requested status",
		Given:     "user-1 owns a ready and a pending track and user-2 owns a ready track",
		When:      "ListByUploader is called for user-1 with status ready and limit 10",
		Then:      "the page holds only the ready track of user-1",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"user": testUserID, "status": string(domain.TrackReady)},
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var match domain.Track
		var got domain.TrackPage
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, _, _ = newTrackFixture()
			match = readyTrack()
			foreign := readyTrackWithID("track-2")
			foreign.UserID = "user-2"
			pending := pendingTrack()
			pending.ID = "track-3"
			tracks.seed(match, foreign, pending)
		})
		r.Act(func(t *testing.T) {
			got, err = service.ListByUploader(context.Background(), domain.TrackOwnerQuery{
				PageQuery: domain.PageQuery{Limit: 10},
				UserID:    testUserID,
				Status:    domain.TrackReady,
			})
		})
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, domain.TrackPage{Items: []domain.Track{match}, Limit: 10}, got)
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-LISTUP-02", Method: method,
		Title:     "returns uploader tracks of every status when status is omitted",
		Given:     "user-1 owns a ready and a pending track",
		When:      "ListByUploader is called for user-1 without status",
		Then:      "the page holds both tracks in insertion order",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"user": testUserID, "status": ""},
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var ready, pending domain.Track
		var got domain.TrackPage
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, _, _ = newTrackFixture()
			ready = readyTrack()
			pending = pendingTrack()
			pending.ID = "track-2"
			tracks.seed(ready, pending)
		})
		r.Act(func(t *testing.T) {
			got, err = service.ListByUploader(context.Background(), domain.TrackOwnerQuery{UserID: testUserID})
		})
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, domain.TrackPage{Items: []domain.Track{ready, pending}, Limit: 20}, got)
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-LISTUP-03", Method: method,
		Title:     "rejects missing uploader before listing",
		Given:     "a track repository whose List is set to fail",
		When:      "ListByUploader is called without user id",
		Then:      "invalid \"user id is required\" is returned instead of the repository error",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"user": ""},
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, _, _ = newTrackFixture()
			tracks.listErr = errDependency
		})
		r.Act(func(t *testing.T) {
			_, err = service.ListByUploader(context.Background(), domain.TrackOwnerQuery{PageQuery: domain.PageQuery{Limit: 10}})
		})
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeInvalid, "user id is required") })
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-LISTUP-04", Method: method,
		Title:     "returns repository list error",
		Given:     "a track repository whose List fails",
		When:      "ListByUploader is called for user-1",
		Then:      "the repository error is returned unchanged",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, _, _ = newTrackFixture()
			tracks.listErr = errDependency
		})
		r.Act(func(t *testing.T) {
			_, err = service.ListByUploader(context.Background(), domain.TrackOwnerQuery{UserID: testUserID})
		})
		r.Assert(func(t *testing.T) { assertErrorIs(t, err, errDependency) })
	})
}

func TestTrackServiceListLiked(t *testing.T) {
	const method = "ListLiked"

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-LISTLIKED-01", Method: method,
		Title:     "returns only liked tracks in ready status",
		Given:     "user-1 likes a ready track and a pending track",
		When:      "ListLiked is called for user-1 with limit 10",
		Then:      "the page holds only the ready liked track",
		Technique: testkit.TechniqueState,
		Params:    map[string]string{"user": testUserID},
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var liked domain.Track
		var got domain.TrackPage
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			var likes *fakeTrackLikeRepository
			service, tracks, likes, _ = newTrackFixture()
			liked = readyTrack()
			pending := pendingTrack()
			pending.ID = "track-2"
			tracks.seed(liked, pending)
			likes.likes[testUserID] = map[string]bool{liked.ID: true, pending.ID: true}
		})
		r.Act(func(t *testing.T) {
			got, err = service.ListLiked(context.Background(), domain.TrackLikeListQuery{
				PageQuery: domain.PageQuery{Limit: 10},
				UserID:    testUserID,
			})
		})
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, domain.TrackPage{Items: []domain.Track{liked}, Limit: 10}, got)
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-LISTLIKED-02", Method: method,
		Title:     "rejects missing user before listing likes",
		Given:     "a like repository whose listing is set to fail",
		When:      "ListLiked is called without user id",
		Then:      "invalid \"user id is required\" is returned instead of the repository error",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var err error
		r.Arrange(func(t *testing.T) {
			var likes *fakeTrackLikeRepository
			service, _, likes, _ = newTrackFixture()
			likes.listErr = errDependency
		})
		r.Act(func(t *testing.T) { _, err = service.ListLiked(context.Background(), domain.TrackLikeListQuery{}) })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeInvalid, "user id is required") })
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-LISTLIKED-03", Method: method,
		Title:     "returns like repository error",
		Given:     "a like repository whose ListReadyByUser fails",
		When:      "ListLiked is called for user-1",
		Then:      "the like repository error is returned unchanged with an empty page",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var got domain.TrackPage
		var err error
		r.Arrange(func(t *testing.T) {
			var likes *fakeTrackLikeRepository
			service, _, likes, _ = newTrackFixture()
			likes.listErr = errDependency
		})
		r.Act(func(t *testing.T) {
			got, err = service.ListLiked(context.Background(), domain.TrackLikeListQuery{UserID: testUserID})
		})
		r.Assert(func(t *testing.T) {
			assertErrorIs(t, err, errDependency)
			assertEqual(t, domain.TrackPage{}, got)
		})
	})
}

func TestTrackServiceLikeClassic(t *testing.T) {
	const method = "Like"

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-LIKE-01", Method: method,
		Title:     "stores like for ready track",
		Given:     "a ready track track-1",
		When:      "Like is called by user-1 for track-1",
		Then:      "no error is returned and the like user-1/track-1 is stored",
		Technique: testkit.TechniqueState,
		Params:    map[string]string{"status": string(domain.TrackReady)},
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, likes, _ = newTrackFixture()
			tracks.seed(readyTrack())
		})
		r.Act(func(t *testing.T) { err = service.Like(context.Background(), validTrackLike()) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, map[string]map[string]bool{testUserID: {testTrackID: true}}, likes.likes)
		})
	})

	for _, tc := range []struct {
		id, title, given, then, technique string
		status                            domain.TrackStatus
	}{
		{"UC-TRK-LIKE-02", "rejects pending track without storing like", "a pending track track-1", "conflict \"track is not ready\" is returned and no like is stored", testkit.TechniqueState, domain.TrackPending},
		{"UC-TRK-LIKE-03", "rejects processing track without storing like", "a processing track track-1", "conflict \"track is not ready\" is returned and no like is stored", testkit.TechniqueState, domain.TrackProcessing},
	} {
		runSpec(t, trackComponent, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     tc.given,
			When:      "Like is called by user-1 for track-1",
			Then:      tc.then,
			Technique: tc.technique,
			Params:    map[string]string{"status": string(tc.status)},
		}, func(t *testing.T, r testkit.Report) {
			var service *TrackService
			var likes *fakeTrackLikeRepository
			var err error
			r.Arrange(func(t *testing.T) {
				var tracks *fakeTrackRepository
				service, tracks, likes, _ = newTrackFixture()
				track := pendingTrack()
				track.Status = tc.status
				tracks.seed(track)
			})
			r.Act(func(t *testing.T) { err = service.Like(context.Background(), validTrackLike()) })
			r.Assert(func(t *testing.T) {
				assertDomainError(t, err, domain.CodeConflict, "track is not ready")
				assertEqual(t, 0, len(likes.likes))
			})
		})
	}

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-LIKE-04", Method: method,
		Title:     "reports missing user id before track lookup",
		Given:     "a track repository whose lookup is set to fail",
		When:      "Like is called with an empty user id and empty track id",
		Then:      "invalid \"user id is required\" is returned instead of the lookup error and no like is stored",
		Technique: testkit.TechniqueDecisionTable,
		Params:    map[string]string{"user": "", "track": ""},
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, likes, _ = newTrackFixture()
			tracks.getErr = errDependency
		})
		r.Act(func(t *testing.T) { err = service.Like(context.Background(), domain.TrackLike{}) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInvalid, "user id is required")
			assertEqual(t, 0, len(likes.likes))
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-LIKE-05", Method: method,
		Title:     "propagates not found for unknown track without storing like",
		Given:     "no stored tracks",
		When:      "Like is called by user-1 for track-1",
		Then:      "not_found \"track not found\" is returned and no like is stored",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) { service, _, likes, _ = newTrackFixture() })
		r.Act(func(t *testing.T) { err = service.Like(context.Background(), validTrackLike()) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeNotFound, "track not found")
			assertEqual(t, 0, len(likes.likes))
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-LIKE-06", Method: method,
		Title:     "returns track lookup error without storing like",
		Given:     "a track repository whose GetByID fails",
		When:      "Like is called by user-1 for track-1",
		Then:      "the lookup error is returned unchanged and no like is stored",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, likes, _ = newTrackFixture()
			tracks.getErr = errDependency
		})
		r.Act(func(t *testing.T) { err = service.Like(context.Background(), validTrackLike()) })
		r.Assert(func(t *testing.T) {
			assertErrorIs(t, err, errDependency)
			assertEqual(t, 0, len(likes.likes))
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-LIKE-07", Method: method,
		Title:     "returns add like error",
		Given:     "a ready track and a like repository whose Add fails",
		When:      "Like is called by user-1 for track-1",
		Then:      "the like repository error is returned unchanged and no like is stored",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, likes, _ = newTrackFixture()
			tracks.seed(readyTrack())
			likes.addErr = errDependency
		})
		r.Act(func(t *testing.T) { err = service.Like(context.Background(), validTrackLike()) })
		r.Assert(func(t *testing.T) {
			assertErrorIs(t, err, errDependency)
			assertEqual(t, 0, len(likes.likes))
		})
	})
}

func TestTrackServiceLikeLondon(t *testing.T) {
	const method = "Like"

	newLondonService := func(tracks *mockTrackRepository, likes *spyTrackLikeRepository) *TrackService {
		return NewTrackService(tracks, likes, stubObjectStorage{}, testMaxSize)
	}

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-LIKE-08", Method: method,
		Title:     "looks up exact track then adds exact like once",
		Given:     "a track repository mock returning a ready track and a like repository spy",
		When:      "Like is called by user-1 for track-1",
		Then:      "GetByID is called once with track-1 and Add is called once with user-1/track-1",
		Technique: testkit.TechniqueEquivalence,
		Kind:      testkit.KindLondon,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var tracks *mockTrackRepository
		var likes *spyTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			tracks = &mockTrackRepository{getResult: readyTrack()}
			likes = &spyTrackLikeRepository{}
			service = newLondonService(tracks, likes)
		})
		r.Act(func(t *testing.T) { err = service.Like(context.Background(), validTrackLike()) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, []string{testTrackID}, tracks.getIDs)
			assertEqual(t, []trackLikeCall{{userID: testUserID, trackID: testTrackID}}, likes.addCalls)
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-LIKE-09", Method: method,
		Title:     "calls no collaborator for invalid input",
		Given:     "a track repository mock and a like repository spy",
		When:      "Like is called with an empty input",
		Then:      "invalid \"user id is required\" is returned, GetByID is not called and Add is not called",
		Technique: testkit.TechniqueEquivalence,
		Kind:      testkit.KindLondon,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var tracks *mockTrackRepository
		var likes *spyTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			tracks = &mockTrackRepository{}
			likes = &spyTrackLikeRepository{}
			service = newLondonService(tracks, likes)
		})
		r.Act(func(t *testing.T) { err = service.Like(context.Background(), domain.TrackLike{}) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInvalid, "user id is required")
			assertEqual(t, 0, len(tracks.getIDs))
			assertEqual(t, 0, len(likes.addCalls))
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-LIKE-10", Method: method,
		Title:     "does not call add when lookup fails",
		Given:     "a track repository mock whose GetByID fails and a like repository spy",
		When:      "Like is called by user-1 for track-1",
		Then:      "the lookup error is returned, GetByID is called once with track-1 and Add is not called",
		Technique: testkit.TechniqueErrorGuessing,
		Kind:      testkit.KindLondon,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var tracks *mockTrackRepository
		var likes *spyTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			tracks = &mockTrackRepository{getErr: errDependency}
			likes = &spyTrackLikeRepository{}
			service = newLondonService(tracks, likes)
		})
		r.Act(func(t *testing.T) { err = service.Like(context.Background(), validTrackLike()) })
		r.Assert(func(t *testing.T) {
			assertErrorIs(t, err, errDependency)
			assertEqual(t, []string{testTrackID}, tracks.getIDs)
			assertEqual(t, 0, len(likes.addCalls))
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-LIKE-11", Method: method,
		Title:     "does not call add for pending track",
		Given:     "a track repository mock returning a pending track and a like repository spy",
		When:      "Like is called by user-1 for track-1",
		Then:      "conflict \"track is not ready\" is returned, GetByID is called once and Add is not called",
		Technique: testkit.TechniqueState,
		Kind:      testkit.KindLondon,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var tracks *mockTrackRepository
		var likes *spyTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			tracks = &mockTrackRepository{getResult: pendingTrack()}
			likes = &spyTrackLikeRepository{}
			service = newLondonService(tracks, likes)
		})
		r.Act(func(t *testing.T) { err = service.Like(context.Background(), validTrackLike()) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeConflict, "track is not ready")
			assertEqual(t, []string{testTrackID}, tracks.getIDs)
			assertEqual(t, 0, len(likes.addCalls))
		})
	})
}

func TestTrackServiceUnlike(t *testing.T) {
	const method = "Unlike"

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-UNLIKE-01", Method: method,
		Title:     "removes only the requested like",
		Given:     "user-1 likes ready tracks track-1 and track-2",
		When:      "Unlike is called by user-1 for track-1",
		Then:      "no error is returned, the track-1 like is removed and the track-2 like remains",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, likes, _ = newTrackFixture()
			tracks.seed(readyTrack(), readyTrackWithID("track-2"))
			likes.likes[testUserID] = map[string]bool{testTrackID: true, "track-2": true}
		})
		r.Act(func(t *testing.T) { err = service.Unlike(context.Background(), validTrackLike()) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, map[string]bool{"track-2": true}, likes.likes[testUserID])
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-UNLIKE-02", Method: method,
		Title:     "succeeds idempotently when track is not liked",
		Given:     "a ready track track-1 that user-1 has not liked",
		When:      "Unlike is called by user-1 for track-1",
		Then:      "no error is returned (Remove is a no-op) and no like is stored",
		Technique: testkit.TechniqueState,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, likes, _ = newTrackFixture()
			tracks.seed(readyTrack())
		})
		r.Act(func(t *testing.T) { err = service.Unlike(context.Background(), validTrackLike()) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, 0, len(likes.likes[testUserID]))
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-UNLIKE-03", Method: method,
		Title:     "removes like of pending track without ready check",
		Given:     "user-1 likes track-1 which is pending",
		When:      "Unlike is called by user-1 for track-1",
		Then:      "no error is returned and the like is removed, unlike Like which requires ready status",
		Technique: testkit.TechniqueState,
		Params:    map[string]string{"status": string(domain.TrackPending)},
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, likes, _ = newTrackFixture()
			tracks.seed(pendingTrack())
			likes.likes[testUserID] = map[string]bool{testTrackID: true}
		})
		r.Act(func(t *testing.T) { err = service.Unlike(context.Background(), validTrackLike()) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, 0, len(likes.likes[testUserID]))
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-UNLIKE-04", Method: method,
		Title:     "rejects missing track id without removing like",
		Given:     "user-1 likes ready track track-1",
		When:      "Unlike is called by user-1 with an empty track id",
		Then:      "invalid \"track id is required\" is returned and the existing like remains",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"track": ""},
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, likes, _ = newTrackFixture()
			tracks.seed(readyTrack())
			likes.likes[testUserID] = map[string]bool{testTrackID: true}
		})
		r.Act(func(t *testing.T) {
			err = service.Unlike(context.Background(), domain.TrackLike{UserID: testUserID})
		})
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInvalid, "track id is required")
			assertEqual(t, map[string]bool{testTrackID: true}, likes.likes[testUserID])
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-UNLIKE-05", Method: method,
		Title:     "propagates not found for unknown track without removing like",
		Given:     "a stale like user-1/track-1 whose track no longer exists",
		When:      "Unlike is called by user-1 for track-1",
		Then:      "not_found \"track not found\" is returned and the stale like remains",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			service, _, likes, _ = newTrackFixture()
			likes.likes[testUserID] = map[string]bool{testTrackID: true}
		})
		r.Act(func(t *testing.T) { err = service.Unlike(context.Background(), validTrackLike()) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeNotFound, "track not found")
			assertEqual(t, map[string]bool{testTrackID: true}, likes.likes[testUserID])
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-UNLIKE-06", Method: method,
		Title:     "returns track lookup error without removing like",
		Given:     "an existing like and a track repository whose GetByID fails",
		When:      "Unlike is called by user-1 for track-1",
		Then:      "the lookup error is returned unchanged and the like remains",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, likes, _ = newTrackFixture()
			likes.likes[testUserID] = map[string]bool{testTrackID: true}
			tracks.getErr = errDependency
		})
		r.Act(func(t *testing.T) { err = service.Unlike(context.Background(), validTrackLike()) })
		r.Assert(func(t *testing.T) {
			assertErrorIs(t, err, errDependency)
			assertEqual(t, map[string]bool{testTrackID: true}, likes.likes[testUserID])
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-UNLIKE-07", Method: method,
		Title:     "returns remove like error",
		Given:     "an existing like on a ready track and a like repository whose Remove fails",
		When:      "Unlike is called by user-1 for track-1",
		Then:      "the like repository error is returned unchanged and the like remains",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, likes, _ = newTrackFixture()
			tracks.seed(readyTrack())
			likes.likes[testUserID] = map[string]bool{testTrackID: true}
			likes.removeErr = errDependency
		})
		r.Act(func(t *testing.T) { err = service.Unlike(context.Background(), validTrackLike()) })
		r.Assert(func(t *testing.T) {
			assertErrorIs(t, err, errDependency)
			assertEqual(t, map[string]bool{testTrackID: true}, likes.likes[testUserID])
		})
	})
}

func TestTrackServiceGetByID(t *testing.T) {
	const method = "GetByID"

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-GET-01", Method: method,
		Title:     "returns track by id",
		Given:     "a stored ready track track-1",
		When:      "GetByID is called with track-1",
		Then:      "the stored track is returned",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"id": testTrackID},
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var got domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, _, _ = newTrackFixture()
			tracks.seed(readyTrack())
		})
		r.Act(func(t *testing.T) { got, err = service.GetByID(context.Background(), testTrackID) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, readyTrack(), got)
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-GET-02", Method: method,
		Title:     "rejects empty id before lookup",
		Given:     "a track repository whose lookup is set to fail",
		When:      "GetByID is called with an empty id",
		Then:      "invalid \"track id is required\" is returned instead of the repository error",
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"id": ""},
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, _, _ = newTrackFixture()
			tracks.getErr = errDependency
		})
		r.Act(func(t *testing.T) { _, err = service.GetByID(context.Background(), "") })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeInvalid, "track id is required") })
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-GET-03", Method: method,
		Title:     "propagates not found for unknown id",
		Given:     "no stored tracks",
		When:      "GetByID is called with track-1",
		Then:      "not_found \"track not found\" is returned",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var err error
		r.Arrange(func(t *testing.T) { service, _, _, _ = newTrackFixture() })
		r.Act(func(t *testing.T) { _, err = service.GetByID(context.Background(), testTrackID) })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeNotFound, "track not found") })
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-GET-04", Method: method,
		Title:     "returns repository lookup error",
		Given:     "a track repository whose GetByID fails",
		When:      "GetByID is called with track-1",
		Then:      "the repository error is returned unchanged",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, _, _ = newTrackFixture()
			tracks.getErr = errDependency
		})
		r.Act(func(t *testing.T) { _, err = service.GetByID(context.Background(), testTrackID) })
		r.Assert(func(t *testing.T) { assertErrorIs(t, err, errDependency) })
	})
}

func TestTrackServiceUpdate(t *testing.T) {
	const method = "Update"

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-UPDATE-01", Method: method,
		Title:     "updates title and artist preserving status, owner and object",
		Given:     "a stored ready track track-1",
		When:      "Update is called for track-1 with title \"New title\" and artist \"New artist\"",
		Then:      "the returned and stored track has the new title and artist and unchanged status, owner, key and size",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var tracks *fakeTrackRepository
		var want, got domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			service, tracks, _, _ = newTrackFixture()
			tracks.seed(readyTrack())
			want = readyTrack()
			want.Title = "New title"
			want.Artist = "New artist"
		})
		r.Act(func(t *testing.T) {
			got, err = service.Update(context.Background(), testTrackID, domain.TrackWrite{Title: "New title", Artist: "New artist"})
		})
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, want, got)
			assertEqual(t, want, tracks.tracks[testTrackID])
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-UPDATE-02", Method: method,
		Title:     "clears artist when artist is omitted",
		Given:     "a stored ready track track-1 with artist \"Artist\"",
		When:      "Update is called for track-1 with title \"New title\" and no artist",
		Then:      "the stored track has title \"New title\" and an empty artist",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"artist": ""},
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var tracks *fakeTrackRepository
		var want domain.Track
		var err error
		r.Arrange(func(t *testing.T) {
			service, tracks, _, _ = newTrackFixture()
			tracks.seed(readyTrack())
			want = readyTrack()
			want.Title = "New title"
			want.Artist = ""
		})
		r.Act(func(t *testing.T) {
			_, err = service.Update(context.Background(), testTrackID, domain.TrackWrite{Title: "New title"})
		})
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, want, tracks.tracks[testTrackID])
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-UPDATE-03", Method: method,
		Title:     "reports empty id before missing title",
		Given:     "a stored ready track track-1",
		When:      "Update is called with an empty id and an empty title",
		Then:      "invalid \"track id is required\" is returned and the stored track is unchanged",
		Technique: testkit.TechniqueDecisionTable,
		Params:    map[string]string{"id": "", "title": ""},
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var tracks *fakeTrackRepository
		var err error
		r.Arrange(func(t *testing.T) {
			service, tracks, _, _ = newTrackFixture()
			tracks.seed(readyTrack())
		})
		r.Act(func(t *testing.T) { _, err = service.Update(context.Background(), "", domain.TrackWrite{}) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInvalid, "track id is required")
			assertEqual(t, readyTrack(), tracks.tracks[testTrackID])
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-UPDATE-04", Method: method,
		Title:     "rejects missing title before lookup",
		Given:     "a stored ready track and a track repository whose lookup is set to fail",
		When:      "Update is called for track-1 with an empty title",
		Then:      "invalid \"title is required\" is returned instead of the lookup error and the stored track is unchanged",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"title": ""},
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var tracks *fakeTrackRepository
		var err error
		r.Arrange(func(t *testing.T) {
			service, tracks, _, _ = newTrackFixture()
			tracks.seed(readyTrack())
			tracks.getErr = errDependency
		})
		r.Act(func(t *testing.T) {
			_, err = service.Update(context.Background(), testTrackID, domain.TrackWrite{Artist: "New artist"})
		})
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInvalid, "title is required")
			assertEqual(t, readyTrack(), tracks.tracks[testTrackID])
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-UPDATE-05", Method: method,
		Title:     "propagates not found for unknown track",
		Given:     "no stored tracks",
		When:      "Update is called for track-1 with a valid input",
		Then:      "not_found \"track not found\" is returned and no track is created",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var tracks *fakeTrackRepository
		var err error
		r.Arrange(func(t *testing.T) { service, tracks, _, _ = newTrackFixture() })
		r.Act(func(t *testing.T) {
			_, err = service.Update(context.Background(), testTrackID, domain.TrackWrite{Title: "Title"})
		})
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeNotFound, "track not found")
			assertEqual(t, 0, len(tracks.tracks))
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-UPDATE-06", Method: method,
		Title:     "returns lookup error",
		Given:     "a track repository whose GetByID fails",
		When:      "Update is called for track-1 with a valid input",
		Then:      "the lookup error is returned unchanged",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, _, _ = newTrackFixture()
			tracks.getErr = errDependency
		})
		r.Act(func(t *testing.T) {
			_, err = service.Update(context.Background(), testTrackID, domain.TrackWrite{Title: "Title"})
		})
		r.Assert(func(t *testing.T) { assertErrorIs(t, err, errDependency) })
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-UPDATE-07", Method: method,
		Title:     "returns update error and keeps stored track",
		Given:     "a stored ready track and a track repository whose Update fails",
		When:      "Update is called for track-1 with title \"Title\"",
		Then:      "the repository error is returned unchanged and the stored track keeps its old title",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var tracks *fakeTrackRepository
		var err error
		r.Arrange(func(t *testing.T) {
			service, tracks, _, _ = newTrackFixture()
			tracks.seed(readyTrack())
			tracks.updateErrors = []error{errDependency}
		})
		r.Act(func(t *testing.T) {
			_, err = service.Update(context.Background(), testTrackID, domain.TrackWrite{Title: "Title"})
		})
		r.Assert(func(t *testing.T) {
			assertErrorIs(t, err, errDependency)
			assertEqual(t, readyTrack(), tracks.tracks[testTrackID])
		})
	})
}

func TestTrackServiceDelete(t *testing.T) {
	const method = "Delete"

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-DELETE-01", Method: method,
		Title:     "deletes stored track",
		Given:     "a stored ready track track-1",
		When:      "Delete is called with track-1",
		Then:      "no error is returned and track-1 is no longer stored",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var tracks *fakeTrackRepository
		var err error
		r.Arrange(func(t *testing.T) {
			service, tracks, _, _ = newTrackFixture()
			tracks.seed(readyTrack())
		})
		r.Act(func(t *testing.T) { err = service.Delete(context.Background(), testTrackID) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, 0, len(tracks.tracks))
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-DELETE-02", Method: method,
		Title:     "rejects empty id without deleting",
		Given:     "a stored ready track track-1",
		When:      "Delete is called with an empty id",
		Then:      "invalid \"track id is required\" is returned and track-1 remains stored",
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"id": ""},
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var tracks *fakeTrackRepository
		var err error
		r.Arrange(func(t *testing.T) {
			service, tracks, _, _ = newTrackFixture()
			tracks.seed(readyTrack())
		})
		r.Act(func(t *testing.T) { err = service.Delete(context.Background(), "") })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInvalid, "track id is required")
			assertEqual(t, readyTrack(), tracks.tracks[testTrackID])
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-DELETE-03", Method: method,
		Title:     "propagates not found for unknown track",
		Given:     "no stored tracks",
		When:      "Delete is called with track-1",
		Then:      "not_found \"track not found\" is returned",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var err error
		r.Arrange(func(t *testing.T) { service, _, _, _ = newTrackFixture() })
		r.Act(func(t *testing.T) { err = service.Delete(context.Background(), testTrackID) })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeNotFound, "track not found") })
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-DELETE-04", Method: method,
		Title:     "returns repository delete error and keeps track",
		Given:     "a stored ready track and a track repository whose Delete fails",
		When:      "Delete is called with track-1",
		Then:      "the repository error is returned unchanged and track-1 remains stored",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var tracks *fakeTrackRepository
		var err error
		r.Arrange(func(t *testing.T) {
			service, tracks, _, _ = newTrackFixture()
			tracks.seed(readyTrack())
			tracks.deleteErr = errDependency
		})
		r.Act(func(t *testing.T) { err = service.Delete(context.Background(), testTrackID) })
		r.Assert(func(t *testing.T) {
			assertErrorIs(t, err, errDependency)
			assertEqual(t, readyTrack(), tracks.tracks[testTrackID])
		})
	})
}

func TestTrackServiceInitUpload(t *testing.T) {
	const method = "InitUpload"

	for _, tc := range []struct {
		id, title, given, then, technique string
		size                              int64
	}{
		{"UC-TRK-INIT-01", "creates pending track and presigns upload", "a valid upload init of 100 bytes with max size 1000", "a session with the pending track track-1 and the presigned put URL is returned, and the track is stored", testkit.TechniqueEquivalence, 100},
		{"UC-TRK-INIT-02", "accepts size exactly at max", "a valid upload init of exactly 1000 bytes with max size 1000", "a session with pending track track-1 of 1000 bytes is returned and the track is stored", testkit.TechniqueBoundary, testMaxSize},
		{"UC-TRK-INIT-03", "accepts minimum size of 1 byte", "a valid upload init of 1 byte", "a session with pending track track-1 of 1 byte is returned and the track is stored", testkit.TechniqueBoundary, 1},
	} {
		runSpec(t, trackComponent, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     tc.given,
			When:      "InitUpload is called",
			Then:      tc.then,
			Technique: tc.technique,
			Severity:  testkit.SeverityCritical,
			Params:    map[string]string{"size": itoa(tc.size), "max": itoa(testMaxSize)},
		}, func(t *testing.T, r testkit.Report) {
			var service *TrackService
			var tracks *fakeTrackRepository
			var objects *fakeObjectStorage
			var in domain.TrackUploadInit
			var wantTrack domain.Track
			var got domain.TrackUploadSession
			var err error
			r.Arrange(func(t *testing.T) {
				service, tracks, _, objects = newTrackFixture()
				in = validTrackUploadInit()
				in.SizeBytes = tc.size
				wantTrack = in.Track()
				wantTrack.ID = testTrackID
			})
			r.Act(func(t *testing.T) { got, err = service.InitUpload(context.Background(), in) })
			r.Assert(func(t *testing.T) {
				assertNoError(t, err)
				assertEqual(t, domain.TrackUploadSession{Track: wantTrack, UploadURL: objects.putURL}, got)
				assertEqual(t, domain.TrackPending, got.Track.Status)
				assertEqual(t, map[string]domain.Track{testTrackID: wantTrack}, tracks.tracks)
			})
		})
	}

	for _, tc := range []struct {
		id, title, given, then, technique, message string
		in                                         domain.TrackUploadInit
	}{
		{
			id: "UC-TRK-INIT-04", title: "rejects size max+1 without creating track",
			given: "an upload init of 1001 bytes with max size 1000", then: "invalid \"file too large\" is returned and no track is created",
			technique: testkit.TechniqueBoundary, message: "file too large",
			in: domain.TrackUploadInit{UserID: testUserID, Title: "Track", SizeBytes: testMaxSize + 1},
		},
		{
			id: "UC-TRK-INIT-05", title: "rejects zero size without creating track",
			given: "an upload init of 0 bytes", then: "invalid \"size is required\" is returned and no track is created",
			technique: testkit.TechniqueBoundary, message: "size is required",
			in: domain.TrackUploadInit{UserID: testUserID, Title: "Track", SizeBytes: 0},
		},
		{
			id: "UC-TRK-INIT-06", title: "reports missing user id first for empty input",
			given: "an upload init with no user, title or size", then: "invalid \"user id is required\" is returned and no track is created",
			technique: testkit.TechniqueDecisionTable, message: "user id is required",
		},
		{
			id: "UC-TRK-INIT-07", title: "reports missing title before oversize",
			given: "an upload init without title and of 1001 bytes", then: "invalid \"title is required\" is returned and no track is created",
			technique: testkit.TechniqueDecisionTable, message: "title is required",
			in: domain.TrackUploadInit{UserID: testUserID, SizeBytes: testMaxSize + 1},
		},
	} {
		runSpec(t, trackComponent, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     tc.given,
			When:      "InitUpload is called",
			Then:      tc.then,
			Technique: tc.technique,
			Severity:  testkit.SeverityCritical,
			Params:    map[string]string{"user": tc.in.UserID, "title": tc.in.Title, "size": itoa(tc.in.SizeBytes), "max": itoa(testMaxSize)},
		}, func(t *testing.T, r testkit.Report) {
			var service *TrackService
			var tracks *fakeTrackRepository
			var got domain.TrackUploadSession
			var err error
			r.Arrange(func(t *testing.T) { service, tracks, _, _ = newTrackFixture() })
			r.Act(func(t *testing.T) { got, err = service.InitUpload(context.Background(), tc.in) })
			r.Assert(func(t *testing.T) {
				assertDomainError(t, err, domain.CodeInvalid, tc.message)
				assertEqual(t, domain.TrackUploadSession{}, got)
				assertEqual(t, 0, len(tracks.tracks))
			})
		})
	}

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-INIT-08", Method: method,
		Title:     "returns create error without session",
		Given:     "a valid upload init and a track repository whose Create fails",
		When:      "InitUpload is called",
		Then:      "the repository error is returned unchanged, the session is empty and no track is stored",
		Technique: testkit.TechniqueErrorGuessing,
		Severity:  testkit.SeverityCritical,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var tracks *fakeTrackRepository
		var got domain.TrackUploadSession
		var err error
		r.Arrange(func(t *testing.T) {
			service, tracks, _, _ = newTrackFixture()
			tracks.createErr = errDependency
		})
		r.Act(func(t *testing.T) { got, err = service.InitUpload(context.Background(), validTrackUploadInit()) })
		r.Assert(func(t *testing.T) {
			assertErrorIs(t, err, errDependency)
			assertEqual(t, domain.TrackUploadSession{}, got)
			assertEqual(t, 0, len(tracks.tracks))
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-INIT-09", Method: method,
		Title:     "deletes created track when presigning fails",
		Given:     "a valid upload init and an object storage whose PresignPut fails",
		When:      "InitUpload is called",
		Then:      "the presign error is returned unchanged, the session is empty and the created track is deleted (rollback)",
		Technique: testkit.TechniqueErrorGuessing,
		Severity:  testkit.SeverityCritical,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var tracks *fakeTrackRepository
		var got domain.TrackUploadSession
		var err error
		r.Arrange(func(t *testing.T) {
			var objects *fakeObjectStorage
			service, tracks, _, objects = newTrackFixture()
			objects.putErr = errDependency
		})
		r.Act(func(t *testing.T) { got, err = service.InitUpload(context.Background(), validTrackUploadInit()) })
		r.Assert(func(t *testing.T) {
			assertErrorIs(t, err, errDependency)
			assertEqual(t, domain.TrackUploadSession{}, got)
			assertEqual(t, 0, len(tracks.tracks))
		})
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-INIT-10", Method: method,
		Title:     "returns rollback error and drops presign error when cleanup fails",
		Given:     "an object storage whose PresignPut fails and a track repository whose Delete fails with internal \"rollback failed\"",
		When:      "InitUpload is called with a valid input",
		Then:      "internal \"rollback failed\" is returned without wrapping the presign error, and the pending track remains stored",
		Technique: testkit.TechniqueErrorGuessing,
		Severity:  testkit.SeverityCritical,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var tracks *fakeTrackRepository
		var err error
		r.Arrange(func(t *testing.T) {
			var objects *fakeObjectStorage
			service, tracks, _, objects = newTrackFixture()
			objects.putErr = errDependency
			tracks.deleteErr = domain.Internal("rollback failed")
		})
		r.Act(func(t *testing.T) { _, err = service.InitUpload(context.Background(), validTrackUploadInit()) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInternal, "rollback failed")
			assertEqual(t, false, errors.Is(err, errDependency))
			assertEqual(t, domain.TrackPending, tracks.tracks[testTrackID].Status)
		})
	})
}

func TestTrackServiceCompleteUpload(t *testing.T) {
	const method = "CompleteUpload"

	for _, tc := range []struct {
		id, title, given, then string
		status                 domain.TrackStatus
	}{
		{"UC-TRK-COMPLETE-01", "publishes pending track after verifying upload", "a pending track of 100 bytes owned by user-1 and an uploaded object of 100 bytes", "the track is returned and stored with status ready", domain.TrackPending},
		{"UC-TRK-COMPLETE-02", "resumes processing track to ready", "a track left in processing by an earlier attempt and an uploaded object of matching size", "the track is returned and stored with status ready (processing to processing is allowed)", domain.TrackProcessing},
	} {
		runSpec(t, trackComponent, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     tc.given,
			When:      "CompleteUpload is called by user-1 for track-1",
			Then:      tc.then,
			Technique: testkit.TechniqueState,
			Severity:  testkit.SeverityCritical,
			Params:    map[string]string{"status": string(tc.status)},
		}, func(t *testing.T, r testkit.Report) {
			var service *TrackService
			var tracks *fakeTrackRepository
			var want, got domain.Track
			var err error
			r.Arrange(func(t *testing.T) {
				var objects *fakeObjectStorage
				service, tracks, _, objects = newTrackFixture()
				track := pendingTrack()
				track.Status = tc.status
				tracks.seed(track)
				objects.sizes[track.AudioObjectKey()] = track.SizeBytes
				want = track
				want.Status = domain.TrackReady
			})
			r.Act(func(t *testing.T) { got, err = service.CompleteUpload(context.Background(), validUploadComplete()) })
			r.Assert(func(t *testing.T) {
				assertNoError(t, err)
				assertEqual(t, want, got)
				assertEqual(t, want, tracks.tracks[testTrackID])
			})
		})
	}

	type completeArrange func(*fakeTrackRepository, *fakeObjectStorage) domain.TrackUploadComplete

	seedUploaded := func(status domain.TrackStatus, uploaded int64) completeArrange {
		return func(tracks *fakeTrackRepository, objects *fakeObjectStorage) domain.TrackUploadComplete {
			track := pendingTrack()
			track.Status = status
			tracks.seed(track)
			objects.sizes[track.AudioObjectKey()] = uploaded
			return validUploadComplete()
		}
	}

	for _, tc := range []struct {
		id, title, given, then, technique, severity string
		arrange                                     completeArrange
		wantErr                                     error
		wantCode                                    domain.Code
		wantMsg                                     string
		wantStatus                                  domain.TrackStatus
	}{
		{
			id: "UC-TRK-COMPLETE-03", title: "reports missing track id first for empty input",
			given: "a stored pending track and an input with no track id and no user id",
			then:  "invalid \"track id is required\" is returned and the track stays pending",
			arrange: func(tracks *fakeTrackRepository, _ *fakeObjectStorage) domain.TrackUploadComplete {
				tracks.seed(pendingTrack())
				return domain.TrackUploadComplete{}
			},
			technique: testkit.TechniqueDecisionTable, wantCode: domain.CodeInvalid, wantMsg: "track id is required", wantStatus: domain.TrackPending,
		},
		{
			id: "UC-TRK-COMPLETE-04", title: "rejects missing user id",
			given: "a stored pending track and an input with track-1 and no user id",
			then:  "invalid \"user id is required\" is returned and the track stays pending",
			arrange: func(tracks *fakeTrackRepository, _ *fakeObjectStorage) domain.TrackUploadComplete {
				tracks.seed(pendingTrack())
				return domain.TrackUploadComplete{TrackID: testTrackID}
			},
			technique: testkit.TechniqueEquivalence, wantCode: domain.CodeInvalid, wantMsg: "user id is required", wantStatus: domain.TrackPending,
		},
		{
			id: "UC-TRK-COMPLETE-05", title: "returns track lookup error",
			given: "a track repository whose GetByID fails",
			then:  "the lookup error is returned unchanged",
			arrange: func(tracks *fakeTrackRepository, _ *fakeObjectStorage) domain.TrackUploadComplete {
				tracks.getErr = errDependency
				return validUploadComplete()
			},
			technique: testkit.TechniqueErrorGuessing, wantErr: errDependency,
		},
		{
			id: "UC-TRK-COMPLETE-06", title: "propagates not found for unknown track",
			given: "no stored tracks",
			then:  "not_found \"track not found\" is returned",
			arrange: func(*fakeTrackRepository, *fakeObjectStorage) domain.TrackUploadComplete {
				return validUploadComplete()
			},
			technique: testkit.TechniqueEquivalence, wantCode: domain.CodeNotFound, wantMsg: "track not found",
		},
		{
			id: "UC-TRK-COMPLETE-07", title: "rejects another user's track before checking storage",
			given: "a pending track owned by user-1 and an object storage whose Stat is set to fail",
			then:  "unauthorized \"not allowed\" is returned instead of the storage error and the track stays pending",
			arrange: func(tracks *fakeTrackRepository, objects *fakeObjectStorage) domain.TrackUploadComplete {
				tracks.seed(pendingTrack())
				objects.statErrors = []error{errDependency}
				return domain.TrackUploadComplete{TrackID: testTrackID, UserID: "user-2"}
			},
			technique: testkit.TechniqueDecisionTable, severity: testkit.SeverityCritical,
			wantCode: domain.CodeUnauthorized, wantMsg: "not allowed", wantStatus: domain.TrackPending,
		},
		{
			id: "UC-TRK-COMPLETE-08", title: "returns initial object stat error",
			given: "a pending track and an object storage whose first Stat fails",
			then:  "the storage error is returned unchanged and the track stays pending",
			arrange: func(tracks *fakeTrackRepository, objects *fakeObjectStorage) domain.TrackUploadComplete {
				tracks.seed(pendingTrack())
				objects.statErrors = []error{errDependency}
				return validUploadComplete()
			},
			technique: testkit.TechniqueErrorGuessing, wantErr: errDependency, wantStatus: domain.TrackPending,
		},
		{
			id: "UC-TRK-COMPLETE-09", title: "rejects missing uploaded object",
			given:     "a pending track of 100 bytes and no uploaded object (size 0)",
			then:      "invalid \"upload not found\" is returned and the track stays pending",
			arrange:   seedUploaded(domain.TrackPending, 0),
			technique: testkit.TechniqueBoundary, wantCode: domain.CodeInvalid, wantMsg: "upload not found", wantStatus: domain.TrackPending,
		},
		{
			id: "UC-TRK-COMPLETE-10", title: "rejects uploaded size one byte larger than declared",
			given:     "a pending track of 100 bytes and an uploaded object of 101 bytes",
			then:      "invalid \"upload size mismatch: expected 100, got 101\" is returned and the track stays pending",
			arrange:   seedUploaded(domain.TrackPending, 101),
			technique: testkit.TechniqueBoundary, wantCode: domain.CodeInvalid, wantMsg: "upload size mismatch: expected 100, got 101", wantStatus: domain.TrackPending,
		},
		{
			id: "UC-TRK-COMPLETE-11", title: "rejects completing an already ready track",
			given:     "a ready track and an uploaded object of matching size",
			then:      "conflict \"track is not awaiting upload\" is returned and the track stays ready",
			arrange:   seedUploaded(domain.TrackReady, 100),
			technique: testkit.TechniqueState, wantCode: domain.CodeConflict, wantMsg: "track is not awaiting upload", wantStatus: domain.TrackReady,
		},
		{
			id: "UC-TRK-COMPLETE-12", title: "returns processing update error and keeps track pending",
			given: "a pending track, a matching uploaded object and a track repository whose first Update fails",
			then:  "the repository error is returned unchanged and the track stays pending",
			arrange: func(tracks *fakeTrackRepository, objects *fakeObjectStorage) domain.TrackUploadComplete {
				in := seedUploaded(domain.TrackPending, 100)(tracks, objects)
				tracks.updateErrors = []error{errDependency}
				return in
			},
			technique: testkit.TechniqueErrorGuessing, severity: testkit.SeverityCritical,
			wantErr: errDependency, wantStatus: domain.TrackPending,
		},
		{
			id: "UC-TRK-COMPLETE-13", title: "returns publish stat error and leaves track processing",
			given: "a pending track, a matching uploaded object and an object storage whose second Stat fails",
			then:  "the storage error is returned unchanged and the track is left in processing",
			arrange: func(tracks *fakeTrackRepository, objects *fakeObjectStorage) domain.TrackUploadComplete {
				in := seedUploaded(domain.TrackPending, 100)(tracks, objects)
				objects.statErrors = []error{nil, errDependency}
				return in
			},
			technique: testkit.TechniqueErrorGuessing, severity: testkit.SeverityCritical,
			wantErr: errDependency, wantStatus: domain.TrackProcessing,
		},
		{
			id: "UC-TRK-COMPLETE-14", title: "returns ready update error and leaves track processing",
			given: "a pending track, a matching uploaded object and a track repository whose second Update fails",
			then:  "the repository error is returned unchanged and the track is left in processing",
			arrange: func(tracks *fakeTrackRepository, objects *fakeObjectStorage) domain.TrackUploadComplete {
				in := seedUploaded(domain.TrackPending, 100)(tracks, objects)
				tracks.updateErrors = []error{nil, errDependency}
				return in
			},
			technique: testkit.TechniqueErrorGuessing, severity: testkit.SeverityCritical,
			wantErr: errDependency, wantStatus: domain.TrackProcessing,
		},
	} {
		severity := tc.severity
		if severity == "" {
			severity = testkit.SeverityNormal
		}
		runSpec(t, trackComponent, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     tc.given,
			When:      "CompleteUpload is called",
			Then:      tc.then,
			Technique: tc.technique,
			Severity:  severity,
		}, func(t *testing.T, r testkit.Report) {
			var service *TrackService
			var tracks *fakeTrackRepository
			var in domain.TrackUploadComplete
			var got domain.Track
			var err error
			r.Arrange(func(t *testing.T) {
				var objects *fakeObjectStorage
				service, tracks, _, objects = newTrackFixture()
				in = tc.arrange(tracks, objects)
			})
			r.Act(func(t *testing.T) { got, err = service.CompleteUpload(context.Background(), in) })
			r.Assert(func(t *testing.T) {
				assertFailure(t, err, tc.wantErr, tc.wantCode, tc.wantMsg)
				assertEqual(t, domain.Track{}, got)
				assertEqual(t, tc.wantStatus, tracks.tracks[testTrackID].Status)
			})
		})
	}
}

func TestTrackServiceStreamURL(t *testing.T) {
	const method = "StreamURL"

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-STREAM-01", Method: method,
		Title:     "presigns stream url for ready track",
		Given:     "a stored ready track track-1",
		When:      "StreamURL is called with track-1",
		Then:      "the presigned get URL is returned",
		Technique: testkit.TechniqueState,
		Params:    map[string]string{"status": string(domain.TrackReady)},
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var objects *fakeObjectStorage
		var got string
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, _, objects = newTrackFixture()
			tracks.seed(readyTrack())
		})
		r.Act(func(t *testing.T) { got, err = service.StreamURL(context.Background(), testTrackID) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, objects.getURL, got)
		})
	})

	for _, tc := range []struct {
		id, title string
		status    domain.TrackStatus
	}{
		{"UC-TRK-STREAM-02", "rejects pending track without presigning", domain.TrackPending},
		{"UC-TRK-STREAM-03", "rejects processing track without presigning", domain.TrackProcessing},
	} {
		runSpec(t, trackComponent, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "a stored " + string(tc.status) + " track and an object storage whose PresignGet is set to fail",
			When:      "StreamURL is called with track-1",
			Then:      "conflict \"track is not ready\" is returned instead of the storage error and the URL is empty",
			Technique: testkit.TechniqueState,
			Params:    map[string]string{"status": string(tc.status)},
		}, func(t *testing.T, r testkit.Report) {
			var service *TrackService
			var got string
			var err error
			r.Arrange(func(t *testing.T) {
				var tracks *fakeTrackRepository
				var objects *fakeObjectStorage
				service, tracks, _, objects = newTrackFixture()
				track := pendingTrack()
				track.Status = tc.status
				tracks.seed(track)
				objects.getErr = errDependency
			})
			r.Act(func(t *testing.T) { got, err = service.StreamURL(context.Background(), testTrackID) })
			r.Assert(func(t *testing.T) {
				assertDomainError(t, err, domain.CodeConflict, "track is not ready")
				assertEqual(t, "", got)
			})
		})
	}

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-STREAM-04", Method: method,
		Title:     "rejects empty id before lookup",
		Given:     "a track repository whose lookup is set to fail",
		When:      "StreamURL is called with an empty id",
		Then:      "invalid \"track id is required\" is returned instead of the repository error",
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"id": ""},
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, _, _ = newTrackFixture()
			tracks.getErr = errDependency
		})
		r.Act(func(t *testing.T) { _, err = service.StreamURL(context.Background(), "") })
		r.Assert(func(t *testing.T) { assertDomainError(t, err, domain.CodeInvalid, "track id is required") })
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-STREAM-05", Method: method,
		Title:     "returns lookup error",
		Given:     "a track repository whose GetByID fails",
		When:      "StreamURL is called with track-1",
		Then:      "the lookup error is returned unchanged",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			service, tracks, _, _ = newTrackFixture()
			tracks.getErr = errDependency
		})
		r.Act(func(t *testing.T) { _, err = service.StreamURL(context.Background(), testTrackID) })
		r.Assert(func(t *testing.T) { assertErrorIs(t, err, errDependency) })
	})

	runSpec(t, trackComponent, testkit.Spec{
		ID: "UC-TRK-STREAM-06", Method: method,
		Title:     "returns presign error",
		Given:     "a stored ready track and an object storage whose PresignGet fails",
		When:      "StreamURL is called with track-1",
		Then:      "the storage error is returned unchanged and the URL is empty",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *TrackService
		var got string
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackRepository
			var objects *fakeObjectStorage
			service, tracks, _, objects = newTrackFixture()
			tracks.seed(readyTrack())
			objects.getErr = errDependency
		})
		r.Act(func(t *testing.T) { got, err = service.StreamURL(context.Background(), testTrackID) })
		r.Assert(func(t *testing.T) {
			assertErrorIs(t, err, errDependency)
			assertEqual(t, "", got)
		})
	})
}
