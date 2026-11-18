package domain_test

import (
	"testing"

	"chimera/social/internal/domain"
	"chimera/social/internal/testkit"
)

func TestTrackLikeValidate(t *testing.T) {
	const component, method = "TrackLike", "Validate"

	runSpec(t, component, testkit.Spec{
		ID: "DOM-TRK-LIKE-01", Method: method,
		Title:     "accepts user and track ids",
		Given:     "a like with user id user-123 and track id track-123",
		When:      "Validate is called",
		Then:      "no error is returned",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"user id": "user-123", "track id": "track-123"},
	}, func(t *testing.T, r testkit.Report) {
		var like domain.TrackLike
		var err error
		r.Arrange(func(t *testing.T) { like = testkit.TrackLikeMother().Build() })
		r.Act(func(t *testing.T) { err = like.Validate() })
		r.Assert(func(t *testing.T) { assertNoError(t, err) })
	})

	cases := []struct {
		id, title, userID, trackID string
		message, technique         string
	}{
		{"DOM-TRK-LIKE-02", "rejects empty user id before track id", "", "", "user id is required", testkit.TechniqueDecisionTable},
		{"DOM-TRK-LIKE-03", "rejects empty track id", "user-123", "", "track id is required", testkit.TechniqueEquivalence},
	}

	for _, tc := range cases {
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "a like with the given user id and track id",
			When:      "Validate is called",
			Then:      `an invalid error "` + tc.message + `" is returned`,
			Technique: tc.technique,
			Params:    map[string]string{"user id": tc.userID, "track id": tc.trackID},
		}, func(t *testing.T, r testkit.Report) {
			var like domain.TrackLike
			var err error
			r.Arrange(func(t *testing.T) {
				like = testkit.TrackLikeMother().WithUserID(tc.userID).WithTrackID(tc.trackID).Build()
			})
			r.Act(func(t *testing.T) { err = like.Validate() })
			r.Assert(func(t *testing.T) {
				assertDomainError(t, domain.CodeInvalid, tc.message, err)
			})
		})
	}
}


func TestLikedTracksQueryValidate(t *testing.T) {
	const component, method = "LikedTracksQuery", "Validate"

	runSpec(t, component, testkit.Spec{
		ID: "DOM-TRK-LIKED-01", Method: method,
		Title:     "accepts the owner and trims the artist filter",
		Given:     "viewer user-1 asks for likes of user-1 with artist \"  Ada  \"",
		When:      "Validate is called",
		Then:      "no error is returned and the artist becomes \"Ada\"",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		q := domain.LikedTracksQuery{ViewerID: "user-1", LikedBy: "user-1", Artist: "  Ada  "}
		var err error
		r.Act(func(t *testing.T) { err = q.Validate() })
		r.Assert(func(t *testing.T) {
			assertNoError(t, err)
			assertEqual(t, "Ada", q.Artist)
		})
	})

	cases := []struct {
		id, title, viewer, likedBy string
		status                     domain.TrackStatus
		code                       domain.Code
		message                    string
	}{
		{"DOM-TRK-LIKED-02", "rejects an unknown status before the viewer check", "", "user-1", "deleted", domain.CodeInvalid, "invalid status"},
		{"DOM-TRK-LIKED-03", "rejects an anonymous viewer", "", "user-1", "", domain.CodeUnauthorized, "missing token"},
		{"DOM-TRK-LIKED-04", "forbids another viewer", "user-2", "user-1", domain.TrackReady, domain.CodeForbidden, "not allowed"},
	}
	for _, tc := range cases {
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "viewer \"" + tc.viewer + "\", liked_by " + tc.likedBy + ", status \"" + string(tc.status) + "\"",
			When:      "Validate is called",
			Then:      string(tc.code) + " \"" + tc.message + "\" is returned",
			Technique: testkit.TechniqueDecisionTable,
			Params:    map[string]string{"viewer": tc.viewer, "liked by": tc.likedBy, "status": string(tc.status)},
		}, func(t *testing.T, r testkit.Report) {
			q := domain.LikedTracksQuery{ViewerID: tc.viewer, LikedBy: tc.likedBy, Status: tc.status}
			var err error
			r.Act(func(t *testing.T) { err = q.Validate() })
			r.Assert(func(t *testing.T) { assertDomainError(t, tc.code, tc.message, err) })
		})
	}
}
