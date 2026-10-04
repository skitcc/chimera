// @title Chimera API
// @version 1.0
// @description Music streaming backend
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT from /v1/auth/login. Paste only the token, or "Bearer <token>".
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"chimera/internal/config"
	"chimera/internal/infra/adapters/postgres"
	"chimera/internal/infra/adapters/s3"
	infraauth "chimera/internal/infra/auth"
	"chimera/internal/infra/health"
	"chimera/internal/infra/logger"
	"chimera/internal/infra/worker"
	httpapi "chimera/internal/transport/http"
	"chimera/internal/transport/http/middleware"
	v1 "chimera/internal/transport/http/v1"
	"chimera/internal/usecase"

	_ "chimera/docs"
)

func main() {
	root := context.Background()

	cfg, err := config.Load()
	if err != nil {
		slog.ErrorContext(root, "load config", "error", err)
		os.Exit(1)
	}

	log := logger.New(cfg)
	ctx, stop := signal.NotifyContext(root, os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := waitPostgres(ctx, log, cfg)
	if err != nil {
		log.ErrorContext(ctx, "postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := postgres.Migrate(ctx, pool); err != nil {
		log.ErrorContext(ctx, "migrate", "error", err)
		os.Exit(1)
	}

	objects, err := waitS3(ctx, log, cfg)
	if err != nil {
		log.ErrorContext(ctx, "s3", "error", err)
		os.Exit(1)
	}

	workers := worker.New(ctx, cfg.Workers.Size)
	monitor := health.NewMonitor(log, cfg.Health.Interval, cfg.Health.Timeout, workers, []health.Check{
		{Name: "postgres", Ping: pool.Ping},
		{Name: "s3", Ping: objects.Ping},
	})
	go monitor.Run(ctx)

	users := postgres.NewUserRepository(pool)
	tracks := postgres.NewTrackRepository(pool)
	likes := postgres.NewTrackLikeRepository(pool)
	hasher := infraauth.NewBcryptHasher()
	tokens := infraauth.NewJWT(cfg.Auth)

	router := httpapi.NewRouter(httpapi.Dependencies{
		Log:         log,
		Tokens:      tokens,
		CORSOrigins: cfg.HTTP.CORSOrigins,
		Health:      monitor,
		Routes: v1.New(
			usecase.NewUserService(users, hasher),
			usecase.NewAuthService(users, hasher, tokens),
			usecase.NewTrackService(tracks, likes, objects, cfg.Upload.MaxBytes),
			tokens,
			middleware.NewLimiter(cfg.Auth.RateLimit, cfg.Auth.RateWindow),
			cfg.S3.PresignHost,
			log,
		),
	})

	server := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           router,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
	}

	log.InfoContext(ctx, "listening",
		"addr", cfg.HTTP.Addr,
		"postgres_min_conns", cfg.Postgres.MinConns,
		"postgres_max_conns", cfg.Postgres.MaxConns,
		"worker_pool_size", cfg.Workers.Size,
	)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.ErrorContext(ctx, "http server", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(root, cfg.HTTP.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.ErrorContext(root, "shutdown", "error", err)
		os.Exit(1)
	}
	workers.Wait()
	log.InfoContext(root, "graceful shutdown")
}

func waitPostgres(ctx context.Context, log *slog.Logger, cfg config.Config) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	err := health.Wait(ctx, log, "postgres", cfg.Health.Interval, cfg.Health.Timeout, func(ctx context.Context) error {
		if pool != nil {
			return pool.Ping(ctx)
		}
		p, err := postgres.NewPool(ctx, cfg.Postgres)
		if err != nil {
			return err
		}
		pool = p
		return nil
	})
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func waitS3(ctx context.Context, log *slog.Logger, cfg config.Config) (*s3.Store, error) {
	objects, err := s3.New(cfg.S3)
	if err != nil {
		return nil, err
	}
	if err := health.Wait(ctx, log, "s3", cfg.Health.Interval, cfg.Health.Timeout, objects.Ping); err != nil {
		return nil, err
	}
	if err := health.Wait(ctx, log, "s3 bucket", cfg.Health.Interval, cfg.Health.Timeout, objects.EnsureBucket); err != nil {
		return nil, err
	}
	return objects, nil
}
