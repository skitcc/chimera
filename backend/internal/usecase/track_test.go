package usecase

import (
	"context"
	"testing"

	"chimera/internal/domain"
)

func TestTrackServiceList(t *testing.T) {
	runCase(t, "returns filtered page", func(t *testing.T) {
		// Arrange
		service, tracks, _, _ := newTrackFixture()
		match := readyTrack()
		other := pendingTrack()
		other.ID = "track-2"
		tracks.seed(match, other)
		query := domain.TrackFeedQuery{
			PageQuery: domain.PageQuery{Limit: 1},
			Artist:    " Artist ",
		}

		// Act
		got, err := service.List(context.Background(), query)

		// Assert
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		assertEqual(t, domain.TrackPage{Items: []domain.Track{match}, Limit: 1}, got)
	})

	runCase(t, "rejects invalid pagination", func(t *testing.T) {
		// Arrange
		service, _, _, _ := newTrackFixture()
		query := domain.TrackFeedQuery{PageQuery: domain.PageQuery{Limit: 101}}

		// Act
		_, err := service.List(context.Background(), query)

		// Assert
		assertErrorCode(t, err, domain.CodeInvalid)
	})

	runCase(t, "returns repository error", func(t *testing.T) {
		// Arrange
		service, tracks, _, _ := newTrackFixture()
		tracks.listErr = errDependency

		// Act
		_, err := service.List(context.Background(), domain.TrackFeedQuery{})

		// Assert
		assertErrorIs(t, err, errDependency)
	})
}

func TestTrackServiceListByUploader(t *testing.T) {
	runCase(t, "returns uploader tracks", func(t *testing.T) {
		// Arrange
		service, tracks, _, _ := newTrackFixture()
		match := readyTrack()
		other := readyTrack()
		other.ID = "track-2"
		other.UserID = "user-2"
		tracks.seed(match, other)
		query := domain.TrackOwnerQuery{
			PageQuery: domain.PageQuery{Limit: 10},
			UserID:    testUserID,
			Status:    domain.TrackReady,
		}

		// Act
		got, err := service.ListByUploader(context.Background(), query)

		// Assert
		if err != nil {
			t.Fatalf("ListByUploader() error = %v", err)
		}
		assertEqual(t, domain.TrackPage{Items: []domain.Track{match}, Limit: 10}, got)
	})

	runCase(t, "rejects missing uploader", func(t *testing.T) {
		// Arrange
		service, _, _, _ := newTrackFixture()
		query := domain.TrackOwnerQuery{PageQuery: domain.PageQuery{Limit: 10}}

		// Act
		_, err := service.ListByUploader(context.Background(), query)

		// Assert
		assertErrorCode(t, err, domain.CodeInvalid)
	})

	runCase(t, "returns repository error", func(t *testing.T) {
		// Arrange
		service, tracks, _, _ := newTrackFixture()
		tracks.listErr = errDependency
		query := domain.TrackOwnerQuery{UserID: testUserID}

		// Act
		_, err := service.ListByUploader(context.Background(), query)

		// Assert
		assertErrorIs(t, err, errDependency)
	})
}

func TestTrackServiceListLiked(t *testing.T) {
	runCase(t, "returns liked ready tracks", func(t *testing.T) {
		// Arrange
		service, tracks, likes, _ := newTrackFixture()
		liked := readyTrack()
		pending := pendingTrack()
		pending.ID = "track-2"
		tracks.seed(liked, pending)
		likes.likes[testUserID] = map[string]bool{
			liked.ID:   true,
			pending.ID: true,
		}
		query := domain.TrackLikeListQuery{
			PageQuery: domain.PageQuery{Limit: 10},
			UserID:    testUserID,
		}

		// Act
		got, err := service.ListLiked(context.Background(), query)

		// Assert
		if err != nil {
			t.Fatalf("ListLiked() error = %v", err)
		}
		assertEqual(t, domain.TrackPage{Items: []domain.Track{liked}, Limit: 10}, got)
	})

	runCase(t, "rejects missing user", func(t *testing.T) {
		// Arrange
		service, _, _, _ := newTrackFixture()

		// Act
		_, err := service.ListLiked(context.Background(), domain.TrackLikeListQuery{})

		// Assert
		assertErrorCode(t, err, domain.CodeInvalid)
	})

	runCase(t, "returns like repository error", func(t *testing.T) {
		// Arrange
		service, _, likes, _ := newTrackFixture()
		likes.listErr = errDependency
		query := domain.TrackLikeListQuery{UserID: testUserID}

		// Act
		_, err := service.ListLiked(context.Background(), query)

		// Assert
		assertErrorIs(t, err, errDependency)
	})
}

func TestTrackServiceLikeClassic(t *testing.T) {
	runCase(t, "adds like for ready track using state", func(t *testing.T) {
		// Arrange
		service, tracks, likes, _ := newTrackFixture()
		tracks.seed(readyTrack())
		in := domain.TrackLike{UserID: testUserID, TrackID: testTrackID}

		// Act
		err := service.Like(context.Background(), in)

		// Assert
		if err != nil {
			t.Fatalf("Like() error = %v", err)
		}
		if !likes.likes[testUserID][testTrackID] {
			t.Fatal("like was not persisted")
		}
	})

	runCase(t, "rejects invalid input", func(t *testing.T) {
		// Arrange
		service, _, _, _ := newTrackFixture()

		// Act
		err := service.Like(context.Background(), domain.TrackLike{})

		// Assert
		assertErrorCode(t, err, domain.CodeInvalid)
	})

	runCase(t, "returns track lookup error", func(t *testing.T) {
		// Arrange
		service, tracks, _, _ := newTrackFixture()
		tracks.getErr = errDependency
		in := domain.TrackLike{UserID: testUserID, TrackID: testTrackID}

		// Act
		err := service.Like(context.Background(), in)

		// Assert
		assertErrorIs(t, err, errDependency)
	})

	runCase(t, "rejects non-ready track", func(t *testing.T) {
		// Arrange
		service, tracks, likes, _ := newTrackFixture()
		tracks.seed(pendingTrack())
		in := domain.TrackLike{UserID: testUserID, TrackID: testTrackID}

		// Act
		err := service.Like(context.Background(), in)

		// Assert
		assertErrorCode(t, err, domain.CodeConflict)
		if likes.likes[testUserID][testTrackID] {
			t.Fatal("like persisted for non-ready track")
		}
	})

	runCase(t, "returns add like error", func(t *testing.T) {
		// Arrange
		service, tracks, likes, _ := newTrackFixture()
		tracks.seed(readyTrack())
		likes.addErr = errDependency
		in := domain.TrackLike{UserID: testUserID, TrackID: testTrackID}

		// Act
		err := service.Like(context.Background(), in)

		// Assert
		assertErrorIs(t, err, errDependency)
	})
}

func TestTrackServiceLikeLondon(t *testing.T) {
	runCase(t, "gets exact track then adds exact like", func(t *testing.T) {
		// Arrange
		tracks := &mockTrackRepository{getResult: readyTrack()}
		likes := &spyTrackLikeRepository{}
		service := NewTrackService(tracks, likes, stubObjectStorage{}, testMaxSize)
		in := domain.TrackLike{UserID: testUserID, TrackID: testTrackID}

		// Act
		err := service.Like(context.Background(), in)

		// Assert
		if err != nil {
			t.Fatalf("Like() error = %v", err)
		}
		assertEqual(t, []string{testTrackID}, tracks.getIDs)
		assertEqual(t, []trackLikeCall{{userID: testUserID, trackID: testTrackID}}, likes.addCalls)
	})

	runCase(t, "does not call collaborators for invalid input", func(t *testing.T) {
		// Arrange
		tracks := &mockTrackRepository{}
		likes := &spyTrackLikeRepository{}
		service := NewTrackService(tracks, likes, stubObjectStorage{}, testMaxSize)

		// Act
		err := service.Like(context.Background(), domain.TrackLike{})

		// Assert
		assertErrorCode(t, err, domain.CodeInvalid)
		assertEqual(t, 0, len(tracks.getIDs))
		assertEqual(t, 0, len(likes.addCalls))
	})

	runCase(t, "does not add like when lookup fails", func(t *testing.T) {
		// Arrange
		tracks := &mockTrackRepository{getErr: errDependency}
		likes := &spyTrackLikeRepository{}
		service := NewTrackService(tracks, likes, stubObjectStorage{}, testMaxSize)
		in := domain.TrackLike{UserID: testUserID, TrackID: testTrackID}

		// Act
		err := service.Like(context.Background(), in)

		// Assert
		assertErrorIs(t, err, errDependency)
		assertEqual(t, []string{testTrackID}, tracks.getIDs)
		assertEqual(t, 0, len(likes.addCalls))
	})

	runCase(t, "does not add like for non-ready track", func(t *testing.T) {
		// Arrange
		tracks := &mockTrackRepository{getResult: pendingTrack()}
		likes := &spyTrackLikeRepository{}
		service := NewTrackService(tracks, likes, stubObjectStorage{}, testMaxSize)
		in := domain.TrackLike{UserID: testUserID, TrackID: testTrackID}

		// Act
		err := service.Like(context.Background(), in)

		// Assert
		assertErrorCode(t, err, domain.CodeConflict)
		assertEqual(t, []string{testTrackID}, tracks.getIDs)
		assertEqual(t, 0, len(likes.addCalls))
	})
}

func TestTrackServiceUnlike(t *testing.T) {
	runCase(t, "removes existing like", func(t *testing.T) {
		// Arrange
		service, tracks, likes, _ := newTrackFixture()
		tracks.seed(readyTrack())
		likes.likes[testUserID] = map[string]bool{testTrackID: true}
		in := domain.TrackLike{UserID: testUserID, TrackID: testTrackID}

		// Act
		err := service.Unlike(context.Background(), in)

		// Assert
		if err != nil {
			t.Fatalf("Unlike() error = %v", err)
		}
		if likes.likes[testUserID][testTrackID] {
			t.Fatal("like remains after unlike")
		}
	})

	runCase(t, "rejects invalid input", func(t *testing.T) {
		// Arrange
		service, _, _, _ := newTrackFixture()

		// Act
		err := service.Unlike(context.Background(), domain.TrackLike{})

		// Assert
		assertErrorCode(t, err, domain.CodeInvalid)
	})

	runCase(t, "returns track lookup error", func(t *testing.T) {
		// Arrange
		service, tracks, _, _ := newTrackFixture()
		tracks.getErr = errDependency
		in := domain.TrackLike{UserID: testUserID, TrackID: testTrackID}

		// Act
		err := service.Unlike(context.Background(), in)

		// Assert
		assertErrorIs(t, err, errDependency)
	})

	runCase(t, "returns remove like error", func(t *testing.T) {
		// Arrange
		service, tracks, likes, _ := newTrackFixture()
		tracks.seed(readyTrack())
		likes.removeErr = errDependency
		in := domain.TrackLike{UserID: testUserID, TrackID: testTrackID}

		// Act
		err := service.Unlike(context.Background(), in)

		// Assert
		assertErrorIs(t, err, errDependency)
	})
}

func TestTrackServiceGetByID(t *testing.T) {
	runCase(t, "returns track", func(t *testing.T) {
		// Arrange
		service, tracks, _, _ := newTrackFixture()
		want := readyTrack()
		tracks.seed(want)

		// Act
		got, err := service.GetByID(context.Background(), testTrackID, "")

		// Assert
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		assertEqual(t, want, got)
	})

	runCase(t, "rejects empty id", func(t *testing.T) {
		// Arrange
		service, _, _, _ := newTrackFixture()

		// Act
		_, err := service.GetByID(context.Background(), "", "")

		// Assert
		assertErrorCode(t, err, domain.CodeInvalid)
	})

	runCase(t, "returns repository error", func(t *testing.T) {
		// Arrange
		service, tracks, _, _ := newTrackFixture()
		tracks.getErr = errDependency

		// Act
		_, err := service.GetByID(context.Background(), testTrackID, "")

		// Assert
		assertErrorIs(t, err, errDependency)
	})

	runCase(t, "hides a draft from a guest", func(t *testing.T) {
		// Arrange
		service, tracks, _, _ := newTrackFixture()
		tracks.seed(pendingTrack())

		// Act
		_, err := service.GetByID(context.Background(), testTrackID, "")

		// Assert
		assertErrorCode(t, err, domain.CodeNotFound)
	})

	runCase(t, "shows a draft to its owner", func(t *testing.T) {
		// Arrange
		service, tracks, _, _ := newTrackFixture()
		want := pendingTrack()
		tracks.seed(want)

		// Act
		got, err := service.GetByID(context.Background(), testTrackID, testUserID)

		// Assert
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		assertEqual(t, want, got)
	})
}

func TestTrackServiceUpdate(t *testing.T) {
	runCase(t, "updates title and artist while preserving state", func(t *testing.T) {
		// Arrange
		service, tracks, _, _ := newTrackFixture()
		existing := readyTrack()
		tracks.seed(existing)
		in := domain.TrackWrite{Title: "New title", Artist: "New artist"}
		want := existing
		want.Title = in.Title
		want.Artist = in.Artist

		// Act
		got, err := service.Update(context.Background(), testUserID, testTrackID, in)

		// Assert
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		assertEqual(t, want, got)
		assertEqual(t, want, tracks.tracks[testTrackID])
	})

	runCase(t, "rejects empty id", func(t *testing.T) {
		// Arrange
		service, _, _, _ := newTrackFixture()

		// Act
		_, err := service.Update(context.Background(), testUserID, "", domain.TrackWrite{Title: "Title"})

		// Assert
		assertErrorCode(t, err, domain.CodeInvalid)
	})

	runCase(t, "rejects missing title", func(t *testing.T) {
		// Arrange
		service, _, _, _ := newTrackFixture()

		// Act
		_, err := service.Update(context.Background(), testUserID, testTrackID, domain.TrackWrite{})

		// Assert
		assertErrorCode(t, err, domain.CodeInvalid)
	})

	runCase(t, "returns lookup error", func(t *testing.T) {
		// Arrange
		service, tracks, _, _ := newTrackFixture()
		tracks.getErr = errDependency

		// Act
		_, err := service.Update(context.Background(), testUserID, testTrackID, domain.TrackWrite{Title: "Title"})

		// Assert
		assertErrorIs(t, err, errDependency)
	})

	runCase(t, "returns update error", func(t *testing.T) {
		// Arrange
		service, tracks, _, _ := newTrackFixture()
		tracks.seed(readyTrack())
		tracks.updateErrors = []error{errDependency}

		// Act
		_, err := service.Update(context.Background(), testUserID, testTrackID, domain.TrackWrite{Title: "Title"})

		// Assert
		assertErrorIs(t, err, errDependency)
	})

	runCase(t, "rejects another users track", func(t *testing.T) {
		// Arrange
		service, tracks, _, _ := newTrackFixture()
		tracks.seed(readyTrack())

		// Act
		_, err := service.Update(context.Background(), "user-2", testTrackID, domain.TrackWrite{Title: "Title"})

		// Assert
		assertErrorCode(t, err, domain.CodeForbidden)
	})
}

func TestTrackServiceDelete(t *testing.T) {
	runCase(t, "deletes track", func(t *testing.T) {
		// Arrange
		service, tracks, _, _ := newTrackFixture()
		tracks.seed(readyTrack())

		// Act
		err := service.Delete(context.Background(), testUserID, testTrackID)

		// Assert
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if _, ok := tracks.tracks[testTrackID]; ok {
			t.Fatal("deleted track remains in repository")
		}
	})

	runCase(t, "rejects empty id", func(t *testing.T) {
		// Arrange
		service, _, _, _ := newTrackFixture()

		// Act
		err := service.Delete(context.Background(), testUserID, "")

		// Assert
		assertErrorCode(t, err, domain.CodeInvalid)
	})

	runCase(t, "returns repository error", func(t *testing.T) {
		// Arrange
		service, tracks, _, _ := newTrackFixture()
		tracks.seed(readyTrack())
		tracks.deleteErr = errDependency

		// Act
		err := service.Delete(context.Background(), testUserID, testTrackID)

		// Assert
		assertErrorIs(t, err, errDependency)
	})

	runCase(t, "rejects another users track", func(t *testing.T) {
		// Arrange
		service, tracks, _, _ := newTrackFixture()
		tracks.seed(readyTrack())

		// Act
		err := service.Delete(context.Background(), "user-2", testTrackID)

		// Assert
		assertErrorCode(t, err, domain.CodeForbidden)
		if _, ok := tracks.tracks[testTrackID]; !ok {
			t.Fatal("foreign delete removed the track")
		}
	})
}

func TestTrackServiceInitUpload(t *testing.T) {
	runCase(t, "creates pending track and presigns upload", func(t *testing.T) {
		// Arrange
		service, tracks, _, objects := newTrackFixture()
		in := validTrackUploadInit()
		wantTrack := in.Track()
		wantTrack.ID = testTrackID

		// Act
		got, err := service.InitUpload(context.Background(), in)

		// Assert
		if err != nil {
			t.Fatalf("InitUpload() error = %v", err)
		}
		assertEqual(t, domain.TrackUploadSession{Track: wantTrack, UploadURL: objects.putURL}, got)
		assertEqual(t, wantTrack, tracks.tracks[testTrackID])
	})

	runCase(t, "rejects invalid input", func(t *testing.T) {
		// Arrange
		service, tracks, _, _ := newTrackFixture()

		// Act
		_, err := service.InitUpload(context.Background(), domain.TrackUploadInit{})

		// Assert
		assertErrorCode(t, err, domain.CodeInvalid)
		assertEqual(t, 0, len(tracks.tracks))
	})

	runCase(t, "rejects oversized upload", func(t *testing.T) {
		// Arrange
		service, tracks, _, _ := newTrackFixture()
		in := validTrackUploadInit()
		in.SizeBytes = testMaxSize + 1

		// Act
		_, err := service.InitUpload(context.Background(), in)

		// Assert
		assertErrorCode(t, err, domain.CodeInvalid)
		assertEqual(t, 0, len(tracks.tracks))
	})

	runCase(t, "returns create error", func(t *testing.T) {
		// Arrange
		service, tracks, _, _ := newTrackFixture()
		tracks.createErr = errDependency

		// Act
		_, err := service.InitUpload(context.Background(), validTrackUploadInit())

		// Assert
		assertErrorIs(t, err, errDependency)
	})

	runCase(t, "rolls back track when presigning fails", func(t *testing.T) {
		// Arrange
		service, tracks, _, objects := newTrackFixture()
		objects.putErr = errDependency

		// Act
		_, err := service.InitUpload(context.Background(), validTrackUploadInit())

		// Assert
		assertErrorIs(t, err, errDependency)
		assertEqual(t, 0, len(tracks.tracks))
	})

	runCase(t, "returns rollback error when cleanup fails", func(t *testing.T) {
		// Arrange
		service, tracks, _, objects := newTrackFixture()
		objects.putErr = errDependency
		tracks.deleteErr = domain.Internal("rollback failed")

		// Act
		_, err := service.InitUpload(context.Background(), validTrackUploadInit())

		// Assert
		assertErrorCode(t, err, domain.CodeInternal)
		if _, ok := tracks.tracks[testTrackID]; !ok {
			t.Fatal("track unexpectedly removed after rollback failure")
		}
	})
}

func TestTrackServiceCompleteUpload(t *testing.T) {
	runCase(t, "verifies upload and publishes ready track", func(t *testing.T) {
		// Arrange
		service, tracks, _, objects := newTrackFixture()
		track := pendingTrack()
		tracks.seed(track)
		objects.sizes[track.AudioObjectKey()] = track.SizeBytes
		in := domain.TrackUploadComplete{TrackID: testTrackID, UserID: testUserID}
		want := track
		want.Status = domain.TrackReady

		// Act
		got, err := service.CompleteUpload(context.Background(), in)

		// Assert
		if err != nil {
			t.Fatalf("CompleteUpload() error = %v", err)
		}
		assertEqual(t, want, got)
		assertEqual(t, want, tracks.tracks[testTrackID])
	})

	runCase(t, "rejects invalid input", func(t *testing.T) {
		// Arrange
		service, _, _, _ := newTrackFixture()

		// Act
		_, err := service.CompleteUpload(context.Background(), domain.TrackUploadComplete{})

		// Assert
		assertErrorCode(t, err, domain.CodeInvalid)
	})

	runCase(t, "returns lookup error", func(t *testing.T) {
		// Arrange
		service, tracks, _, _ := newTrackFixture()
		tracks.getErr = errDependency
		in := domain.TrackUploadComplete{TrackID: testTrackID, UserID: testUserID}

		// Act
		_, err := service.CompleteUpload(context.Background(), in)

		// Assert
		assertErrorIs(t, err, errDependency)
	})

	runCase(t, "rejects another users track", func(t *testing.T) {
		// Arrange
		service, tracks, _, _ := newTrackFixture()
		tracks.seed(pendingTrack())
		in := domain.TrackUploadComplete{TrackID: testTrackID, UserID: "user-2"}

		// Act
		_, err := service.CompleteUpload(context.Background(), in)

		// Assert
		assertErrorCode(t, err, domain.CodeForbidden)
	})

	runCase(t, "returns initial object stat error", func(t *testing.T) {
		// Arrange
		service, tracks, _, objects := newTrackFixture()
		tracks.seed(pendingTrack())
		objects.statErrors = []error{errDependency}
		in := domain.TrackUploadComplete{TrackID: testTrackID, UserID: testUserID}

		// Act
		_, err := service.CompleteUpload(context.Background(), in)

		// Assert
		assertErrorIs(t, err, errDependency)
	})

	runCase(t, "rejects uploaded size mismatch", func(t *testing.T) {
		// Arrange
		service, tracks, _, objects := newTrackFixture()
		track := pendingTrack()
		tracks.seed(track)
		objects.sizes[track.AudioObjectKey()] = track.SizeBytes + 1
		in := domain.TrackUploadComplete{TrackID: testTrackID, UserID: testUserID}

		// Act
		_, err := service.CompleteUpload(context.Background(), in)

		// Assert
		assertErrorCode(t, err, domain.CodeInvalid)
	})

	runCase(t, "rejects invalid status transition", func(t *testing.T) {
		// Arrange
		service, tracks, _, objects := newTrackFixture()
		track := readyTrack()
		tracks.seed(track)
		objects.sizes[track.AudioObjectKey()] = track.SizeBytes
		in := domain.TrackUploadComplete{TrackID: testTrackID, UserID: testUserID}

		// Act
		_, err := service.CompleteUpload(context.Background(), in)

		// Assert
		assertErrorCode(t, err, domain.CodeConflict)
	})

	runCase(t, "returns processing update error", func(t *testing.T) {
		// Arrange
		service, tracks, _, objects := newTrackFixture()
		track := pendingTrack()
		tracks.seed(track)
		tracks.updateErrors = []error{errDependency}
		objects.sizes[track.AudioObjectKey()] = track.SizeBytes
		in := domain.TrackUploadComplete{TrackID: testTrackID, UserID: testUserID}

		// Act
		_, err := service.CompleteUpload(context.Background(), in)

		// Assert
		assertErrorIs(t, err, errDependency)
	})

	runCase(t, "returns publish stat error", func(t *testing.T) {
		// Arrange
		service, tracks, _, objects := newTrackFixture()
		track := pendingTrack()
		tracks.seed(track)
		objects.sizes[track.AudioObjectKey()] = track.SizeBytes
		objects.statErrors = []error{nil, errDependency}
		in := domain.TrackUploadComplete{TrackID: testTrackID, UserID: testUserID}

		// Act
		_, err := service.CompleteUpload(context.Background(), in)

		// Assert
		assertErrorIs(t, err, errDependency)
		assertEqual(t, domain.TrackProcessing, tracks.tracks[testTrackID].Status)
	})

	runCase(t, "returns ready update error", func(t *testing.T) {
		// Arrange
		service, tracks, _, objects := newTrackFixture()
		track := pendingTrack()
		tracks.seed(track)
		tracks.updateErrors = []error{nil, errDependency}
		objects.sizes[track.AudioObjectKey()] = track.SizeBytes
		in := domain.TrackUploadComplete{TrackID: testTrackID, UserID: testUserID}

		// Act
		_, err := service.CompleteUpload(context.Background(), in)

		// Assert
		assertErrorIs(t, err, errDependency)
		assertEqual(t, domain.TrackProcessing, tracks.tracks[testTrackID].Status)
	})
}

func TestTrackServiceStreamURL(t *testing.T) {
	runCase(t, "presigns ready track stream", func(t *testing.T) {
		// Arrange
		service, tracks, _, objects := newTrackFixture()
		tracks.seed(readyTrack())

		// Act
		got, err := service.StreamURL(context.Background(), testTrackID)

		// Assert
		if err != nil {
			t.Fatalf("StreamURL() error = %v", err)
		}
		assertEqual(t, objects.getURL, got)
	})

	runCase(t, "rejects empty id", func(t *testing.T) {
		// Arrange
		service, _, _, _ := newTrackFixture()

		// Act
		_, err := service.StreamURL(context.Background(), "")

		// Assert
		assertErrorCode(t, err, domain.CodeInvalid)
	})

	runCase(t, "returns lookup error", func(t *testing.T) {
		// Arrange
		service, tracks, _, _ := newTrackFixture()
		tracks.getErr = errDependency

		// Act
		_, err := service.StreamURL(context.Background(), testTrackID)

		// Assert
		assertErrorIs(t, err, errDependency)
	})

	runCase(t, "rejects non-ready track", func(t *testing.T) {
		// Arrange
		service, tracks, _, _ := newTrackFixture()
		tracks.seed(pendingTrack())

		// Act
		_, err := service.StreamURL(context.Background(), testTrackID)

		// Assert
		assertErrorCode(t, err, domain.CodeConflict)
	})

	runCase(t, "returns presign error", func(t *testing.T) {
		// Arrange
		service, tracks, _, objects := newTrackFixture()
		tracks.seed(readyTrack())
		objects.getErr = errDependency

		// Act
		_, err := service.StreamURL(context.Background(), testTrackID)

		// Assert
		assertErrorIs(t, err, errDependency)
	})
}
