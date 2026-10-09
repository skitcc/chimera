package testkit

import (
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
)

// runLog writes test boundaries with run_id when TEST_RUN_ID is set. Records go
// to TEST_LOG_FILE, opened in append mode so packages run by one go test
// invocation share it; without the file they go to stdout.
var runLog = sync.OnceValue(func() *slog.Logger {
	id := os.Getenv("TEST_RUN_ID")
	if id == "" {
		return nil
	}
	var w io.Writer = os.Stdout
	if name := os.Getenv("TEST_LOG_FILE"); name != "" {
		f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err == nil {
			w = f
		}
	}
	return slog.New(slog.NewTextHandler(w, nil)).With("run_id", id)
})

func logRun(t *testing.T, s Spec) {
	log := runLog()
	if log == nil {
		return
	}
	log.Info("test started", "spec_id", s.ID, "test", t.Name())
	t.Cleanup(func() {
		result := "passed"
		if t.Failed() {
			result = "failed"
		}
		log.Info("test finished", "spec_id", s.ID, "test", t.Name(), "result", result)
	})
}
