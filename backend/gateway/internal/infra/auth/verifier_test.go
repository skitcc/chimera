package auth_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"chimera/gateway/internal/domain"
	"chimera/gateway/internal/infra/auth"
)

func newKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return pub, priv
}

func sign(t *testing.T, method jwt.SigningMethod, key any, claims jwt.MapClaims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return token
}

func TestVerifierParse(t *testing.T) {
	pub, priv := newKey(t)
	_, otherPriv := newKey(t)
	future := time.Now().Add(time.Hour).Unix()
	verifier := auth.NewVerifier(pub)

	t.Run("accepts a token signed by Identity", func(t *testing.T) {
		token := sign(t, jwt.SigningMethodEdDSA, priv, jwt.MapClaims{"sub": "user-1", "exp": future})
		got, err := verifier.Parse(token)
		if err != nil || got != "user-1" {
			t.Fatalf("Parse() = %q, %v", got, err)
		}
	})

	rejected := []struct {
		name  string
		token string
	}{
		{"another key", sign(t, jwt.SigningMethodEdDSA, otherPriv, jwt.MapClaims{"sub": "user-1", "exp": future})},
		{"HS256 with the public key as secret", sign(t, jwt.SigningMethodHS256, []byte(pub), jwt.MapClaims{"sub": "user-1", "exp": future})},
		{"expired", sign(t, jwt.SigningMethodEdDSA, priv, jwt.MapClaims{"sub": "user-1", "exp": time.Now().Add(-time.Minute).Unix()})},
		{"without exp", sign(t, jwt.SigningMethodEdDSA, priv, jwt.MapClaims{"sub": "user-1"})},
		{"without sub", sign(t, jwt.SigningMethodEdDSA, priv, jwt.MapClaims{"exp": future})},
		{"garbage", "not-a-jwt"},
	}
	for _, tc := range rejected {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			_, err := verifier.Parse(tc.token)
			app, ok := domain.As(err)
			if !ok || app.Code != domain.CodeUnauthorized || app.Message != "invalid token" {
				t.Fatalf("Parse() error = %v, want unauthorized invalid token", err)
			}
		})
	}
}

func TestLoadPublicKey(t *testing.T) {
	pub, _ := newKey(t)
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(t.TempDir(), "jwt.pub")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := auth.LoadPublicKey(path)
	if err != nil || !got.Equal(pub) {
		t.Fatalf("LoadPublicKey() = %v, %v", got, err)
	}

	bad := filepath.Join(t.TempDir(), "bad.pub")
	if err := os.WriteFile(bad, []byte("nope"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := auth.LoadPublicKey(bad); err == nil {
		t.Fatal("LoadPublicKey() accepted a file without PEM")
	}
}

func TestVerifierValidatesPrimaryVersionAndFailsClosed(t *testing.T) {
	pub, priv := newKey(t)
	token := sign(t, jwt.SigningMethodEdDSA, priv, jwt.MapClaims{"sub": "user-1", "ver": 2, "exp": time.Now().Add(time.Hour).Unix()})
	calls := 0
	verifier := auth.NewVerifier(pub).WithValidation(func(ctx context.Context, id string, version int64) error {
		calls++
		if id != "user-1" || version != 2 {
			t.Fatal("wrong validation input")
		}
		return nil
	})
	if _, err := verifier.ParseContext(context.Background(), token); err != nil || calls != 1 {
		t.Fatal("current session rejected")
	}
	verifier.WithValidation(func(context.Context, string, int64) error { return errors.New("primary unavailable") })
	if _, err := verifier.ParseContext(context.Background(), token); !domain.Is(err, domain.CodeUnavailable) {
		t.Fatal("primary outage must reject access without declaring the token revoked")
	}
	legacy := sign(t, jwt.SigningMethodEdDSA, priv, jwt.MapClaims{"sub": "user-1", "exp": time.Now().Add(time.Hour).Unix()})
	if _, err := verifier.ParseContext(context.Background(), legacy); err == nil {
		t.Fatal("unversioned token accepted in secure mode")
	}
	if _, err := auth.NewVerifier(pub).Parse(legacy); err != nil {
		t.Fatal("legacy perf token rejected in legacy mode")
	}
}
