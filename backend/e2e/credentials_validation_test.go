//go:build e2e || authe2e || perfauth

package e2e

import "testing"

func TestE2ECredentials(t *testing.T) {
	t.Run("CI refuses a missing secret instead of generating a password", func(t *testing.T) {
		t.Setenv("GITHUB_ACTIONS", "true")
		t.Setenv("E2E_USER_PASSWORD", "")
		t.Setenv("E2E_NEW_PASSWORD", "")
		if _, _, err := testPasswords(); err == nil {
			t.Fatal("missing CI secrets accepted")
		}
	})
	t.Run("configured values reach the test unchanged", func(t *testing.T) {
		t.Setenv("GITHUB_ACTIONS", "true")
		t.Setenv("E2E_USER_PASSWORD", "fixture-original-password")
		t.Setenv("E2E_NEW_PASSWORD", "fixture-replacement-password")
		password, next, err := testPasswords()
		if err != nil || password != "fixture-original-password" || next != "fixture-replacement-password" {
			t.Fatal("configured credentials were not used")
		}
	})
	t.Run("a partial local configuration cannot silently fall back", func(t *testing.T) {
		t.Setenv("GITHUB_ACTIONS", "")
		t.Setenv("E2E_USER_PASSWORD", "fixture-original-password")
		t.Setenv("E2E_NEW_PASSWORD", "")
		if _, _, err := testPasswords(); err == nil {
			t.Fatal("partial configuration accepted")
		}
	})
	t.Run("local fallback generates different credentials for each run", func(t *testing.T) {
		t.Setenv("GITHUB_ACTIONS", "")
		t.Setenv("E2E_USER_PASSWORD", "")
		t.Setenv("E2E_NEW_PASSWORD", "")
		password, next, err := testPasswords()
		other, _, secondErr := testPasswords()
		if err != nil || secondErr != nil || len(password) < 16 || len(next) < 16 || password == next || password == other {
			t.Fatal("disposable credentials were not generated independently")
		}
	})
}
