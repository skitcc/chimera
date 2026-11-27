//go:build integration

package postgres

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"chimera/identity/internal/domain"
	"chimera/identity/internal/infra/auth"
	"chimera/identity/internal/usecase"
)

type integrationMail struct{ code string }

func (m *integrationMail) SendCode(_ context.Context, _ string, _ string, c string) error {
	m.code = c
	return nil
}

type integrationToken struct{}

func (integrationToken) Issue(u domain.User) (string, error) { return fmt.Sprint(u.AuthVersion), nil }
func TestSecurityConcurrentAttemptsIntegration(t *testing.T) {
	f := newPostgresFixture(t)
	hasher := auth.NewBcryptHasher()
	hash, err := hasher.Hash("password1")
	if err != nil {
		t.Fatal(err)
	}
	u, err := f.users.Create(f.ctx, domain.User{Email: "concurrent@example.test"}, hash)
	if err != nil {
		t.Fatal(err)
	}
	// Separate service instances share only the PostgreSQL primary and HMAC key.
	mail := &integrationMail{}
	key := []byte("integration-shared-key-32-bytes!!")
	first := usecase.NewSecurityService(f.users, hasher, integrationToken{}, mail, key)
	second := usecase.NewSecurityService(f.users, hasher, integrationToken{}, mail, key)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			svc := first
			if i%2 == 0 {
				svc = second
			}
			_, err := svc.Login(f.ctx, domain.LoginInput{Email: u.Email, Password: "wrong"})
			if !domain.Is(err, domain.CodeUnauthorized) {
				t.Errorf("wrong-password result: %v", err)
			}
		}(i)
	}
	wg.Wait()
	var locked bool
	if err = f.pool.QueryRow(f.ctx, `SELECT locked_until>now() FROM users WHERE id=$1::uuid`, u.ID).Scan(&locked); err != nil || !locked {
		t.Fatal("concurrent fifth failure did not lock account")
	}
	if _, err = first.Recover(f.ctx, u.Email); err != nil {
		t.Fatal(err)
	}
	if _, err = second.Verify(f.ctx, usecase.SecurityLookup{Email: u.Email}, "recovery", mail.code, "password2"); err != nil {
		t.Fatal(err)
	}
	if first.Validate(f.ctx, u.ID, 1) == nil || first.Validate(f.ctx, u.ID, 2) != nil {
		t.Fatal("auth version did not revoke old sessions")
	}
	challenge, err := first.Login(f.ctx, domain.LoginInput{Email: u.Email, Password: "password2"})
	if err != nil {
		t.Fatal(err)
	}
	code := mail.code
	var successes atomic.Int32
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := second.Verify(f.ctx, usecase.SecurityLookup{ChallengeID: challenge.ChallengeID}, "login", code, "")
			if err == nil {
				successes.Add(1)
			} else if !domain.Is(err, domain.CodeUnauthorized) {
				t.Errorf("verify: %v", err)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("single-use OTP successes=%d", successes.Load())
	}
}
