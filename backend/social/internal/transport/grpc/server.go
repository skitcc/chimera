// Package grpc exposes the Social use cases as chimera.social.v1.SocialService.
package grpc

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	catalogv1 "chimera/contracts/gen/catalog/v1"
	commonv1 "chimera/contracts/gen/common/v1"
	identityv1 "chimera/contracts/gen/identity/v1"
	socialv1 "chimera/contracts/gen/social/v1"
	"chimera/platform/idempotency"
	"chimera/social/internal/domain"
)

// ReplicaMethods may read a replica. Mutations read the primary, and the
// Catalog and Identity checks they make ask for CONSISTENCY_STRONG.
var ReplicaMethods = []string{
	socialv1.SocialService_ListLikedTracks_FullMethodName,
	socialv1.SocialService_ListFollows_FullMethodName,
	socialv1.SocialService_GetFollow_FullMethodName,
	socialv1.SocialService_ListPlaylists_FullMethodName,
	socialv1.SocialService_GetPlaylist_FullMethodName,
	socialv1.SocialService_ListPlaylistTracks_FullMethodName,
}

type LikeService interface {
	PutLike(ctx context.Context, actorID string, in domain.TrackLike) error
	DeleteLike(ctx context.Context, actorID string, in domain.TrackLike) error
	ListLiked(ctx context.Context, q domain.LikedTracksQuery) (domain.TrackPage, error)
}

type FollowService interface {
	List(ctx context.Context, q domain.FollowListQuery) (domain.FollowPage, error)
	Get(ctx context.Context, followerID, followingID string) (domain.Follow, error)
	Put(ctx context.Context, actorID, followerID, followingID string) error
	Delete(ctx context.Context, actorID, followerID, followingID string) error
	CreateMany(ctx context.Context, actorID string, in domain.FollowBatch) error
	DeleteMany(ctx context.Context, actorID string, in domain.FollowBatch) error
}

type PlaylistService interface {
	List(ctx context.Context, q domain.PlaylistListQuery) (domain.PlaylistPage, error)
	Create(ctx context.Context, ownerID string, in domain.PlaylistReplace) (domain.Playlist, error)
	Get(ctx context.Context, id, viewerID string) (domain.Playlist, error)
	Replace(ctx context.Context, actorID, id string, in domain.PlaylistReplace) (domain.Playlist, error)
	Patch(ctx context.Context, actorID, id string, in domain.PlaylistPatch) (domain.Playlist, error)
	Delete(ctx context.Context, actorID, id string) error
	ListTracks(ctx context.Context, id, viewerID string, q domain.PageQuery) (domain.PlaylistTrackPage, error)
	AddTrack(ctx context.Context, actorID, id string, in domain.PlaylistTrackAdd) (domain.PlaylistTrack, error)
	MoveTrack(ctx context.Context, actorID, id, trackID string, in domain.PlaylistTrackMove) (domain.PlaylistTrack, error)
	RemoveTrack(ctx context.Context, actorID, id, trackID string) error
}

type Server struct {
	socialv1.UnimplementedSocialServiceServer
	likes     LikeService
	follows   FollowService
	playlists PlaylistService
}

func NewServer(likes LikeService, follows FollowService, playlists PlaylistService) *Server {
	return &Server{likes: likes, follows: follows, playlists: playlists}
}

func (s *Server) PutLike(ctx context.Context, req *socialv1.PutLikeRequest) (*socialv1.PutLikeResponse, error) {
	in := domain.TrackLike{UserID: req.GetUserId(), TrackID: req.GetTrackId()}
	if err := s.likes.PutLike(ctx, req.GetActorId(), in); err != nil {
		return nil, err
	}
	return &socialv1.PutLikeResponse{}, nil
}

func (s *Server) DeleteLike(ctx context.Context, req *socialv1.DeleteLikeRequest) (*socialv1.DeleteLikeResponse, error) {
	in := domain.TrackLike{UserID: req.GetUserId(), TrackID: req.GetTrackId()}
	if err := s.likes.DeleteLike(ctx, req.GetActorId(), in); err != nil {
		return nil, err
	}
	return &socialv1.DeleteLikeResponse{}, nil
}

func (s *Server) ListLikedTracks(ctx context.Context, req *socialv1.ListLikedTracksRequest) (*socialv1.ListLikedTracksResponse, error) {
	page, err := s.likes.ListLiked(ctx, domain.LikedTracksQuery{
		Page:     pageQuery(req.GetPage()),
		ViewerID: req.GetViewerId(),
		LikedBy:  req.GetLikedBy(),
		UserID:   req.GetUserId(),
		Artist:   req.GetArtist(),
		Status:   domain.TrackStatus(req.GetStatus()),
	})
	if err != nil {
		return nil, err
	}
	items := make([]*catalogv1.Track, 0, len(page.Items))
	for _, t := range page.Items {
		items = append(items, toTrack(t))
	}
	return &socialv1.ListLikedTracksResponse{Items: items, NextCursor: page.NextCursor, Limit: int64(page.Limit)}, nil
}

func (s *Server) ListFollows(ctx context.Context, req *socialv1.ListFollowsRequest) (*socialv1.ListFollowsResponse, error) {
	page, err := s.follows.List(ctx, domain.FollowListQuery{
		PageQuery:   pageQuery(req.GetPage()),
		FollowerID:  req.GetFollowerId(),
		FollowingID: req.GetFollowingId(),
	})
	if err != nil {
		return nil, err
	}
	items := make([]*socialv1.Follow, 0, len(page.Items))
	for _, f := range page.Items {
		items = append(items, toFollow(f))
	}
	return &socialv1.ListFollowsResponse{Items: items, NextCursor: page.NextCursor, Limit: int64(page.Limit)}, nil
}

func (s *Server) GetFollow(ctx context.Context, req *socialv1.GetFollowRequest) (*socialv1.GetFollowResponse, error) {
	follow, err := s.follows.Get(ctx, req.GetFollowerId(), req.GetFollowingId())
	if err != nil {
		return nil, err
	}
	return &socialv1.GetFollowResponse{Follow: toFollow(follow)}, nil
}

func (s *Server) PutFollow(ctx context.Context, req *socialv1.PutFollowRequest) (*socialv1.PutFollowResponse, error) {
	if err := s.follows.Put(ctx, req.GetActorId(), req.GetFollowerId(), req.GetFollowingId()); err != nil {
		return nil, err
	}
	return &socialv1.PutFollowResponse{}, nil
}

func (s *Server) DeleteFollow(ctx context.Context, req *socialv1.DeleteFollowRequest) (*socialv1.DeleteFollowResponse, error) {
	if err := s.follows.Delete(ctx, req.GetActorId(), req.GetFollowerId(), req.GetFollowingId()); err != nil {
		return nil, err
	}
	return &socialv1.DeleteFollowResponse{}, nil
}

func (s *Server) CreateFollows(ctx context.Context, req *socialv1.CreateFollowsRequest) (*socialv1.CreateFollowsResponse, error) {
	in := domain.FollowBatch{FollowerID: req.GetFollowerId(), FollowingIDs: req.GetFollowingIds()}
	if err := s.follows.CreateMany(ctx, req.GetActorId(), in); err != nil {
		return nil, err
	}
	return &socialv1.CreateFollowsResponse{}, nil
}

func (s *Server) DeleteFollows(ctx context.Context, req *socialv1.DeleteFollowsRequest) (*socialv1.DeleteFollowsResponse, error) {
	in := domain.FollowBatch{FollowerID: req.GetFollowerId(), FollowingIDs: req.GetFollowingIds()}
	if err := s.follows.DeleteMany(ctx, req.GetActorId(), in); err != nil {
		return nil, err
	}
	return &socialv1.DeleteFollowsResponse{}, nil
}

func (s *Server) ListPlaylists(ctx context.Context, req *socialv1.ListPlaylistsRequest) (*socialv1.ListPlaylistsResponse, error) {
	page, err := s.playlists.List(ctx, domain.PlaylistListQuery{
		PageQuery: pageQuery(req.GetPage()),
		ViewerID:  req.GetViewerId(),
		OwnerID:   req.GetOwnerId(),
		IsPublic:  req.IsPublic,
	})
	if err != nil {
		return nil, err
	}
	items := make([]*socialv1.Playlist, 0, len(page.Items))
	for _, p := range page.Items {
		items = append(items, toPlaylist(p))
	}
	return &socialv1.ListPlaylistsResponse{Items: items, NextCursor: page.NextCursor, Limit: int64(page.Limit)}, nil
}

func (s *Server) CreatePlaylist(ctx context.Context, req *socialv1.CreatePlaylistRequest) (*socialv1.CreatePlaylistResponse, error) {
	ctx, err := idempotency.With(ctx, req.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	playlist, err := s.playlists.Create(ctx, req.GetActorId(), domain.PlaylistReplace{
		Title:    req.GetTitle(),
		IsPublic: req.GetIsPublic(),
	})
	if err != nil {
		return nil, err
	}
	return &socialv1.CreatePlaylistResponse{Playlist: toPlaylist(playlist)}, nil
}

func (s *Server) GetPlaylist(ctx context.Context, req *socialv1.GetPlaylistRequest) (*socialv1.GetPlaylistResponse, error) {
	playlist, err := s.playlists.Get(ctx, req.GetId(), req.GetViewerId())
	if err != nil {
		return nil, err
	}
	return &socialv1.GetPlaylistResponse{Playlist: toPlaylist(playlist)}, nil
}

func (s *Server) ReplacePlaylist(ctx context.Context, req *socialv1.ReplacePlaylistRequest) (*socialv1.ReplacePlaylistResponse, error) {
	playlist, err := s.playlists.Replace(ctx, req.GetActorId(), req.GetId(), domain.PlaylistReplace{
		Title:    req.GetTitle(),
		IsPublic: req.GetIsPublic(),
	})
	if err != nil {
		return nil, err
	}
	return &socialv1.ReplacePlaylistResponse{Playlist: toPlaylist(playlist)}, nil
}

func (s *Server) PatchPlaylist(ctx context.Context, req *socialv1.PatchPlaylistRequest) (*socialv1.PatchPlaylistResponse, error) {
	playlist, err := s.playlists.Patch(ctx, req.GetActorId(), req.GetId(), domain.PlaylistPatch{
		Title:    req.Title,
		IsPublic: req.IsPublic,
	})
	if err != nil {
		return nil, err
	}
	return &socialv1.PatchPlaylistResponse{Playlist: toPlaylist(playlist)}, nil
}

func (s *Server) DeletePlaylist(ctx context.Context, req *socialv1.DeletePlaylistRequest) (*socialv1.DeletePlaylistResponse, error) {
	if err := s.playlists.Delete(ctx, req.GetActorId(), req.GetId()); err != nil {
		return nil, err
	}
	return &socialv1.DeletePlaylistResponse{}, nil
}

func (s *Server) ListPlaylistTracks(ctx context.Context, req *socialv1.ListPlaylistTracksRequest) (*socialv1.ListPlaylistTracksResponse, error) {
	page, err := s.playlists.ListTracks(ctx, req.GetId(), req.GetViewerId(), pageQuery(req.GetPage()))
	if err != nil {
		return nil, err
	}
	items := make([]*socialv1.PlaylistTrack, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, toPlaylistTrack(item))
	}
	return &socialv1.ListPlaylistTracksResponse{Items: items, NextCursor: page.NextCursor, Limit: int64(page.Limit)}, nil
}

func (s *Server) AddPlaylistTrack(ctx context.Context, req *socialv1.AddPlaylistTrackRequest) (*socialv1.AddPlaylistTrackResponse, error) {
	item, err := s.playlists.AddTrack(ctx, req.GetActorId(), req.GetPlaylistId(), domain.PlaylistTrackAdd{
		TrackID:  req.GetTrackId(),
		Position: int(req.GetPosition()),
	})
	if err != nil {
		return nil, err
	}
	return &socialv1.AddPlaylistTrackResponse{Item: toPlaylistTrack(item)}, nil
}

func (s *Server) MovePlaylistTrack(ctx context.Context, req *socialv1.MovePlaylistTrackRequest) (*socialv1.MovePlaylistTrackResponse, error) {
	item, err := s.playlists.MoveTrack(ctx, req.GetActorId(), req.GetPlaylistId(), req.GetTrackId(), domain.PlaylistTrackMove{
		Position: int(req.GetPosition()),
	})
	if err != nil {
		return nil, err
	}
	return &socialv1.MovePlaylistTrackResponse{Item: toPlaylistTrack(item)}, nil
}

func (s *Server) RemovePlaylistTrack(ctx context.Context, req *socialv1.RemovePlaylistTrackRequest) (*socialv1.RemovePlaylistTrackResponse, error) {
	if err := s.playlists.RemoveTrack(ctx, req.GetActorId(), req.GetPlaylistId(), req.GetTrackId()); err != nil {
		return nil, err
	}
	return &socialv1.RemovePlaylistTrackResponse{}, nil
}

func pageQuery(p *commonv1.PageRequest) domain.PageQuery {
	return domain.NewPageQuery(int(p.GetLimit()), p.GetCursor(), p.GetInvalidLimit())
}

func toTrack(t domain.Track) *catalogv1.Track {
	return &catalogv1.Track{
		Id:        t.ID,
		UserId:    t.UserID,
		Title:     t.Title,
		Artist:    t.Artist,
		Status:    string(t.Status),
		SizeBytes: t.SizeBytes,
	}
}

func toUser(u domain.User) *identityv1.User {
	return &identityv1.User{Id: u.ID, Email: u.Email, Name: u.Name}
}

func toFollow(f domain.Follow) *socialv1.Follow {
	return &socialv1.Follow{
		Follower:  toUser(f.Follower),
		Following: toUser(f.Following),
		CreatedAt: timestamppb.New(f.CreatedAt),
	}
}

func toPlaylist(p domain.Playlist) *socialv1.Playlist {
	return &socialv1.Playlist{Id: p.ID, OwnerId: p.OwnerID, Title: p.Title, IsPublic: p.IsPublic}
}

func toPlaylistTrack(item domain.PlaylistTrack) *socialv1.PlaylistTrack {
	return &socialv1.PlaylistTrack{Position: int64(item.Position), Track: toTrack(item.Track)}
}
