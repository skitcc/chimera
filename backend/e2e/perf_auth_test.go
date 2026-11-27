//go:build perfauth

package e2e

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"chimera/testkit"
)

// No load generation: checks only the explicitly disabled security configuration.
func TestPerfAuthCompatibility(t *testing.T) {
	base := os.Getenv("E2E_API_URL")
	if base == "" {
		t.Skip("E2E_API_URL is not set")
	}
	testkit.RunSpec(t, testkit.Spec{
		ID: "E2E-PERF-AUTH-01", Layer: testkit.LayerE2E, Component: "PerfAuth", Method: "Compatibility",
		Title:     "legacy perf login and profile password changes preserve JWT behavior",
		Given:     "a private stack with authentication security explicitly disabled",
		When:      "a technical user signs in and changes the password through the profile",
		Then:      "the one-stage login issues a token and the existing token remains valid",
		Technique: testkit.TechniqueState, Kind: testkit.KindE2E,
	}, func(t *testing.T, _ testkit.Report) { checkPerfAuthCompatibility(t, base) })
}

func checkPerfAuthCompatibility(t *testing.T, base string) {
	var random [24]byte
	password, newPassword, err := testPasswords()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	email := "perf-auth-" + hex.EncodeToString(random[:]) + "@example.test"
	client := &http.Client{Timeout: 10 * time.Second}
	request := func(method, path, token string, body any) (int, map[string]any) {
		t.Helper()
		raw, _ := json.Marshal(body)
		req, err := http.NewRequest(method, base+"/v1"+path, bytes.NewReader(raw))
		if err != nil {
			t.Fatal("invalid request")
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal("request unavailable")
		}
		defer res.Body.Close()
		data := map[string]any{}
		b, _ := io.ReadAll(res.Body)
		if len(b) > 0 {
			json.Unmarshal(b, &data)
		}
		return res.StatusCode, data
	}
	status, u := request("POST", "/users", "", map[string]string{"email": email, "password": password})
	if status != 201 {
		t.Fatal("registration failed")
	}
	id, _ := u["id"].(string)
	status, session := request("POST", "/sessions", "", map[string]string{"email": email, "password": password})
	if status != 201 || session["challenge_id"] != nil {
		t.Fatal("legacy login contract changed")
	}
	token, _ := session["token"].(string)
	if token == "" {
		t.Fatal("legacy login missing token")
	}
	t.Cleanup(func() {
		status, _ := request("DELETE", "/users/"+id, token, nil)
		if status != 204 {
			t.Error("technical user cleanup failed")
		}
	})
	status, _ = request("PATCH", "/users/"+id, token, map[string]string{"password": newPassword})
	if status != 200 {
		t.Fatal("legacy profile password update changed")
	}
	status, _ = request("PATCH", "/users/"+id, token, map[string]string{"name": "old JWT still works in perf"})
	if status != 200 {
		t.Fatal("legacy JWT behavior changed")
	}
}
