package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"chimera/identity/internal/domain"
)

type securityMemory struct{ account SecurityAccount }

func (m *securityMemory) WithAccount(_ context.Context, l SecurityLookup, fn func(*SecurityAccount) error) error {
	if (l.Email != "" && l.Email != m.account.User.Email) || (l.UserID != "" && l.UserID != m.account.User.ID) {
		return domain.NotFound("missing")
	}
	if l.ChallengeID != "" && l.UserID == "" {
		found := false
		for _, c := range m.account.Challenges {
			if c.ID == l.ChallengeID {
				found = true
			}
		}
		if !found {
			return domain.NotFound("missing")
		}
	}
	a := m.account
	a.Challenges = map[string]Challenge{}
	for p, c := range m.account.Challenges {
		a.Challenges[p] = c
	}
	if err := fn(&a); err != nil {
		return err
	}
	m.account = a
	return nil
}
func (m *securityMemory) Version(context.Context, string) (int64, error) {
	return m.account.User.AuthVersion, nil
}

type securityHash struct{}

func (securityHash) Hash(p string) (string, error) { return "hash:" + p, nil }
func (securityHash) Compare(h, p string) error {
	if h != "hash:"+p {
		return rejected()
	}
	return nil
}

type securityTokens struct{}

func (securityTokens) Issue(domain.User) (string, error) { return "test-token", nil }

type securityMail struct {
	code string
	err  error
}

func (m *securityMail) SendCode(_ context.Context, _ string, _ string, c string) error {
	m.code = c
	return m.err
}
func securityFixture() (*SecurityService, *securityMemory, *securityMail) {
	store := &securityMemory{account: SecurityAccount{User: domain.User{ID: "account", Email: "security@example.test", AuthVersion: 1}, PasswordHash: "hash:password1", Challenges: map[string]Challenge{}}}
	mail := &securityMail{}
	s := NewSecurityService(store, securityHash{}, securityTokens{}, mail, []byte("a-test-key-with-at-least-32-bytes!!"))
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	return s, store, mail
}
func mustSecurityLogin(t *testing.T, s *SecurityService) domain.AuthResult {
	t.Helper()
	out, err := s.Login(context.Background(), domain.LoginInput{Email: "security@example.test", Password: "password1"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Token != "" || out.ChallengeID == "" {
		t.Fatal("password must not issue token")
	}
	return out
}
func TestSecurityCodeLifecycle(t *testing.T) {
	ctx := context.Background()
	s, store, mail := securityFixture()
	out := mustSecurityLogin(t, s)
	c := store.account.Challenges["login"]
	if string(c.Digest) == mail.code || len(c.Digest) != 32 {
		t.Fatal("OTP must be HMAC only")
	}
	if _, err := s.Login(ctx, domain.LoginInput{Email: store.account.User.Email, Password: "password1"}); !domain.Is(err, domain.CodeInvalid) {
		t.Fatal("resend cooldown missing")
	}
	result, err := s.Verify(ctx, SecurityLookup{ChallengeID: out.ChallengeID}, "login", mail.code, "")
	if err != nil || result.Token == "" {
		t.Fatal("correct second factor rejected")
	}
	if _, err = s.Verify(ctx, SecurityLookup{ChallengeID: out.ChallengeID}, "login", mail.code, ""); !domain.Is(err, domain.CodeUnauthorized) {
		t.Fatal("OTP replay accepted")
	}
	out = mustSecurityLogin(t, s)
	s.now = func() time.Time { return c.SentAt.Add(5 * time.Minute) }
	if _, err = s.Verify(ctx, SecurityLookup{ChallengeID: out.ChallengeID}, "login", mail.code, ""); !domain.Is(err, domain.CodeUnauthorized) {
		t.Fatal("OTP accepted at expiry boundary")
	}
}
func TestSecurityFifthMixedFailureAndRecovery(t *testing.T) {
	ctx := context.Background()
	s, store, mail := securityFixture()
	for i := 0; i < 4; i++ {
		s.Login(ctx, domain.LoginInput{Email: store.account.User.Email, Password: "wrong"})
	}
	out := mustSecurityLogin(t, s)
	if store.account.Failures != 4 {
		t.Fatal("password success reset attempts before 2FA")
	}
	wrong := "000000"
	if mail.code == wrong {
		wrong = "111111"
	}
	s.Verify(ctx, SecurityLookup{ChallengeID: out.ChallengeID}, "login", wrong, "")
	if !s.now().Before(store.account.LockedUntil) {
		t.Fatal("fifth attempt did not lock")
	}
	if _, err := s.Login(ctx, domain.LoginInput{Email: store.account.User.Email, Password: "password1"}); !domain.Is(err, domain.CodeUnauthorized) {
		t.Fatal("locked account logged in")
	}
	if _, err := s.Recover(ctx, store.account.User.Email); err != nil {
		t.Fatal(err)
	}
	result, err := s.Verify(ctx, SecurityLookup{Email: store.account.User.Email}, "recovery", mail.code, "password2")
	if err != nil || result.Token != "" {
		t.Fatal("recovery failed or issued a token")
	}
	if store.account.Failures != 0 || !store.account.LockedUntil.IsZero() || store.account.User.AuthVersion != 2 || len(store.account.Challenges) != 0 {
		t.Fatal("recovery did not reset account/revoke sessions")
	}
	if s.Validate(ctx, "account", 1) == nil {
		t.Fatal("old version accepted")
	}
	if s.Validate(ctx, "account", 2) != nil {
		t.Fatal("current version rejected")
	}
}
func TestSecurityResendInvalidatesPreviousCode(t *testing.T) {
	ctx := context.Background()
	s, _, mail := securityFixture()
	old := mustSecurityLogin(t, s)
	oldCode := mail.code
	now := s.now()
	s.now = func() time.Time { return now.Add(time.Minute) }
	next := mustSecurityLogin(t, s)
	if next.ChallengeID == old.ChallengeID {
		t.Fatal("resend reused challenge")
	}
	if _, err := s.Verify(ctx, SecurityLookup{ChallengeID: old.ChallengeID}, "login", oldCode, ""); !domain.Is(err, domain.CodeUnauthorized) {
		t.Fatal("previous challenge accepted")
	}
	if _, err := s.Verify(ctx, SecurityLookup{ChallengeID: next.ChallengeID}, "login", mail.code, ""); err != nil {
		t.Fatal(err)
	}
}
func TestSecuritySMTPFailureRollsBack(t *testing.T) {
	s, store, mail := securityFixture()
	before := store.account
	mail.err = errors.New("SMTP unavailable")
	if _, err := s.Login(context.Background(), domain.LoginInput{Email: store.account.User.Email, Password: "password1"}); err == nil {
		t.Fatal("SMTP failure accepted")
	}
	if !reflect.DeepEqual(before, store.account) {
		t.Fatal("failed delivery changed auth state")
	}
	// Recovery still hides account existence and does not persist an undelivered code.
	if _, err := s.Recover(context.Background(), store.account.User.Email); err != nil {
		t.Fatal("recovery disclosed SMTP outcome")
	}
	if len(store.account.Challenges) != 0 {
		t.Fatal("undelivered recovery challenge persisted")
	}
}
func TestSecurityChangeRequiresCurrentAndDifferentPassword(t *testing.T) {
	ctx := context.Background()
	s, store, mail := securityFixture()
	if _, err := s.Change(ctx, "account", "wrong"); !domain.Is(err, domain.CodeUnauthorized) {
		t.Fatal("wrong current password accepted")
	}
	c, err := s.Change(ctx, "account", "password1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Verify(ctx, SecurityLookup{UserID: "account", ChallengeID: c.ChallengeID}, "change", mail.code, "password1"); !domain.Is(err, domain.CodeInvalid) {
		t.Fatal("same password accepted")
	}
	if _, err = s.Verify(ctx, SecurityLookup{UserID: "account", ChallengeID: c.ChallengeID, Email: "untrusted@example.test"}, "change", mail.code, "password2"); err != nil {
		t.Fatal(err)
	}
	if store.account.User.AuthVersion != 2 {
		t.Fatal("password change did not revoke sessions")
	}
}
func TestSecurityLockExpiresAfterFifteenMinutes(t *testing.T) {
	s, _, _ := securityFixture()
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		s.Login(ctx, domain.LoginInput{Email: "security@example.test", Password: "wrong"})
	}
	now := s.now()
	s.now = func() time.Time { return now.Add(15*time.Minute - time.Nanosecond) }
	if _, err := s.Login(ctx, domain.LoginInput{Email: "security@example.test", Password: "password1"}); !domain.Is(err, domain.CodeUnauthorized) {
		t.Fatal("lock expired too early")
	}
	s.now = func() time.Time { return now.Add(15 * time.Minute) }
	mustSecurityLogin(t, s)
}

func TestSecureProfileCannotBypassPasswordChange(t *testing.T) {
	users := newFakeUserRepository()
	u := validUser()
	users.users[u.ID] = u
	svc := NewUserService(users, users, securityHash{})
	svc.EnableSecurity()
	password := "new-password"
	if _, err := svc.Patch(context.Background(), u.ID, u.ID, domain.UserPatch{Password: &password}); !domain.Is(err, domain.CodeInvalid) {
		t.Fatal("legacy profile bypass accepted")
	}
}
func TestPerfLegacyLoginStillIssuesSessionImmediately(t *testing.T) {
	users := newFakeUserRepository()
	u := validUser()
	users.users[u.ID] = u
	users.passwords[u.ID] = "hash:password1"
	svc := NewAuthService(users, securityHash{}, securityTokens{})
	out, err := svc.Login(context.Background(), domain.LoginInput{Email: u.Email, Password: "password1"})
	if err != nil || out.Token == "" || out.ChallengeID != "" {
		t.Fatal("legacy perf login changed")
	}
}
