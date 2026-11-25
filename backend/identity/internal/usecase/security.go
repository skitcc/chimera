package usecase

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"chimera/identity/internal/domain"
)

// Aliases keep the service-facing port names close to the use case, while
// adapters depend only on domain contracts.
type SecurityAccount = domain.SecurityAccount
type Challenge = domain.Challenge
type SecurityLookup = domain.SecurityLookup
type SecurityStore = domain.SecurityStore
type CodeMailer interface {
	SendCode(context.Context, string, string, string) error
}
type SecurityService struct {
	store  SecurityStore
	hasher PasswordHasher
	tokens TokenIssuer
	mail   CodeMailer
	key    []byte
	now    func() time.Time
}

func NewSecurityService(store SecurityStore, hasher PasswordHasher, tokens TokenIssuer, mail CodeMailer, key []byte) *SecurityService {
	return &SecurityService{store: store, hasher: hasher, tokens: tokens, mail: mail, key: key, now: time.Now}
}
func rejected() error { return domain.Unauthorized("invalid credentials") }
func (s *SecurityService) fail(a *SecurityAccount) {
	a.Failures++
	if a.Failures >= 5 {
		a.Failures = 0
		a.LockedUntil = s.now().Add(15 * time.Minute)
		for purpose, c := range a.Challenges {
			c.Digest = nil
			a.Challenges[purpose] = c
		}
	}
}
func (s *SecurityService) digest(id, code string) []byte {
	h := hmac.New(sha256.New, s.key)
	h.Write([]byte(id + ":" + code))
	return h.Sum(nil)
}
func randomID() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}
func (s *SecurityService) issue(ctx context.Context, a *SecurityAccount, purpose string) (domain.AuthResult, error) {
	now := s.now()
	if previous, ok := a.Challenges[purpose]; ok && now.Before(previous.SentAt.Add(time.Minute)) {
		return domain.AuthResult{}, domain.Invalid("retry after 60 seconds")
	}
	id, err := randomID()
	if err != nil {
		return domain.AuthResult{}, err
	}
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return domain.AuthResult{}, err
	}
	code := fmt.Sprintf("%06d", n.Int64())
	c := Challenge{ID: id, Digest: s.digest(id, code), Version: a.User.AuthVersion, ExpiresAt: now.Add(5 * time.Minute), SentAt: now}
	a.Challenges[purpose] = c
	if err = s.mail.SendCode(ctx, a.User.Email, purpose, code); err != nil {
		return domain.AuthResult{}, err
	}
	return domain.AuthResult{ChallengeID: id, ExpiresAt: c.ExpiresAt.Unix()}, nil
}
func (s *SecurityService) Login(ctx context.Context, in domain.LoginInput) (domain.AuthResult, error) {
	if err := in.Validate(); err != nil {
		return domain.AuthResult{}, err
	}
	return s.start(ctx, SecurityLookup{Email: in.Email}, "login", in.Password)
}
func (s *SecurityService) Recover(ctx context.Context, email string) (domain.AuthResult, error) {
	in := domain.LoginInput{Email: email, Password: "recovery"}
	if err := in.Validate(); err != nil {
		return domain.AuthResult{}, err
	}
	return s.start(ctx, SecurityLookup{Email: in.Email}, "recovery", "")
}
func (s *SecurityService) Change(ctx context.Context, userID, password string) (domain.AuthResult, error) {
	if userID == "" {
		return domain.AuthResult{}, rejected()
	}
	return s.start(ctx, SecurityLookup{UserID: userID}, "change", password)
}
func (s *SecurityService) start(ctx context.Context, lookup SecurityLookup, purpose, password string) (domain.AuthResult, error) {
	var out domain.AuthResult
	var decision error
	err := s.store.WithAccount(ctx, lookup, func(a *SecurityAccount) error {
		if purpose != "recovery" {
			if s.now().Before(a.LockedUntil) {
				decision = rejected()
				return nil
			}
			if s.hasher.Compare(a.PasswordHash, password) != nil {
				s.fail(a)
				decision = rejected()
				return nil
			}
		}
		var err error
		out, err = s.issue(ctx, a, purpose)
		// Recovery always has the same public response, including during cooldown.
		if purpose == "recovery" && domain.Is(err, domain.CodeInvalid) {
			return nil
		}
		return err
	})
	if purpose == "recovery" {
		// Neither delivery outcome nor cooldown/challenge metadata is public.
		return domain.AuthResult{}, nil
	}
	if domain.Is(err, domain.CodeNotFound) {
		err = rejected()
	}
	if err != nil {
		return domain.AuthResult{}, err
	}
	return out, decision
}

// Login/change use an unguessable challenge id. Recovery uses email and code;
// its request endpoint never reveals whether a challenge exists.
func (s *SecurityService) Verify(ctx context.Context, lookup SecurityLookup, purpose, code, password string) (domain.AuthResult, error) {
	if purpose != "login" && purpose != "recovery" && purpose != "change" {
		return domain.AuthResult{}, domain.Invalid("invalid purpose")
	}
	if purpose == "recovery" {
		lookup = SecurityLookup{Email: domain.NormalizeEmail(lookup.Email)}
		if lookup.Email == "" {
			return domain.AuthResult{}, rejected()
		}
	}
	if purpose == "change" && lookup.UserID == "" {
		return domain.AuthResult{}, rejected()
	}
	// Select the account only by the identifier trusted for this flow. In
	// particular, an email in a change request must never override its actor.
	if purpose == "login" {
		lookup = SecurityLookup{ChallengeID: lookup.ChallengeID}
	}
	if purpose == "change" {
		lookup = SecurityLookup{UserID: lookup.UserID, ChallengeID: lookup.ChallengeID}
	}
	if purpose != "login" && len(password) < 8 {
		return domain.AuthResult{}, domain.Invalid("password must be at least 8 characters")
	}
	var out domain.AuthResult
	var decision error
	err := s.store.WithAccount(ctx, lookup, func(a *SecurityAccount) error {
		if purpose != "recovery" && s.now().Before(a.LockedUntil) {
			decision = rejected()
			return nil
		}
		c, ok := a.Challenges[purpose]
		if !ok || (purpose != "recovery" && c.ID != lookup.ChallengeID) || c.Version != a.User.AuthVersion || !s.now().Before(c.ExpiresAt) || !hmac.Equal(c.Digest, s.digest(c.ID, code)) {
			// During recovery a locked account can still prove email ownership, but each bad code extends the lock.
			s.fail(a)
			decision = rejected()
			return nil
		}
		if purpose != "login" {
			if s.hasher.Compare(a.PasswordHash, password) == nil {
				decision = domain.Invalid("new password must differ from current password")
				return nil
			}
			hash, err := s.hasher.Hash(password)
			if err != nil {
				return err
			}
			a.PasswordHash = hash
			a.User.AuthVersion++
			a.Challenges = map[string]Challenge{}
		} else {
			delete(a.Challenges, purpose)
			token, err := s.tokens.Issue(a.User)
			if err != nil {
				return err
			}
			out = domain.AuthResult{Token: token, User: a.User}
		}
		a.Failures = 0
		a.LockedUntil = time.Time{}
		return nil
	})
	if domain.Is(err, domain.CodeNotFound) {
		err = rejected()
	}
	if err != nil {
		return domain.AuthResult{}, err
	}
	return out, decision
}
func (s *SecurityService) Validate(ctx context.Context, id string, version int64) error {
	v, err := s.store.Version(ctx, id)
	if err != nil {
		return err
	}
	if version < 1 || version != v {
		return rejected()
	}
	return nil
}
