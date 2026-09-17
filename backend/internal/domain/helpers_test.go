package domain_test

import (
	"errors"
	"testing"

	"chimera/internal/domain"
	"chimera/internal/testkit"
)

func runCase(t *testing.T, name string, body func(*testing.T)) {
	t.Helper()
	testkit.Run(t, name, body)
}

func assertEqual[T comparable](t *testing.T, want, got T) {
	t.Helper()
	if got != want {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertErrorCode(t *testing.T, want domain.Code, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %q error, got nil", want)
	}
	if !domain.Is(err, want) {
		t.Fatalf("expected %q error, got %v", want, err)
	}
}

func assertErrorIs(t *testing.T, want, got error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("expected error chain to contain %v, got %v", want, got)
	}
}
