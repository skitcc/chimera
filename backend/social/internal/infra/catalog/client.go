// Package catalog reads tracks from the Catalog service over gRPC.
package catalog

import (
	"context"

	commonv1 "chimera/contracts/gen/common/v1"
	catalogv1 "chimera/contracts/gen/catalog/v1"
	"chimera/social/internal/domain"
)

type Client struct {
	api catalogv1.CatalogServiceClient
}

func New(api catalogv1.CatalogServiceClient) *Client {
	return &Client{api: api}
}

func (c *Client) GetTrack(ctx context.Context, id string) (domain.Track, error) {
	tracks, err := c.GetTracks(ctx, []string{id})
	if err != nil {
		return domain.Track{}, err
	}
	if len(tracks) == 0 {
		return domain.Track{}, domain.NotFound("track not found")
	}
	return tracks[0], nil
}

func (c *Client) GetTracks(ctx context.Context, ids []string) ([]domain.Track, error) {
	if len(ids) == 0 {
		return []domain.Track{}, nil
	}
	res, err := c.api.BatchGetTracks(ctx, &catalogv1.BatchGetTracksRequest{Ids: ids})
	if err != nil {
		return nil, err
	}
	return fromTracks(res.GetTracks()), nil
}

func (c *Client) ListByIDs(ctx context.Context, q domain.LikedTracksQuery, ids []string) (domain.TrackPage, error) {
	res, err := c.api.ListTracks(ctx, &catalogv1.ListTracksRequest{
		Page: &commonv1.PageRequest{
			Limit:        int64(q.Page.Limit),
			Cursor:       q.Page.Cursor,
			InvalidLimit: q.Page.InvalidLimit(),
		},
		ViewerId: q.ViewerID,
		UserId:   q.UserID,
		Artist:   q.Artist,
		Status:   string(q.Status),
		TrackIds: &catalogv1.TrackIDs{Ids: ids},
	})
	if err != nil {
		return domain.TrackPage{}, err
	}
	return domain.TrackPage{
		Items:      fromTracks(res.GetItems()),
		NextCursor: res.GetNextCursor(),
		Limit:      int(res.GetLimit()),
	}, nil
}

func fromTracks(tracks []*catalogv1.Track) []domain.Track {
	out := make([]domain.Track, 0, len(tracks))
	for _, t := range tracks {
		out = append(out, domain.Track{
			ID:        t.GetId(),
			UserID:    t.GetUserId(),
			Title:     t.GetTitle(),
			Artist:    t.GetArtist(),
			SizeBytes: t.GetSizeBytes(),
			Status:    domain.TrackStatus(t.GetStatus()),
		})
	}
	return out
}
