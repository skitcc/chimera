package usecase

import (
	"errors"
	"reflect"
	"testing"

	"chimera/internal/domain"
	"chimera/internal/testkit"
)

const (
	testUserID  = "user-1"
	testTrackID = "track-1"
	testMaxSize = int64(1_000)
)

var errDependency = errors.New("dependency failed")

func runCase(t *testing.T, name string, body func(*testing.T)) {
	t.Helper()
	testkit.Run(t, name, body)
}

func newAuthFixture() (*AuthService, *fakeUserRepository, *fakePasswordHasher, *fakeTokenIssuer) {
	users := newFakeUserRepository()
	hasher := &fakePasswordHasher{}
	tokens := &fakeTokenIssuer{token: "token-1"}
	return NewAuthService(users, hasher, tokens), users, hasher, tokens
}

func newUserFixture() (*UserService, *fakeUserRepository, *fakePasswordHasher) {
	users := newFakeUserRepository()
	hasher := &fakePasswordHasher{}
	return NewUserService(users, hasher), users, hasher
}

func newTrackFixture() (*TrackService, *fakeTrackRepository, *fakeTrackLikeRepository, *fakeObjectStorage) {
	tracks := newFakeTrackRepository()
	likes := newFakeTrackLikeRepository(tracks)
	objects := newFakeObjectStorage()
	return NewTrackService(tracks, likes, objects, testMaxSize), tracks, likes, objects
}

func validUser() domain.User {
	return domain.User{ID: testUserID, Email: "listener@example.com", Name: "Listener"}
}

func validUserWrite() domain.UserWrite {
	return domain.UserWrite{Email: "listener@example.com", Name: "Listener", Password: "password"}
}

func validRegisterInput() domain.RegisterInput {
	return domain.RegisterInput{Email: "listener@example.com", Name: "Listener", Password: "password"}
}

func validLoginInput() domain.LoginInput {
	return domain.LoginInput{Email: "listener@example.com", Password: "password"}
}

func pendingTrack() domain.Track {
	return domain.Track{
		ID:        testTrackID,
		UserID:    testUserID,
		Title:     "Track",
		Artist:    "Artist",
		ObjectKey: "audio/track-1.mp3",
		SizeBytes: 100,
		Status:    domain.TrackPending,
	}
}

func processingTrack() domain.Track {
	track := pendingTrack()
	track.Status = domain.TrackProcessing
	return track
}

func readyTrack() domain.Track {
	track := pendingTrack()
	track.Status = domain.TrackReady
	return track
}

func validTrackUploadInit() domain.TrackUploadInit {
	return domain.TrackUploadInit{
		UserID:    testUserID,
		Title:     "Track",
		Artist:    "Artist",
		SizeBytes: 100,
	}
}

func assertEqual[T any](t *testing.T, want, got T) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("want %#v, got %#v", want, got)
	}
}

func assertErrorIs(t *testing.T, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("want error %v, got %v", want, got)
	}
}

func assertErrorCode(t *testing.T, err error, code domain.Code) {
	t.Helper()
	if !domain.Is(err, code) {
		t.Fatalf("want error code %q, got %v", code, err)
	}
}
