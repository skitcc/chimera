package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Mode string

const (
	ModeDevelopment Mode = "development"
	ModeProduction  Mode = "production"
)

func (m Mode) IsProduction() bool {
	return m == ModeProduction
}

type Config struct {
	Mode Mode `yaml:"mode"`
	HTTP HTTP `yaml:"http"`
	Log  Log  `yaml:"log"`
}

type HTTP struct {
	Addr string `yaml:"addr"`
}

type Log struct {
	Level string `yaml:"level"`
}

func Defaults() Config {
	return Config{
		Mode: ModeDevelopment,
		HTTP: HTTP{Addr: ":8080"},
	}
}

func Load(path string) (Config, error) {
	cfg := Defaults()

	if path == "" {
		path = os.Getenv("CHIMERA_CONFIG")
	}
	if path == "" {
		path = "configs/config.yaml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
	} else if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	applyEnv(&cfg)
	if err := cfg.normalize(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("CHIMERA_MODE"); v != "" {
		cfg.Mode = Mode(v)
	}
	if v := os.Getenv("CHIMERA_HTTP_ADDR"); v != "" {
		cfg.HTTP.Addr = v
	}
	if v := os.Getenv("CHIMERA_LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
}

func (c *Config) normalize() error {
	c.Mode = Mode(strings.ToLower(string(c.Mode)))
	switch c.Mode {
	case ModeDevelopment, ModeProduction:
	default:
		return fmt.Errorf("unknown mode %q, use development or production", c.Mode)
	}
	if c.HTTP.Addr == "" {
		return fmt.Errorf("http.addr is empty")
	}
	return nil
}
