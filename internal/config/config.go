package config

import (
	"bufio"
	"errors"
	"os"
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

	cfg := Config{
		Mode:     Mode(strings.ToLower(mode)),
		HTTP:     HTTP{Addr: addr, ShutdownTimeout: shutdown},
		Log:      Log{Level: os.Getenv("LOG_LEVEL")},
		Postgres: Postgres{DSN: dsn},
		Auth:     Auth{JWTSecret: secret, JWTTTL: ttl},
	}
	if cfg.Mode != ModeDevelopment && cfg.Mode != ModeProduction {
		return Config{}, errors.New("APP_MODE must be development or production")
	}
	return cfg, nil
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
}
