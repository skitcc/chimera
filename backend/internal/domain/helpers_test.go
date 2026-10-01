package domain_test

import (
	"errors"
	"strconv"
	"testing"

	"chimera/internal/domain"
	"chimera/internal/testkit"
)

func runSpec(t *testing.T, component string, s testkit.Spec, body func(*testing.T, testkit.Report)) {
	t.Helper()
	s.Layer = testkit.LayerDomain
	s.Component = component
	if s.Kind == "" {
		s.Kind = testkit.KindClassic
	}
	testkit.RunSpec(t, s, body)
}

func assertEqual[T comparable](t *testing.T, want, got T) {
	t.Helper()
	if got != want {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func itoa(n int) string {
	return strconv.Itoa(n)
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

func assertDomainError(t *testing.T, code domain.Code, message string, err error) {
	t.Helper()
	assertErrorCode(t, code, err)
	assertEqual(t, string(code)+": "+message, err.Error())
}

func assertErrorIs(t *testing.T, want, got error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("expected error chain to contain %v, got %v", want, got)
	}
}
