package testkit

import (
	"strings"
	"testing"

	allure "github.com/allure-framework/allure-go/commons/gotest"
)

const (
	TechniqueBoundary      = "boundary-value analysis"
	TechniqueEquivalence   = "equivalence partitioning"
	TechniqueState         = "state-transition testing"
	TechniqueDecisionTable = "decision-table testing"
	TechniqueErrorGuessing = "error guessing"
)

// Run reports one test scenario to both Go testing and Allure.
func Run(t *testing.T, name string, body func(*testing.T)) {
	t.Helper()

	technique := techniqueFromName(name)
	allure.Test(t, name, func(a *allure.Context) {
		a.Label("testTechnique", technique)
		a.Step("Arrange, Act, Assert", func(a *allure.Context) {
			body(a.T())
		})
	},
		allure.WithParentSuite("Chimera backend"),
		allure.WithDescription("Arrange-Act-Assert scenario using "+technique+"."),
	)
}

func techniqueFromName(name string) string {
	name = strings.ToLower(name)
	switch {
	case containsAny(name, "limit", "size", "empty", "zero", "maximum", "minimum", "cursor"):
		return TechniqueBoundary
	case containsAny(name, "pending", "processing", "ready", "transition", "status"):
		return TechniqueState
	case containsAny(name, "filter", "combination", "artist", "uploader"):
		return TechniqueDecisionTable
	case containsAny(name, "error", "fails", "failure", "not found", "missing", "duplicate", "invalid"):
		return TechniqueErrorGuessing
	default:
		return TechniqueEquivalence
	}
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}
