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

	"chimera/internal/config"
	"chimera/internal/infra/adapters/postgres"
	"chimera/internal/infra/adapters/s3"
	infraauth "chimera/internal/infra/auth"
	"chimera/internal/infra/logger"
	httpapi "chimera/internal/transport/http"
	v1 "chimera/internal/transport/http/v1"
	"chimera/internal/usecase"

	_ "chimera/docs"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		slog.ErrorContext(ctx, "load config", "error", err)
		os.Exit(1)
	}

	log := logger.New(cfg)

	pool, err := postgres.NewPool(ctx, cfg.Postgres)
	if err != nil {
		log.ErrorContext(ctx, "postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := postgres.Migrate(ctx, pool); err != nil {
		log.ErrorContext(ctx, "migrate", "error", err)
		os.Exit(1)
	}

	users := postgres.NewUserRepository(pool)
	tracks := postgres.NewTrackRepository(pool)
	likes := postgres.NewTrackLikeRepository(pool)
	objects, err := s3.New(cfg.S3)
	if err != nil {
		log.ErrorContext(ctx, "s3", "error", err)
		os.Exit(1)
	}
	if err := objects.EnsureBucket(ctx); err != nil {
		log.ErrorContext(ctx, "s3 bucket", "error", err)
		os.Exit(1)
	}

	hasher := infraauth.NewBcryptHasher()
	tokens := infraauth.NewJWT(cfg.Auth)

	router := httpapi.NewRouter(httpapi.Dependencies{
		Log:         log,
		Tokens:      tokens,
		CORSOrigins: cfg.HTTP.CORSOrigins,
		Routes: v1.New(
			usecase.NewUserService(users, hasher),
			usecase.NewAuthService(users, hasher, tokens),
			usecase.NewTrackService(tracks, likes, objects, cfg.Upload.MaxBytes),
			log,
		),
	})

	server := &http.Server{
		Addr:    cfg.HTTP.Addr,
		Handler: router,
	}

	log.InfoContext(ctx, "listening", "addr", cfg.HTTP.Addr)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.ErrorContext(ctx, "http server", "error", err)
			os.Exit(1)
		}
	}()

	closeCh := make(chan os.Signal, 1)
	signal.Notify(closeCh, os.Interrupt, syscall.SIGTERM)

	<-closeCh
	shutdownCtx, cancel := context.WithTimeout(ctx, cfg.HTTP.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.ErrorContext(ctx, "shutdown", "error", err)
		os.Exit(1)
	}

	log.InfoContext(ctx, "graceful shutdown")
}
