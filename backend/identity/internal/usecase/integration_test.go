//go:build integration

package usecase

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"chimera/identity/internal/domain"
	infraauth "chimera/identity/internal/infra/auth"
	"chimera/identity/internal/infra/postgres"
	"chimera/identity/internal/testkit"
	"chimera/identity/migrations"
)

const (
	nilUUID     = "00000000-0000-0000-0000-000000000000"
	resetTables = `TRUNCATE outbox, users RESTART IDENTITY CASCADE`
)

type appFixture struct {
	ctx     context.Context
	pool    *pgxpool.Pool
	auth    *AuthService
	members *UserService
	public  ed25519.PublicKey
}

func newAppFixture(t *testing.T) *appFixture {
	t.Helper()
	ctx := context.Background()
	pool := testkit.Open(t, migrations.FS, resetTables)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	users := postgres.NewUserRepository(pool)
	hasher := infraauth.NewBcryptHasher()
	return &appFixture{
		ctx:     ctx,
		pool:    pool,
		auth:    NewAuthService(users, hasher, infraauth.NewJWT(private, time.Hour)),
		members: NewUserService(users, users, hasher),
		public:  public,
	}
}

// subject verifies token the way the Gateway does and returns its sub claim.
func (f *appFixture) subject(token string) (string, error) {
	parsed, err := jwt.Parse(token, func(*jwt.Token) (any, error) { return f.public, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}))
	if err != nil {
		return "", err
	}
	return parsed.Claims.GetSubject()
}

func (f *appFixture) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(f.ctx, query, args...).Scan(&n); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return n
}

func (f *appFixture) mustUser(t *testing.T, email string) domain.User {
	t.Helper()
	user, err := f.members.Create(f.ctx, domain.UserWrite{Email: email, Name: "Member", Password: "password1"})
	if err != nil {
		t.Fatalf("create user %s: %v", email, err)
	}
	return user
}

func runService(t *testing.T, component string, s testkit.Spec, body func(*testing.T, testkit.Report)) {
	t.Helper()
	s.Layer = testkit.LayerUsecase
	s.Component = component
	s.Kind = testkit.KindIntegration
	testkit.RunSpec(t, s, body)
}

func assertCode(t *testing.T, err error, code domain.Code, message string) {
	t.Helper()
	app, ok := domain.As(err)
	if !ok {
		t.Fatalf("got error %v, want domain error", err)
	}
	if app.Code != code || app.Message != message {
		t.Fatalf("got domain error (%s, %q), want (%s, %q)", app.Code, app.Message, code, message)
	}
}

func requireEqual(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestAuthServiceIntegration(t *testing.T) {
	const component = "AuthService"

	runService(t, component, testkit.Spec{
		ID: "IT-UC-AUTH-REG-01", Method: "Register",
		Title:     "register stores a bcrypt hash and login issues a token",
		Given:     "an empty users table, real bcrypt hasher and JWT issuer",
		When:      "Register is called with `  Alice@Example.com ` / password1, then Login with alice@example.com / password1",
		Then:      "email is stored lowercased; password_hash is bcrypt (`$2...`) and not the plain password; the login token subject equals the stored user id",
		Technique: testkit.TechniqueEquivalence,
		Severity:  testkit.SeverityBlocker,
		Params:    map[string]string{"email": "  Alice@Example.com ", "password": "password1"},
	}, func(t *testing.T, r testkit.Report) {
		var f *appFixture
		var registered, logged domain.AuthResult
		var regErr, loginErr error
		r.Arrange(func(t *testing.T) { f = newAppFixture(t) })
		r.Act(func(t *testing.T) {
			registered, regErr = f.auth.Register(f.ctx, domain.RegisterInput{
				Email: "  Alice@Example.com ", Password: "password1", Name: "Alice",
			})
			logged, loginErr = f.auth.Login(f.ctx, domain.LoginInput{Email: "alice@example.com", Password: "password1"})
		})
		r.Assert(func(t *testing.T) {
			if regErr != nil || loginErr != nil {
				t.Fatalf("Register() error = %v, Login() error = %v", regErr, loginErr)
			}
			requireEqual(t, registered.User.Email, "alice@example.com")
			requireEqual(t, logged.User.ID, registered.User.ID)
			var hash string
			if err := f.pool.QueryRow(f.ctx, `SELECT password_hash FROM users WHERE id = $1::uuid`, registered.User.ID).Scan(&hash); err != nil {
				t.Fatalf("read hash: %v", err)
			}
			if !strings.HasPrefix(hash, "$2") || hash == "password1" {
				t.Fatalf("password hash %q is not bcrypt", hash)
			}
			subject, err := f.subject(logged.Token)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			requireEqual(t, subject, registered.User.ID)
			requireEqual(t, f.count(t, `SELECT count(*) FROM users`), 1)
		})
	})

	runService(t, component, testkit.Spec{
		ID: "IT-UC-AUTH-REG-02", Method: "Register",
		Title:     "duplicate registration returns conflict",
		Given:     "a registered user alice@example.com",
		When:      "Register is called again with ALICE@example.com",
		Then:      "after normalization the unique constraint fires: conflict `email already exists`; one row remains",
		Technique: testkit.TechniqueErrorGuessing,
		Severity:  testkit.SeverityCritical,
		Params:    map[string]string{"second email": "ALICE@example.com"},
	}, func(t *testing.T, r testkit.Report) {
		var f *appFixture
		var err error
		r.Arrange(func(t *testing.T) {
			f = newAppFixture(t)
			if _, regErr := f.auth.Register(f.ctx, domain.RegisterInput{Email: "alice@example.com", Password: "password1", Name: "Alice"}); regErr != nil {
				t.Fatalf("seed register: %v", regErr)
			}
		})
		r.Act(func(t *testing.T) {
			_, err = f.auth.Register(f.ctx, domain.RegisterInput{Email: "ALICE@example.com", Password: "password1", Name: "Other"})
		})
		r.Assert(func(t *testing.T) {
			assertCode(t, err, domain.CodeConflict, "email already exists")
			requireEqual(t, f.count(t, `SELECT count(*) FROM users`), 1)
		})
	})

	runService(t, component, testkit.Spec{
		ID: "IT-UC-AUTH-REG-03", Method: "Register",
		Title:     "short password is rejected before anything is stored",
		Given:     "an empty users table",
		When:      "Register is called with a 7-character password",
		Then:      "invalid `password must be at least 8 characters`; users stays empty",
		Technique: testkit.TechniqueBoundary,
		Params:    map[string]string{"password length": "7"},
	}, func(t *testing.T, r testkit.Report) {
		var f *appFixture
		var err error
		r.Arrange(func(t *testing.T) { f = newAppFixture(t) })
		r.Act(func(t *testing.T) {
			_, err = f.auth.Register(f.ctx, domain.RegisterInput{Email: "alice@example.com", Password: "1234567"})
		})
		r.Assert(func(t *testing.T) {
			assertCode(t, err, domain.CodeInvalid, "password must be at least 8 characters")
			requireEqual(t, f.count(t, `SELECT count(*) FROM users`), 0)
		})
	})

	runService(t, component, testkit.Spec{
		ID: "IT-UC-AUTH-LOGIN-01", Method: "Login",
		Title:     "wrong password returns unauthorized",
		Given:     "a registered user alice@example.com / password1",
		When:      "Login is called with wrong-password",
		Then:      "unauthorized `invalid credentials`; no token is issued",
		Technique: testkit.TechniqueErrorGuessing,
		Severity:  testkit.SeverityBlocker,
	}, func(t *testing.T, r testkit.Report) {
		var f *appFixture
		var got domain.AuthResult
		var err error
		r.Arrange(func(t *testing.T) {
			f = newAppFixture(t)
			if _, regErr := f.auth.Register(f.ctx, domain.RegisterInput{Email: "alice@example.com", Password: "password1", Name: "Alice"}); regErr != nil {
				t.Fatalf("seed register: %v", regErr)
			}
		})
		r.Act(func(t *testing.T) {
			got, err = f.auth.Login(f.ctx, domain.LoginInput{Email: "alice@example.com", Password: "wrong-password"})
		})
		r.Assert(func(t *testing.T) {
			assertCode(t, err, domain.CodeUnauthorized, "invalid credentials")
			requireEqual(t, got.Token, "")
		})
	})

	runService(t, component, testkit.Spec{
		ID: "IT-UC-AUTH-LOGIN-02", Method: "Login",
		Title:     "unknown email returns the same unauthorized error",
		Given:     "an empty users table",
		When:      "Login is called with missing@example.com",
		Then:      "repository not_found is hidden: unauthorized `invalid credentials`, identical to a wrong password",
		Technique: testkit.TechniqueErrorGuessing,
		Severity:  testkit.SeverityCritical,
	}, func(t *testing.T, r testkit.Report) {
		var f *appFixture
		var err error
		r.Arrange(func(t *testing.T) { f = newAppFixture(t) })
		r.Act(func(t *testing.T) {
			_, err = f.auth.Login(f.ctx, domain.LoginInput{Email: "missing@example.com", Password: "password1"})
		})
		r.Assert(func(t *testing.T) {
			assertCode(t, err, domain.CodeUnauthorized, "invalid credentials")
			requireEqual(t, f.count(t, `SELECT count(*) FROM users`), 0)
		})
	})

	runService(t, component, testkit.Spec{
		ID: "IT-UC-AUTH-LOGIN-03", Method: "Login",
		Title:     "login normalizes the email before lookup",
		Given:     "a registered user alice@example.com / password1",
		When:      "Login is called with `  ALICE@EXAMPLE.COM  `",
		Then:      "login succeeds and returns the stored user",
		Technique: testkit.TechniqueEquivalence,
		Params:    map[string]string{"email": "  ALICE@EXAMPLE.COM  "},
	}, func(t *testing.T, r testkit.Report) {
		var f *appFixture
		var registered, got domain.AuthResult
		var err error
		r.Arrange(func(t *testing.T) {
			f = newAppFixture(t)
			registered, err = f.auth.Register(f.ctx, domain.RegisterInput{Email: "alice@example.com", Password: "password1", Name: "Alice"})
			if err != nil {
				t.Fatalf("seed register: %v", err)
			}
		})
		r.Act(func(t *testing.T) {
			got, err = f.auth.Login(f.ctx, domain.LoginInput{Email: "  ALICE@EXAMPLE.COM  ", Password: "password1"})
		})
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("Login() error = %v", err)
			}
			requireEqual(t, got.User, registered.User)
		})
	})
}

func TestUserServiceIntegration(t *testing.T) {
	const component = "UserService"

	runService(t, component, testkit.Spec{
		ID: "IT-UC-USR-UPDATE-01", Method: "Update",
		Title:     "update persists the profile",
		Given:     "a user created through UserService",
		When:      "Update is called with email New@Example.com and name New",
		Then:      "the returned user has the lowercased email; GetByID reads the same values",
		Technique: testkit.TechniqueEquivalence,
	}, func(t *testing.T, r testkit.Report) {
		var f *appFixture
		var created, updated domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			f = newAppFixture(t)
			created = f.mustUser(t, "alice@example.com")
		})
		r.Act(func(t *testing.T) {
			updated, err = f.members.Update(f.ctx, created.ID, created.ID, domain.UserWrite{Email: "New@Example.com", Name: "New"})
		})
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("Update() error = %v", err)
			}
			requireEqual(t, updated, domain.User{ID: created.ID, Email: "new@example.com", Name: "New"})
			stored, readErr := f.members.GetByID(f.ctx, created.ID)
			if readErr != nil {
				t.Fatalf("GetByID() error = %v", readErr)
			}
			requireEqual(t, stored, updated)
		})
	})

	runService(t, component, testkit.Spec{
		ID: "IT-UC-USR-DELETE-01", Method: "Delete",
		Title:     "delete removes the account and queues UserDeleted for the other services",
		Given:     "a user deleting their own account",
		When:      "Delete is called by that user",
		Then:      "the users row is gone and one unsent identity.user.v1 outbox row keyed by the user id exists",
		Technique: testkit.TechniqueEquivalence,
		Severity:  testkit.SeverityCritical,
	}, func(t *testing.T, r testkit.Report) {
		var f *appFixture
		var user domain.User
		var err error
		r.Arrange(func(t *testing.T) {
			f = newAppFixture(t)
			user = f.mustUser(t, "alice@example.com")
		})
		r.Act(func(t *testing.T) { err = f.members.Delete(f.ctx, user.ID, user.ID) })
		r.Assert(func(t *testing.T) {
			if err != nil {
				t.Fatalf("Delete() error = %v", err)
			}
			requireEqual(t, f.count(t, `SELECT count(*) FROM users`), 0)
			requireEqual(t, f.count(t,
				`SELECT count(*) FROM outbox WHERE topic = 'identity.user.v1' AND record_key = $1 AND sent_at IS NULL`, user.ID), 1)
		})
	})
}
