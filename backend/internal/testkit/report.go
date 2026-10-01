package testkit

import (
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
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

const (
	LayerDomain  = "Domain"
	LayerUsecase = "Business logic"
	LayerData    = "Data access"
	LayerE2E     = "End to end"
)

const (
	KindClassic     = "classic"
	KindLondon      = "london"
	KindStub        = "stub"
	KindPgxmock     = "pgxmock"
	KindIntegration = "integration"
	KindE2E         = "e2e"
)

const (
	SeverityBlocker  = "blocker"
	SeverityCritical = "critical"
	SeverityNormal   = "normal"
	SeverityMinor    = "minor"
)

const specFile = "backend/TEST_SPEC.md"

var techniques = map[string]bool{
	TechniqueBoundary:      true,
	TechniqueEquivalence:   true,
	TechniqueState:         true,
	TechniqueDecisionTable: true,
	TechniqueErrorGuessing: true,
}

var (
	seenMu sync.Mutex
	seen   = map[string]string{}
)

// Spec describes one test case for Allure and for the traceability matrix in TEST_SPEC.md.
type Spec struct {
	ID        string
	Layer     string
	Component string
	Method    string
	Title     string
	Given     string
	When      string
	Then      string
	Technique string
	Severity  string
	Kind      string
	Params    map[string]string
}

// Report records Arrange, Act, and Assert as separate Allure steps.
type Report struct {
	t *testing.T
	a *allure.Context
}

// RunSpec runs body as a Go subtest named after s.Title and reports it to Allure.
func RunSpec(t *testing.T, s Spec, body func(*testing.T, Report)) {
	t.Helper()
	if s.Severity == "" {
		s.Severity = SeverityNormal
	}
	if err := s.problem(); err != "" {
		t.Fatalf("spec %q: %s", s.ID, err)
	}
	if other := claim(s.ID, t.Name()+"/"+s.Title); other != "" {
		t.Fatalf("spec id %s is already used by %s", s.ID, other)
	}

	src := callerSource()
	allure.Test(t, s.Title, func(a *allure.Context) {
		body(a.T(), Report{t: a.T(), a: a})
	}, s.options(src, t.Name()+"/"+strings.ReplaceAll(s.Title, " ", "_"))...)
}

type source struct {
	pkg  string
	file string
}

// callerSource finds the Go test function that declared the spec. allure-go
// derives the title path from its direct caller, which is always this file.
func callerSource() source {
	pcs := make([]uintptr, 32)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs)])
	for {
		f, more := frames.Next()
		if strings.Contains(f.Function[strings.LastIndex(f.Function, "/")+1:], ".Test") {
			file := moduleRelative(f.File)
			return source{pkg: path.Dir(file), file: file + ":" + strconv.Itoa(f.Line)}
		}
		if !more {
			return source{}
		}
	}
}

func moduleRelative(file string) string {
	file = filepath.ToSlash(file)
	for _, marker := range []string{"/internal/", "/e2e/"} {
		if i := strings.LastIndex(file, marker); i >= 0 {
			return file[i+1:]
		}
	}
	return path.Base(file)
}

func (s Spec) problem() string {
	switch {
	case s.ID == "":
		return "ID is required"
	case s.Layer == "" || s.Component == "" || s.Method == "":
		return "Layer, Component, and Method are required"
	case s.Title == "":
		return "Title is required"
	case s.When == "" || s.Then == "":
		return "When and Then are required"
	case !techniques[s.Technique]:
		return "Technique must be one of the testkit Technique constants"
	case s.Kind == "":
		return "Kind is required"
	}
	return ""
}

func (s Spec) options(src source, goTest string) []allure.Option {
	opts := []allure.Option{
		allure.WithPackage(src.pkg),
		allure.WithLabel("source", src.file),
		allure.WithDisplayName("[" + s.ID + "] " + s.Title),
		allure.WithAllureID(s.ID),
		allure.WithTestCaseID(s.ID),
		allure.WithHistoryID(s.ID),
		allure.WithParentSuite(s.Layer),
		allure.WithSuite(s.Component),
		allure.WithSubSuite(s.Method),
		allure.WithEpic(s.Layer),
		allure.WithFeature(s.Component),
		allure.WithStory(s.Method),
		allure.WithSeverity(s.Severity),
		allure.WithTag(s.Kind),
		allure.WithLabel("testTechnique", s.Technique),
		allure.WithDescription(s.description(src, goTest)),
	}
	for _, key := range sortedKeys(s.Params) {
		opts = append(opts, allure.WithParameter(key, s.Params[key]))
	}
	return opts
}

func (s Spec) description(src source, goTest string) string {
	var b strings.Builder
	b.WriteString("| | |\n|---|---|\n")
	row(&b, "Spec ID", "`"+s.ID+"`")
	row(&b, "Component", "`"+s.Component+"."+s.Method+"`")
	row(&b, "Given", s.Given)
	row(&b, "When", s.When)
	row(&b, "Then", s.Then)
	row(&b, "Technique", s.Technique)
	row(&b, "Test double style", s.Kind)
	row(&b, "Go test", "`"+goTest+"`")
	if src.file != "" {
		row(&b, "Source", "`"+src.file+"`")
	}
	str := "\nSpecification: `" + specFile + "`, section `" + s.Component + "." + s.Method + "`.\n"
	b.WriteString(str)
	return b.String()
}

func row(b *strings.Builder, name, value string) {
	if value == "" {
		value = "-"
	}
	str := "| **" + name + "** | " + strings.ReplaceAll(value, "|", `\|`) + " |\n"
	b.WriteString(str)
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func claim(id, owner string) string {
	seenMu.Lock()
	defer seenMu.Unlock()
	if other, ok := seen[id]; ok && other != owner {
		return other
	}
	seen[id] = owner
	return ""
}

func (r Report) Arrange(body func(*testing.T)) {
	r.t.Helper()
	r.step("Arrange", body)
}

func (r Report) Act(body func(*testing.T)) {
	r.t.Helper()
	r.step("Act", body)
}

func (r Report) Assert(body func(*testing.T)) {
	r.t.Helper()
	r.step("Assert", body)
}

// Step records a named Allure step. End-to-end cases use it for each request.
func (r Report) Step(name string, body func(*testing.T)) {
	r.t.Helper()
	r.step(name, body)
}

func (r Report) step(name string, body func(*testing.T)) {
	r.t.Helper()
	r.a.Step(name, func(a *allure.Context) {
		body(a.T())
	})
}
