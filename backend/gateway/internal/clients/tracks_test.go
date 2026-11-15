package clients

import (
	"context"
	"testing"

	"google.golang.org/grpc"

	catalogv1 "chimera/contracts/gen/catalog/v1"
	socialv1 "chimera/contracts/gen/social/v1"
	"chimera/gateway/internal/domain"
)

// Only the methods a test expects are implemented; any other call panics
// through the nil embedded interface.
type spyCatalog struct {
	catalogv1.CatalogServiceClient
	list *catalogv1.ListTracksRequest
}

func (s *spyCatalog) ListTracks(_ context.Context, in *catalogv1.ListTracksRequest, _ ...grpc.CallOption) (*catalogv1.ListTracksResponse, error) {
	s.list = in
	return &catalogv1.ListTracksResponse{Items: []*catalogv1.Track{{Id: "track-1", Status: "ready"}}, Limit: 20}, nil
}

type spySocial struct {
	socialv1.SocialServiceClient
	liked *socialv1.ListLikedTracksRequest
}

func (s *spySocial) ListLikedTracks(_ context.Context, in *socialv1.ListLikedTracksRequest, _ ...grpc.CallOption) (*socialv1.ListLikedTracksResponse, error) {
	s.liked = in
	return &socialv1.ListLikedTracksResponse{Items: []*catalogv1.Track{{Id: "track-2", Status: "ready"}}, Limit: 5}, nil
}

func TestTracksSearchRouting(t *testing.T) {
	t.Run("plain listing goes to catalog", func(t *testing.T) {
		catalog, social := &spyCatalog{}, &spySocial{}
		page, err := NewTracks(catalog, social).Search(context.Background(), domain.TrackListQuery{
			PageQuery: domain.NewPageQuery(0, "", true),
			ViewerID:  "user-1",
			UserID:    "user-1",
			Status:    "pending",
		})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if social.liked != nil {
			t.Fatal("social was called for a plain listing")
		}
		if catalog.list.GetViewerId() != "user-1" || catalog.list.GetStatus() != "pending" || !catalog.list.GetPage().GetInvalidLimit() {
			t.Fatalf("catalog request = %v", catalog.list)
		}
		if catalog.list.GetTrackIds() != nil {
			t.Fatal("plain listing must not restrict track ids")
		}
		if len(page.Items) != 1 || page.Items[0].ID != "track-1" || page.Limit != 20 {
			t.Fatalf("page = %+v", page)
		}
	})

	t.Run("liked_by goes to social", func(t *testing.T) {
		catalog, social := &spyCatalog{}, &spySocial{}
		page, err := NewTracks(catalog, social).Search(context.Background(), domain.TrackListQuery{
			PageQuery: domain.NewPageQuery(5, "10", false),
			ViewerID:  "user-1",
			LikedBy:   "user-1",
			Artist:    "Ada",
		})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if catalog.list != nil {
			t.Fatal("catalog was called directly for liked_by")
		}
		if social.liked.GetLikedBy() != "user-1" || social.liked.GetViewerId() != "user-1" || social.liked.GetArtist() != "Ada" {
			t.Fatalf("social request = %v", social.liked)
		}
		if social.liked.GetPage().GetLimit() != 5 || social.liked.GetPage().GetCursor() != "10" {
			t.Fatalf("social page = %v", social.liked.GetPage())
		}
		if len(page.Items) != 1 || page.Items[0].ID != "track-2" || page.Limit != 5 {
			t.Fatalf("page = %+v", page)
		}
	})
}
