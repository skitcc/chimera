// Package clients implements the v1 controller ports with the Identity, Catalog
// and Social gRPC clients. Errors arrive as application errors already: the
// client interceptor from platform/grpcx converts the gRPC status.
package clients

import (
	catalogv1 "chimera/contracts/gen/catalog/v1"
	commonv1 "chimera/contracts/gen/common/v1"
	identityv1 "chimera/contracts/gen/identity/v1"
	socialv1 "chimera/contracts/gen/social/v1"
	"chimera/gateway/internal/domain"
)

func page(q domain.PageQuery) *commonv1.PageRequest {
	return &commonv1.PageRequest{Limit: int64(q.Limit), Cursor: q.Cursor, InvalidLimit: q.InvalidLimit}
}

func toUser(u *identityv1.User) domain.User {
	return domain.User{ID: u.GetId(), Email: u.GetEmail(), Name: u.GetName()}
}

func toTrack(t *catalogv1.Track) domain.Track {
	return domain.Track{
		ID:        t.GetId(),
		UserID:    t.GetUserId(),
		Title:     t.GetTitle(),
		Artist:    t.GetArtist(),
		SizeBytes: t.GetSizeBytes(),
		Status:    domain.TrackStatus(t.GetStatus()),
	}
}

func toTracks(items []*catalogv1.Track) []domain.Track {
	tracks := make([]domain.Track, 0, len(items))
	for _, t := range items {
		tracks = append(tracks, toTrack(t))
	}
	return tracks
}

func toFollow(f *socialv1.Follow) domain.Follow {
	return domain.Follow{
		Follower:  toUser(f.GetFollower()),
		Following: toUser(f.GetFollowing()),
		CreatedAt: f.GetCreatedAt().AsTime(),
	}
}

func toPlaylist(p *socialv1.Playlist) domain.Playlist {
	return domain.Playlist{ID: p.GetId(), OwnerID: p.GetOwnerId(), Title: p.GetTitle(), IsPublic: p.GetIsPublic()}
}

func toPlaylistTrack(item *socialv1.PlaylistTrack) domain.PlaylistTrack {
	return domain.PlaylistTrack{Position: int(item.GetPosition()), Track: toTrack(item.GetTrack())}
}
