package auth

import (
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
	key ed25519.PublicKey
}

func NewVerifier(key ed25519.PublicKey) *Verifier {
	return &Verifier{key: key}
}

func (v *Verifier) Parse(token string) (string, error) {
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
