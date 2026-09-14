package config

import (
	"bufio"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Mode string

const (
	ModeDevelopment Mode = "development"
	ModeProduction  Mode = "production"
)

type Config struct {
	Mode     Mode
	HTTP     HTTP
	Log      Log
	Postgres Postgres
	Auth     Auth
	S3       S3
	Upload   Upload
	Workers  Workers
	Health   Health
}

type HTTP struct {
	Addr            string
	ShutdownTimeout time.Duration
}

type Log struct {
	Level string
}

type Postgres struct {
	DSN               string
	MinConns          int32
	MaxConns          int32
	MaxConnIdleTime   time.Duration
	MaxConnLifetime   time.Duration
	HealthCheckPeriod time.Duration
}

type Auth struct {
	JWTSecret string
	JWTTTL    time.Duration
}

type Workers struct {
	Size int
}

type Health struct {
	Interval time.Duration
	Timeout  time.Duration
}

type S3 struct {
	Endpoint        string
	PresignEndpoint string
	AccessKey       string
	SecretKey       string
	Bucket          string
	Region          string
	UseSSL          bool
	PresignTTL      time.Duration
}

type Upload struct {
	MaxBytes int64
}

func Load() (Config, error) {
	loadDotEnv(".env")

	mode, err := require("APP_MODE")
	if err != nil {
		return Config{}, err
	}
	addr, err := require("HTTP_ADDR")
	if err != nil {
		return Config{}, err
	}
	dsn, err := require("DATABASE_URL")
	if err != nil {
		return Config{}, err
	}
	secret, err := require("JWT_SECRET")
	if err != nil {
		return Config{}, err
	}
	ttl, err := requireDuration("JWT_TTL")
	if err != nil {
		return Config{}, err
	}
	shutdown, err := requireDuration("HTTP_SHUTDOWN_TIMEOUT")
	if err != nil {
		return Config{}, err
	}
	pg, err := loadPostgres(dsn)
	if err != nil {
		return Config{}, err
	}
	s3, err := loadS3()
	if err != nil {
		return Config{}, err
	}
	upload, err := loadUpload()
	if err != nil {
		return Config{}, err
	}
	workers, err := loadWorkers()
	if err != nil {
		return Config{}, err
	}
	health, err := loadHealth()
	if err != nil {
		return Config{}, err
	}
	pg.HealthCheckPeriod = health.Interval

	cfg := Config{
		Mode:     Mode(strings.ToLower(mode)),
		HTTP:     HTTP{Addr: addr, ShutdownTimeout: shutdown},
		Log:      Log{Level: os.Getenv("LOG_LEVEL")},
		Postgres: pg,
		Auth:     Auth{JWTSecret: secret, JWTTTL: ttl},
		S3:       s3,
		Upload:   upload,
		Workers:  workers,
		Health:   health,
	}
	if cfg.Mode != ModeDevelopment && cfg.Mode != ModeProduction {
		return Config{}, errors.New("APP_MODE must be development or production")
	}
	return cfg, nil
}

func loadPostgres(dsn string) (Postgres, error) {
	minConns, err := requireInt32("POSTGRES_MIN_CONNS")
	if err != nil {
		return Postgres{}, err
	}
	maxConns, err := requireInt32("POSTGRES_MAX_CONNS")
	if err != nil {
		return Postgres{}, err
	}
	if minConns > maxConns {
		return Postgres{}, errors.New("POSTGRES_MIN_CONNS must be <= POSTGRES_MAX_CONNS")
	}
	idle, err := requireDuration("POSTGRES_MAX_CONN_IDLE")
	if err != nil {
		return Postgres{}, err
	}
	lifetime, err := requireDuration("POSTGRES_MAX_CONN_LIFETIME")
	if err != nil {
		return Postgres{}, err
	}
	return Postgres{
		DSN:             dsn,
		MinConns:        minConns,
		MaxConns:        maxConns,
		MaxConnIdleTime: idle,
		MaxConnLifetime: lifetime,
	}, nil
}

func loadWorkers() (Workers, error) {
	size, err := requireInt("WORKER_POOL_SIZE")
	if err != nil {
		return Workers{}, err
	}
	return Workers{Size: size}, nil
}

func loadHealth() (Health, error) {
	interval, err := requireDuration("HEALTH_CHECK_INTERVAL")
	if err != nil {
		return Health{}, err
	}
	timeout, err := requireDuration("HEALTH_CHECK_TIMEOUT")
	if err != nil {
		return Health{}, err
	}
	if timeout >= interval {
		return Health{}, errors.New("HEALTH_CHECK_TIMEOUT must be < HEALTH_CHECK_INTERVAL")
	}
	return Health{Interval: interval, Timeout: timeout}, nil
}

func loadS3() (S3, error) {
	endpoint, err := require("S3_ENDPOINT")
	if err != nil {
		return S3{}, err
	}
	access, err := require("S3_ACCESS_KEY")
	if err != nil {
		return S3{}, err
	}
	secret, err := require("S3_SECRET_KEY")
	if err != nil {
		return S3{}, err
	}
	bucket, err := require("S3_BUCKET")
	if err != nil {
		return S3{}, err
	}
	region, err := require("S3_REGION")
	if err != nil {
		return S3{}, err
	}
	ttlRaw, err := require("S3_PRESIGN_TTL")
	if err != nil {
		return S3{}, err
	}
	ttl, err := time.ParseDuration(ttlRaw)
	if err != nil || ttl <= 0 {
		return S3{}, errors.New("S3_PRESIGN_TTL is invalid")
	}

	host, useSSL, err := parseS3Endpoint(endpoint)
	if err != nil {
		return S3{}, err
	}
	presign := strings.TrimSpace(os.Getenv("S3_PRESIGN_ENDPOINT"))
	if presign == "" {
		presign = endpoint
	}
	if _, _, err := parseS3Endpoint(presign); err != nil {
		return S3{}, errors.New("S3_PRESIGN_ENDPOINT is invalid")
	}

	return S3{
		Endpoint:        host,
		PresignEndpoint: presign,
		AccessKey:       access,
		SecretKey:       secret,
		Bucket:          bucket,
		Region:          region,
		UseSSL:          useSSL,
		PresignTTL:      ttl,
	}, nil
}

func loadUpload() (Upload, error) {
	raw, err := require("TRACK_MAX_BYTES")
	if err != nil {
		return Upload{}, err
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return Upload{}, errors.New("TRACK_MAX_BYTES is invalid")
	}
	return Upload{MaxBytes: n}, nil
}

func parseS3Endpoint(raw string) (host string, useSSL bool, err error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", false, errors.New("s3 endpoint is invalid")
	}
	return u.Host, u.Scheme == "https", nil
}

func require(name string) (string, error) {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return "", errors.New("missing required env " + name)
	}
	return v, nil
}

func requireInt(name string) (int, error) {
	raw, err := require(name)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, errors.New(name + " is invalid")
	}
	return n, nil
}

func requireInt32(name string) (int32, error) {
	n, err := requireInt(name)
	if err != nil {
		return 0, err
	}
	return int32(n), nil
}

func requireDuration(name string) (time.Duration, error) {
	raw, err := require(name)
	if err != nil {
		return 0, err
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0, errors.New(name + " is invalid")
	}
	return d, nil
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
	if sc.Err() != nil {
		return
	}
}
