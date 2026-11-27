package config

import (
	"chimera/identity/internal/infra/auth"
	"errors"
	"os"
	"strconv"
	"time"

	"chimera/platform/env"
)

type Auth struct {
	SecurityEnabled bool
	CodeKey         []byte
	SMTP            auth.SMTP
	PrivateKeyFile  string
	TTL             time.Duration
}

type Config struct {
	Base     env.Base
	GRPC     env.GRPCServer
	Postgres env.Postgres
	Kafka    env.Kafka
	Health   env.Health
	Auth     Auth
}

func Load() (Config, error) {
	base, err := env.LoadBase()
	if err != nil {
		return Config{}, err
	}
	grpcCfg, err := env.LoadGRPCServer()
	if err != nil {
		return Config{}, err
	}
	health, err := env.LoadHealth()
	if err != nil {
		return Config{}, err
	}
	pg, err := env.LoadPostgres(health)
	if err != nil {
		return Config{}, err
	}
	kafka, err := env.LoadKafka()
	if err != nil {
		return Config{}, err
	}
	keyFile, err := env.String("JWT_PRIVATE_KEY_FILE")
	if err != nil {
		return Config{}, err
	}
	ttl, err := env.Duration("JWT_TTL")
	if err != nil {
		return Config{}, err
	}
	securityEnabled := true
	if value := os.Getenv("AUTH_SECURITY_ENABLED"); value != "" {
		securityEnabled, err = strconv.ParseBool(value)
		if err != nil {
			return Config{}, err
		}
	}
	codeKey := []byte(os.Getenv("AUTH_CODE_HMAC_KEY"))
	if securityEnabled && len(codeKey) < 32 {
		return Config{}, errors.New("AUTH_CODE_HMAC_KEY must contain at least 32 bytes")
	}
	smtpAddr := os.Getenv("SMTP_ADDR")
	if smtpAddr == "" {
		smtpAddr = "mailpit:1025"
	}
	smtpFrom := os.Getenv("SMTP_FROM")
	if smtpFrom == "" {
		smtpFrom = "no-reply@chimera.local"
	}
	requireTLS := true
	if v := os.Getenv("SMTP_REQUIRE_TLS"); v != "" {
		requireTLS, err = strconv.ParseBool(v)
		if err != nil {
			return Config{}, err
		}
	}
	return Config{
		Base:     base,
		GRPC:     grpcCfg,
		Postgres: pg,
		Kafka:    kafka,
		Health:   health,
		Auth:     Auth{PrivateKeyFile: keyFile, TTL: ttl, SecurityEnabled: securityEnabled, CodeKey: codeKey, SMTP: auth.SMTP{Addr: smtpAddr, From: smtpFrom, Username: os.Getenv("SMTP_USERNAME"), Password: os.Getenv("SMTP_PASSWORD"), RequireTLS: requireTLS}},
	}, nil
}
