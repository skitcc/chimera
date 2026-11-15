package testkit

import "chimera/catalog/internal/domain"

var Tracks TrackObjectMother

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
