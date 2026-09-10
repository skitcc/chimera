package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"

	"chimera/internal/config"
	"chimera/internal/domain"
)

type JWT struct {
	secret []byte
	ttl    time.Duration
}

func NewJWT(cfg config.Auth) *JWT {
	return &JWT{secret: []byte(cfg.JWTSecret), ttl: cfg.JWTTTL}
}

func (j *JWT) Issue(user domain.User) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":   user.ID,
		"email": user.Email,
		"exp":   now.Add(j.ttl).Unix(),
		"iat":   now.Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secret)
	if err != nil {
		return "", domain.Wrap(domain.CodeInternal, "sign token", err)
	}
	return token, nil
}

func (j *JWT) Parse(token string) (string, error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, domain.Unauthorized("invalid token")
		}
		return j.secret, nil
	})
	if err != nil || !parsed.Valid {
		return "", domain.Unauthorized("invalid token")
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return "", domain.Unauthorized("invalid token")
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", domain.Unauthorized("invalid token")
	}
	return sub, nil
}
