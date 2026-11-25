// @title Sound Chimera API
// @version 1.0.0
// @BasePath /api
// @tag.name sessions
// @tag.description Вход
// @tag.name users
// @tag.description Аккаунты и профили
// @tag.name tracks
// @tag.description Каталог и публикация
// @tag.name likes
// @tag.description Любимое
// @tag.name playlists
// @tag.description Списки пользователя
// @tag.name follows
// @tag.description Подписки на авторов
// @securityDefinitions.apikey bearerAuth
// @in header
// @name Authorization
// @description Заголовок Authorization: Bearer <token>
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	catalogv1 "chimera/contracts/gen/catalog/v1"
	identityv1 "chimera/contracts/gen/identity/v1"
	socialv1 "chimera/contracts/gen/social/v1"
	"chimera/contracts/rpc"
	"chimera/gateway/internal/clients"
	"chimera/gateway/internal/config"
	"chimera/gateway/internal/infra/auth"
	httpapi "chimera/gateway/internal/transport/http"
	"chimera/gateway/internal/transport/http/middleware"
	v1 "chimera/gateway/internal/transport/http/v1"
	"chimera/platform/env"
	"chimera/platform/grpcx"
	"chimera/platform/health"
	"chimera/platform/logger"
	"chimera/platform/telemetry"
	"chimera/platform/worker"

	_ "chimera/gateway/docs"
)

const service = "gateway"

func main() {
	root := context.Background()
	env.LoadDotEnv(".")

	cfg, err := config.Load()
	if err != nil {
		slog.ErrorContext(root, "load config", "error", err)
		os.Exit(1)
	}

	log := logger.New(service, cfg.Base)
	ctx, stop := signal.NotifyContext(root, os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, log, cfg); err != nil {
		log.ErrorContext(ctx, "gateway stopped", "error", err)
		os.Exit(1)
	}
	log.InfoContext(root, "graceful shutdown")
}

func run(ctx context.Context, log *slog.Logger, cfg config.Config) error {
	tel, err := telemetry.Start(ctx, service, cfg.Base)
	if err != nil {
		return err
	}
	defer tel.Flush(log, cfg.HTTP.ShutdownTimeout)
	tel.Serve(ctx, log, cfg.Base.MetricsAddr, cfg.Health.Timeout)

	key, err := auth.LoadPublicKey(cfg.Auth.PublicKeyFile)
	if err != nil {
		return err
	}

	upstreams := []struct {
		name string
		addr string
	}{
		{"identity", cfg.Upstreams.IdentityAddr},
		{"catalog", cfg.Upstreams.CatalogAddr},
		{"social", cfg.Upstreams.SocialAddr},
	}
	conns := make(map[string]*grpc.ClientConn, len(upstreams))
	checks := make([]health.Check, 0, len(upstreams))
	for _, u := range upstreams {
		conn, err := grpcx.Dial(u.addr, cfg.Upstreams.Client, log, rpc.RepeatUnsafe...)
		if err != nil {
			return err
		}
		defer conn.Close()
		conns[u.name] = conn
		checks = append(checks, health.Check{Name: u.name, Ping: grpcx.HealthPing(conn)})
	}

	identity := identityv1.NewIdentityServiceClient(conns["identity"])
	catalog := catalogv1.NewCatalogServiceClient(conns["catalog"])
	social := socialv1.NewSocialServiceClient(conns["social"])
	users := clients.NewIdentity(identity)

	workers := worker.New(ctx, len(checks))
	defer workers.Wait()
	monitor := health.NewMonitor(log, cfg.Health.Interval, cfg.Health.Timeout, workers, checks)
	go monitor.Run(ctx)

	verifier := auth.NewVerifier(key)
	if cfg.Auth.SecurityEnabled {
		verifier.WithValidation(users.Validate)
	}
	router := httpapi.NewRouter(httpapi.Dependencies{
		Log:            log,
		Tokens:         verifier,
		CORSOrigins:    cfg.HTTP.CORSOrigins,
		Health:         monitor,
		Instance:       cfg.Base.Instance,
		Metrics:        tel.Registerer,
		MetricsHandler: tel.MetricsHandler(),
		Routes: v1.New(
			users,
			users,
			clients.NewTracks(catalog, social),
			clients.NewFollows(social),
			clients.NewPlaylists(social),
			verifier,
			middleware.NewLimiter(cfg.Auth.RateLimit, cfg.Auth.RateWindow),
			middleware.NewProxies(cfg.Auth.TrustedProxies),
			cfg.StreamHost,
			log,
		),
	})
	server := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           router,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
	}

	errc := make(chan error, 1)
	go func() {
		log.InfoContext(ctx, "listening", "addr", cfg.HTTP.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
		close(errc)
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}
