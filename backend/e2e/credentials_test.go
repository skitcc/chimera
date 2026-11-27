//go:build e2e || authe2e || perfauth

package e2e

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// CI receives both values from GitHub Actions Secrets. Local runs may generate
// disposable credentials. They reach Identity only through ordinary HTTP APIs.
func testPasswords() (string, string, error) {
	password, next := os.Getenv("E2E_USER_PASSWORD"), os.Getenv("E2E_NEW_PASSWORD")
	if password == "" && next == "" && os.Getenv("GITHUB_ACTIONS") != "true" {
		values := make([]byte, 48)
		if _, err := rand.Read(values); err != nil {
			return "", "", fmt.Errorf("random source unavailable")
		}
		return hex.EncodeToString(values[:24]), hex.EncodeToString(values[24:]), nil
	}
	if len(password) < 8 || len(password) > 72 || len(next) < 8 || len(next) > 72 || password == next || strings.ContainsAny(password+next, "\r\n") {
		return "", "", fmt.Errorf("configure distinct E2E_USER_PASSWORD and E2E_NEW_PASSWORD (8–72 bytes) in GitHub Actions Secrets or local environment")
	}
	return password, next, nil
}
