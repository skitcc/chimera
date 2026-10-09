package domain_test

import (
	"testing"

	"chimera/internal/domain"
	"chimera/internal/testkit"
)

func TestPageQueryValidate(t *testing.T) {
	const component, method = "PageQuery", "Validate"

	limitCases := []struct {
		id, title string
		limit     int
		want      int
	}{
		{"DOM-PAGE-VAL-01", "defaults negative limit to 20", -1, 20},
		{"DOM-PAGE-VAL-02", "defaults zero limit to 20", 0, 20},
		{"DOM-PAGE-VAL-03", "accepts minimum limit 1", 1, 1},
		{"DOM-PAGE-VAL-04", "accepts maximum limit 100", 100, 100},
	}

	for _, tc := range limitCases {
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "a page query with the given limit and an empty cursor",
			When:      "Validate is called",
			Then:      "no error is returned and the limit becomes " + itoa(tc.want),
			Technique: testkit.TechniqueBoundary,
			Params:    map[string]string{"limit": itoa(tc.limit)},
		}, func(t *testing.T, r testkit.Report) {
			var q domain.PageQuery
			var err error
			r.Arrange(func(t *testing.T) { q = testkit.PageQueryMother().WithLimit(tc.limit).Build() })
			r.Act(func(t *testing.T) { err = q.Validate() })
			r.Assert(func(t *testing.T) {
				assertNoError(t, err)
				assertEqual(t, tc.want, q.Limit)
			})
		})
	}

	runSpec(t, component, testkit.Spec{
		ID: "DOM-PAGE-VAL-05", Method: method,
		Title:     "rejects limit 101 above maximum",
		Given:     "a page query with limit 101",
		When:      "Validate is called",
		Then:      `an invalid error "limit exceeded" is returned`,
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"limit": "101"},
	}, func(t *testing.T, r testkit.Report) {
		var q domain.PageQuery
		var err error
		r.Arrange(func(t *testing.T) { q = testkit.PageQueryMother().WithLimit(101).Build() })
		r.Act(func(t *testing.T) { err = q.Validate() })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, domain.CodeInvalid, "limit exceeded", err)
		})
	})

	validCursorCases := []struct {
		id, title, cursor string
		technique         string
	}{
		{"DOM-PAGE-VAL-06", "accepts empty cursor as first page", "", testkit.TechniqueBoundary},
		{"DOM-PAGE-VAL-07", "accepts zero cursor", "0", testkit.TechniqueBoundary},
		{"DOM-PAGE-VAL-08", "accepts positive cursor", "25", testkit.TechniqueEquivalence},
	}

	for _, tc := range validCursorCases {
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "a page query with limit 20 and the given cursor",
			When:      "Validate is called",
			Then:      "no error is returned",
			Technique: tc.technique,
			Params:    map[string]string{"cursor": tc.cursor},
		}, func(t *testing.T, r testkit.Report) {
			var q domain.PageQuery
			var err error
			r.Arrange(func(t *testing.T) { q = testkit.PageQueryMother().WithCursor(tc.cursor).Build() })
			r.Act(func(t *testing.T) { err = q.Validate() })
			r.Assert(func(t *testing.T) { assertNoError(t, err) })
		})
	}

	invalidCursorCases := []struct {
		id, title, cursor string
		technique         string
	}{
		{"DOM-PAGE-VAL-09", "rejects negative cursor", "-1", testkit.TechniqueBoundary},
		{"DOM-PAGE-VAL-10", "rejects non-numeric cursor", "abc", testkit.TechniqueErrorGuessing},
		{"DOM-PAGE-VAL-11", "rejects cursor with surrounding spaces", " 1 ", testkit.TechniqueErrorGuessing},
	}

	for _, tc := range invalidCursorCases {
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "a page query with limit 20 and the given cursor",
			When:      "Validate is called",
			Then:      `an invalid error "invalid cursor" is returned`,
			Technique: tc.technique,
			Params:    map[string]string{"cursor": tc.cursor},
		}, func(t *testing.T, r testkit.Report) {
			var q domain.PageQuery
			var err error
			r.Arrange(func(t *testing.T) { q = testkit.PageQueryMother().WithCursor(tc.cursor).Build() })
			r.Act(func(t *testing.T) { err = q.Validate() })
			r.Assert(func(t *testing.T) {
				assertDomainError(t, domain.CodeInvalid, "invalid cursor", err)
			})
		})
	}

	runSpec(t, component, testkit.Spec{
		ID: "DOM-PAGE-VAL-12", Method: method,
		Title:     "checks limit before cursor",
		Given:     "a page query with limit 101 and cursor abc",
		When:      "Validate is called",
		Then:      `the limit check wins: an invalid error "limit exceeded" is returned`,
		Technique: testkit.TechniqueDecisionTable,
		Params:    map[string]string{"limit": "101", "cursor": "abc"},
	}, func(t *testing.T, r testkit.Report) {
		var q domain.PageQuery
		var err error
		r.Arrange(func(t *testing.T) { q = testkit.PageQueryMother().WithLimit(101).WithCursor("abc").Build() })
		r.Act(func(t *testing.T) { err = q.Validate() })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, domain.CodeInvalid, "limit exceeded", err)
		})
	})
}

func TestPageQueryPage(t *testing.T) {
	const component, method = "PageQuery", "Page"

	tracks := []domain.Track{
		testkit.TrackMother().WithID("track-1").Build(),
		testkit.TrackMother().WithID("track-2").Build(),
		testkit.TrackMother().WithID("track-3").Build(),
	}

	cases := []struct {
		id, title, cursor string
		limit             int
		wantIDs           []string
		wantNext          string
		then              string
		technique         string
	}{
		{"DOM-PAGE-PAGE-01", "returns first page and next cursor", "", 2, []string{"track-1", "track-2"}, "2",
			"items track-1 and track-2 are returned with next cursor 2", testkit.TechniqueEquivalence},
		{"DOM-PAGE-PAGE-02", "returns middle slice and next cursor", "1", 1, []string{"track-2"}, "2",
			"only track-2 is returned with next cursor 2", testkit.TechniqueEquivalence},
		{"DOM-PAGE-PAGE-03", "omits next cursor on exactly last page", "1", 2, []string{"track-2", "track-3"}, "",
			"track-2 and track-3 are returned and the next cursor is empty", testkit.TechniqueBoundary},
		{"DOM-PAGE-PAGE-04", "omits next cursor when limit equals collection size", "", 3, []string{"track-1", "track-2", "track-3"}, "",
			"all three tracks are returned and the next cursor is empty", testkit.TechniqueBoundary},
		{"DOM-PAGE-PAGE-05", "returns empty page when cursor equals collection size", "3", 2, nil, "",
			"no items are returned and the next cursor is empty", testkit.TechniqueBoundary},
		{"DOM-PAGE-PAGE-06", "returns empty page when cursor exceeds collection", "100", 2, nil, "",
			"no items are returned and the next cursor is empty", testkit.TechniqueBoundary},
	}

	for _, tc := range cases {
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "a validated page query with the given limit and cursor over tracks track-1, track-2, track-3",
			When:      "Page is called",
			Then:      tc.then + ", and the page limit equals the query limit",
			Technique: tc.technique,
			Params:    map[string]string{"limit": itoa(tc.limit), "cursor": tc.cursor, "collection size": "3"},
		}, func(t *testing.T, r testkit.Report) {
			var q domain.PageQuery
			var got domain.TrackPage
			r.Arrange(func(t *testing.T) {
				q = testkit.PageQueryMother().WithLimit(tc.limit).WithCursor(tc.cursor).Build()
				assertNoError(t, q.Validate())
			})
			r.Act(func(t *testing.T) { got = q.Page(tracks) })
			r.Assert(func(t *testing.T) {
				assertEqual(t, len(tc.wantIDs), len(got.Items))
				for i, id := range tc.wantIDs {
					assertEqual(t, id, got.Items[i].ID)
				}
				assertEqual(t, tc.wantNext, got.NextCursor)
				assertEqual(t, tc.limit, got.Limit)
			})
		})
	}

	runSpec(t, component, testkit.Spec{
		ID: "DOM-PAGE-PAGE-07", Method: method,
		Title:     "returns empty page for empty collection",
		Given:     "a validated page query with limit 20 and an empty track collection",
		When:      "Page is called",
		Then:      "no items are returned, the next cursor is empty, and the limit is 20",
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"limit": "20", "collection size": "0"},
	}, func(t *testing.T, r testkit.Report) {
		var q domain.PageQuery
		var got domain.TrackPage
		r.Arrange(func(t *testing.T) {
			q = testkit.PageQueryMother().Build()
			assertNoError(t, q.Validate())
		})
		r.Act(func(t *testing.T) { got = q.Page(nil) })
		r.Assert(func(t *testing.T) {
			assertEqual(t, 0, len(got.Items))
			assertEqual(t, "", got.NextCursor)
			assertEqual(t, 20, got.Limit)
		})
	})

	runSpec(t, component, testkit.Spec{
		ID: "DOM-PAGE-PAGE-08", Method: method,
		Title:     "unvalidated zero limit yields empty page with cursor 0",
		Given:     "a page query with limit 0 on which Validate was never called, over three tracks",
		When:      "Page is called",
		Then:      "no items are returned and the next cursor is 0, pointing back at the same position",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"limit": "0", "collection size": "3"},
	}, func(t *testing.T, r testkit.Report) {
		var q domain.PageQuery
		var got domain.TrackPage
		r.Arrange(func(t *testing.T) { q = testkit.PageQueryMother().WithLimit(0).Build() })
		r.Act(func(t *testing.T) { got = q.Page(tracks) })
		r.Assert(func(t *testing.T) {
			assertEqual(t, 0, len(got.Items))
			assertEqual(t, "0", got.NextCursor)
			assertEqual(t, 0, got.Limit)
		})
	})
}
