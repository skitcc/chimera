//go:build authe2e

package e2e

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/jackc/pgx/v5"
)

type authWorld struct {
	base, email, password, newPassword, id, token, oldToken, challenge, code string
	status                                                                   int
	response                                                                 map[string]any
	db                                                                       *pgx.Conn
	http                                                                     *http.Client
}

func randomSecret() string {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("random source unavailable")
	}
	return hex.EncodeToString(b[:])
}
func (w *authWorld) request(method, path, token string, body any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode request")
	}
	req, err := http.NewRequest(method, w.base+"/v1"+path, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("invalid request URL")
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := w.http.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request unavailable")
	}
	defer res.Body.Close()
	w.status = res.StatusCode
	w.response = map[string]any{}
	b, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read response")
	}
	if len(b) > 0 {
		if json.Unmarshal(b, &w.response) != nil {
			return fmt.Errorf("invalid response JSON")
		}
	}
	return nil
}
func (w *authWorld) expect(status int) error {
	if w.status != status {
		return fmt.Errorf("expected HTTP %d, got %d", status, w.status)
	}
	return nil
}
func (w *authWorld) fresh() error {
	w.email = "auth-" + randomSecret() + "@example.test"
	var err error
	w.password, w.newPassword, err = testPasswords()
	if err != nil {
		return err
	}
	if err := w.request("POST", "/users", "", map[string]string{"email": w.email, "password": w.password, "name": "BDD technical account"}); err != nil {
		return err
	}
	if err := w.expect(201); err != nil {
		return err
	}
	if w.response["token"] != nil {
		return fmt.Errorf("registration issued a session before 2FA")
	}
	w.id, _ = w.response["id"].(string)
	if w.id == "" {
		return fmt.Errorf("missing account id")
	}
	return nil
}
func (w *authWorld) login(password string) error {
	if err := w.request("POST", "/sessions", "", map[string]string{"email": w.email, "password": password}); err != nil {
		return err
	}
	if w.status == 202 {
		w.challenge, _ = w.response["challenge_id"].(string)
		w.code = ""
	}
	return nil
}
func (w *authWorld) confirm(wrong bool) error {
	code := w.code
	if code == "" {
		var err error
		code, err = emailCode(w.email, "login")
		if err != nil {
			return err
		}
		w.code = code
	}
	if wrong {
		if code == "000000" {
			code = "111111"
		} else {
			code = "000000"
		}
	}
	if err := w.request("POST", "/sessions/verify", "", map[string]string{"challenge_id": w.challenge, "code": code}); err != nil {
		return err
	}
	if w.status == 201 {
		w.token, _ = w.response["token"].(string)
	}
	return nil
}
func (w *authWorld) signIn(password string) error {
	if err := w.login(password); err != nil {
		return err
	}
	if err := w.expect(202); err != nil {
		return err
	}
	w.code = ""
	if err := w.confirm(false); err != nil {
		return err
	}
	return w.expect(201)
}
func (w *authWorld) passwordFlow(purpose, password string) error {
	w.oldToken = w.token
	start := map[string]string{"email": w.email}
	token := ""
	if purpose == "change" {
		start = map[string]string{"current_password": w.password}
		token = w.token
	}
	if err := w.request("POST", "/password-"+purpose, token, start); err != nil {
		return err
	}
	if err := w.expect(202); err != nil {
		return err
	}
	id, _ := w.response["challenge_id"].(string)
	code, err := emailCode(w.email, purpose)
	if err != nil {
		return err
	}
	return w.request("POST", "/password-"+purpose+"/verify", token, map[string]string{"challenge_id": id, "code": code, "new_password": password, "email": w.email})
}
func (w *authWorld) cleanup() error {
	// Test-only primary access makes cleanup independent of authentication state.
	if w.email == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := w.db.Exec(ctx, `DELETE FROM users WHERE email=$1`, w.email)
	mailErr := deleteMail(w.email)
	if err != nil {
		return fmt.Errorf("technical account cleanup failed")
	}
	if mailErr != nil {
		return mailErr
	}
	var count int
	if err = w.db.QueryRow(ctx, `SELECT count(*) FROM users WHERE email=$1`, w.email).Scan(&count); err != nil || count != 0 {
		return fmt.Errorf("technical account remains")
	}
	messages, err := findMail(w.email)
	if err != nil || len(messages) != 0 {
		return fmt.Errorf("technical mail remains")
	}
	return nil
}
func TestAuthBDD(t *testing.T) {
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
	dir := os.Getenv("AUTH_BDD_RESULTS_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	if err = os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	suite := godog.TestSuite{Name: "authentication", Options: &godog.Options{Paths: []string{"features/auth.feature"}, Format: "pretty,cucumber:" + filepath.Join(dir, "cucumber.json") + ",junit:" + filepath.Join(dir, "junit.xml"), TestingT: t, Strict: true, Concurrency: 1}, ScenarioInitializer: func(sc *godog.ScenarioContext) {
		w := &authWorld{}
		sc.Before(func(ctx context.Context, s *godog.Scenario) (context.Context, error) {
			*w = authWorld{base: os.Getenv("E2E_API_URL"), db: db, http: &http.Client{Timeout: 10 * time.Second}}
			return ctx, nil
		})
		sc.After(func(ctx context.Context, s *godog.Scenario, scenarioErr error) (context.Context, error) {
			cleanupErr := w.cleanup()
			reportErr := writeAuthAllure(s.Name, scenarioErr, cleanupErr)
			return ctx, errors.Join(cleanupErr, reportErr)
		})
		sc.Step(`^a fresh technical account$`, w.fresh)
		sc.Step(`^I submit the correct password$`, func() error { return w.login(w.password) })
		sc.Step(`^I submit a wrong password$`, func() error { return w.login("wrong-password") })
		sc.Step(`^I submit a wrong password (\d+) times$`, func(n int) error {
			for i := 0; i < n; i++ {
				if err := w.login("wrong-password"); err != nil {
					return err
				}
				if err := w.expect(401); err != nil {
					return err
				}
			}
			return nil
		})
		sc.Step(`^a challenge is returned without a token$`, func() error {
			if err := w.expect(202); err != nil {
				return err
			}
			if w.response["token"] != nil || w.response["challenge_id"] == nil || w.response["expires_at"] == nil {
				return fmt.Errorf("invalid challenge contract")
			}
			return nil
		})
		sc.Step(`^protected access is denied$`, func() error {
			if err := w.request("PATCH", "/users/"+w.id, "", map[string]string{"name": "before 2FA"}); err != nil {
				return err
			}
			return w.expect(401)
		})
		sc.Step(`^protected access is allowed$`, func() error {
			if err := w.request("PATCH", "/users/"+w.id, w.token, map[string]string{"name": "after 2FA"}); err != nil {
				return err
			}
			return w.expect(200)
		})
		sc.Step(`^I confirm the emailed login code$`, func() error { return w.confirm(false) })
		sc.Step(`^I confirm an incorrect login code$`, func() error { return w.confirm(true) })
		sc.Step(`^I confirm an incorrect login code (\d+) times$`, func(n int) error {
			for i := 0; i < n; i++ {
				if err := w.confirm(true); err != nil {
					return err
				}
				if err := w.expect(401); err != nil {
					return err
				}
			}
			return nil
		})
		sc.Step(`^authentication is rejected$`, func() error { return w.expect(401) })
		sc.Step(`^the request is invalid$`, func() error { return w.expect(400) })
		sc.Step(`^the challenge expires$`, func() error {
			_, err := db.Exec(context.Background(), `UPDATE auth_challenges SET expires_at=now()-interval '1 second' WHERE user_id=$1::uuid`, w.id)
			return err
		})
		sc.Step(`^I recover with an emailed code and a new password$`, func() error { return w.passwordFlow("recovery", w.newPassword) })
		sc.Step(`^no session is issued$`, func() error {
			if err := w.expect(204); err != nil {
				return err
			}
			if w.response["token"] != nil {
				return fmt.Errorf("unexpected session")
			}
			return nil
		})
		sc.Step(`^the old password is rejected$`, func() error {
			if err := w.login(w.password); err != nil {
				return err
			}
			return w.expect(401)
		})
		sc.Step(`^I sign in with the new password and email code$`, func() error { return w.signIn(w.newPassword) })
		sc.Step(`^I sign in with both factors$`, func() error { return w.signIn(w.password) })
		sc.Step(`^I change the password with the current password and email code$`, func() error { return w.passwordFlow("change", w.newPassword) })
		sc.Step(`^the old token is rejected$`, func() error {
			if err := w.request("PATCH", "/users/"+w.id, w.oldToken, map[string]string{"name": "revoked"}); err != nil {
				return err
			}
			return w.expect(401)
		})
		sc.Step(`^I request a password change with a wrong current password$`, func() error {
			return w.request("POST", "/password-change", w.token, map[string]string{"current_password": "wrong-password"})
		})
		sc.Step(`^I attempt to reuse the current password$`, func() error { return w.passwordFlow("change", w.password) })
		sc.Step(`^I attempt to change the password through the profile$`, func() error {
			return w.request("PATCH", "/users/"+w.id, w.token, map[string]string{"password": w.newPassword})
		})
		sc.Step(`^recovery gives a generic response$`, func() error {
			if err := w.expect(202); err != nil {
				return err
			}
			if w.response["message"] == nil || w.response["challenge_id"] != nil || w.response["token"] != nil {
				return fmt.Errorf("invalid recovery response")
			}
			return nil
		})
		sc.Step(`^I request recovery for an unknown email$`, func() error {
			return w.request("POST", "/password-recovery", "", map[string]string{"email": "absent-" + randomSecret() + "@example.test"})
		})
	}}
	if suite.Run() != 0 {
		t.Fatal("authentication BDD failed; see sanitized reports")
	}
}
func writeAuthAllure(name string, scenarioErr, cleanupErr error) error {
	dir := os.Getenv("ALLURE_RESULTS_DIR")
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create BDD Allure directory")
	}
	status := "passed"
	if scenarioErr != nil || cleanupErr != nil {
		status = "failed"
	}
	// Only static scenario names/status are exported, never request bodies or errors.
	data, _ := json.Marshal(map[string]any{"uuid": randomSecret(), "name": name, "fullName": "auth-bdd." + name, "status": status, "stage": "finished", "start": time.Now().UnixMilli(), "stop": time.Now().UnixMilli(), "labels": []map[string]string{{"name": "parentSuite", "value": "End to end"}, {"name": "suite", "value": "Authentication BDD"}, {"name": "framework", "value": "godog"}}})
	if err := os.WriteFile(filepath.Join(dir, randomSecret()+"-result.json"), data, 0600); err != nil {
		return fmt.Errorf("write BDD Allure result")
	}
	return nil
}
