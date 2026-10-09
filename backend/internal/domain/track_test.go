package domain_test

import (
	"strconv"
	"testing"

	"chimera/internal/domain"
	"chimera/internal/testkit"
)

func i64toa(n int64) string {
	return strconv.FormatInt(n, 10)
}

func TestTrackAudioObjectKey(t *testing.T) {
	const component, method = "Track", "AudioObjectKey"

	runSpec(t, component, testkit.Spec{
		ID: "DOM-TRK-KEY-01", Method: method,
		Title:     "returns stored object key",
		Given:     "a track with object key audio/custom.mp3",
		When:      "AudioObjectKey is called",
		Then:      "audio/custom.mp3 is returned",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"object key": "audio/custom.mp3"},
	}, func(t *testing.T, r testkit.Report) {
		var track domain.Track
		var got string
		r.Arrange(func(t *testing.T) { track = testkit.TrackMother().WithObjectKey("audio/custom.mp3").Build() })
		r.Act(func(t *testing.T) { got = track.AudioObjectKey() })
		r.Assert(func(t *testing.T) { assertEqual(t, "audio/custom.mp3", got) })
	})

	runSpec(t, component, testkit.Spec{
		ID: "DOM-TRK-KEY-02", Method: method,
		Title:     "derives object key when stored key is empty",
		Given:     "a track with id track-456 and an empty object key",
		When:      "AudioObjectKey is called",
		Then:      "track-456.mp3 is returned",
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"id": "track-456", "object key": ""},
	}, func(t *testing.T, r testkit.Report) {
		var track domain.Track
		var got string
		r.Arrange(func(t *testing.T) {
			track = testkit.TrackMother().WithID("track-456").WithObjectKey("").Build()
		})
		r.Act(func(t *testing.T) { got = track.AudioObjectKey() })
		r.Assert(func(t *testing.T) { assertEqual(t, "track-456.mp3", got) })
	})
}

func TestTrackOwnedBy(t *testing.T) {
	const component, method = "Track", "OwnedBy"

	runSpec(t, component, testkit.Spec{
		ID: "DOM-TRK-OWN-01", Method: method,
		Title:     "accepts owner",
		Given:     "a track owned by user owner",
		When:      "OwnedBy is called with owner",
		Then:      "no error is returned",
		Technique: testkit.TechniqueEquivalence,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"track owner": "owner", "caller": "owner"},
	}, func(t *testing.T, r testkit.Report) {
		var track domain.Track
		var err error
		r.Arrange(func(t *testing.T) { track = testkit.TrackMother().WithUserID("owner").Build() })
		r.Act(func(t *testing.T) { err = track.OwnedBy("owner") })
		r.Assert(func(t *testing.T) { assertNoError(t, err) })
	})

	cases := []struct {
		id, title, caller string
		technique         string
	}{
		{"DOM-TRK-OWN-02", "rejects another user", "other-user", testkit.TechniqueEquivalence},
		{"DOM-TRK-OWN-03", "rejects empty caller", "", testkit.TechniqueErrorGuessing},
	}

	for _, tc := range cases {
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "a track owned by user owner",
			When:      "OwnedBy is called with a different caller id",
			Then:      `an unauthorized error "not allowed" is returned`,
			Technique: tc.technique,
			Severity:  testkit.SeverityCritical,
			Params:    map[string]string{"track owner": "owner", "caller": tc.caller},
		}, func(t *testing.T, r testkit.Report) {
			var track domain.Track
			var err error
			r.Arrange(func(t *testing.T) { track = testkit.TrackMother().WithUserID("owner").Build() })
			r.Act(func(t *testing.T) { err = track.OwnedBy(tc.caller) })
			r.Assert(func(t *testing.T) {
				assertDomainError(t, domain.CodeUnauthorized, "not allowed", err)
			})
		})
	}

	runSpec(t, component, testkit.Spec{
		ID: "DOM-TRK-OWN-04", Method: method,
		Title:     "accepts empty caller for ownerless track",
		Given:     "a track whose owner id is empty",
		When:      "OwnedBy is called with an empty caller id",
		Then:      "no error is returned because the ids compare equal",
		Technique: testkit.TechniqueErrorGuessing,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"track owner": "", "caller": ""},
	}, func(t *testing.T, r testkit.Report) {
		var track domain.Track
		var err error
		r.Arrange(func(t *testing.T) { track = testkit.TrackMother().WithUserID("").Build() })
		r.Act(func(t *testing.T) { err = track.OwnedBy("") })
		r.Assert(func(t *testing.T) { assertNoError(t, err) })
	})
}

func TestTrackConfirmUpload(t *testing.T) {
	const component, method = "Track", "ConfirmUpload"

	acceptedCases := []struct {
		id, title        string
		expected, size   int64
		given, technique string
	}{
		{"DOM-TRK-CONFIRM-01", "accepts matching expected size", 1024, 1024, "a track expecting 1024 bytes", testkit.TechniqueEquivalence},
		{"DOM-TRK-CONFIRM-02", "accepts any positive size when expected size is absent", 0, 1, "a track with no expected size", testkit.TechniqueBoundary},
		{"DOM-TRK-CONFIRM-03", "treats negative expected size as absent", -5, 7, "a track with a negative expected size", testkit.TechniqueErrorGuessing},
	}

	for _, tc := range acceptedCases {
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     tc.given,
			When:      "ConfirmUpload is called with the uploaded size",
			Then:      "no error is returned",
			Technique: tc.technique,
			Params:    map[string]string{"expected size": i64toa(tc.expected), "uploaded size": i64toa(tc.size)},
		}, func(t *testing.T, r testkit.Report) {
			var track domain.Track
			var err error
			r.Arrange(func(t *testing.T) { track = testkit.TrackMother().WithSizeBytes(tc.expected).Build() })
			r.Act(func(t *testing.T) { err = track.ConfirmUpload(tc.size) })
			r.Assert(func(t *testing.T) { assertNoError(t, err) })
		})
	}

	nonPositiveCases := []struct {
		id, title string
		size      int64
	}{
		{"DOM-TRK-CONFIRM-04", "rejects zero uploaded size", 0},
		{"DOM-TRK-CONFIRM-05", "rejects negative uploaded size", -1},
	}

	for _, tc := range nonPositiveCases {
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "a track expecting 1024 bytes",
			When:      "ConfirmUpload is called with a non-positive size",
			Then:      `an invalid error "upload not found" is returned`,
			Technique: testkit.TechniqueBoundary,
			Params:    map[string]string{"expected size": "1024", "uploaded size": i64toa(tc.size)},
		}, func(t *testing.T, r testkit.Report) {
			var track domain.Track
			var err error
			r.Arrange(func(t *testing.T) { track = testkit.TrackMother().Build() })
			r.Act(func(t *testing.T) { err = track.ConfirmUpload(tc.size) })
			r.Assert(func(t *testing.T) {
				assertDomainError(t, domain.CodeInvalid, "upload not found", err)
			})
		})
	}

	mismatchCases := []struct {
		id, title string
		size      int64
	}{
		{"DOM-TRK-CONFIRM-06", "rejects size one byte below expected", 1023},
		{"DOM-TRK-CONFIRM-07", "rejects size one byte above expected", 1025},
	}

	for _, tc := range mismatchCases {
		want := "upload size mismatch: expected 1024, got " + i64toa(tc.size)
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "a track expecting 1024 bytes",
			When:      "ConfirmUpload is called with a different positive size",
			Then:      `an invalid error "` + want + `" is returned`,
			Technique: testkit.TechniqueBoundary,
			Params:    map[string]string{"expected size": "1024", "uploaded size": i64toa(tc.size)},
		}, func(t *testing.T, r testkit.Report) {
			var track domain.Track
			var err error
			r.Arrange(func(t *testing.T) { track = testkit.TrackMother().WithSizeBytes(1024).Build() })
			r.Act(func(t *testing.T) { err = track.ConfirmUpload(tc.size) })
			r.Assert(func(t *testing.T) {
				assertDomainError(t, domain.CodeInvalid, want, err)
			})
		})
	}
}

type transitionCase struct {
	id, title string
	from      domain.TrackStatus
	ok        bool
	technique string
}

func runTransitions(t *testing.T, method, to, conflict string, cases []transitionCase, transition func(*domain.Track) error) {
	t.Helper()
	for _, tc := range cases {
		then := "no error is returned and the status becomes " + to
		if !tc.ok {
			then = `a conflict error "` + conflict + `" is returned and the track is unchanged`
		}
		runSpec(t, "Track", testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "a track in status " + string(tc.from),
			When:      method + " is called",
			Then:      then,
			Technique: tc.technique,
			Severity:  testkit.SeverityCritical,
			Params:    map[string]string{"from": string(tc.from), "to": to},
		}, func(t *testing.T, r testkit.Report) {
			var track, before domain.Track
			var err error
			r.Arrange(func(t *testing.T) {
				track = testkit.TrackMother().WithStatus(tc.from).Build()
				before = track
			})
			r.Act(func(t *testing.T) { err = transition(&track) })
			r.Assert(func(t *testing.T) {
				if tc.ok {
					assertNoError(t, err)
					assertEqual(t, domain.TrackStatus(to), track.Status)
					return
				}
				assertDomainError(t, domain.CodeConflict, conflict, err)
				assertEqual(t, before, track)
			})
		})
	}
}

func TestTrackMarkProcessing(t *testing.T) {
	runTransitions(t, "MarkProcessing", string(domain.TrackProcessing), "track is not awaiting upload", []transitionCase{
		{"DOM-TRK-PROC-01", "transitions pending track to processing", domain.TrackPending, true, testkit.TechniqueState},
		{"DOM-TRK-PROC-02", "is idempotent for processing track", domain.TrackProcessing, true, testkit.TechniqueState},
		{"DOM-TRK-PROC-03", "rejects ready track without changing state", domain.TrackReady, false, testkit.TechniqueState},
		{"DOM-TRK-PROC-04", "rejects unknown status without changing state", "unknown", false, testkit.TechniqueErrorGuessing},
	}, (*domain.Track).MarkProcessing)
}

func TestTrackMarkReady(t *testing.T) {
	runTransitions(t, "MarkReady", string(domain.TrackReady), "track is not ready to publish", []transitionCase{
		{"DOM-TRK-READY-01", "transitions processing track to ready", domain.TrackProcessing, true, testkit.TechniqueState},
		{"DOM-TRK-READY-02", "rejects pending track without changing state", domain.TrackPending, false, testkit.TechniqueState},
		{"DOM-TRK-READY-03", "rejects repeated ready transition", domain.TrackReady, false, testkit.TechniqueState},
		{"DOM-TRK-READY-04", "rejects empty status without changing state", "", false, testkit.TechniqueErrorGuessing},
	}, (*domain.Track).MarkReady)
}

func TestTrackEnsureReady(t *testing.T) {
	const component, method = "Track", "EnsureReady"

	runSpec(t, component, testkit.Spec{
		ID: "DOM-TRK-ENSURE-01", Method: method,
		Title:     "accepts ready track",
		Given:     "a track in status ready",
		When:      "EnsureReady is called",
		Then:      "no error is returned",
		Technique: testkit.TechniqueState,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"status": "ready"},
	}, func(t *testing.T, r testkit.Report) {
		var track domain.Track
		var err error
		r.Arrange(func(t *testing.T) { track = testkit.Tracks.Ready() })
		r.Act(func(t *testing.T) { err = track.EnsureReady() })
		r.Assert(func(t *testing.T) { assertNoError(t, err) })
	})

	cases := []struct {
		id, title string
		status    domain.TrackStatus
		technique string
	}{
		{"DOM-TRK-ENSURE-02", "rejects pending track", domain.TrackPending, testkit.TechniqueState},
		{"DOM-TRK-ENSURE-03", "rejects processing track", domain.TrackProcessing, testkit.TechniqueState},
		{"DOM-TRK-ENSURE-04", "rejects unknown status", "unknown", testkit.TechniqueErrorGuessing},
	}

	for _, tc := range cases {
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "a track in status " + string(tc.status),
			When:      "EnsureReady is called",
			Then:      `a conflict error "track is not ready" is returned`,
			Technique: tc.technique,
			Severity:  testkit.SeverityCritical,
			Params:    map[string]string{"status": string(tc.status)},
		}, func(t *testing.T, r testkit.Report) {
			var track domain.Track
			var err error
			r.Arrange(func(t *testing.T) { track = testkit.TrackMother().WithStatus(tc.status).Build() })
			r.Act(func(t *testing.T) { err = track.EnsureReady() })
			r.Assert(func(t *testing.T) {
				assertDomainError(t, domain.CodeConflict, "track is not ready", err)
			})
		})
	}
}

func TestTrackIDValidate(t *testing.T) {
	const component, method = "TrackID", "Validate"

	cases := []struct {
		id, title, value string
		technique        string
	}{
		{"DOM-TRK-ID-01", "accepts non-empty id", "track-123", testkit.TechniqueEquivalence},
		{"DOM-TRK-ID-02", "accepts whitespace because it is non-empty", " ", testkit.TechniqueErrorGuessing},
	}

	for _, tc := range cases {
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "a track id with the given value",
			When:      "Validate is called",
			Then:      "no error is returned",
			Technique: tc.technique,
			Params:    map[string]string{"id": tc.value},
		}, func(t *testing.T, r testkit.Report) {
			var id domain.TrackID
			var err error
			r.Arrange(func(t *testing.T) { id = domain.TrackID(tc.value) })
			r.Act(func(t *testing.T) { err = id.Validate() })
			r.Assert(func(t *testing.T) { assertNoError(t, err) })
		})
	}

	runSpec(t, component, testkit.Spec{
		ID: "DOM-TRK-ID-03", Method: method,
		Title:     "rejects empty id",
		Given:     "an empty track id",
		When:      "Validate is called",
		Then:      `an invalid error "track id is required" is returned`,
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"id": ""},
	}, func(t *testing.T, r testkit.Report) {
		var id domain.TrackID
		var err error
		r.Arrange(func(t *testing.T) { id = domain.TrackID("") })
		r.Act(func(t *testing.T) { err = id.Validate() })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, domain.CodeInvalid, "track id is required", err)
		})
	})
}

func TestTrackWriteValidate(t *testing.T) {
	const component, method = "TrackWrite", "Validate"

	cases := []struct {
		id, title, trackTitle, artist string
		technique                     string
	}{
		{"DOM-TRK-WRITE-01", "accepts title without artist", "Test Track", "", testkit.TechniqueEquivalence},
		{"DOM-TRK-WRITE-02", "accepts whitespace title because it is non-empty", " ", "Test Artist", testkit.TechniqueErrorGuessing},
	}

	for _, tc := range cases {
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "a track write with the given title and artist",
			When:      "Validate is called",
			Then:      "no error is returned",
			Technique: tc.technique,
			Params:    map[string]string{"title": tc.trackTitle, "artist": tc.artist},
		}, func(t *testing.T, r testkit.Report) {
			var write domain.TrackWrite
			var err error
			r.Arrange(func(t *testing.T) {
				write = testkit.TrackMother().WithTitle(tc.trackTitle).WithArtist(tc.artist).BuildWrite()
			})
			r.Act(func(t *testing.T) { err = write.Validate() })
			r.Assert(func(t *testing.T) { assertNoError(t, err) })
		})
	}

	runSpec(t, component, testkit.Spec{
		ID: "DOM-TRK-WRITE-03", Method: method,
		Title:     "rejects empty title",
		Given:     "a track write with an empty title",
		When:      "Validate is called",
		Then:      `an invalid error "title is required" is returned`,
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"title": ""},
	}, func(t *testing.T, r testkit.Report) {
		var write domain.TrackWrite
		var err error
		r.Arrange(func(t *testing.T) { write = testkit.TrackMother().WithTitle("").BuildWrite() })
		r.Act(func(t *testing.T) { err = write.Validate() })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, domain.CodeInvalid, "title is required", err)
		})
	})
}

func TestTrackUploadInitValidate(t *testing.T) {
	const component, method = "TrackUploadInit", "Validate"

	acceptedCases := []struct {
		id, title string
		size      int64
		technique string
	}{
		{"DOM-TRK-INIT-01", "accepts complete upload input", 1024, testkit.TechniqueEquivalence},
		{"DOM-TRK-INIT-02", "accepts one byte size", 1, testkit.TechniqueBoundary},
	}

	for _, tc := range acceptedCases {
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "an upload init with user id, title, and the given size",
			When:      "Validate is called",
			Then:      "no error is returned",
			Technique: tc.technique,
			Params:    map[string]string{"size": i64toa(tc.size)},
		}, func(t *testing.T, r testkit.Report) {
			var in domain.TrackUploadInit
			var err error
			r.Arrange(func(t *testing.T) { in = testkit.TrackMother().WithSizeBytes(tc.size).BuildUploadInit() })
			r.Act(func(t *testing.T) { err = in.Validate() })
			r.Assert(func(t *testing.T) { assertNoError(t, err) })
		})
	}

	nonPositiveCases := []struct {
		id, title string
		size      int64
	}{
		{"DOM-TRK-INIT-03", "rejects zero size", 0},
		{"DOM-TRK-INIT-04", "rejects negative size", -1},
	}

	for _, tc := range nonPositiveCases {
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "an upload init with user id and title but a non-positive size",
			When:      "Validate is called",
			Then:      `an invalid error "size is required" is returned`,
			Technique: testkit.TechniqueBoundary,
			Params:    map[string]string{"size": i64toa(tc.size)},
		}, func(t *testing.T, r testkit.Report) {
			var in domain.TrackUploadInit
			var err error
			r.Arrange(func(t *testing.T) { in = testkit.TrackMother().WithSizeBytes(tc.size).BuildUploadInit() })
			r.Act(func(t *testing.T) { err = in.Validate() })
			r.Assert(func(t *testing.T) {
				assertDomainError(t, domain.CodeInvalid, "size is required", err)
			})
		})
	}

	fieldCases := []struct {
		id, title, given   string
		userID, trackTitle string
		size               int64
		message, technique string
	}{
		{"DOM-TRK-INIT-05", "rejects empty user id", "an upload init with an empty user id", "", "Test Track", 1024, "user id is required", testkit.TechniqueEquivalence},
		{"DOM-TRK-INIT-06", "rejects empty title", "an upload init with an empty title", "user-123", "", 1024, "title is required", testkit.TechniqueEquivalence},
		{"DOM-TRK-INIT-07", "checks user id before title and size", "an upload init with empty user id, empty title, and size 0", "", "", 0, "user id is required", testkit.TechniqueDecisionTable},
		{"DOM-TRK-INIT-08", "checks title before size", "an upload init with a user id, empty title, and size 0", "user-123", "", 0, "title is required", testkit.TechniqueDecisionTable},
	}

	for _, tc := range fieldCases {
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     tc.given,
			When:      "Validate is called",
			Then:      `an invalid error "` + tc.message + `" is returned`,
			Technique: tc.technique,
			Params:    map[string]string{"user id": tc.userID, "title": tc.trackTitle, "size": i64toa(tc.size)},
		}, func(t *testing.T, r testkit.Report) {
			var in domain.TrackUploadInit
			var err error
			r.Arrange(func(t *testing.T) {
				in = testkit.TrackMother().
					WithUserID(tc.userID).
					WithTitle(tc.trackTitle).
					WithSizeBytes(tc.size).
					BuildUploadInit()
			})
			r.Act(func(t *testing.T) { err = in.Validate() })
			r.Assert(func(t *testing.T) {
				assertDomainError(t, domain.CodeInvalid, tc.message, err)
			})
		})
	}
}

func TestTrackUploadInitValidateSize(t *testing.T) {
	const component, method = "TrackUploadInit", "ValidateSize"

	cases := []struct {
		id, title string
		size, max int64
		then      string
		technique string
	}{
		{"DOM-TRK-SIZE-01", "accepts size equal to maximum", 1024, 1024, "no error is returned", testkit.TechniqueBoundary},
		{"DOM-TRK-SIZE-02", "treats zero maximum as unlimited", 1 << 40, 0, "no error is returned because the limit is disabled", testkit.TechniqueBoundary},
		{"DOM-TRK-SIZE-03", "treats negative maximum as unlimited", 1 << 40, -1, "no error is returned because the limit is disabled", testkit.TechniqueBoundary},
	}

	for _, tc := range cases {
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "an upload init with the given size",
			When:      "ValidateSize is called with the given maximum",
			Then:      tc.then,
			Technique: tc.technique,
			Params:    map[string]string{"size": i64toa(tc.size), "max bytes": i64toa(tc.max)},
		}, func(t *testing.T, r testkit.Report) {
			var in domain.TrackUploadInit
			var err error
			r.Arrange(func(t *testing.T) { in = testkit.TrackMother().WithSizeBytes(tc.size).BuildUploadInit() })
			r.Act(func(t *testing.T) { err = in.ValidateSize(tc.max) })
			r.Assert(func(t *testing.T) { assertNoError(t, err) })
		})
	}

	runSpec(t, component, testkit.Spec{
		ID: "DOM-TRK-SIZE-04", Method: method,
		Title:     "rejects size one byte above maximum",
		Given:     "an upload init with size 1025",
		When:      "ValidateSize is called with maximum 1024",
		Then:      `an invalid error "file too large" is returned`,
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"size": "1025", "max bytes": "1024"},
	}, func(t *testing.T, r testkit.Report) {
		var in domain.TrackUploadInit
		var err error
		r.Arrange(func(t *testing.T) { in = testkit.TrackMother().WithSizeBytes(1025).BuildUploadInit() })
		r.Act(func(t *testing.T) { err = in.ValidateSize(1024) })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, domain.CodeInvalid, "file too large", err)
		})
	})
}

func TestTrackUploadInitTrack(t *testing.T) {
	const component, method = "TrackUploadInit", "Track"

	runSpec(t, component, testkit.Spec{
		ID: "DOM-TRK-TOTRACK-01", Method: method,
		Title:     "maps upload input to pending track",
		Given:     "a complete upload init",
		When:      "Track is called",
		Then:      "the track copies user id, title, artist, and size and has status pending",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var in domain.TrackUploadInit
		var got domain.Track
		r.Arrange(func(t *testing.T) { in = testkit.TrackMother().BuildUploadInit() })
		r.Act(func(t *testing.T) { got = in.Track() })
		r.Assert(func(t *testing.T) {
			assertEqual(t, in.UserID, got.UserID)
			assertEqual(t, in.Title, got.Title)
			assertEqual(t, in.Artist, got.Artist)
			assertEqual(t, in.SizeBytes, got.SizeBytes)
			assertEqual(t, domain.TrackPending, got.Status)
		})
	})

	runSpec(t, component, testkit.Spec{
		ID: "DOM-TRK-TOTRACK-02", Method: method,
		Title:     "leaves persistence-assigned fields empty",
		Given:     "a complete upload init",
		When:      "Track is called",
		Then:      "the track id and object key are empty",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var in domain.TrackUploadInit
		var got domain.Track
		r.Arrange(func(t *testing.T) { in = testkit.TrackMother().BuildUploadInit() })
		r.Act(func(t *testing.T) { got = in.Track() })
		r.Assert(func(t *testing.T) {
			assertEqual(t, "", got.ID)
			assertEqual(t, "", got.ObjectKey)
		})
	})
}

func TestTrackUploadCompleteValidate(t *testing.T) {
	const component, method = "TrackUploadComplete", "Validate"

	runSpec(t, component, testkit.Spec{
		ID: "DOM-TRK-COMPLETE-01", Method: method,
		Title:     "accepts track and user ids",
		Given:     "an upload completion with track id track-123 and user id user-123",
		When:      "Validate is called",
		Then:      "no error is returned",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"track id": "track-123", "user id": "user-123"},
	}, func(t *testing.T, r testkit.Report) {
		var in domain.TrackUploadComplete
		var err error
		r.Arrange(func(t *testing.T) { in = testkit.TrackMother().BuildUploadComplete() })
		r.Act(func(t *testing.T) { err = in.Validate() })
		r.Assert(func(t *testing.T) { assertNoError(t, err) })
	})

	cases := []struct {
		id, title, trackID, userID string
		message, technique         string
	}{
		{"DOM-TRK-COMPLETE-02", "rejects empty track id", "", "user-123", "track id is required", testkit.TechniqueEquivalence},
		{"DOM-TRK-COMPLETE-03", "rejects empty user id", "track-123", "", "user id is required", testkit.TechniqueEquivalence},
		{"DOM-TRK-COMPLETE-04", "checks track id before user id", "", "", "track id is required", testkit.TechniqueDecisionTable},
	}

	for _, tc := range cases {
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "an upload completion with the given track id and user id",
			When:      "Validate is called",
			Then:      `an invalid error "` + tc.message + `" is returned`,
			Technique: tc.technique,
			Params:    map[string]string{"track id": tc.trackID, "user id": tc.userID},
		}, func(t *testing.T, r testkit.Report) {
			var in domain.TrackUploadComplete
			var err error
			r.Arrange(func(t *testing.T) {
				in = testkit.TrackMother().WithID(tc.trackID).WithUserID(tc.userID).BuildUploadComplete()
			})
			r.Act(func(t *testing.T) { err = in.Validate() })
			r.Assert(func(t *testing.T) {
				assertDomainError(t, domain.CodeInvalid, tc.message, err)
			})
		})
	}
}

func TestTrackFeedQueryValidate(t *testing.T) {
	const component, method = "TrackFeedQuery", "Validate"

	cases := []struct {
		id, title, artist, want string
		technique               string
	}{
		{"DOM-TRK-FEED-01", "trims artist and validates page", "  Test Artist  ", "Test Artist", testkit.TechniqueEquivalence},
		{"DOM-TRK-FEED-02", "accepts blank artist as unfiltered query", "   ", "", testkit.TechniqueBoundary},
	}

	for _, tc := range cases {
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "a feed query with a valid page and the given artist",
			When:      "Validate is called",
			Then:      `no error is returned and the artist becomes "` + tc.want + `"`,
			Technique: tc.technique,
			Params:    map[string]string{"artist": tc.artist},
		}, func(t *testing.T, r testkit.Report) {
			var q domain.TrackFeedQuery
			var err error
			r.Arrange(func(t *testing.T) { q = testkit.TrackFeedQueryMother().WithArtist(tc.artist).Build() })
			r.Act(func(t *testing.T) { err = q.Validate() })
			r.Assert(func(t *testing.T) {
				assertNoError(t, err)
				assertEqual(t, tc.want, q.Artist)
			})
		})
	}

	runSpec(t, component, testkit.Spec{
		ID: "DOM-TRK-FEED-03", Method: method,
		Title:     "rejects invalid page limit",
		Given:     "a feed query with page limit 101",
		When:      "Validate is called",
		Then:      `an invalid error "limit exceeded" is returned`,
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"limit": "101"},
	}, func(t *testing.T, r testkit.Report) {
		var q domain.TrackFeedQuery
		var err error
		r.Arrange(func(t *testing.T) {
			q = testkit.TrackFeedQueryMother().WithPageQuery(testkit.PageQueryMother().WithLimit(101).Build()).Build()
		})
		r.Act(func(t *testing.T) { err = q.Validate() })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, domain.CodeInvalid, "limit exceeded", err)
		})
	})
}

func TestTrackFeedQueryFilter(t *testing.T) {
	const component, method = "TrackFeedQuery", "Filter"

	runSpec(t, component, testkit.Spec{
		ID: "DOM-TRK-FEEDFLT-01", Method: method,
		Title:     "builds ready artist filter",
		Given:     "a feed query with artist Artist",
		When:      "Filter is called",
		Then:      "the filter has status ready and artist Artist",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"artist": "Artist"},
	}, func(t *testing.T, r testkit.Report) {
		var q domain.TrackFeedQuery
		var got domain.TrackFilter
		r.Arrange(func(t *testing.T) { q = testkit.TrackFeedQueryMother().WithArtist("Artist").Build() })
		r.Act(func(t *testing.T) { got = q.Filter() })
		r.Assert(func(t *testing.T) {
			assertEqual(t, domain.TrackReady, got.Status)
			assertEqual(t, "Artist", got.Artist)
		})
	})

	runSpec(t, component, testkit.Spec{
		ID: "DOM-TRK-FEEDFLT-02", Method: method,
		Title:     "does not scope feed to a user",
		Given:     "a default feed query",
		When:      "Filter is called",
		Then:      "the filter user id is empty",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var q domain.TrackFeedQuery
		var got domain.TrackFilter
		r.Arrange(func(t *testing.T) { q = testkit.TrackFeedQueryMother().Build() })
		r.Act(func(t *testing.T) { got = q.Filter() })
		r.Assert(func(t *testing.T) { assertEqual(t, "", got.UserID) })
	})
}

func TestTrackOwnerQueryValidate(t *testing.T) {
	const component, method = "TrackOwnerQuery", "Validate"

	runSpec(t, component, testkit.Spec{
		ID: "DOM-TRK-OWNERQ-01", Method: method,
		Title:     "accepts owner and valid page",
		Given:     "an owner query with user id user-123 and a valid page",
		When:      "Validate is called",
		Then:      "no error is returned",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"user id": "user-123"},
	}, func(t *testing.T, r testkit.Report) {
		var q domain.TrackOwnerQuery
		var err error
		r.Arrange(func(t *testing.T) { q = testkit.TrackOwnerQueryMother().Build() })
		r.Act(func(t *testing.T) { err = q.Validate() })
		r.Assert(func(t *testing.T) { assertNoError(t, err) })
	})

	runSpec(t, component, testkit.Spec{
		ID: "DOM-TRK-OWNERQ-02", Method: method,
		Title:     "rejects empty owner before page validation",
		Given:     "an owner query with an empty user id and page limit 101",
		When:      "Validate is called",
		Then:      `the owner check wins: an invalid error "user id is required" is returned`,
		Technique: testkit.TechniqueDecisionTable,
		Params:    map[string]string{"user id": "", "limit": "101"},
	}, func(t *testing.T, r testkit.Report) {
		var q domain.TrackOwnerQuery
		var err error
		r.Arrange(func(t *testing.T) {
			q = testkit.TrackOwnerQueryMother().
				WithUserID("").
				WithPageQuery(testkit.PageQueryMother().WithLimit(101).Build()).
				Build()
		})
		r.Act(func(t *testing.T) { err = q.Validate() })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, domain.CodeInvalid, "user id is required", err)
		})
	})

	runSpec(t, component, testkit.Spec{
		ID: "DOM-TRK-OWNERQ-03", Method: method,
		Title:     "rejects invalid page for valid owner",
		Given:     "an owner query with user id user-123 and cursor -1",
		When:      "Validate is called",
		Then:      `an invalid error "invalid cursor" is returned`,
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"user id": "user-123", "cursor": "-1"},
	}, func(t *testing.T, r testkit.Report) {
		var q domain.TrackOwnerQuery
		var err error
		r.Arrange(func(t *testing.T) {
			q = testkit.TrackOwnerQueryMother().WithPageQuery(testkit.PageQueryMother().WithCursor("-1").Build()).Build()
		})
		r.Act(func(t *testing.T) { err = q.Validate() })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, domain.CodeInvalid, "invalid cursor", err)
		})
	})
}

func TestTrackOwnerQueryFilter(t *testing.T) {
	const component, method = "TrackOwnerQuery", "Filter"

	runSpec(t, component, testkit.Spec{
		ID: "DOM-TRK-OWNERFLT-01", Method: method,
		Title:     "builds owner and status filter",
		Given:     "an owner query for user owner with status processing",
		When:      "Filter is called",
		Then:      "the filter has user id owner and status processing",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"user id": "owner", "status": "processing"},
	}, func(t *testing.T, r testkit.Report) {
		var q domain.TrackOwnerQuery
		var got domain.TrackFilter
		r.Arrange(func(t *testing.T) {
			q = testkit.TrackOwnerQueryMother().WithUserID("owner").WithStatus(domain.TrackProcessing).Build()
		})
		r.Act(func(t *testing.T) { got = q.Filter() })
		r.Assert(func(t *testing.T) {
			assertEqual(t, "owner", got.UserID)
			assertEqual(t, domain.TrackProcessing, got.Status)
		})
	})

	runSpec(t, component, testkit.Spec{
		ID: "DOM-TRK-OWNERFLT-02", Method: method,
		Title:     "does not populate artist filter",
		Given:     "a default owner query",
		When:      "Filter is called",
		Then:      "the filter artist is empty",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var q domain.TrackOwnerQuery
		var got domain.TrackFilter
		r.Arrange(func(t *testing.T) { q = testkit.TrackOwnerQueryMother().Build() })
		r.Act(func(t *testing.T) { got = q.Filter() })
		r.Assert(func(t *testing.T) { assertEqual(t, "", got.Artist) })
	})
}

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

func TestTrackLikeListQueryValidate(t *testing.T) {
	const component, method = "TrackLikeListQuery", "Validate"

	runSpec(t, component, testkit.Spec{
		ID: "DOM-TRK-LIKELIST-01", Method: method,
		Title:     "accepts user and valid page",
		Given:     "a like list query with user id user-123 and a valid page",
		When:      "Validate is called",
		Then:      "no error is returned",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"user id": "user-123"},
	}, func(t *testing.T, r testkit.Report) {
		var q domain.TrackLikeListQuery
		var err error
		r.Arrange(func(t *testing.T) { q = testkit.TrackLikeListQueryMother().Build() })
		r.Act(func(t *testing.T) { err = q.Validate() })
		r.Assert(func(t *testing.T) { assertNoError(t, err) })
	})

	runSpec(t, component, testkit.Spec{
		ID: "DOM-TRK-LIKELIST-02", Method: method,
		Title:     "rejects empty user before page validation",
		Given:     "a like list query with an empty user id and page limit 101",
		When:      "Validate is called",
		Then:      `the user check wins: an invalid error "user id is required" is returned`,
		Technique: testkit.TechniqueDecisionTable,
		Params:    map[string]string{"user id": "", "limit": "101"},
	}, func(t *testing.T, r testkit.Report) {
		var q domain.TrackLikeListQuery
		var err error
		r.Arrange(func(t *testing.T) {
			q = testkit.TrackLikeListQueryMother().
				WithUserID("").
				WithPageQuery(testkit.PageQueryMother().WithLimit(101).Build()).
				Build()
		})
		r.Act(func(t *testing.T) { err = q.Validate() })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, domain.CodeInvalid, "user id is required", err)
		})
	})

	runSpec(t, component, testkit.Spec{
		ID: "DOM-TRK-LIKELIST-03", Method: method,
		Title:     "rejects invalid page for valid user",
		Given:     "a like list query with user id user-123 and cursor invalid",
		When:      "Validate is called",
		Then:      `an invalid error "invalid cursor" is returned`,
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"user id": "user-123", "cursor": "invalid"},
	}, func(t *testing.T, r testkit.Report) {
		var q domain.TrackLikeListQuery
		var err error
		r.Arrange(func(t *testing.T) {
			q = testkit.TrackLikeListQueryMother().WithPageQuery(testkit.PageQueryMother().WithCursor("invalid").Build()).Build()
		})
		r.Act(func(t *testing.T) { err = q.Validate() })
		r.Assert(func(t *testing.T) {
			assertDomainError(t, domain.CodeInvalid, "invalid cursor", err)
		})
	})
}
