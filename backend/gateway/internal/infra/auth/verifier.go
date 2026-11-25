package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"

	"github.com/golang-jwt/jwt/v5"

	"chimera/gateway/internal/domain"
)

// Verifier checks access tokens issued by Identity. It holds only the public
// key, so the Gateway can verify tokens but never mint them.
type Verifier struct {
	key   ed25519.PublicKey
	check func(context.Context, string, int64) error
}

func NewVerifier(key ed25519.PublicKey) *Verifier {
	return &Verifier{key: key}
}

func (v *Verifier) WithValidation(check func(context.Context, string, int64) error) *Verifier {
	v.check = check
	return v
}
func (v *Verifier) Parse(token string) (string, error) {
	return v.ParseContext(context.Background(), token)
}
func (v *Verifier) ParseContext(ctx context.Context, token string) (string, error) {
	parsed, err := jwt.Parse(token, func(*jwt.Token) (any, error) {
		return v.key, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}), jwt.WithExpirationRequired())
	if err != nil || !parsed.Valid {
		return "", domain.Unauthorized("invalid token")
	}
	sub, err := parsed.Claims.GetSubject()
	if err != nil || sub == "" {
		return "", domain.Unauthorized("invalid token")
	}
	if v.check != nil {
		claims, ok := parsed.Claims.(jwt.MapClaims)
		if !ok {
			return "", domain.Unauthorized("invalid token")
		}
		version, ok := claims["ver"].(float64)
		if !ok || version < 1 || version != float64(int64(version)) {
			return "", domain.Unauthorized("invalid token")
		}
		if err := v.check(ctx, sub, int64(version)); err != nil {
			if domain.Is(err, domain.CodeUnauthorized) || domain.Is(err, domain.CodeNotFound) {
				return "", domain.Unauthorized("invalid token")
			}
			return "", domain.Unavailable("session validation unavailable")
		}
	}
	return sub, nil
}

// LoadPublicKey reads a PKIX PEM file, e.g. from `openssl pkey -pubout`.
func LoadPublicKey(path string) (ed25519.PublicKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("no PEM block in public key file")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("public key is not Ed25519")
	}
	return key, nil
}
