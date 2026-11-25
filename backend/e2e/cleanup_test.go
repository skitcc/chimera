//go:build authe2e

package e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/jackc/pgx/v5"
)

func TestAuthCleanupAfterFailedScenario(t *testing.T) {
	if os.Getenv("E2E_API_URL") == "" {
		t.Skip("E2E_API_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := pgx.Connect(ctx, os.Getenv("E2E_IDENTITY_DATABASE_URL"))
	if err != nil {
		t.Fatal("test database unavailable")
	}
	defer db.Close(context.Background())
	w := &authWorld{base: os.Getenv("E2E_API_URL"), db: db, http: &http.Client{Timeout: 10 * time.Second}}
	cleanupCalled := false
	deliberateFailureReached := false
	var cleanupErr error
	suite := godog.TestSuite{Name: "cleanup-failure-probe", Options: &godog.Options{Format: "progress", Output: io.Discard, Strict: true, FeatureContents: []godog.Feature{{Name: "cleanup.feature", Contents: []byte(`Feature: Cleanup on failure
  Scenario: Failure after an account and email exist
    Given an account with an emailed code
    When the scenario fails deliberately
`)}}}, ScenarioInitializer: func(sc *godog.ScenarioContext) {
		sc.Step(`^an account with an emailed code$`, func() error {
			if err := w.fresh(); err != nil {
				return err
			}
			if err := w.login(w.password); err != nil {
				return err
			}
			if err := w.expect(202); err != nil {
				return err
			}
			_, err := emailCode(w.email, "login")
			return err
		})
		sc.Step(`^the scenario fails deliberately$`, func() error {
			deliberateFailureReached = true
			return fmt.Errorf("expected failure for cleanup verification")
		})
		sc.After(func(ctx context.Context, _ *godog.Scenario, scenarioErr error) (context.Context, error) {
			cleanupCalled = scenarioErr != nil
			cleanupErr = w.cleanup()
			return ctx, cleanupErr
		})
	}}
	if suite.Run() != 1 || !deliberateFailureReached || !cleanupCalled || cleanupErr != nil {
		t.Fatal("failed-scenario cleanup was not completed")
	}
	if err := writeAuthAllure("Cleanup removes the account and mail after a failed scenario", nil, nil); err != nil {
		t.Fatal(err)
	}
}
