package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"chimera/internal/domain"
	"chimera/internal/testkit"
)

const errComponent = "Error"

func TestErrorError(t *testing.T) {
	const method = "Error"

	runSpec(t, errComponent, testkit.Spec{
		ID: "DOM-ERR-STR-01", Method: method,
		Title:     "formats code and message without cause",
		Given:     `an invalid error with message "bad input" and no cause`,
		When:      "Error is called",
		Then:      `the string is "invalid: bad input"`,
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"code": "invalid", "message": "bad input"},
	}, func(t *testing.T, r testkit.Report) {
		var err *domain.Error
		var got string
		r.Arrange(func(t *testing.T) { err = domain.NewError(domain.CodeInvalid, "bad input") })
		r.Act(func(t *testing.T) { got = err.Error() })
		r.Assert(func(t *testing.T) { assertEqual(t, "invalid: bad input", got) })
	})

	runSpec(t, errComponent, testkit.Spec{
		ID: "DOM-ERR-STR-02", Method: method,
		Title:     "includes wrapped cause",
		Given:     `an internal error "operation failed" wrapping cause "disk unavailable"`,
		When:      "Error is called",
		Then:      `the string is "internal: operation failed: disk unavailable"`,
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"code": "internal", "message": "operation failed", "cause": "disk unavailable"},
	}, func(t *testing.T, r testkit.Report) {
		var err *domain.Error
		var got string
		r.Arrange(func(t *testing.T) {
			err = domain.Wrap(domain.CodeInternal, "operation failed", errors.New("disk unavailable"))
		})
		r.Act(func(t *testing.T) { got = err.Error() })
		r.Assert(func(t *testing.T) { assertEqual(t, "internal: operation failed: disk unavailable", got) })
	})
}

func TestErrorUnwrap(t *testing.T) {
	const method = "Unwrap"

	runSpec(t, errComponent, testkit.Spec{
		ID: "DOM-ERR-UNWRAP-01", Method: method,
		Title:     "returns wrapped cause",
		Given:     "an internal error wrapping a cause",
		When:      "Unwrap is called",
		Then:      "the same cause instance is returned",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var cause error
		var err *domain.Error
		var got error
		r.Arrange(func(t *testing.T) {
			cause = errors.New("cause")
			err = domain.Wrap(domain.CodeInternal, "failed", cause)
		})
		r.Act(func(t *testing.T) { got = err.Unwrap() })
		r.Assert(func(t *testing.T) { assertEqual(t, cause, got) })
	})

	runSpec(t, errComponent, testkit.Spec{
		ID: "DOM-ERR-UNWRAP-02", Method: method,
		Title:     "returns nil when error has no cause",
		Given:     "an invalid error created without a cause",
		When:      "Unwrap is called",
		Then:      "nil is returned",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var err *domain.Error
		var got error
		r.Arrange(func(t *testing.T) { err = domain.NewError(domain.CodeInvalid, "bad input") })
		r.Act(func(t *testing.T) { got = err.Unwrap() })
		r.Assert(func(t *testing.T) { assertEqual[error](t, nil, got) })
	})
}

func TestNewError(t *testing.T) {
	const method = "NewError"

	runSpec(t, errComponent, testkit.Spec{
		ID: "DOM-ERR-NEW-01", Method: method,
		Title:     "constructs error with code and message",
		Given:     `code conflict and message "already exists"`,
		When:      "NewError is called",
		Then:      "the error carries code conflict, the message, and no cause",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"code": "conflict", "message": "already exists"},
	}, func(t *testing.T, r testkit.Report) {
		var got *domain.Error
		r.Act(func(t *testing.T) { got = domain.NewError(domain.CodeConflict, "already exists") })
		r.Assert(func(t *testing.T) {
			assertEqual(t, domain.CodeConflict, got.Code)
			assertEqual(t, "already exists", got.Message)
			assertEqual[error](t, nil, got.Unwrap())
		})
	})

	runSpec(t, errComponent, testkit.Spec{
		ID: "DOM-ERR-NEW-02", Method: method,
		Title:     "preserves empty message",
		Given:     "code invalid and an empty message",
		When:      "NewError is called",
		Then:      `the message stays empty and the string is "invalid: "`,
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"code": "invalid", "message": ""},
	}, func(t *testing.T, r testkit.Report) {
		var got *domain.Error
		r.Act(func(t *testing.T) { got = domain.NewError(domain.CodeInvalid, "") })
		r.Assert(func(t *testing.T) {
			assertEqual(t, "", got.Message)
			assertEqual(t, "invalid: ", got.Error())
		})
	})
}

func TestWrap(t *testing.T) {
	const method = "Wrap"

	runSpec(t, errComponent, testkit.Spec{
		ID: "DOM-ERR-WRAP-01", Method: method,
		Title:     "constructs error retaining cause",
		Given:     `code internal, message "lookup failed", and cause "database unavailable"`,
		When:      "Wrap is called",
		Then:      "the error carries the code and message and errors.Is finds the cause in its chain",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"code": "internal", "message": "lookup failed", "cause": "database unavailable"},
	}, func(t *testing.T, r testkit.Report) {
		var cause error
		var got *domain.Error
		r.Arrange(func(t *testing.T) { cause = errors.New("database unavailable") })
		r.Act(func(t *testing.T) { got = domain.Wrap(domain.CodeInternal, "lookup failed", cause) })
		r.Assert(func(t *testing.T) {
			assertEqual(t, domain.CodeInternal, got.Code)
			assertEqual(t, "lookup failed", got.Message)
			assertErrorIs(t, cause, got)
		})
	})

	runSpec(t, errComponent, testkit.Spec{
		ID: "DOM-ERR-WRAP-02", Method: method,
		Title:     "accepts nil cause",
		Given:     `code internal, message "lookup failed", and a nil cause`,
		When:      "Wrap is called",
		Then:      `Unwrap returns nil and the string is "internal: lookup failed"`,
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"code": "internal", "message": "lookup failed", "cause": "nil"},
	}, func(t *testing.T, r testkit.Report) {
		var got *domain.Error
		r.Act(func(t *testing.T) { got = domain.Wrap(domain.CodeInternal, "lookup failed", nil) })
		r.Assert(func(t *testing.T) {
			assertEqual[error](t, nil, got.Unwrap())
			assertEqual(t, "internal: lookup failed", got.Error())
		})
	})
}

func TestErrorConstructors(t *testing.T) {
	for _, tc := range []struct {
		method, idPrefix, message string
		code                      domain.Code
		build                     func(string) *domain.Error
	}{
		{"NotFound", "DOM-ERR-NOTFOUND", "track not found", domain.CodeNotFound, domain.NotFound},
		{"Invalid", "DOM-ERR-INVALID", "invalid request", domain.CodeInvalid, domain.Invalid},
		{"Unauthorized", "DOM-ERR-UNAUTH", "access denied", domain.CodeUnauthorized, domain.Unauthorized},
		{"Conflict", "DOM-ERR-CONFLICT", "state conflict", domain.CodeConflict, domain.Conflict},
		{"Internal", "DOM-ERR-INTERNAL", "unexpected failure", domain.CodeInternal, domain.Internal},
	} {
		runSpec(t, errComponent, testkit.Spec{
			ID: tc.idPrefix + "-01", Method: tc.method,
			Title:     "uses " + string(tc.code) + " code",
			Given:     `message "` + tc.message + `"`,
			When:      tc.method + " is called",
			Then:      "the error has code " + string(tc.code) + " and the given message",
			Technique: testkit.TechniqueEquivalence,
			Params:    map[string]string{"message": tc.message},
		}, func(t *testing.T, r testkit.Report) {
			var got *domain.Error
			r.Act(func(t *testing.T) { got = tc.build(tc.message) })
			r.Assert(func(t *testing.T) {
				assertEqual(t, tc.code, got.Code)
				assertEqual(t, tc.message, got.Message)
			})
		})

		runSpec(t, errComponent, testkit.Spec{
			ID: tc.idPrefix + "-02", Method: tc.method,
			Title:     "does not attach a cause to " + string(tc.code) + " error",
			Given:     `message "` + tc.message + `"`,
			When:      tc.method + " is called",
			Then:      "Unwrap returns nil",
			Technique: testkit.TechniqueEquivalence,
			Params:    map[string]string{"message": tc.message},
		}, func(t *testing.T, r testkit.Report) {
			var got *domain.Error
			r.Act(func(t *testing.T) { got = tc.build(tc.message) })
			r.Assert(func(t *testing.T) { assertEqual[error](t, nil, got.Unwrap()) })
		})
	}

	runSpec(t, errComponent, testkit.Spec{
		ID: "DOM-ERR-NOTFOUND-03", Method: "NotFound",
		Title:     "creates independent errors for different messages",
		Given:     `messages "first" and "second"`,
		When:      "NotFound is called once for each message",
		Then:      "each error keeps its own message",
		Technique: testkit.TechniqueErrorGuessing,
		Params:    map[string]string{"first message": "first", "second message": "second"},
	}, func(t *testing.T, r testkit.Report) {
		var first, second *domain.Error
		r.Act(func(t *testing.T) {
			first = domain.NotFound("first")
			second = domain.NotFound("second")
		})
		r.Assert(func(t *testing.T) {
			assertEqual(t, "first", first.Message)
			assertEqual(t, "second", second.Message)
			assertEqual(t, false, first == second)
		})
	})

	runSpec(t, errComponent, testkit.Spec{
		ID: "DOM-ERR-UNAUTH-03", Method: "Unauthorized",
		Title:     "preserves empty unauthorized message",
		Given:     "an empty message",
		When:      "Unauthorized is called",
		Then:      `the message stays empty and the string is "unauthorized: "`,
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"message": ""},
	}, func(t *testing.T, r testkit.Report) {
		var got *domain.Error
		r.Act(func(t *testing.T) { got = domain.Unauthorized("") })
		r.Assert(func(t *testing.T) {
			assertEqual(t, "", got.Message)
			assertEqual(t, "unauthorized: ", got.Error())
		})
	})
}

func TestAs(t *testing.T) {
	const method = "As"

	runSpec(t, errComponent, testkit.Spec{
		ID: "DOM-ERR-AS-01", Method: method,
		Title:     "extracts domain error through error chain",
		Given:     "an invalid domain error wrapped with fmt.Errorf %w",
		When:      "As is called",
		Then:      "ok is true and the original domain error instance is returned",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var appErr *domain.Error
		var err error
		var got *domain.Error
		var ok bool
		r.Arrange(func(t *testing.T) {
			appErr = domain.Invalid("bad input")
			err = fmt.Errorf("request failed: %w", appErr)
		})
		r.Act(func(t *testing.T) { got, ok = domain.As(err) })
		r.Assert(func(t *testing.T) {
			assertEqual(t, true, ok)
			assertEqual(t, appErr, got)
		})
	})

	for _, tc := range []struct {
		id, title, kind string
		err             error
		technique       string
	}{
		{"DOM-ERR-AS-02", "returns false for ordinary error", "ordinary", errors.New("ordinary error"), testkit.TechniqueEquivalence},
		{"DOM-ERR-AS-03", "returns false for nil error", "nil", nil, testkit.TechniqueErrorGuessing},
	} {
		runSpec(t, errComponent, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "an error that contains no domain error (" + tc.kind + ")",
			When:      "As is called",
			Then:      "ok is false and the returned error is nil",
			Technique: tc.technique,
			Params:    map[string]string{"error": tc.kind},
		}, func(t *testing.T, r testkit.Report) {
			var got *domain.Error
			var ok bool
			r.Act(func(t *testing.T) { got, ok = domain.As(tc.err) })
			r.Assert(func(t *testing.T) {
				assertEqual(t, false, ok)
				assertEqual[*domain.Error](t, nil, got)
			})
		})
	}
}

func TestIs(t *testing.T) {
	const method = "Is"

	runSpec(t, errComponent, testkit.Spec{
		ID: "DOM-ERR-IS-01", Method: method,
		Title:     "matches code through error chain",
		Given:     "a not found domain error wrapped with fmt.Errorf %w",
		When:      "Is is called with code not_found",
		Then:      "true is returned",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"code": "not_found"},
	}, func(t *testing.T, r testkit.Report) {
		var err error
		var got bool
		r.Arrange(func(t *testing.T) { err = fmt.Errorf("request failed: %w", domain.NotFound("missing")) })
		r.Act(func(t *testing.T) { got = domain.Is(err, domain.CodeNotFound) })
		r.Assert(func(t *testing.T) { assertEqual(t, true, got) })
	})

	for _, tc := range []struct {
		id, title, kind string
		err             error
		code            domain.Code
		technique       string
	}{
		{"DOM-ERR-IS-02", "rejects different code", "invalid domain error", domain.Invalid("bad input"), domain.CodeConflict, testkit.TechniqueEquivalence},
		{"DOM-ERR-IS-03", "rejects ordinary error", "ordinary", errors.New("ordinary"), domain.CodeInternal, testkit.TechniqueEquivalence},
		{"DOM-ERR-IS-04", "rejects nil error", "nil", nil, domain.CodeInternal, testkit.TechniqueErrorGuessing},
	} {
		runSpec(t, errComponent, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     "an error of kind: " + tc.kind,
			When:      "Is is called with code " + string(tc.code),
			Then:      "false is returned",
			Technique: tc.technique,
			Params:    map[string]string{"error": tc.kind, "code": string(tc.code)},
		}, func(t *testing.T, r testkit.Report) {
			var got bool
			r.Act(func(t *testing.T) { got = domain.Is(tc.err, tc.code) })
			r.Assert(func(t *testing.T) { assertEqual(t, false, got) })
		})
	}
}
