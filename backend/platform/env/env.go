// Package env reads typed settings from the process environment.
// Every service parses its Config once at startup and aborts on the first error.
package env

import (
	"bufio"
	"errors"
	"math"
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

// LoadDotEnv fills unset variables from .env in dir. A missing file is not an error.
func LoadDotEnv(dir string) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return
	}
	defer root.Close()
	f, err := root.Open(".env")
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

func Optional(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

func String(name string) (string, error) {
	v := Optional(name)
	if v == "" {
		return "", errors.New("missing required env " + name)
	}
	return v, nil
}

func Int(name string) (int, error) {
	raw, err := String(name)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, errors.New(name + " is invalid")
	}
	return n, nil
}

func Int32(name string) (int32, error) {
	n, err := Int(name)
	if err != nil {
		return 0, err
	}
	if n > math.MaxInt32 {
		return 0, errors.New(name + " is invalid")
	}
	return int32(n), nil
}

func Int64(name string) (int64, error) {
	raw, err := String(name)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return 0, errors.New(name + " is invalid")
	}
	return n, nil
}

func Duration(name string) (time.Duration, error) {
	raw, err := String(name)
	if err != nil {
		return 0, err
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0, errors.New(name + " is invalid")
	}
	return d, nil
}

// List splits a comma-separated value and drops empty items. At least one item is required.
func List(name string) ([]string, error) {
	raw, err := String(name)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil, errors.New(name + " is invalid")
	}
	return out, nil
}

func AppMode() (Mode, error) {
	raw, err := String("APP_MODE")
	if err != nil {
		return "", err
	}
	mode := Mode(strings.ToLower(raw))
	if mode != ModeDevelopment && mode != ModeProduction {
		return "", errors.New("APP_MODE must be development or production")
	}
	return mode, nil
}
