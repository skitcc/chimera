package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"chimera/internal/domain"
)

func TestErrorError(t *testing.T) {
	runCase(t, "formats code and message without cause", func(t *testing.T) {
		// Arrange
		err := domain.NewError(domain.CodeInvalid, "bad input")

		// Act
		got := err.Error()

		// Assert
		assertEqual(t, "invalid: bad input", got)
	})

	runCase(t, "includes wrapped cause", func(t *testing.T) {
		// Arrange
		err := domain.Wrap(domain.CodeInternal, "operation failed", errors.New("disk unavailable"))

		// Act
		got := err.Error()

		// Assert
		assertEqual(t, "internal: operation failed: disk unavailable", got)
	})
}

func TestErrorUnwrap(t *testing.T) {
	runCase(t, "returns wrapped cause", func(t *testing.T) {
		// Arrange
		cause := errors.New("cause")
		err := domain.Wrap(domain.CodeInternal, "failed", cause)

		// Act
		got := err.Unwrap()

		// Assert
		assertEqual(t, cause, got)
	})

	runCase(t, "returns nil when error has no cause", func(t *testing.T) {
		// Arrange
		err := domain.NewError(domain.CodeInvalid, "bad input")

		// Act
		got := err.Unwrap()

		// Assert
		assertEqual[error](t, nil, got)
	})
}

func TestNewError(t *testing.T) {
	runCase(t, "constructs error with code and message", func(t *testing.T) {
		// Arrange
		code := domain.CodeConflict
		message := "already exists"

		// Act
		got := domain.NewError(code, message)

		// Assert
		assertEqual(t, code, got.Code)
		assertEqual(t, message, got.Message)
		assertEqual[error](t, nil, got.Unwrap())
	})

	runCase(t, "preserves empty message", func(t *testing.T) {
		// Arrange
		message := ""

		// Act
		got := domain.NewError(domain.CodeInvalid, message)

		// Assert
		assertEqual(t, message, got.Message)
		assertEqual(t, "invalid: ", got.Error())
	})
}

func TestWrap(t *testing.T) {
	runCase(t, "constructs error retaining cause", func(t *testing.T) {
		// Arrange
		cause := errors.New("database unavailable")

		// Act
		got := domain.Wrap(domain.CodeInternal, "lookup failed", cause)

		// Assert
		assertEqual(t, domain.CodeInternal, got.Code)
		assertEqual(t, "lookup failed", got.Message)
		assertErrorIs(t, cause, got)
	})

	runCase(t, "accepts nil cause as alternative", func(t *testing.T) {
		// Arrange
		var cause error

		// Act
		got := domain.Wrap(domain.CodeInternal, "lookup failed", cause)

		// Assert
		assertEqual[error](t, nil, got.Unwrap())
		assertEqual(t, "internal: lookup failed", got.Error())
	})
}

func TestNotFound(t *testing.T) {
	runCase(t, "uses not found code", func(t *testing.T) {
		// Arrange
		message := "track not found"

		// Act
		got := domain.NotFound(message)

		// Assert
		assertEqual(t, domain.CodeNotFound, got.Code)
		assertEqual(t, message, got.Message)
	})

	runCase(t, "creates independent errors for alternative messages", func(t *testing.T) {
		// Arrange
		firstMessage := "first"
		secondMessage := "second"

		// Act
		first := domain.NotFound(firstMessage)
		second := domain.NotFound(secondMessage)

		// Assert
		assertEqual(t, firstMessage, first.Message)
		assertEqual(t, secondMessage, second.Message)
	})
}

func TestInvalid(t *testing.T) {
	runCase(t, "uses invalid code", func(t *testing.T) {
		// Arrange
		message := "invalid request"

		// Act
		got := domain.Invalid(message)

		// Assert
		assertEqual(t, domain.CodeInvalid, got.Code)
		assertEqual(t, message, got.Message)
	})

	runCase(t, "does not attach a cause", func(t *testing.T) {
		// Arrange
		message := "invalid request"

		// Act
		got := domain.Invalid(message)

		// Assert
		assertEqual[error](t, nil, got.Unwrap())
	})
}

func TestUnauthorized(t *testing.T) {
	runCase(t, "uses unauthorized code", func(t *testing.T) {
		// Arrange
		message := "access denied"

		// Act
		got := domain.Unauthorized(message)

		// Assert
		assertEqual(t, domain.CodeUnauthorized, got.Code)
		assertEqual(t, message, got.Message)
	})

	runCase(t, "preserves empty message alternative", func(t *testing.T) {
		// Arrange
		message := ""

		// Act
		got := domain.Unauthorized(message)

		// Assert
		assertEqual(t, message, got.Message)
	})
}

func TestConflict(t *testing.T) {
	runCase(t, "uses conflict code", func(t *testing.T) {
		// Arrange
		message := "state conflict"

		// Act
		got := domain.Conflict(message)

		// Assert
		assertEqual(t, domain.CodeConflict, got.Code)
		assertEqual(t, message, got.Message)
	})

	runCase(t, "does not attach a cause", func(t *testing.T) {
		// Arrange
		message := "state conflict"

		// Act
		got := domain.Conflict(message)

		// Assert
		assertEqual[error](t, nil, got.Unwrap())
	})
}

func TestInternal(t *testing.T) {
	runCase(t, "uses internal code", func(t *testing.T) {
		// Arrange
		message := "unexpected failure"

		// Act
		got := domain.Internal(message)

		// Assert
		assertEqual(t, domain.CodeInternal, got.Code)
		assertEqual(t, message, got.Message)
	})

	runCase(t, "does not attach a cause", func(t *testing.T) {
		// Arrange
		message := "unexpected failure"

		// Act
		got := domain.Internal(message)

		// Assert
		assertEqual[error](t, nil, got.Unwrap())
	})
}

func TestAs(t *testing.T) {
	runCase(t, "extracts domain error through error chain", func(t *testing.T) {
		// Arrange
		appErr := domain.Invalid("bad input")
		err := fmt.Errorf("request failed: %w", appErr)

		// Act
		got, ok := domain.As(err)

		// Assert
		assertEqual(t, true, ok)
		assertEqual(t, appErr, got)
	})

	runCase(t, "returns false for ordinary error", func(t *testing.T) {
		// Arrange
		err := errors.New("ordinary error")

		// Act
		got, ok := domain.As(err)

		// Assert
		assertEqual(t, false, ok)
		assertEqual[*domain.Error](t, nil, got)
	})

	runCase(t, "returns false for nil error", func(t *testing.T) {
		// Arrange
		var err error

		// Act
		got, ok := domain.As(err)

		// Assert
		assertEqual(t, false, ok)
		assertEqual[*domain.Error](t, nil, got)
	})
}

func TestIs(t *testing.T) {
	runCase(t, "matches code through error chain", func(t *testing.T) {
		// Arrange
		err := fmt.Errorf("request failed: %w", domain.NotFound("missing"))

		// Act
		got := domain.Is(err, domain.CodeNotFound)

		// Assert
		assertEqual(t, true, got)
	})

	runCase(t, "rejects different code", func(t *testing.T) {
		// Arrange
		err := domain.Invalid("bad input")

		// Act
		got := domain.Is(err, domain.CodeConflict)

		// Assert
		assertEqual(t, false, got)
	})

	runCase(t, "rejects ordinary and nil errors", func(t *testing.T) {
		for name, err := range map[string]error{
			"ordinary": errors.New("ordinary"),
			"nil":      nil,
		} {
			runCase(t, name, func(t *testing.T) {
				// Arrange
				candidate := err

				// Act
				got := domain.Is(candidate, domain.CodeInternal)

				// Assert
				assertEqual(t, false, got)
			})
		}
	})
}
