package usecase

import (
	"context"
	"testing"

	"chimera/internal/domain"
	"chimera/internal/testkit"
)

func TestAuthServiceRegister(t *testing.T) {
	const component, method = "AuthService", "Register"

	cases := []struct {
		id, title, given, then, technique string
		arrange                           func(*fakeUserRepository, *fakePasswordHasher, *fakeTokenIssuer) domain.RegisterInput
		want                              domain.AuthResult
		wantErr                           error
		wantCode                          domain.Code
		wantMsg                           string
		wantUsers                         int
		wantHash                          string
	}{
		{
			id:        "UC-AUTH-REG-01",
			title:     "registers user with normalized email and issues token",
			given:     "a valid register input whose email is padded and uppercase",
			then:      "the stored user with id user-1 and lowercase email is returned with token token-1, and the password is stored as its hash",
			technique: testkit.TechniqueEquivalence,
			arrange: func(_ *fakeUserRepository, _ *fakePasswordHasher, _ *fakeTokenIssuer) domain.RegisterInput {
				in := validRegisterInput()
				in.Email = " LISTENER@EXAMPLE.COM "
				return in
			},
			want:      domain.AuthResult{Token: "token-1", User: validUser()},
			wantUsers: 1,
			wantHash:  "hash:password",
		},
		{
			id:        "UC-AUTH-REG-02",
			title:     "rejects invalid email before hashing or storing",
			given:     "an email without @ while the hasher and the repository are both set to fail",
			then:      "invalid \"valid email is required\" is returned instead of the dependency error, and no user is stored",
			technique: testkit.TechniqueDecisionTable,
			arrange: func(users *fakeUserRepository, hasher *fakePasswordHasher, _ *fakeTokenIssuer) domain.RegisterInput {
				hasher.hashErr = errDependency
				users.createErr = errDependency
				in := validRegisterInput()
				in.Email = "invalid"
				return in
			},
			wantCode: domain.CodeInvalid,
			wantMsg:  "valid email is required",
		},
		{
			id:        "UC-AUTH-REG-03",
			title:     "rejects password of length 7 before hashing",
			given:     "a 7-character password while the hasher is set to fail",
			then:      "invalid \"password must be at least 8 characters\" is returned instead of the hashing error, and no user is stored",
			technique: testkit.TechniqueBoundary,
			arrange: func(_ *fakeUserRepository, hasher *fakePasswordHasher, _ *fakeTokenIssuer) domain.RegisterInput {
				hasher.hashErr = errDependency
				in := validRegisterInput()
				in.Password = "1234567"
				return in
			},
			wantCode: domain.CodeInvalid,
			wantMsg:  "password must be at least 8 characters",
		},
		{
			id:        "UC-AUTH-REG-04",
			title:     "returns hashing error without storing user",
			given:     "a valid input and a hasher that fails",
			then:      "the hasher error is returned unchanged, the result is empty, and no user is stored",
			technique: testkit.TechniqueErrorGuessing,
			arrange: func(_ *fakeUserRepository, hasher *fakePasswordHasher, _ *fakeTokenIssuer) domain.RegisterInput {
				hasher.hashErr = errDependency
				return validRegisterInput()
			},
			wantErr: errDependency,
		},
		{
			id:        "UC-AUTH-REG-05",
			title:     "returns repository create error without token",
			given:     "a valid input and a user repository whose Create fails",
			then:      "the repository error is returned unchanged, the result is empty, and no user is stored",
			technique: testkit.TechniqueErrorGuessing,
			arrange: func(users *fakeUserRepository, _ *fakePasswordHasher, _ *fakeTokenIssuer) domain.RegisterInput {
				users.createErr = errDependency
				return validRegisterInput()
			},
			wantErr: errDependency,
		},
		{
			id:        "UC-AUTH-REG-06",
			title:     "returns token error and keeps the created user",
			given:     "a valid input and a token issuer that fails",
			then:      "the issuer error is returned unchanged with an empty result, while the user stays stored (no rollback)",
			technique: testkit.TechniqueErrorGuessing,
			arrange: func(_ *fakeUserRepository, _ *fakePasswordHasher, tokens *fakeTokenIssuer) domain.RegisterInput {
				tokens.err = errDependency
				return validRegisterInput()
			},
			wantErr:   errDependency,
			wantUsers: 1,
			wantHash:  "hash:password",
		},
	}

	for _, tc := range cases {
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     tc.given,
			When:      "Register is called",
			Then:      tc.then,
			Technique: tc.technique,
			Severity:  testkit.SeverityCritical,
		}, func(t *testing.T, r testkit.Report) {
			var service *AuthService
			var users *fakeUserRepository
			var in domain.RegisterInput
			var got domain.AuthResult
			var err error
			r.Arrange(func(t *testing.T) {
				var hasher *fakePasswordHasher
				var tokens *fakeTokenIssuer
				service, users, hasher, tokens = newAuthFixture()
				in = tc.arrange(users, hasher, tokens)
			})
			r.Act(func(t *testing.T) { got, err = service.Register(context.Background(), in) })
			r.Assert(func(t *testing.T) {
				switch {
				case tc.wantErr != nil:
					assertErrorIs(t, err, tc.wantErr)
					assertEqual(t, domain.AuthResult{}, got)
				case tc.wantCode != "":
					assertDomainError(t, err, tc.wantCode, tc.wantMsg)
					assertEqual(t, domain.AuthResult{}, got)
				default:
					assertNoError(t, err)
					assertEqual(t, tc.want, got)
				}
				assertEqual(t, tc.wantUsers, len(users.users))
				assertEqual(t, tc.wantHash, users.passwords[testUserID])
			})
		})
	}
}

func TestAuthServiceLogin(t *testing.T) {
	const component, method = "AuthService", "Login"

	seedUser := func(users *fakeUserRepository) {
		user := validUser()
		users.users[user.ID] = user
		users.passwords[user.ID] = "hash:password"
	}

	cases := []struct {
		id, title, given, then, technique string
		arrange                           func(*fakeUserRepository, *fakePasswordHasher, *fakeTokenIssuer) domain.LoginInput
		want                              domain.AuthResult
		wantErr                           error
		wantErrText                       string
		wantCode                          domain.Code
		wantMsg                           string
	}{
		{
			id:        "UC-AUTH-LOGIN-01",
			title:     "authenticates normalized email and issues token",
			given:     "a stored user with hash of \"password\" and a login input with padded uppercase email",
			then:      "the stored user is returned with token token-1",
			technique: testkit.TechniqueEquivalence,
			arrange: func(users *fakeUserRepository, _ *fakePasswordHasher, _ *fakeTokenIssuer) domain.LoginInput {
				seedUser(users)
				in := validLoginInput()
				in.Email = " LISTENER@EXAMPLE.COM "
				return in
			},
			want: domain.AuthResult{Token: "token-1", User: validUser()},
		},
		{
			id:        "UC-AUTH-LOGIN-02",
			title:     "rejects empty password before user lookup",
			given:     "an empty password while the user repository is set to fail",
			then:      "invalid \"password is required\" is returned instead of the repository error",
			technique: testkit.TechniqueBoundary,
			arrange: func(users *fakeUserRepository, _ *fakePasswordHasher, _ *fakeTokenIssuer) domain.LoginInput {
				users.getErr = errDependency
				in := validLoginInput()
				in.Password = ""
				return in
			},
			wantCode: domain.CodeInvalid,
			wantMsg:  "password is required",
		},
		{
			id:        "UC-AUTH-LOGIN-03",
			title:     "rejects whitespace-only email before user lookup",
			given:     "an email of three spaces while the user repository is set to fail",
			then:      "invalid \"email is required\" is returned instead of the repository error",
			technique: testkit.TechniqueBoundary,
			arrange: func(users *fakeUserRepository, _ *fakePasswordHasher, _ *fakeTokenIssuer) domain.LoginInput {
				users.getErr = errDependency
				in := validLoginInput()
				in.Email = "   "
				return in
			},
			wantCode: domain.CodeInvalid,
			wantMsg:  "email is required",
		},
		{
			id:        "UC-AUTH-LOGIN-04",
			title:     "maps unknown email to invalid credentials",
			given:     "no stored user with the login email",
			then:      "unauthorized \"invalid credentials\" is returned instead of not_found, so account existence is not revealed",
			technique: testkit.TechniqueEquivalence,
			arrange: func(_ *fakeUserRepository, _ *fakePasswordHasher, _ *fakeTokenIssuer) domain.LoginInput {
				return validLoginInput()
			},
			wantCode: domain.CodeUnauthorized,
			wantMsg:  "invalid credentials",
		},
		{
			id:        "UC-AUTH-LOGIN-05",
			title:     "returns repository lookup error unchanged",
			given:     "a user repository whose GetByEmail fails with a non-domain error",
			then:      "the repository error is returned unchanged and not mapped to unauthorized",
			technique: testkit.TechniqueErrorGuessing,
			arrange: func(users *fakeUserRepository, _ *fakePasswordHasher, _ *fakeTokenIssuer) domain.LoginInput {
				users.getErr = errDependency
				return validLoginInput()
			},
			wantErr: errDependency,
		},
		{
			id:        "UC-AUTH-LOGIN-06",
			title:     "returns hasher mismatch error for wrong password",
			given:     "a stored user with hash of \"password\" and a login with password \"wrong-password\"",
			then:      "the hasher mismatch error \"password mismatch\" is returned as is and no token is issued",
			technique: testkit.TechniqueEquivalence,
			arrange: func(users *fakeUserRepository, _ *fakePasswordHasher, _ *fakeTokenIssuer) domain.LoginInput {
				seedUser(users)
				in := validLoginInput()
				in.Password = "wrong-password"
				return in
			},
			wantErrText: "password mismatch",
		},
		{
			id:        "UC-AUTH-LOGIN-07",
			title:     "returns password comparison failure without token",
			given:     "a stored user and a hasher whose Compare fails",
			then:      "the hasher error is returned unchanged and the result is empty",
			technique: testkit.TechniqueErrorGuessing,
			arrange: func(users *fakeUserRepository, hasher *fakePasswordHasher, _ *fakeTokenIssuer) domain.LoginInput {
				seedUser(users)
				hasher.compareErr = errDependency
				return validLoginInput()
			},
			wantErr: errDependency,
		},
		{
			id:        "UC-AUTH-LOGIN-08",
			title:     "returns token issuer error",
			given:     "a stored user with matching password and a token issuer that fails",
			then:      "the issuer error is returned unchanged and the result is empty",
			technique: testkit.TechniqueErrorGuessing,
			arrange: func(users *fakeUserRepository, _ *fakePasswordHasher, tokens *fakeTokenIssuer) domain.LoginInput {
				seedUser(users)
				tokens.err = errDependency
				return validLoginInput()
			},
			wantErr: errDependency,
		},
	}

	for _, tc := range cases {
		runSpec(t, component, testkit.Spec{
			ID: tc.id, Method: method, Title: tc.title,
			Given:     tc.given,
			When:      "Login is called",
			Then:      tc.then,
			Technique: tc.technique,
			Severity:  testkit.SeverityCritical,
		}, func(t *testing.T, r testkit.Report) {
			var service *AuthService
			var in domain.LoginInput
			var got domain.AuthResult
			var err error
			r.Arrange(func(t *testing.T) {
				var users *fakeUserRepository
				var hasher *fakePasswordHasher
				var tokens *fakeTokenIssuer
				service, users, hasher, tokens = newAuthFixture()
				in = tc.arrange(users, hasher, tokens)
			})
			r.Act(func(t *testing.T) { got, err = service.Login(context.Background(), in) })
			r.Assert(func(t *testing.T) {
				switch {
				case tc.wantErr != nil:
					assertErrorIs(t, err, tc.wantErr)
					assertEqual(t, domain.AuthResult{}, got)
				case tc.wantErrText != "":
					if err == nil || err.Error() != tc.wantErrText {
						t.Fatalf("want error %q, got %v", tc.wantErrText, err)
					}
					assertEqual(t, domain.AuthResult{}, got)
				case tc.wantCode != "":
					assertDomainError(t, err, tc.wantCode, tc.wantMsg)
					assertEqual(t, domain.AuthResult{}, got)
				default:
					assertNoError(t, err)
					assertEqual(t, tc.want, got)
				}
			})
		})
	}
}
