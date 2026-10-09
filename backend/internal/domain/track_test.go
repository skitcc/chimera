package domain_test

import (
	"strconv"
	"testing"

	"chimera/internal/domain"
	"chimera/internal/testkit"
)

func TestTrackAudioObjectKey(t *testing.T) {
	runCase(t, "returns stored object key", func(t *testing.T) {
		// Arrange
		track := testkit.TrackMother().
			WithObjectKey("audio/custom.mp3").
			Build()

		// Act
		got := track.AudioObjectKey()

		// Assert
		assertEqual(t, "audio/custom.mp3", got)
	})

	runCase(t, "derives object key when stored key is empty", func(t *testing.T) {
		// Arrange
		track := testkit.TrackMother().
			WithID("track-456").
			WithObjectKey("").
			Build()

		// Act
		got := track.AudioObjectKey()

		// Assert
		assertEqual(t, "track-456.mp3", got)
	})
}

func TestTrackOwnedBy(t *testing.T) {
	runCase(t, "accepts owner", func(t *testing.T) {
		// Arrange
		track := testkit.TrackMother().WithUserID("owner").Build()

		// Act
		err := track.OwnedBy("owner")

		// Assert
		assertNoError(t, err)
	})

	runCase(t, "rejects another user", func(t *testing.T) {
		// Arrange
		track := testkit.TrackMother().WithUserID("owner").Build()

		// Act
		err := track.OwnedBy("other-user")

		// Assert
		assertErrorCode(t, domain.CodeForbidden, err)
	})
}

func TestTrackVisibleTo(t *testing.T) {
	runCase(t, "shows a ready track to a guest", func(t *testing.T) {
		// Arrange
		track := testkit.TrackMother().WithStatus(domain.TrackReady).Build()

		// Act
		err := track.VisibleTo("")

		// Assert
		assertNoError(t, err)
	})

	runCase(t, "hides a draft from a guest", func(t *testing.T) {
		// Arrange
		track := testkit.TrackMother().WithUserID("owner").WithStatus(domain.TrackPending).Build()

		// Act
		err := track.VisibleTo("")

		// Assert
		assertErrorCode(t, domain.CodeNotFound, err)
	})

	runCase(t, "shows a draft to its owner", func(t *testing.T) {
		// Arrange
		track := testkit.TrackMother().WithUserID("owner").WithStatus(domain.TrackPending).Build()

		// Act
		err := track.VisibleTo("owner")

		// Assert
		assertNoError(t, err)
	})

	runCase(t, "hides a draft from another user", func(t *testing.T) {
		// Arrange
		track := testkit.TrackMother().WithUserID("owner").WithStatus(domain.TrackProcessing).Build()

		// Act
		err := track.VisibleTo("other")

		// Assert
		assertErrorCode(t, domain.CodeNotFound, err)
	})
}

func TestTrackConfirmUpload(t *testing.T) {
	runCase(t, "accepts matching expected size", func(t *testing.T) {
		// Arrange
		track := testkit.TrackMother().WithSizeBytes(1024).Build()

		// Act
		err := track.ConfirmUpload(1024)

		// Assert
		assertNoError(t, err)
	})

	runCase(t, "accepts any positive size when expected size is absent", func(t *testing.T) {
		// Arrange
		track := testkit.TrackMother().WithSizeBytes(0).Build()

		// Act
		err := track.ConfirmUpload(1)

		// Assert
		assertNoError(t, err)
	})

	runCase(t, "rejects zero and negative sizes", func(t *testing.T) {
		for _, size := range []int64{0, -1} {
			runCase(t, strconv.FormatInt(size, 10), func(t *testing.T) {
				// Arrange
				track := testkit.TrackMother().Build()

				// Act
				err := track.ConfirmUpload(size)

				// Assert
				assertErrorCode(t, domain.CodeInvalid, err)
			})
		}
	})

	runCase(t, "rejects positive size mismatch", func(t *testing.T) {
		// Arrange
		track := testkit.TrackMother().WithSizeBytes(1024).Build()

		// Act
		err := track.ConfirmUpload(1023)

		// Assert
		assertErrorCode(t, domain.CodeInvalid, err)
		assertEqual(t, "invalid: upload size mismatch: expected 1024, got 1023", err.Error())
	})
}

func TestTrackMarkProcessing(t *testing.T) {
	runCase(t, "transitions pending track to processing", func(t *testing.T) {
		// Arrange
		track := testkit.Tracks.Pending()

		// Act
		err := track.MarkProcessing()

		// Assert
		assertNoError(t, err)
		assertEqual(t, domain.TrackProcessing, track.Status)
	})

	runCase(t, "is idempotent for processing track", func(t *testing.T) {
		// Arrange
		track := testkit.Tracks.Processing()

		// Act
		err := track.MarkProcessing()

		// Assert
		assertNoError(t, err)
		assertEqual(t, domain.TrackProcessing, track.Status)
	})

	runCase(t, "rejects ready track without changing state", func(t *testing.T) {
		// Arrange
		track := testkit.Tracks.Ready()

		// Act
		err := track.MarkProcessing()

		// Assert
		assertErrorCode(t, domain.CodeConflict, err)
		assertEqual(t, domain.TrackReady, track.Status)
	})
}

func TestTrackMarkReady(t *testing.T) {
	runCase(t, "transitions processing track to ready", func(t *testing.T) {
		// Arrange
		track := testkit.TrackMother().WithStatus(domain.TrackProcessing).Build()

		// Act
		err := track.MarkReady()

		// Assert
		assertNoError(t, err)
		assertEqual(t, domain.TrackReady, track.Status)
	})

	runCase(t, "rejects pending track without changing state", func(t *testing.T) {
		// Arrange
		track := testkit.TrackMother().WithStatus(domain.TrackPending).Build()

		// Act
		err := track.MarkReady()

		// Assert
		assertErrorCode(t, domain.CodeConflict, err)
		assertEqual(t, domain.TrackPending, track.Status)
	})

	runCase(t, "rejects repeated ready transition", func(t *testing.T) {
		// Arrange
		track := testkit.TrackMother().WithStatus(domain.TrackReady).Build()

		// Act
		err := track.MarkReady()

		// Assert
		assertErrorCode(t, domain.CodeConflict, err)
		assertEqual(t, domain.TrackReady, track.Status)
	})
}

func TestTrackEnsureReady(t *testing.T) {
	runCase(t, "accepts ready track", func(t *testing.T) {
		// Arrange
		track := testkit.TrackMother().WithStatus(domain.TrackReady).Build()

		// Act
		err := track.EnsureReady()

		// Assert
		assertNoError(t, err)
	})

	runCase(t, "rejects every non-ready state", func(t *testing.T) {
		for _, status := range []domain.TrackStatus{domain.TrackPending, domain.TrackProcessing, "unknown"} {
			runCase(t, string(status), func(t *testing.T) {
				// Arrange
				track := testkit.TrackMother().WithStatus(status).Build()

				// Act
				err := track.EnsureReady()

				// Assert
				assertErrorCode(t, domain.CodeConflict, err)
			})
		}
	})
}

func TestTrackIDValidate(t *testing.T) {
	runCase(t, "accepts non-empty id", func(t *testing.T) {
		// Arrange
		id := domain.TrackID("track-123")

		// Act
		err := id.Validate()

		// Assert
		assertNoError(t, err)
	})

	runCase(t, "accepts whitespace because it is non-empty", func(t *testing.T) {
		// Arrange
		id := domain.TrackID(" ")

		// Act
		err := id.Validate()

		// Assert
		assertNoError(t, err)
	})

	runCase(t, "rejects empty id", func(t *testing.T) {
		// Arrange
		id := domain.TrackID("")

		// Act
		err := id.Validate()

		// Assert
		assertErrorCode(t, domain.CodeInvalid, err)
	})
}

func TestTrackWriteValidate(t *testing.T) {
	runCase(t, "accepts title with or without artist", func(t *testing.T) {
		// Arrange
		write := testkit.TrackMother().
			WithArtist("").
			BuildWrite()

		// Act
		err := write.Validate()

		// Assert
		assertNoError(t, err)
	})

	runCase(t, "accepts whitespace title because it is non-empty", func(t *testing.T) {
		// Arrange
		write := testkit.TrackMother().WithTitle(" ").BuildWrite()

		// Act
		err := write.Validate()

		// Assert
		assertNoError(t, err)
	})

	runCase(t, "rejects empty title", func(t *testing.T) {
		// Arrange
		write := testkit.TrackMother().WithTitle("").BuildWrite()

		// Act
		err := write.Validate()

		// Assert
		assertErrorCode(t, domain.CodeInvalid, err)
	})
}

func TestTrackUploadInitValidate(t *testing.T) {
	runCase(t, "accepts complete upload input", func(t *testing.T) {
		// Arrange
		in := testkit.TrackMother().BuildUploadInit()

		// Act
		err := in.Validate()

		// Assert
		assertNoError(t, err)
	})

	runCase(t, "accepts one byte size boundary", func(t *testing.T) {
		// Arrange
		in := testkit.TrackMother().WithSizeBytes(1).BuildUploadInit()

		// Act
		err := in.Validate()

		// Assert
		assertNoError(t, err)
	})

	runCase(t, "rejects empty user id", func(t *testing.T) {
		// Arrange
		in := testkit.TrackMother().WithUserID("").BuildUploadInit()

		// Act
		err := in.Validate()

		// Assert
		assertErrorCode(t, domain.CodeInvalid, err)
	})

	runCase(t, "rejects empty title", func(t *testing.T) {
		// Arrange
		in := testkit.TrackMother().WithTitle("").BuildUploadInit()

		// Act
		err := in.Validate()

		// Assert
		assertErrorCode(t, domain.CodeInvalid, err)
	})

	runCase(t, "rejects non-positive size", func(t *testing.T) {
		for _, size := range []int64{0, -1} {
			runCase(t, strconv.FormatInt(size, 10), func(t *testing.T) {
				// Arrange
				in := testkit.TrackMother().WithSizeBytes(size).BuildUploadInit()

				// Act
				err := in.Validate()

				// Assert
				assertErrorCode(t, domain.CodeInvalid, err)
			})
		}
	})
}

func TestTrackUploadInitValidateSize(t *testing.T) {
	runCase(t, "accepts size equal to maximum boundary", func(t *testing.T) {
		// Arrange
		in := testkit.TrackMother().WithSizeBytes(1024).BuildUploadInit()

		// Act
		err := in.ValidateSize(1024)

		// Assert
		assertNoError(t, err)
	})

	runCase(t, "treats non-positive maximum as unlimited", func(t *testing.T) {
		// Arrange
		in := testkit.TrackMother().WithSizeBytes(1024).BuildUploadInit()

		// Act
		err := in.ValidateSize(0)

		// Assert
		assertNoError(t, err)
	})

	runCase(t, "rejects size above maximum boundary", func(t *testing.T) {
		// Arrange
		in := testkit.TrackMother().WithSizeBytes(1025).BuildUploadInit()

		// Act
		err := in.ValidateSize(1024)

		// Assert
		assertErrorCode(t, domain.CodeInvalid, err)
	})
}

func TestTrackUploadInitTrack(t *testing.T) {
	runCase(t, "maps upload input to pending track", func(t *testing.T) {
		// Arrange
		in := testkit.TrackMother().BuildUploadInit()

		// Act
		got := in.Track()

		// Assert
		assertEqual(t, in.UserID, got.UserID)
		assertEqual(t, in.Title, got.Title)
		assertEqual(t, in.Artist, got.Artist)
		assertEqual(t, in.SizeBytes, got.SizeBytes)
		assertEqual(t, domain.TrackPending, got.Status)
	})

	runCase(t, "leaves persistence-assigned fields empty", func(t *testing.T) {
		// Arrange
		in := testkit.TrackMother().BuildUploadInit()

		// Act
		got := in.Track()

		// Assert
		assertEqual(t, "", got.ID)
		assertEqual(t, "", got.ObjectKey)
	})
}

func TestTrackUploadCompleteValidate(t *testing.T) {
	runCase(t, "accepts track and user ids", func(t *testing.T) {
		// Arrange
		in := testkit.TrackMother().BuildUploadComplete()

		// Act
		err := in.Validate()

		// Assert
		assertNoError(t, err)
	})

	runCase(t, "rejects empty track id", func(t *testing.T) {
		// Arrange
		in := testkit.TrackMother().WithID("").BuildUploadComplete()

		// Act
		err := in.Validate()

		// Assert
		assertErrorCode(t, domain.CodeInvalid, err)
	})

	runCase(t, "rejects empty user id", func(t *testing.T) {
		// Arrange
		in := testkit.TrackMother().WithUserID("").BuildUploadComplete()

		// Act
		err := in.Validate()

		// Assert
		assertErrorCode(t, domain.CodeInvalid, err)
	})
}

func TestTrackFeedQueryValidate(t *testing.T) {
	runCase(t, "trims artist and validates page", func(t *testing.T) {
		// Arrange
		q := testkit.TrackFeedQueryMother().
			WithArtist("  Test Artist  ").
			Build()

		// Act
		err := q.Validate()

		// Assert
		assertNoError(t, err)
		assertEqual(t, "Test Artist", q.Artist)
	})

	runCase(t, "accepts empty artist as unfiltered query", func(t *testing.T) {
		// Arrange
		q := testkit.TrackFeedQueryMother().WithArtist("   ").Build()

		// Act
		err := q.Validate()

		// Assert
		assertNoError(t, err)
		assertEqual(t, "", q.Artist)
	})

	runCase(t, "rejects invalid page limit", func(t *testing.T) {
		// Arrange
		pageQuery := testkit.PageQueryMother().WithLimit(101).Build()
		q := testkit.TrackFeedQueryMother().WithPageQuery(pageQuery).Build()

		// Act
		err := q.Validate()

		// Assert
		assertErrorCode(t, domain.CodeInvalid, err)
	})
}

func TestTrackFeedQueryFilter(t *testing.T) {
	runCase(t, "builds ready artist filter", func(t *testing.T) {
		// Arrange
		q := testkit.TrackFeedQueryMother().WithArtist("Artist").Build()

		// Act
		got := q.Filter()

		// Assert
		assertEqual(t, domain.TrackReady, got.Status)
		assertEqual(t, "Artist", got.Artist)
	})

	runCase(t, "does not scope feed to a user", func(t *testing.T) {
		// Arrange
		q := testkit.TrackFeedQueryMother().Build()

		// Act
		got := q.Filter()

		// Assert
		assertEqual(t, "", got.UserID)
	})
}

func TestTrackOwnerQueryValidate(t *testing.T) {
	runCase(t, "accepts owner and valid page", func(t *testing.T) {
		// Arrange
		q := testkit.TrackOwnerQueryMother().Build()

		// Act
		err := q.Validate()

		// Assert
		assertNoError(t, err)
	})

	runCase(t, "rejects empty owner before page validation", func(t *testing.T) {
		// Arrange
		pageQuery := testkit.PageQueryMother().WithLimit(101).Build()
		q := testkit.TrackOwnerQueryMother().
			WithUserID("").
			WithPageQuery(pageQuery).
			Build()

		// Act
		err := q.Validate()

		// Assert
		assertErrorCode(t, domain.CodeInvalid, err)
		assertEqual(t, "invalid: user id is required", err.Error())
	})

	runCase(t, "rejects invalid page for valid owner", func(t *testing.T) {
		// Arrange
		pageQuery := testkit.PageQueryMother().WithCursor("-1").Build()
		q := testkit.TrackOwnerQueryMother().WithPageQuery(pageQuery).Build()

		// Act
		err := q.Validate()

		// Assert
		assertErrorCode(t, domain.CodeInvalid, err)
	})
}

func TestTrackOwnerQueryFilter(t *testing.T) {
	runCase(t, "builds owner and status filter", func(t *testing.T) {
		// Arrange
		q := testkit.TrackOwnerQueryMother().
			WithUserID("owner").
			WithStatus(domain.TrackProcessing).
			Build()

		// Act
		got := q.Filter()

		// Assert
		assertEqual(t, "owner", got.UserID)
		assertEqual(t, domain.TrackProcessing, got.Status)
	})

	runCase(t, "does not populate artist filter", func(t *testing.T) {
		// Arrange
		q := testkit.TrackOwnerQueryMother().Build()

		// Act
		got := q.Filter()

		// Assert
		assertEqual(t, "", got.Artist)
	})
}

func TestTrackLikeValidate(t *testing.T) {
	runCase(t, "accepts user and track ids", func(t *testing.T) {
		// Arrange
		like := testkit.TrackLikeMother().Build()

		// Act
		err := like.Validate()

		// Assert
		assertNoError(t, err)
	})

	runCase(t, "rejects empty user id before track id", func(t *testing.T) {
		// Arrange
		like := testkit.TrackLikeMother().
			WithUserID("").
			WithTrackID("").
			Build()

		// Act
		err := like.Validate()

		// Assert
		assertErrorCode(t, domain.CodeInvalid, err)
		assertEqual(t, "invalid: user id is required", err.Error())
	})

	runCase(t, "rejects empty track id", func(t *testing.T) {
		// Arrange
		like := testkit.TrackLikeMother().WithTrackID("").Build()

		// Act
		err := like.Validate()

		// Assert
		assertErrorCode(t, domain.CodeInvalid, err)
	})
}

func TestTrackLikeListQueryValidate(t *testing.T) {
	runCase(t, "accepts user and valid page", func(t *testing.T) {
		// Arrange
		q := testkit.TrackLikeListQueryMother().Build()

		// Act
		err := q.Validate()

		// Assert
		assertNoError(t, err)
	})

	runCase(t, "rejects empty user before page validation", func(t *testing.T) {
		// Arrange
		pageQuery := testkit.PageQueryMother().WithLimit(101).Build()
		q := testkit.TrackLikeListQueryMother().
			WithUserID("").
			WithPageQuery(pageQuery).
			Build()

		// Act
		err := q.Validate()

		// Assert
		assertErrorCode(t, domain.CodeInvalid, err)
		assertEqual(t, "invalid: user id is required", err.Error())
	})

	runCase(t, "rejects invalid page for valid user", func(t *testing.T) {
		// Arrange
		pageQuery := testkit.PageQueryMother().WithCursor("invalid").Build()
		q := testkit.TrackLikeListQueryMother().WithPageQuery(pageQuery).Build()

		// Act
		err := q.Validate()

		// Assert
		assertErrorCode(t, domain.CodeInvalid, err)
	})
}
