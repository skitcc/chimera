package config

import (
	"time"

	"chimera/platform/env"
)

type Auth struct {
	PrivateKeyFile string
	TTL            time.Duration
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
	return Config{
		Base:     base,
		GRPC:     grpcCfg,
		Postgres: pg,
		Kafka:    kafka,
		Health:   health,
		Auth:     Auth{PrivateKeyFile: keyFile, TTL: ttl},
	}, nil
}
