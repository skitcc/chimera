// Package grpc exposes the Catalog use cases as chimera.catalog.v1.CatalogService.
package grpc

import (
	"context"

	"chimera/catalog/internal/domain"
	catalogv1 "chimera/contracts/gen/catalog/v1"
	commonv1 "chimera/contracts/gen/common/v1"
	"chimera/platform/idempotency"
	"chimera/platform/readpref"
)

// ReplicaMethods may read a replica. The owner checks of mutations read the primary.
var ReplicaMethods = []string{
	catalogv1.CatalogService_ListTracks_FullMethodName,
	catalogv1.CatalogService_GetTrack_FullMethodName,
	catalogv1.CatalogService_BatchGetTracks_FullMethodName,
	catalogv1.CatalogService_GetStreamURL_FullMethodName,
}

type TrackService interface {
	Search(ctx context.Context, q domain.TrackListQuery) (domain.TrackPage, error)
	GetByID(ctx context.Context, id, viewerID string) (domain.Track, error)
	BatchGet(ctx context.Context, ids domain.TrackIDs) ([]domain.Track, error)
	InitUpload(ctx context.Context, in domain.TrackUploadInit) (domain.TrackUploadSession, error)
	ReplaceOwned(ctx context.Context, actorID, id string, in domain.TrackReplace) (domain.Track, error)
	PatchOwned(ctx context.Context, actorID, id string, in domain.TrackPatch) (domain.Track, error)
	DeleteOwned(ctx context.Context, actorID, id string) error
	StreamURL(ctx context.Context, id string) (string, error)
}

type Server struct {
	catalogv1.UnimplementedCatalogServiceServer
	tracks TrackService
}

func NewServer(tracks TrackService) *Server {
	return &Server{tracks: tracks}
}

func (s *Server) ListTracks(ctx context.Context, req *catalogv1.ListTracksRequest) (*catalogv1.ListTracksResponse, error) {
	p := req.GetPage()
	page, err := s.tracks.Search(ctx, domain.TrackListQuery{
		PageQuery: domain.NewPageQuery(int(p.GetLimit()), p.GetCursor(), p.GetInvalidLimit()),
		ViewerID:  req.GetViewerId(),
		UserID:    req.GetUserId(),
		Artist:    req.GetArtist(),
		Status:    domain.TrackStatus(req.GetStatus()),
		ByIDs:     req.GetTrackIds() != nil,
		TrackIDs:  req.GetTrackIds().GetIds(),
	})
	if err != nil {
		return nil, err
	}
	return &catalogv1.ListTracksResponse{
		Items:      ToTracks(page.Items),
		NextCursor: page.NextCursor,
		Limit:      int64(page.Limit),
	}, nil
}

func (s *Server) GetTrack(ctx context.Context, req *catalogv1.GetTrackRequest) (*catalogv1.GetTrackResponse, error) {
	track, err := s.tracks.GetByID(withConsistency(ctx, req.GetConsistency()), req.GetId(), req.GetViewerId())
	if err != nil {
		return nil, err
	}
	return &catalogv1.GetTrackResponse{Track: ToTrack(track)}, nil
}

func (s *Server) BatchGetTracks(ctx context.Context, req *catalogv1.BatchGetTracksRequest) (*catalogv1.BatchGetTracksResponse, error) {
	tracks, err := s.tracks.BatchGet(withConsistency(ctx, req.GetConsistency()), req.GetIds())
	if err != nil {
		return nil, err
	}
	return &catalogv1.BatchGetTracksResponse{Tracks: ToTracks(tracks)}, nil
}

func (s *Server) CreateTrack(ctx context.Context, req *catalogv1.CreateTrackRequest) (*catalogv1.CreateTrackResponse, error) {
	ctx, err := idempotency.With(ctx, req.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	session, err := s.tracks.InitUpload(ctx, domain.TrackUploadInit{
		UserID:    req.GetActorId(),
		Title:     req.GetTitle(),
		Artist:    req.GetArtist(),
		SizeBytes: req.GetSizeBytes(),
	})
	if err != nil {
		return nil, err
	}
	return &catalogv1.CreateTrackResponse{Track: ToTrack(session.Track), UploadUrl: session.UploadURL}, nil
}

func (s *Server) ReplaceTrack(ctx context.Context, req *catalogv1.ReplaceTrackRequest) (*catalogv1.ReplaceTrackResponse, error) {
	track, err := s.tracks.ReplaceOwned(ctx, req.GetActorId(), req.GetId(), domain.TrackReplace{
		Title:  req.GetTitle(),
		Artist: req.GetArtist(),
	})
	if err != nil {
		return nil, err
	}
	return &catalogv1.ReplaceTrackResponse{Track: ToTrack(track)}, nil
}

func (s *Server) PatchTrack(ctx context.Context, req *catalogv1.PatchTrackRequest) (*catalogv1.PatchTrackResponse, error) {
	track, err := s.tracks.PatchOwned(ctx, req.GetActorId(), req.GetId(), domain.TrackPatch{
		Title:   req.Title,
		Artist:  req.Artist,
		Publish: req.GetPublish(),
	})
	if err != nil {
		return nil, err
	}
	return &catalogv1.PatchTrackResponse{Track: ToTrack(track)}, nil
}

func (s *Server) DeleteTrack(ctx context.Context, req *catalogv1.DeleteTrackRequest) (*catalogv1.DeleteTrackResponse, error) {
	if err := s.tracks.DeleteOwned(ctx, req.GetActorId(), req.GetId()); err != nil {
		return nil, err
	}
	return &catalogv1.DeleteTrackResponse{}, nil
}

func (s *Server) GetStreamURL(ctx context.Context, req *catalogv1.GetStreamURLRequest) (*catalogv1.GetStreamURLResponse, error) {
	url, err := s.tracks.StreamURL(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &catalogv1.GetStreamURLResponse{Url: url}, nil
}

func ToTrack(t domain.Track) *catalogv1.Track {
	return &catalogv1.Track{
		Id:        t.ID,
		UserId:    t.UserID,
		Title:     t.Title,
		Artist:    t.Artist,
		Status:    string(t.Status),
		SizeBytes: t.SizeBytes,
	}
}

func ToTracks(tracks []domain.Track) []*catalogv1.Track {
	out := make([]*catalogv1.Track, 0, len(tracks))
	for _, t := range tracks {
		out = append(out, ToTrack(t))
	}
	return out
}

// withConsistency sends a STRONG read to the primary.
func withConsistency(ctx context.Context, c commonv1.Consistency) context.Context {
	if c == commonv1.Consistency_CONSISTENCY_STRONG {
		return readpref.Primary(ctx)
	}
	return ctx
}
