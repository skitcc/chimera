package config

import (
	"errors"
	"net/url"

	"chimera/catalog/internal/infra/s3"
	"chimera/platform/env"
)

type Config struct {
	Base          env.Base
	GRPC          env.GRPCServer
	Postgres      env.Postgres
	Kafka         env.Kafka
	Health        env.Health
	S3            s3.Config
	TrackMaxBytes int64
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
	store, err := loadS3()
	if err != nil {
		return Config{}, err
	}
	maxBytes, err := env.Int64("TRACK_MAX_BYTES")
	if err != nil {
		return Config{}, err
	}
	if maxBytes <= 0 {
		return Config{}, errors.New("TRACK_MAX_BYTES is invalid")
	}
	return Config{
		Base:          base,
		GRPC:          grpcCfg,
		Postgres:      pg,
		Kafka:         kafka,
		Health:        health,
		S3:            store,
		TrackMaxBytes: maxBytes,
	}, nil
}

func loadS3() (s3.Config, error) {
	endpoint, err := env.String("S3_ENDPOINT")
	if err != nil {
		return s3.Config{}, err
	}
	host, err := endpointHost(endpoint)
	if err != nil {
		return s3.Config{}, errors.New("S3_ENDPOINT is invalid")
	}
	presign := env.Optional("S3_PRESIGN_ENDPOINT")
	if presign == "" {
		presign = endpoint
	}
	if _, err := endpointHost(presign); err != nil {
		return s3.Config{}, errors.New("S3_PRESIGN_ENDPOINT is invalid")
	}
	access, err := env.String("S3_ACCESS_KEY")
	if err != nil {
		return s3.Config{}, err
	}
	secret, err := env.String("S3_SECRET_KEY")
	if err != nil {
		return s3.Config{}, err
	}
	bucket, err := env.String("S3_BUCKET")
	if err != nil {
		return s3.Config{}, err
	}
	region, err := env.String("S3_REGION")
	if err != nil {
		return s3.Config{}, err
	}
	ttl, err := env.Duration("S3_PRESIGN_TTL")
	if err != nil {
		return s3.Config{}, err
	}
	return s3.Config{
		Endpoint:        host,
		PresignEndpoint: presign,
		AccessKey:       access,
		SecretKey:       secret,
		Bucket:          bucket,
		Region:          region,
		PresignTTL:      ttl,
	}, nil
}

func endpointHost(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", errors.New("s3 endpoint is invalid")
	}
	return u.Host, nil
}
