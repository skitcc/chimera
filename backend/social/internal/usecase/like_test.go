package usecase

import (
	"context"
	"testing"

	"chimera/social/internal/domain"
	"chimera/social/internal/testkit"
)

const likeComponent = "LikeService"

func TestLikeServiceLikeClassic(t *testing.T) {
	const method = "Like"

	runSpec(t, likeComponent, testkit.Spec{
		ID: "UC-TRK-LIKE-01", Method: method,
		Title:     "stores like for ready track",
		Given:     "a ready track track-1",
		When:      "Like is called by user-1 for track-1",
		Then:      "no error is returned and the like user-1/track-1 is stored",
		Technique: testkit.TechniqueState,
		Params:    map[string]string{"status": string(domain.TrackReady)},
	}, func(t *testing.T, r testkit.Report) {
		var service *LikeService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackCatalog
			service, tracks, likes = newLikeFixture()
			tracks.seed(readyTrack())
		})
		r.Act(func(t *testing.T) { err = service.Like(context.Background(), validTrackLike()) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, map[string]map[string]bool{testUserID: {testTrackID: true}}, likes.likes)
		})
	})

	cases := []struct {
		id, title, given, then, technique string
		status                            domain.TrackStatus
	}{
		{"UC-TRK-LIKE-02", "rejects pending track without storing like", "a pending track track-1", "conflict \"track is not ready\" is returned and no like is stored", testkit.TechniqueState, domain.TrackPending},
		{"UC-TRK-LIKE-03", "rejects processing track without storing like", "a processing track track-1", "conflict \"track is not ready\" is returned and no like is stored", testkit.TechniqueState, domain.TrackProcessing},
	}

	for _, tc := range cases {
		runSpec(t, likeComponent, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     tc.given,
			When:      "Like is called by user-1 for track-1",
			Then:      tc.then,
			Technique: tc.technique,
			Params:    map[string]string{"status": string(tc.status)},
		}, func(t *testing.T, r testkit.Report) {
			var service *LikeService
			var likes *fakeTrackLikeRepository
			var err error
			r.Arrange(func(t *testing.T) {
				var tracks *fakeTrackCatalog
				service, tracks, likes = newLikeFixture()
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

	runSpec(t, likeComponent, testkit.Spec{
		ID: "UC-TRK-LIKE-04", Method: method,
		Title:     "reports missing user id before track lookup",
		Given:     "a track catalog whose lookup is set to fail",
		When:      "Like is called with an empty user id and empty track id",
		Then:      "invalid \"user id is required\" is returned instead of the lookup error and no like is stored",
		Technique: testkit.TechniqueDecisionTable,
		Params:    map[string]string{"user": "", "track": ""},
	}, func(t *testing.T, r testkit.Report) {
		var service *LikeService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackCatalog
			service, tracks, likes = newLikeFixture()
			tracks.getErr = errDependency
		})
		r.Act(func(t *testing.T) { err = service.Like(context.Background(), domain.TrackLike{}) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeInvalid, "user id is required")
			assertEqual(t, 0, len(likes.likes))
		})
	})

	runSpec(t, likeComponent, testkit.Spec{
		ID: "UC-TRK-LIKE-05", Method: method,
		Title:     "propagates not found for unknown track without storing like",
		Given:     "no stored tracks",
		When:      "Like is called by user-1 for track-1",
		Then:      "not_found \"track not found\" is returned and no like is stored",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var service *LikeService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) { service, _, likes = newLikeFixture() })
		r.Act(func(t *testing.T) { err = service.Like(context.Background(), validTrackLike()) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeNotFound, "track not found")
			assertEqual(t, 0, len(likes.likes))
		})
	})

	runSpec(t, likeComponent, testkit.Spec{
		ID: "UC-TRK-LIKE-06", Method: method,
		Title:     "returns track lookup error without storing like",
		Given:     "a track catalog whose GetTrack fails",
		When:      "Like is called by user-1 for track-1",
		Then:      "the lookup error is returned unchanged and no like is stored",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *LikeService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackCatalog
			service, tracks, likes = newLikeFixture()
			tracks.getErr = errDependency
		})
		r.Act(func(t *testing.T) { err = service.Like(context.Background(), validTrackLike()) })
		r.Assert(func(t *testing.T) {
			assertErrorIs(t, err, errDependency)
			assertEqual(t, 0, len(likes.likes))
		})
	})

	runSpec(t, likeComponent, testkit.Spec{
		ID: "UC-TRK-LIKE-07", Method: method,
		Title:     "returns add like error",
		Given:     "a ready track and a like repository whose Add fails",
		When:      "Like is called by user-1 for track-1",
		Then:      "the like repository error is returned unchanged and no like is stored",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *LikeService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackCatalog
			service, tracks, likes = newLikeFixture()
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

func TestLikeServiceLikeLondon(t *testing.T) {
	const method = "Like"

	newLondonService := func(tracks *mockTrackCatalog, likes *spyTrackLikeRepository) *LikeService {
		return NewLikeService(tracks, likes)
	}

	runSpec(t, likeComponent, testkit.Spec{
		ID: "UC-TRK-LIKE-08", Method: method,
		Title:     "looks up exact track then adds exact like once",
		Given:     "a track catalog mock returning a ready track and a like repository spy",
		When:      "Like is called by user-1 for track-1",
		Then:      "GetTrack is called once with track-1 and Add is called once with user-1/track-1",
		Technique: testkit.TechniqueEquivalence,
		Kind:      testkit.KindLondon,
	}, func(t *testing.T, r testkit.Report) {
		var service *LikeService
		var tracks *mockTrackCatalog
		var likes *spyTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			tracks = &mockTrackCatalog{getResult: readyTrack()}
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

	runSpec(t, likeComponent, testkit.Spec{
		ID: "UC-TRK-LIKE-09", Method: method,
		Title:     "calls no collaborator for invalid input",
		Given:     "a track catalog mock and a like repository spy",
		When:      "Like is called with an empty input",
		Then:      "invalid \"user id is required\" is returned, GetTrack is not called and Add is not called",
		Technique: testkit.TechniqueEquivalence,
		Kind:      testkit.KindLondon,
	}, func(t *testing.T, r testkit.Report) {
		var service *LikeService
		var tracks *mockTrackCatalog
		var likes *spyTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			tracks = &mockTrackCatalog{}
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

	runSpec(t, likeComponent, testkit.Spec{
		ID: "UC-TRK-LIKE-10", Method: method,
		Title:     "does not call add when lookup fails",
		Given:     "a track catalog mock whose GetTrack fails and a like repository spy",
		When:      "Like is called by user-1 for track-1",
		Then:      "the lookup error is returned, GetTrack is called once with track-1 and Add is not called",
		Technique: testkit.TechniqueErrorGuessing,
		Kind:      testkit.KindLondon,
	}, func(t *testing.T, r testkit.Report) {
		var service *LikeService
		var tracks *mockTrackCatalog
		var likes *spyTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			tracks = &mockTrackCatalog{getErr: errDependency}
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

	runSpec(t, likeComponent, testkit.Spec{
		ID: "UC-TRK-LIKE-11", Method: method,
		Title:     "does not call add for pending track",
		Given:     "a track catalog mock returning a pending track and a like repository spy",
		When:      "Like is called by user-1 for track-1",
		Then:      "conflict \"track is not ready\" is returned, GetTrack is called once and Add is not called",
		Technique: testkit.TechniqueState,
		Kind:      testkit.KindLondon,
	}, func(t *testing.T, r testkit.Report) {
		var service *LikeService
		var tracks *mockTrackCatalog
		var likes *spyTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			tracks = &mockTrackCatalog{getResult: pendingTrack()}
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

func TestLikeServiceUnlike(t *testing.T) {
	const method = "Unlike"

	runSpec(t, likeComponent, testkit.Spec{
		ID: "UC-TRK-UNLIKE-01", Method: method,
		Title:     "removes only the requested like",
		Given:     "user-1 likes ready tracks track-1 and track-2",
		When:      "Unlike is called by user-1 for track-1",
		Then:      "no error is returned, the track-1 like is removed and the track-2 like remains",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var service *LikeService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackCatalog
			service, tracks, likes = newLikeFixture()
			tracks.seed(readyTrack(), readyTrackWithID("track-2"))
			likes.likes[testUserID] = map[string]bool{testTrackID: true, "track-2": true}
		})
		r.Act(func(t *testing.T) { err = service.Unlike(context.Background(), validTrackLike()) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, map[string]bool{"track-2": true}, likes.likes[testUserID])
		})
	})

	runSpec(t, likeComponent, testkit.Spec{
		ID: "UC-TRK-UNLIKE-02", Method: method,
		Title:     "succeeds idempotently when track is not liked",
		Given:     "a ready track track-1 that user-1 has not liked",
		When:      "Unlike is called by user-1 for track-1",
		Then:      "no error is returned (Remove is a no-op) and no like is stored",
		Technique: testkit.TechniqueState,
	}, func(t *testing.T, r testkit.Report) {
		var service *LikeService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackCatalog
			service, tracks, likes = newLikeFixture()
			tracks.seed(readyTrack())
		})
		r.Act(func(t *testing.T) { err = service.Unlike(context.Background(), validTrackLike()) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, 0, len(likes.likes[testUserID]))
		})
	})

	runSpec(t, likeComponent, testkit.Spec{
		ID: "UC-TRK-UNLIKE-03", Method: method,
		Title:     "removes like of pending track without ready check",
		Given:     "user-1 likes track-1 which is pending",
		When:      "Unlike is called by user-1 for track-1",
		Then:      "no error is returned and the like is removed, unlike Like which requires ready status",
		Technique: testkit.TechniqueState,
		Params:    map[string]string{"status": string(domain.TrackPending)},
	}, func(t *testing.T, r testkit.Report) {
		var service *LikeService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackCatalog
			service, tracks, likes = newLikeFixture()
			tracks.seed(pendingTrack())
			likes.likes[testUserID] = map[string]bool{testTrackID: true}
		})
		r.Act(func(t *testing.T) { err = service.Unlike(context.Background(), validTrackLike()) })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, 0, len(likes.likes[testUserID]))
		})
	})

	runSpec(t, likeComponent, testkit.Spec{
		ID: "UC-TRK-UNLIKE-04", Method: method,
		Title:     "rejects missing track id without removing like",
		Given:     "user-1 likes ready track track-1",
		When:      "Unlike is called by user-1 with an empty track id",
		Then:      "invalid \"track id is required\" is returned and the existing like remains",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"track": ""},
	}, func(t *testing.T, r testkit.Report) {
		var service *LikeService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackCatalog
			service, tracks, likes = newLikeFixture()
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

	runSpec(t, likeComponent, testkit.Spec{
		ID: "UC-TRK-UNLIKE-05", Method: method,
		Title:     "propagates not found for unknown track without removing like",
		Given:     "a stale like user-1/track-1 whose track no longer exists",
		When:      "Unlike is called by user-1 for track-1",
		Then:      "not_found \"track not found\" is returned and the stale like remains",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var service *LikeService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			service, _, likes = newLikeFixture()
			likes.likes[testUserID] = map[string]bool{testTrackID: true}
		})
		r.Act(func(t *testing.T) { err = service.Unlike(context.Background(), validTrackLike()) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeNotFound, "track not found")
			assertEqual(t, map[string]bool{testTrackID: true}, likes.likes[testUserID])
		})
	})

	runSpec(t, likeComponent, testkit.Spec{
		ID: "UC-TRK-UNLIKE-06", Method: method,
		Title:     "returns track lookup error without removing like",
		Given:     "an existing like and a track catalog whose GetTrack fails",
		When:      "Unlike is called by user-1 for track-1",
		Then:      "the lookup error is returned unchanged and the like remains",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *LikeService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackCatalog
			service, tracks, likes = newLikeFixture()
			likes.likes[testUserID] = map[string]bool{testTrackID: true}
			tracks.getErr = errDependency
		})
		r.Act(func(t *testing.T) { err = service.Unlike(context.Background(), validTrackLike()) })
		r.Assert(func(t *testing.T) {
			assertErrorIs(t, err, errDependency)
			assertEqual(t, map[string]bool{testTrackID: true}, likes.likes[testUserID])
		})
	})

	runSpec(t, likeComponent, testkit.Spec{
		ID: "UC-TRK-UNLIKE-07", Method: method,
		Title:     "returns remove like error",
		Given:     "an existing like on a ready track and a like repository whose Remove fails",
		When:      "Unlike is called by user-1 for track-1",
		Then:      "the like repository error is returned unchanged and the like remains",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *LikeService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackCatalog
			service, tracks, likes = newLikeFixture()
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


func TestLikeServiceListLiked(t *testing.T) {
	const method = "ListLiked"

	runSpec(t, likeComponent, testkit.Spec{
		ID: "UC-LIK-LIST-01", Method: method,
		Title:     "passes liked ids to the catalog with the viewer filters",
		Given:     "user-1 likes a ready track-1 and a pending track-2",
		When:      "ListLiked is called by user-1 for user-1 with status ready and limit 10",
		Then:      "the catalog receives track-1 and track-2 in like order and the page holds only track-1",
		Technique: testkit.TechniqueState,
	}, func(t *testing.T, r testkit.Report) {
		var service *LikeService
		var tracks *fakeTrackCatalog
		var got domain.TrackPage
		var err error
		r.Arrange(func(t *testing.T) {
			var likes *fakeTrackLikeRepository
			service, tracks, likes = newLikeFixture()
			pending := readyTrackWithID("track-2")
			pending.Status = domain.TrackPending
			tracks.seed(readyTrack(), pending)
			assertNoError(t, likes.Add(context.Background(), testUserID, testTrackID))
			assertNoError(t, likes.Add(context.Background(), testUserID, "track-2"))
		})
		r.Act(func(t *testing.T) {
			got, err = service.ListLiked(context.Background(), domain.LikedTracksQuery{
				Page:     domain.PageQuery{Limit: 10},
				ViewerID: testUserID,
				LikedBy:  testUserID,
				Status:   domain.TrackReady,
			})
		})
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, []string{testTrackID, "track-2"}, tracks.listIDs)
			assertEqual(t, domain.TrackPage{Items: []domain.Track{readyTrack()}, Limit: 10}, got)
		})
	})

	cases := []struct {
		id, title, viewer, likedBy string
		code                       domain.Code
		message                    string
	}{
		{"UC-LIK-LIST-02", "rejects anonymous viewer before reading likes", "", testUserID, domain.CodeUnauthorized, "missing token"},
		{"UC-LIK-LIST-03", "forbids reading likes of another user", "user-2", testUserID, domain.CodeForbidden, "not allowed"},
	}
	for _, tc := range cases {
		runSpec(t, likeComponent, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "a like repository whose listing is set to fail",
			When:      "ListLiked is called by viewer " + tc.viewer + " for " + tc.likedBy,
			Then:      string(tc.code) + " \"" + tc.message + "\" is returned instead of the repository error",
			Technique: testkit.TechniqueDecisionTable,
			Params:    map[string]string{"viewer": tc.viewer, "liked by": tc.likedBy},
		}, func(t *testing.T, r testkit.Report) {
			var service *LikeService
			var err error
			r.Arrange(func(t *testing.T) {
				var likes *fakeTrackLikeRepository
				service, _, likes = newLikeFixture()
				likes.listErr = errDependency
			})
			r.Act(func(t *testing.T) {
				_, err = service.ListLiked(context.Background(), domain.LikedTracksQuery{ViewerID: tc.viewer, LikedBy: tc.likedBy})
			})
			r.Assert(func(t *testing.T) { assertDomainError(t, err, tc.code, tc.message) })
		})
	}

	runSpec(t, likeComponent, testkit.Spec{
		ID: "UC-LIK-LIST-04", Method: method,
		Title:     "returns like repository error without calling the catalog",
		Given:     "a like repository whose ListTrackIDs fails",
		When:      "ListLiked is called by user-1 for user-1",
		Then:      "the repository error is returned unchanged and the catalog is not asked",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *LikeService
		var tracks *fakeTrackCatalog
		var err error
		r.Arrange(func(t *testing.T) {
			var likes *fakeTrackLikeRepository
			service, tracks, likes = newLikeFixture()
			likes.listErr = errDependency
		})
		r.Act(func(t *testing.T) {
			_, err = service.ListLiked(context.Background(), domain.LikedTracksQuery{ViewerID: testUserID, LikedBy: testUserID})
		})
		r.Assert(func(t *testing.T) {
			assertErrorIs(t, err, errDependency)
			assertEqual(t, []string(nil), tracks.listIDs)
		})
	})

	runSpec(t, likeComponent, testkit.Spec{
		ID: "UC-LIK-LIST-05", Method: method,
		Title:     "returns catalog error",
		Given:     "user-1 likes track-1 and a catalog whose ListByIDs fails",
		When:      "ListLiked is called by user-1 for user-1",
		Then:      "the catalog error is returned unchanged with an empty page",
		Technique: testkit.TechniqueErrorGuessing,
	}, func(t *testing.T, r testkit.Report) {
		var service *LikeService
		var got domain.TrackPage
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackCatalog
			var likes *fakeTrackLikeRepository
			service, tracks, likes = newLikeFixture()
			tracks.listErr = errDependency
			assertNoError(t, likes.Add(context.Background(), testUserID, testTrackID))
		})
		r.Act(func(t *testing.T) {
			got, err = service.ListLiked(context.Background(), domain.LikedTracksQuery{ViewerID: testUserID, LikedBy: testUserID})
		})
		r.Assert(func(t *testing.T) {
			assertErrorIs(t, err, errDependency)
			assertEqual(t, domain.TrackPage{}, got)
		})
	})
}

func TestLikeServicePutLike(t *testing.T) {
	const method = "PutLike"

	runSpec(t, likeComponent, testkit.Spec{
		ID: "UC-LIK-PUT-01", Method: method,
		Title:     "treats an existing like as success",
		Given:     "a ready track and a like repository whose Add reports conflict",
		When:      "PutLike is called by user-1 for user-1/track-1",
		Then:      "no error is returned",
		Technique: testkit.TechniqueState,
	}, func(t *testing.T, r testkit.Report) {
		var service *LikeService
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackCatalog
			var likes *fakeTrackLikeRepository
			service, tracks, likes = newLikeFixture()
			tracks.seed(readyTrack())
			likes.addErr = domain.Conflict("track already liked")
		})
		r.Act(func(t *testing.T) { err = service.PutLike(context.Background(), testUserID, validTrackLike()) })
		r.Assert(func(t *testing.T) { assertNoError(t, err) })
	})

	runSpec(t, likeComponent, testkit.Spec{
		ID: "UC-LIK-PUT-02", Method: method,
		Title:     "forbids liking on behalf of another user",
		Given:     "a ready track track-1",
		When:      "PutLike is called by user-2 for user-1/track-1",
		Then:      "forbidden \"not allowed\" is returned and no like is stored",
		Technique: testkit.TechniqueDecisionTable,
	}, func(t *testing.T, r testkit.Report) {
		var service *LikeService
		var likes *fakeTrackLikeRepository
		var err error
		r.Arrange(func(t *testing.T) {
			var tracks *fakeTrackCatalog
			service, tracks, likes = newLikeFixture()
			tracks.seed(readyTrack())
		})
		r.Act(func(t *testing.T) { err = service.PutLike(context.Background(), "user-2", validTrackLike()) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, err, domain.CodeForbidden, "not allowed")
			assertEqual(t, 0, len(likes.likes))
		})
	})
}
