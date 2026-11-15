package config

import (
	"time"

	"chimera/platform/env"
)

type Upstreams struct {
	CatalogAddr  string
	IdentityAddr string
	Timeout      time.Duration
}

type Config struct {
	Base      env.Base
	GRPC      env.GRPCServer
	Postgres  env.Postgres
	Kafka     env.Kafka
	Health    env.Health
	Upstreams Upstreams
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
	// Social only consumes, so it has no outbox settings.
	brokers, err := env.List("KAFKA_BROKERS")
	if err != nil {
		return Config{}, err
	}
	upstreams, err := loadUpstreams()
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:      base,
		GRPC:      grpcCfg,
		Postgres:  pg,
		Kafka:     env.Kafka{Brokers: brokers},
		Health:    health,
		Upstreams: upstreams,
	}, nil
}

func loadUpstreams() (Upstreams, error) {
	catalog, err := env.String("CATALOG_GRPC_ADDR")
	if err != nil {
		return Upstreams{}, err
	}
	identity, err := env.String("IDENTITY_GRPC_ADDR")
	if err != nil {
		return Upstreams{}, err
	}
	timeout, err := env.Duration("GRPC_CLIENT_TIMEOUT")
	if err != nil {
		return Upstreams{}, err
	}
	return Upstreams{CatalogAddr: catalog, IdentityAddr: identity, Timeout: timeout}, nil
}
