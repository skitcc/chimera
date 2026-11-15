package clients

import (
	"context"

	socialv1 "chimera/contracts/gen/social/v1"
	"chimera/gateway/internal/domain"
)

// Follows serves /v1/follows.
type Follows struct {
	client socialv1.SocialServiceClient
}

func NewFollows(client socialv1.SocialServiceClient) *Follows {
	return &Follows{client: client}
}

func (c *Follows) List(ctx context.Context, q domain.FollowListQuery) (domain.FollowPage, error) {
	resp, err := c.client.ListFollows(ctx, &socialv1.ListFollowsRequest{
		Page:        page(q.PageQuery),
		FollowerId:  q.FollowerID,
		FollowingId: q.FollowingID,
	})
	if err != nil {
		return domain.FollowPage{}, err
	}
	items := make([]domain.Follow, 0, len(resp.GetItems()))
	for _, f := range resp.GetItems() {
		items = append(items, toFollow(f))
	}
	return domain.FollowPage{Items: items, NextCursor: resp.GetNextCursor(), Limit: int(resp.GetLimit())}, nil
}

func (c *Follows) Get(ctx context.Context, followerID, followingID string) (domain.Follow, error) {
	resp, err := c.client.GetFollow(ctx, &socialv1.GetFollowRequest{FollowerId: followerID, FollowingId: followingID})
	if err != nil {
		return domain.Follow{}, err
	}
	return toFollow(resp.GetFollow()), nil
}

func (c *Follows) Put(ctx context.Context, actorID, followerID, followingID string) error {
	_, err := c.client.PutFollow(ctx, &socialv1.PutFollowRequest{ActorId: actorID, FollowerId: followerID, FollowingId: followingID})
	return err
}

func (c *Follows) Delete(ctx context.Context, actorID, followerID, followingID string) error {
	_, err := c.client.DeleteFollow(ctx, &socialv1.DeleteFollowRequest{ActorId: actorID, FollowerId: followerID, FollowingId: followingID})
	return err
}

func (c *Follows) CreateMany(ctx context.Context, actorID string, in domain.FollowBatch) error {
	_, err := c.client.CreateFollows(ctx, &socialv1.CreateFollowsRequest{
		ActorId:      actorID,
		FollowerId:   in.FollowerID,
		FollowingIds: in.FollowingIDs,
	})
	return err
}

func (c *Follows) DeleteMany(ctx context.Context, actorID string, in domain.FollowBatch) error {
	_, err := c.client.DeleteFollows(ctx, &socialv1.DeleteFollowsRequest{
		ActorId:      actorID,
		FollowerId:   in.FollowerID,
		FollowingIds: in.FollowingIDs,
	})
	return err
}

// Playlists serves /v1/playlists.
type Playlists struct {
	client socialv1.SocialServiceClient
}

func NewPlaylists(client socialv1.SocialServiceClient) *Playlists {
	return &Playlists{client: client}
}

func (c *Playlists) List(ctx context.Context, q domain.PlaylistListQuery) (domain.PlaylistPage, error) {
	resp, err := c.client.ListPlaylists(ctx, &socialv1.ListPlaylistsRequest{
		Page:     page(q.PageQuery),
		ViewerId: q.ViewerID,
		OwnerId:  q.OwnerID,
		IsPublic: q.IsPublic,
	})
	if err != nil {
		return domain.PlaylistPage{}, err
	}
	items := make([]domain.Playlist, 0, len(resp.GetItems()))
	for _, p := range resp.GetItems() {
		items = append(items, toPlaylist(p))
	}
	return domain.PlaylistPage{Items: items, NextCursor: resp.GetNextCursor(), Limit: int(resp.GetLimit())}, nil
}

func (c *Playlists) Create(ctx context.Context, ownerID string, in domain.PlaylistReplace) (domain.Playlist, error) {
	resp, err := c.client.CreatePlaylist(ctx, &socialv1.CreatePlaylistRequest{ActorId: ownerID, Title: in.Title, IsPublic: in.IsPublic})
	if err != nil {
		return domain.Playlist{}, err
	}
	return toPlaylist(resp.GetPlaylist()), nil
}

func (c *Playlists) Get(ctx context.Context, id, viewerID string) (domain.Playlist, error) {
	resp, err := c.client.GetPlaylist(ctx, &socialv1.GetPlaylistRequest{Id: id, ViewerId: viewerID})
	if err != nil {
		return domain.Playlist{}, err
	}
	return toPlaylist(resp.GetPlaylist()), nil
}

func (c *Playlists) Replace(ctx context.Context, actorID, id string, in domain.PlaylistReplace) (domain.Playlist, error) {
	resp, err := c.client.ReplacePlaylist(ctx, &socialv1.ReplacePlaylistRequest{
		ActorId:  actorID,
		Id:       id,
		Title:    in.Title,
		IsPublic: in.IsPublic,
	})
	if err != nil {
		return domain.Playlist{}, err
	}
	return toPlaylist(resp.GetPlaylist()), nil
}

func (c *Playlists) Patch(ctx context.Context, actorID, id string, in domain.PlaylistPatch) (domain.Playlist, error) {
	resp, err := c.client.PatchPlaylist(ctx, &socialv1.PatchPlaylistRequest{
		ActorId:  actorID,
		Id:       id,
		Title:    in.Title,
		IsPublic: in.IsPublic,
	})
	if err != nil {
		return domain.Playlist{}, err
	}
	return toPlaylist(resp.GetPlaylist()), nil
}

func (c *Playlists) Delete(ctx context.Context, actorID, id string) error {
	_, err := c.client.DeletePlaylist(ctx, &socialv1.DeletePlaylistRequest{ActorId: actorID, Id: id})
	return err
}

func (c *Playlists) ListTracks(ctx context.Context, id, viewerID string, q domain.PageQuery) (domain.PlaylistTrackPage, error) {
	resp, err := c.client.ListPlaylistTracks(ctx, &socialv1.ListPlaylistTracksRequest{Id: id, ViewerId: viewerID, Page: page(q)})
	if err != nil {
		return domain.PlaylistTrackPage{}, err
	}
	items := make([]domain.PlaylistTrack, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, toPlaylistTrack(item))
	}
	return domain.PlaylistTrackPage{Items: items, NextCursor: resp.GetNextCursor(), Limit: int(resp.GetLimit())}, nil
}

func (c *Playlists) AddTrack(ctx context.Context, actorID, id string, in domain.PlaylistTrackAdd) (domain.PlaylistTrack, error) {
	resp, err := c.client.AddPlaylistTrack(ctx, &socialv1.AddPlaylistTrackRequest{
		ActorId:    actorID,
		PlaylistId: id,
		TrackId:    in.TrackID,
		Position:   int64(in.Position),
	})
	if err != nil {
		return domain.PlaylistTrack{}, err
	}
	return toPlaylistTrack(resp.GetItem()), nil
}

func (c *Playlists) MoveTrack(ctx context.Context, actorID, id, trackID string, in domain.PlaylistTrackMove) (domain.PlaylistTrack, error) {
	resp, err := c.client.MovePlaylistTrack(ctx, &socialv1.MovePlaylistTrackRequest{
		ActorId:    actorID,
		PlaylistId: id,
		TrackId:    trackID,
		Position:   int64(in.Position),
	})
	if err != nil {
		return domain.PlaylistTrack{}, err
	}
	return toPlaylistTrack(resp.GetItem()), nil
}

func (c *Playlists) RemoveTrack(ctx context.Context, actorID, id, trackID string) error {
	_, err := c.client.RemovePlaylistTrack(ctx, &socialv1.RemovePlaylistTrackRequest{ActorId: actorID, PlaylistId: id, TrackId: trackID})
	return err
}
