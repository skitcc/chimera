//go:build e2e || authe2e || perfauth

package e2e

import (
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"chimera/testkit"
)

// The existing CI smoke endpoints, including the built frontend's API proxy.
func TestWebAndGatewaySmoke(t *testing.T) {
	web, api := os.Getenv("E2E_WEB_URL"), os.Getenv("E2E_API_URL")
	if web == "" || api == "" {
		t.Skip("E2E_WEB_URL and E2E_API_URL are required")
	}
	id, mode := "E2E-SMOKE-01", "secured"
	if os.Getenv("E2E_AUTH_MODE") == "legacy" {
		id, mode = "E2E-SMOKE-02", "legacy perf"
	}
	testkit.RunSpec(t, testkit.Spec{
		ID: id, Layer: testkit.LayerE2E, Component: "WebAndGateway", Method: "Smoke",
		Title:     "web and Gateway endpoints are healthy in " + mode + " mode",
		Given:     "a private stack with a built frontend and Gateway",
		When:      "readiness and public API endpoints are requested directly and through the frontend proxy",
		Then:      "every endpoint returns HTTP 200",
		Technique: testkit.TechniqueState, Kind: testkit.KindE2E,
	}, func(t *testing.T, _ testkit.Report) { checkWebAndGatewaySmoke(t, web, api) })
}

func checkWebAndGatewaySmoke(t *testing.T, web, api string) {
	client := &http.Client{Timeout: 5 * time.Second}
	for _, url := range []string{api + "/live", api + "/ready", api + "/v1/tracks", web + "/", web + "/live", web + "/api/v1/tracks"} {
		deadline := time.Now().Add(30 * time.Second)
		for {
			res, err := client.Get(url)
			if err == nil {
				io.Copy(io.Discard, res.Body)
				res.Body.Close()
				if res.StatusCode == http.StatusOK {
					break
				}
			}
			if time.Now().After(deadline) {
				t.Fatal("smoke endpoint did not become healthy")
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
}
