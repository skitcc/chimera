package config

import (
	"errors"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"chimera/platform/env"
)

type HTTP struct {
	Addr              string
	ShutdownTimeout   time.Duration
	ReadHeaderTimeout time.Duration
	// WriteTimeout must outlast every retry of the slowest upstream call.
	WriteTimeout time.Duration
	CORSOrigins  []string
}

type Auth struct {
	SecurityEnabled bool
	PublicKeyFile   string
	RateLimit       int
	RateWindow      time.Duration
	// TrustedProxies may set X-Forwarded-For. Empty means the peer address
	// is the client address.
	TrustedProxies []netip.Prefix
}

type Upstreams struct {
	IdentityAddr string
	CatalogAddr  string
	SocialAddr   string
	Client       env.GRPCClient
}

type Config struct {
	Base      env.Base
	HTTP      HTTP
	Auth      Auth
	Health    env.Health
	Upstreams Upstreams
	// StreamHost is the only host a stream redirect may point to.
	StreamHost string
}

func Load() (Config, error) {
	base, err := env.LoadBase()
	if err != nil {
		return Config{}, err
	}
	httpCfg, err := loadHTTP()
	if err != nil {
		return Config{}, err
	}
	auth, err := loadAuth()
	if err != nil {
		return Config{}, err
	}
	auth.SecurityEnabled = true
	if value := os.Getenv("AUTH_SECURITY_ENABLED"); value != "" {
		auth.SecurityEnabled, err = strconv.ParseBool(value)
		if err != nil {
			return Config{}, err
		}
	}
	health, err := env.LoadHealth()
	if err != nil {
		return Config{}, err
	}
	upstreams, err := loadUpstreams()
	if err != nil {
		return Config{}, err
	}
	streamHost, err := env.String("STREAM_HOST")
	if err != nil {
		return Config{}, err
	}
	if strings.ContainsAny(streamHost, "/@") {
		return Config{}, errors.New("STREAM_HOST must be host[:port]")
	}
	return Config{
		Base:       base,
		HTTP:       httpCfg,
		Auth:       auth,
		Health:     health,
		Upstreams:  upstreams,
		StreamHost: streamHost,
	}, nil
}

func loadHTTP() (HTTP, error) {
	addr, err := env.String("HTTP_ADDR")
	if err != nil {
		return HTTP{}, err
	}
	shutdown, err := env.Duration("HTTP_SHUTDOWN_TIMEOUT")
	if err != nil {
		return HTTP{}, err
	}
	readHeader, err := env.Duration("HTTP_READ_HEADER_TIMEOUT")
	if err != nil {
		return HTTP{}, err
	}
	write, err := env.Duration("HTTP_WRITE_TIMEOUT")
	if err != nil {
		return HTTP{}, err
	}
	origins, err := env.List("HTTP_CORS_ORIGIN")
	if err != nil {
		return HTTP{}, err
	}
	return HTTP{Addr: addr, ShutdownTimeout: shutdown, ReadHeaderTimeout: readHeader, WriteTimeout: write, CORSOrigins: origins}, nil
}

func loadAuth() (Auth, error) {
	keyFile, err := env.String("JWT_PUBLIC_KEY_FILE")
	if err != nil {
		return Auth{}, err
	}
	limit, err := env.Int("AUTH_RATE_LIMIT")
	if err != nil {
		return Auth{}, err
	}
	window, err := env.Duration("AUTH_RATE_WINDOW")
	if err != nil {
		return Auth{}, err
	}
	proxies, err := parsePrefixes(env.Optional("TRUSTED_PROXIES"))
	if err != nil {
		return Auth{}, err
	}
	return Auth{PublicKeyFile: keyFile, RateLimit: limit, RateWindow: window, TrustedProxies: proxies}, nil
}

// parsePrefixes reads a comma-separated list of CIDRs or single addresses.
func parsePrefixes(raw string) ([]netip.Prefix, error) {
	var prefixes []netip.Prefix
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !strings.Contains(part, "/") {
			addr, err := netip.ParseAddr(part)
			if err != nil {
				return nil, errors.New("TRUSTED_PROXIES is invalid: " + part)
			}
			prefixes = append(prefixes, netip.PrefixFrom(addr.Unmap(), addr.Unmap().BitLen()))
			continue
		}
		prefix, err := netip.ParsePrefix(part)
		if err != nil {
			return nil, errors.New("TRUSTED_PROXIES is invalid: " + part)
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes, nil
}

func loadUpstreams() (Upstreams, error) {
	identity, err := env.String("IDENTITY_GRPC_ADDR")
	if err != nil {
		return Upstreams{}, err
	}
	catalog, err := env.String("CATALOG_GRPC_ADDR")
	if err != nil {
		return Upstreams{}, err
	}
	social, err := env.String("SOCIAL_GRPC_ADDR")
	if err != nil {
		return Upstreams{}, err
	}
	client, err := env.LoadGRPCClient()
	if err != nil {
		return Upstreams{}, err
	}
	return Upstreams{IdentityAddr: identity, CatalogAddr: catalog, SocialAddr: social, Client: client}, nil
}
