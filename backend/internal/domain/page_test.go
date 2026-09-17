package domain_test

import (
	"strconv"
	"testing"

	"chimera/internal/domain"
	"chimera/internal/testkit"
)

func TestPageQueryValidate(t *testing.T) {
	runCase(t, "accepts maximum limit boundary and empty cursor", func(t *testing.T) {
		// Arrange
		q := testkit.PageQueryMother().WithLimit(100).Build()

		// Act
		err := q.Validate()

		// Assert
		assertNoError(t, err)
		assertEqual(t, 100, q.Limit)
	})

	runCase(t, "defaults every non-positive limit", func(t *testing.T) {
		for _, limit := range []int{0, -1} {
			runCase(t, strconv.Itoa(limit), func(t *testing.T) {
				// Arrange
				q := testkit.PageQueryMother().WithLimit(limit).Build()

				// Act
				err := q.Validate()

				// Assert
				assertNoError(t, err)
				assertEqual(t, 20, q.Limit)
			})
		}
	})

	runCase(t, "accepts zero and positive cursor boundaries", func(t *testing.T) {
		for _, cursor := range []string{"0", "25"} {
			runCase(t, cursor, func(t *testing.T) {
				// Arrange
				q := testkit.PageQueryMother().WithCursor(cursor).Build()

				// Act
				err := q.Validate()

				// Assert
				assertNoError(t, err)
			})
		}
	})

	runCase(t, "rejects limit above maximum boundary", func(t *testing.T) {
		// Arrange
		q := testkit.PageQueryMother().WithLimit(101).Build()

		// Act
		err := q.Validate()

		// Assert
		assertErrorCode(t, domain.CodeInvalid, err)
	})

	runCase(t, "rejects malformed and negative cursors", func(t *testing.T) {
		for _, cursor := range []string{"not-a-number", "-1"} {
			runCase(t, cursor, func(t *testing.T) {
				// Arrange
				q := testkit.PageQueryMother().WithCursor(cursor).Build()

				// Act
				err := q.Validate()

				// Assert
				assertErrorCode(t, domain.CodeInvalid, err)
			})
		}
	})
}

func TestPageQueryPage(t *testing.T) {
	tracks := []domain.Track{
		testkit.TrackMother().WithID("track-1").Build(),
		testkit.TrackMother().WithID("track-2").Build(),
		testkit.TrackMother().WithID("track-3").Build(),
	}

	runCase(t, "returns requested slice and next cursor", func(t *testing.T) {
		// Arrange
		q := testkit.PageQueryMother().
			WithLimit(1).
			WithCursor("1").
			Build()
		assertNoError(t, q.Validate())

		// Act
		got := q.Page(tracks)

		// Assert
		assertEqual(t, 1, len(got.Items))
		assertEqual(t, "track-2", got.Items[0].ID)
		assertEqual(t, "2", got.NextCursor)
		assertEqual(t, 1, got.Limit)
	})

	runCase(t, "omits next cursor on final page", func(t *testing.T) {
		// Arrange
		q := testkit.PageQueryMother().
			WithLimit(2).
			WithCursor("1").
			Build()
		assertNoError(t, q.Validate())

		// Act
		got := q.Page(tracks)

		// Assert
		assertEqual(t, 2, len(got.Items))
		assertEqual(t, "track-2", got.Items[0].ID)
		assertEqual(t, "track-3", got.Items[1].ID)
		assertEqual(t, "", got.NextCursor)
	})

	runCase(t, "returns empty page when cursor exceeds collection", func(t *testing.T) {
		// Arrange
		q := testkit.PageQueryMother().
			WithLimit(2).
			WithCursor("100").
			Build()
		assertNoError(t, q.Validate())

		// Act
		got := q.Page(tracks)

		// Assert
		assertEqual(t, 0, len(got.Items))
		assertEqual(t, "", got.NextCursor)
		assertEqual(t, 2, got.Limit)
	})
}
