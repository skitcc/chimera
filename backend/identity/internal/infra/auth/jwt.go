package auth

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"chimera/identity/internal/domain"
)

// JWT signs access tokens with Ed25519. Only Identity holds the private key;
// the Gateway verifies tokens with the public half.
type JWT struct {
	key ed25519.PrivateKey
	ttl time.Duration
}

func NewJWT(key ed25519.PrivateKey, ttl time.Duration) *JWT {
	return &JWT{key: key, ttl: ttl}
}

func (j *JWT) Issue(user domain.User) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":   user.ID,
		"ver":   user.AuthVersion,
		"email": user.Email,
		"exp":   now.Add(j.ttl).Unix(),
		"iat":   now.Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).SignedString(j.key)
	if err != nil {
		return "", domain.Wrap(domain.CodeInternal, "sign token", err)
	}
	return token, nil
}

// LoadPrivateKey reads a PKCS#8 PEM file, e.g. from `openssl genpkey -algorithm ed25519`.
func LoadPrivateKey(path string) (ed25519.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("no PEM block in private key file")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not Ed25519")
	}
	return key, nil
}
