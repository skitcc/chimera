package usecase

import (
	"errors"
	"reflect"
	"strconv"
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

func runSpec(t *testing.T, component string, s testkit.Spec, body func(*testing.T, testkit.Report)) {
	t.Helper()
	s.Layer = testkit.LayerUsecase
	s.Component = component
	if s.Kind == "" {
		s.Kind = testkit.KindClassic
	}
	testkit.RunSpec(t, s, body)
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

func validTrackLike() domain.TrackLike {
	return domain.TrackLike{UserID: testUserID, TrackID: testTrackID}
}

func validUploadComplete() domain.TrackUploadComplete {
	return domain.TrackUploadComplete{TrackID: testTrackID, UserID: testUserID}
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

func assertDomainError(t *testing.T, err error, code domain.Code, message string) {
	t.Helper()
	app, ok := domain.As(err)
	if !ok {
		t.Fatalf("want domain error (%s, %q), got %v", code, message, err)
	}
	if app.Code != code || app.Message != message {
		t.Fatalf("want domain error (%s, %q), got (%s, %q)", code, message, app.Code, app.Message)
	}
}

// assertFailure checks wantErr through errors.Is when set, otherwise the exact domain code and message.
func assertFailure(t *testing.T, err, wantErr error, code domain.Code, message string) {
	t.Helper()
	if wantErr != nil {
		assertErrorIs(t, err, wantErr)
		return
	}
	assertDomainError(t, err, code, message)
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
