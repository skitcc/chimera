package clients

import (
	"context"

	catalogv1 "chimera/contracts/gen/catalog/v1"
	socialv1 "chimera/contracts/gen/social/v1"
	"chimera/gateway/internal/domain"
)

// Tracks serves /v1/tracks from Catalog. Likes live in Social, so
// GET /v1/tracks?liked_by= and /v1/users/{id}/likes go there.
type Tracks struct {
	catalog catalogv1.CatalogServiceClient
	social  socialv1.SocialServiceClient
}

func NewTracks(catalog catalogv1.CatalogServiceClient, social socialv1.SocialServiceClient) *Tracks {
	return &Tracks{catalog: catalog, social: social}
}

func (c *Tracks) Search(ctx context.Context, q domain.TrackListQuery) (domain.TrackPage, error) {
	if q.LikedBy != "" {
		return c.searchLiked(ctx, q)
	}
	resp, err := c.catalog.ListTracks(ctx, &catalogv1.ListTracksRequest{
		Page:     page(q.PageQuery),
		ViewerId: q.ViewerID,
		UserId:   q.UserID,
		Artist:   q.Artist,
		Status:   string(q.Status),
	})
	if err != nil {
		return domain.TrackPage{}, err
	}
	return domain.TrackPage{Items: toTracks(resp.GetItems()), NextCursor: resp.GetNextCursor(), Limit: int(resp.GetLimit())}, nil
}

func (c *Tracks) searchLiked(ctx context.Context, q domain.TrackListQuery) (domain.TrackPage, error) {
	resp, err := c.social.ListLikedTracks(ctx, &socialv1.ListLikedTracksRequest{
		Page:     page(q.PageQuery),
		ViewerId: q.ViewerID,
		LikedBy:  q.LikedBy,
		UserId:   q.UserID,
		Artist:   q.Artist,
		Status:   string(q.Status),
	})
	if err != nil {
		return domain.TrackPage{}, err
	}
	return domain.TrackPage{Items: toTracks(resp.GetItems()), NextCursor: resp.GetNextCursor(), Limit: int(resp.GetLimit())}, nil
}

func (c *Tracks) GetByID(ctx context.Context, id, viewerID string) (domain.Track, error) {
	resp, err := c.catalog.GetTrack(ctx, &catalogv1.GetTrackRequest{Id: id, ViewerId: viewerID})
	if err != nil {
		return domain.Track{}, err
	}
	return toTrack(resp.GetTrack()), nil
}

func (c *Tracks) InitUpload(ctx context.Context, in domain.TrackUploadInit) (domain.TrackUploadSession, error) {
	resp, err := c.catalog.CreateTrack(ctx, &catalogv1.CreateTrackRequest{
		ActorId:   in.UserID,
		Title:     in.Title,
		Artist:    in.Artist,
		SizeBytes: in.SizeBytes,
	})
	if err != nil {
		return domain.TrackUploadSession{}, err
	}
	return domain.TrackUploadSession{Track: toTrack(resp.GetTrack()), UploadURL: resp.GetUploadUrl()}, nil
}

func (c *Tracks) StreamURL(ctx context.Context, id string) (string, error) {
	resp, err := c.catalog.GetStreamURL(ctx, &catalogv1.GetStreamURLRequest{Id: id})
	if err != nil {
		return "", err
	}
	return resp.GetUrl(), nil
}

func (c *Tracks) ReplaceOwned(ctx context.Context, actorID, id string, in domain.TrackReplace) (domain.Track, error) {
	resp, err := c.catalog.ReplaceTrack(ctx, &catalogv1.ReplaceTrackRequest{ActorId: actorID, Id: id, Title: in.Title, Artist: in.Artist})
	if err != nil {
		return domain.Track{}, err
	}
	return toTrack(resp.GetTrack()), nil
}

func (c *Tracks) PatchOwned(ctx context.Context, actorID, id string, in domain.TrackPatch) (domain.Track, error) {
	resp, err := c.catalog.PatchTrack(ctx, &catalogv1.PatchTrackRequest{
		ActorId: actorID,
		Id:      id,
		Title:   in.Title,
		Artist:  in.Artist,
		Publish: in.Publish,
	})
	if err != nil {
		return domain.Track{}, err
	}
	return toTrack(resp.GetTrack()), nil
}

func (c *Tracks) DeleteOwned(ctx context.Context, actorID, id string) error {
	_, err := c.catalog.DeleteTrack(ctx, &catalogv1.DeleteTrackRequest{ActorId: actorID, Id: id})
	return err
}

func (c *Tracks) PutLike(ctx context.Context, actorID string, in domain.TrackLike) error {
	_, err := c.social.PutLike(ctx, &socialv1.PutLikeRequest{ActorId: actorID, UserId: in.UserID, TrackId: in.TrackID})
	return err
}

func (c *Tracks) DeleteLike(ctx context.Context, actorID string, in domain.TrackLike) error {
	_, err := c.social.DeleteLike(ctx, &socialv1.DeleteLikeRequest{ActorId: actorID, UserId: in.UserID, TrackId: in.TrackID})
	return err
}
