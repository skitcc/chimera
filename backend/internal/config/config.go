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
}

type HTTP struct {
	Addr            string
	ShutdownTimeout time.Duration
}

type Log struct {
	Level string
}

type Postgres struct {
	DSN string
}

type Auth struct {
	JWTSecret string
	JWTTTL    time.Duration
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
	ttlRaw, err := require("JWT_TTL")
	if err != nil {
		return Config{}, err
	}
	ttl, err := time.ParseDuration(ttlRaw)
	if err != nil || ttl <= 0 {
		return Config{}, errors.New("JWT_TTL is invalid")
	}
	shutdownRaw, err := require("HTTP_SHUTDOWN_TIMEOUT")
	if err != nil {
		return Config{}, err
	}
	shutdown, err := time.ParseDuration(shutdownRaw)
	if err != nil || shutdown <= 0 {
		return Config{}, errors.New("HTTP_SHUTDOWN_TIMEOUT is invalid")
	}

	s3, err := loadS3()
	if err != nil {
		return Config{}, err
	}
	upload, err := loadUpload()
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Mode:     Mode(strings.ToLower(mode)),
		HTTP:     HTTP{Addr: addr, ShutdownTimeout: shutdown},
		Log:      Log{Level: os.Getenv("LOG_LEVEL")},
		Postgres: Postgres{DSN: dsn},
		Auth:     Auth{JWTSecret: secret, JWTTTL: ttl},
		S3:       s3,
		Upload:   upload,
	}
	if cfg.Mode != ModeDevelopment && cfg.Mode != ModeProduction {
		return Config{}, errors.New("APP_MODE must be development or production")
	}
	return cfg, nil
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
