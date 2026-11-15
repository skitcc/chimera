package usecase

import (
	"context"

	"chimera/social/internal/domain"
)

type PlaylistService struct {
	playlists PlaylistRepository
	tracks    TrackCatalog
	users     UserDirectory
}

func NewPlaylistService(playlists PlaylistRepository, tracks TrackCatalog, users UserDirectory) *PlaylistService {
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

func (s *PlaylistService) List(ctx context.Context, q domain.PlaylistListQuery) (domain.PlaylistPage, error) {
	if err := q.Validate(); err != nil {
		return domain.PlaylistPage{}, err
	}
	if q.OwnerID != "" {
		if _, err := s.users.GetUser(ctx, q.OwnerID); err != nil {
			return domain.PlaylistPage{}, err
		}
	}
	playlists, err := s.playlists.List(ctx, q.Filter())
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

// ListTracks shows only ready tracks, in position order.
func (s *PlaylistService) ListTracks(ctx context.Context, id, viewerID string, q domain.PageQuery) (domain.PlaylistTrackPage, error) {
	if _, err := s.Get(ctx, id, viewerID); err != nil {
		return domain.PlaylistTrackPage{}, err
	}
	if err := q.Validate(); err != nil {
		return domain.PlaylistTrackPage{}, err
	}
	entries, err := s.playlists.ListEntries(ctx, id)
	if err != nil {
		return domain.PlaylistTrackPage{}, err
	}
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		ids = append(ids, e.TrackID)
	}
	tracks, err := s.trackIndex(ctx, ids)
	if err != nil {
		return domain.PlaylistTrackPage{}, err
	}
	items := make([]domain.PlaylistTrack, 0, len(entries))
	for _, e := range entries {
		track, ok := tracks[e.TrackID]
		if !ok || track.Status != domain.TrackReady {
			continue
		}
		items = append(items, domain.PlaylistTrack{Position: e.Position, Track: track})
	}
	return q.PagePlaylistTracks(items), nil
}

func (s *PlaylistService) AddTrack(ctx context.Context, actorID, id string, in domain.PlaylistTrackAdd) (domain.PlaylistTrack, error) {
	if err := in.Validate(); err != nil {
		return domain.PlaylistTrack{}, err
	}
	if _, err := s.owned(ctx, actorID, id); err != nil {
		return domain.PlaylistTrack{}, err
	}
	track, err := s.tracks.GetTrack(ctx, in.TrackID)
	if err != nil {
		return domain.PlaylistTrack{}, err
	}
	if err := track.EnsureReady(); err != nil {
		return domain.PlaylistTrack{}, err
	}
	entry, err := s.playlists.AddTrack(ctx, id, in)
	if err != nil {
		return domain.PlaylistTrack{}, err
	}
	return domain.PlaylistTrack{Position: entry.Position, Track: track}, nil
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
	entry, err := s.playlists.MoveTrack(ctx, id, trackID, in.Position)
	if err != nil {
		return domain.PlaylistTrack{}, err
	}
	tracks, err := s.trackIndex(ctx, []string{trackID})
	if err != nil {
		return domain.PlaylistTrack{}, err
	}
	track, ok := tracks[trackID]
	if !ok {
		return domain.PlaylistTrack{}, domain.NotFound("playlist track not found")
	}
	return domain.PlaylistTrack{Position: entry.Position, Track: track}, nil
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

func (s *PlaylistService) trackIndex(ctx context.Context, ids []string) (map[string]domain.Track, error) {
	tracks, err := s.tracks.GetTracks(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]domain.Track, len(tracks))
	for _, t := range tracks {
		byID[t.ID] = t
	}
	return byID, nil
}
