package usecase

import (
	"context"

	"chimera/internal/domain"
)

type PlaylistService struct {
	playlists PlaylistRepository
	tracks    TrackRepository
	users     UserRepository
}

func NewPlaylistService(playlists PlaylistRepository, tracks TrackRepository, users UserRepository) *PlaylistService {
	return &PlaylistService{playlists: playlists, tracks: tracks, users: users}
}

func (s *PlaylistService) Create(ctx context.Context, ownerID string, in domain.PlaylistReplace) (domain.Playlist, error) {
	if err := domain.UserID(ownerID).Validate(); err != nil {
		return domain.Playlist{}, err
	}
	if err := in.Validate(); err != nil {
		return domain.Playlist{}, err
	}
	return s.playlists.Create(ctx, domain.Playlist{
		OwnerID: ownerID, Title: in.Title, IsPublic: in.IsPublic,
	})
}

func (s *PlaylistService) Get(ctx context.Context, id, viewerID string) (domain.Playlist, error) {
	if err := domain.PlaylistID(id).Validate(); err != nil {
		return domain.Playlist{}, err
	}
	playlist, err := s.playlists.GetByID(ctx, id)
	if err != nil {
		return domain.Playlist{}, err
	}
	if err := playlist.VisibleTo(viewerID); err != nil {
		return domain.Playlist{}, err
	}
	return playlist, nil
}

func (s *PlaylistService) ListByOwner(ctx context.Context, ownerID string, publicOnly bool, q domain.PageQuery) (domain.PlaylistPage, error) {
	if err := domain.UserID(ownerID).Validate(); err != nil {
		return domain.PlaylistPage{}, err
	}
	if err := q.Validate(); err != nil {
		return domain.PlaylistPage{}, err
	}
	if _, err := s.users.GetByID(ctx, ownerID); err != nil {
		return domain.PlaylistPage{}, err
	}
	playlists, err := s.playlists.ListByOwner(ctx, ownerID, publicOnly)
	if err != nil {
		return domain.PlaylistPage{}, err
	}
	return q.PagePlaylists(playlists), nil
}

func (s *PlaylistService) Replace(ctx context.Context, actorID, id string, in domain.PlaylistReplace) (domain.Playlist, error) {
	if err := in.Validate(); err != nil {
		return domain.Playlist{}, err
	}
	playlist, err := s.owned(ctx, actorID, id)
	if err != nil {
		return domain.Playlist{}, err
	}
	playlist.Title = in.Title
	playlist.IsPublic = in.IsPublic
	return s.playlists.Update(ctx, playlist)
}

func (s *PlaylistService) Patch(ctx context.Context, actorID, id string, in domain.PlaylistPatch) (domain.Playlist, error) {
	if err := in.Validate(); err != nil {
		return domain.Playlist{}, err
	}
	playlist, err := s.owned(ctx, actorID, id)
	if err != nil {
		return domain.Playlist{}, err
	}
	if in.Title != nil {
		playlist.Title = *in.Title
	}
	if in.IsPublic != nil {
		playlist.IsPublic = *in.IsPublic
	}
	return s.playlists.Update(ctx, playlist)
}

func (s *PlaylistService) Delete(ctx context.Context, actorID, id string) error {
	if _, err := s.owned(ctx, actorID, id); err != nil {
		return err
	}
	return s.playlists.Delete(ctx, id)
}

func (s *PlaylistService) ListTracks(ctx context.Context, id, viewerID string, q domain.PageQuery) (domain.PlaylistTrackPage, error) {
	if _, err := s.Get(ctx, id, viewerID); err != nil {
		return domain.PlaylistTrackPage{}, err
	}
	if err := q.Validate(); err != nil {
		return domain.PlaylistTrackPage{}, err
	}
	tracks, err := s.playlists.ListTracks(ctx, id)
	if err != nil {
		return domain.PlaylistTrackPage{}, err
	}
	return q.PagePlaylistTracks(tracks), nil
}

func (s *PlaylistService) AddTrack(ctx context.Context, actorID, id string, in domain.PlaylistTrackAdd) (domain.PlaylistTrack, error) {
	if err := in.Validate(); err != nil {
		return domain.PlaylistTrack{}, err
	}
	if _, err := s.owned(ctx, actorID, id); err != nil {
		return domain.PlaylistTrack{}, err
	}
	track, err := s.tracks.GetByID(ctx, in.TrackID)
	if err != nil {
		return domain.PlaylistTrack{}, err
	}
	if err := track.EnsureReady(); err != nil {
		return domain.PlaylistTrack{}, err
	}
	return s.playlists.AddTrack(ctx, id, in)
}

func (s *PlaylistService) MoveTrack(ctx context.Context, actorID, id, trackID string, in domain.PlaylistTrackMove) (domain.PlaylistTrack, error) {
	if err := domain.TrackID(trackID).Validate(); err != nil {
		return domain.PlaylistTrack{}, err
	}
	if err := in.Validate(); err != nil {
		return domain.PlaylistTrack{}, err
	}
	if _, err := s.owned(ctx, actorID, id); err != nil {
		return domain.PlaylistTrack{}, err
	}
	return s.playlists.MoveTrack(ctx, id, trackID, in.Position)
}

func (s *PlaylistService) RemoveTrack(ctx context.Context, actorID, id, trackID string) error {
	if err := domain.TrackID(trackID).Validate(); err != nil {
		return err
	}
	if _, err := s.owned(ctx, actorID, id); err != nil {
		return err
	}
	return s.playlists.RemoveTrack(ctx, id, trackID)
}

func (s *PlaylistService) owned(ctx context.Context, actorID, id string) (domain.Playlist, error) {
	if err := domain.PlaylistID(id).Validate(); err != nil {
		return domain.Playlist{}, err
	}
	playlist, err := s.playlists.GetByID(ctx, id)
	if err != nil {
		return domain.Playlist{}, err
	}
	if err := playlist.RequireOwner(actorID); err != nil {
		return domain.Playlist{}, err
	}
	return playlist, nil
}
