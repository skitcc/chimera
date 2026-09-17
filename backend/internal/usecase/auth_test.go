package usecase

import (
	"context"
	"testing"

	"chimera/internal/domain"
)

func TestAuthServiceRegister(t *testing.T) {
	tests := []struct {
		name      string
		arrange   func(*fakeUserRepository, *fakePasswordHasher, *fakeTokenIssuer) domain.RegisterInput
		want      domain.AuthResult
		wantErr   error
		wantCode  domain.Code
		wantUsers int
	}{
		{
			name: "registers user and issues token",
			arrange: func(_ *fakeUserRepository, _ *fakePasswordHasher, _ *fakeTokenIssuer) domain.RegisterInput {
				in := validRegisterInput()
				in.Email = " LISTENER@EXAMPLE.COM "
				return in
			},
			want: domain.AuthResult{
				Token: "token-1",
				User:  validUser(),
			},
			wantUsers: 1,
		},
		{
			name: "rejects invalid input before dependencies",
			arrange: func(_ *fakeUserRepository, _ *fakePasswordHasher, _ *fakeTokenIssuer) domain.RegisterInput {
				in := validRegisterInput()
				in.Email = "invalid"
				return in
			},
			wantCode: domain.CodeInvalid,
		},
		{
			name: "returns hashing error",
			arrange: func(_ *fakeUserRepository, hasher *fakePasswordHasher, _ *fakeTokenIssuer) domain.RegisterInput {
				hasher.hashErr = errDependency
				return validRegisterInput()
			},
			wantErr: errDependency,
		},
		{
			name: "returns repository error",
			arrange: func(users *fakeUserRepository, _ *fakePasswordHasher, _ *fakeTokenIssuer) domain.RegisterInput {
				users.createErr = errDependency
				return validRegisterInput()
			},
			wantErr: errDependency,
		},
		{
			name: "returns token error after creating user",
			arrange: func(_ *fakeUserRepository, _ *fakePasswordHasher, tokens *fakeTokenIssuer) domain.RegisterInput {
				tokens.err = errDependency
				return validRegisterInput()
			},
			wantErr:   errDependency,
			wantUsers: 1,
		},
	}

	for _, tt := range tests {
		runCase(t, tt.name, func(t *testing.T) {
			// Arrange
			service, users, hasher, tokens := newAuthFixture()
			in := tt.arrange(users, hasher, tokens)

			// Act
			got, err := service.Register(context.Background(), in)

			// Assert
			if tt.wantErr != nil {
				assertErrorIs(t, err, tt.wantErr)
			} else if tt.wantCode != "" {
				assertErrorCode(t, err, tt.wantCode)
			} else {
				if err != nil {
					t.Fatalf("Register() error = %v", err)
				}
				assertEqual(t, tt.want, got)
			}
			assertEqual(t, tt.wantUsers, len(users.users))
		})
	}
}

func TestAuthServiceLogin(t *testing.T) {
	tests := []struct {
		name     string
		arrange  func(*fakeUserRepository, *fakePasswordHasher, *fakeTokenIssuer) domain.LoginInput
		want     domain.AuthResult
		wantErr  error
		wantCode domain.Code
	}{
		{
			name: "authenticates user and issues token",
			arrange: func(users *fakeUserRepository, _ *fakePasswordHasher, _ *fakeTokenIssuer) domain.LoginInput {
				user := validUser()
				users.users[user.ID] = user
				users.passwords[user.ID] = "hash:password"
				in := validLoginInput()
				in.Email = " LISTENER@EXAMPLE.COM "
				return in
			},
			want: domain.AuthResult{Token: "token-1", User: validUser()},
		},
		{
			name: "rejects invalid input",
			arrange: func(_ *fakeUserRepository, _ *fakePasswordHasher, _ *fakeTokenIssuer) domain.LoginInput {
				in := validLoginInput()
				in.Password = ""
				return in
			},
			wantCode: domain.CodeInvalid,
		},
		{
			name: "maps missing user to unauthorized",
			arrange: func(_ *fakeUserRepository, _ *fakePasswordHasher, _ *fakeTokenIssuer) domain.LoginInput {
				return validLoginInput()
			},
			wantCode: domain.CodeUnauthorized,
		},
		{
			name: "returns repository error",
			arrange: func(users *fakeUserRepository, _ *fakePasswordHasher, _ *fakeTokenIssuer) domain.LoginInput {
				users.getErr = errDependency
				return validLoginInput()
			},
			wantErr: errDependency,
		},
		{
			name: "returns password comparison error",
			arrange: func(users *fakeUserRepository, hasher *fakePasswordHasher, _ *fakeTokenIssuer) domain.LoginInput {
				user := validUser()
				users.users[user.ID] = user
				users.passwords[user.ID] = "hash:password"
				hasher.compareErr = errDependency
				return validLoginInput()
			},
			wantErr: errDependency,
		},
		{
			name: "returns token error",
			arrange: func(users *fakeUserRepository, _ *fakePasswordHasher, tokens *fakeTokenIssuer) domain.LoginInput {
				user := validUser()
				users.users[user.ID] = user
				users.passwords[user.ID] = "hash:password"
				tokens.err = errDependency
				return validLoginInput()
			},
			wantErr: errDependency,
		},
	}

	for _, tt := range tests {
		runCase(t, tt.name, func(t *testing.T) {
			// Arrange
			service, users, hasher, tokens := newAuthFixture()
			in := tt.arrange(users, hasher, tokens)

			// Act
			got, err := service.Login(context.Background(), in)

			// Assert
			if tt.wantErr != nil {
				assertErrorIs(t, err, tt.wantErr)
				return
			}
			if tt.wantCode != "" {
				assertErrorCode(t, err, tt.wantCode)
				return
			}
			if err != nil {
				t.Fatalf("Login() error = %v", err)
			}
			assertEqual(t, tt.want, got)
		})
	}
}
