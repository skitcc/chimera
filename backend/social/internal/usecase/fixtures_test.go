package usecase

import (
	"errors"
	"reflect"
	"testing"

	"chimera/social/internal/domain"
	"chimera/social/internal/testkit"
)

const (
	testUserID  = "user-1"
	testTrackID = "track-1"
	testEventID = "8f14e45f-ceea-467a-9575-3a2b1c9d0e11"
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

func newLikeFixture() (*LikeService, *fakeTrackCatalog, *fakeTrackLikeRepository) {
	tracks := newFakeTrackCatalog()
	likes := newFakeTrackLikeRepository()
	return NewLikeService(tracks, likes), tracks, likes
}

func pendingTrack() domain.Track {
	return domain.Track{
		ID:        testTrackID,
		UserID:    testUserID,
		Title:     "Track",
		Artist:    "Artist",
		SizeBytes: 100,
		Status:    domain.TrackPending,
	}
}

func readyTrack() domain.Track {
	track := pendingTrack()
	track.Status = domain.TrackReady
	return track
}

func readyTrackWithID(id string) domain.Track {
	track := readyTrack()
	track.ID = id
	return track
}

func validTrackLike() domain.TrackLike {
	return domain.TrackLike{UserID: testUserID, TrackID: testTrackID}
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

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
