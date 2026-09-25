package testkit

import "chimera/internal/domain"

var (
	Users  UserObjectMother
	Auth   AuthObjectMother
	Tracks TrackObjectMother
	Likes  TrackLikeObjectMother
)

type UserObjectMother struct{}

func (UserObjectMother) Alice() domain.User {
	return UserMother().
		WithID("user-alice").
		WithEmail("alice@example.com").
		WithName("Alice").
		Build()
}

func (UserObjectMother) Bob() domain.User {
	return UserMother().
		WithID("user-bob").
		WithEmail("bob@example.com").
		WithName("Bob").
		Build()
}

type AuthObjectMother struct{}

func (AuthObjectMother) ValidRegister() domain.RegisterInput {
	return AuthMother().BuildRegisterInput()
}

func (AuthObjectMother) ValidLogin() domain.LoginInput {
	return AuthMother().BuildLoginInput()
}

type TrackObjectMother struct{}

func (TrackObjectMother) Pending() domain.Track {
	return TrackMother().WithStatus(domain.TrackPending).Build()
}

func (TrackObjectMother) Processing() domain.Track {
	return TrackMother().WithStatus(domain.TrackProcessing).Build()
}

func (TrackObjectMother) Ready() domain.Track {
	return TrackMother().WithStatus(domain.TrackReady).Build()
}

type TrackLikeObjectMother struct{}

func (TrackLikeObjectMother) For(track domain.Track) domain.TrackLike {
	return TrackLikeMother().
		WithUserID(track.UserID).
		WithTrackID(track.ID).
		Build()
}
