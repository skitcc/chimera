package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	topics "chimera/contracts/events"
	catalogv1 "chimera/contracts/gen/catalog/v1"
	identityv1 "chimera/contracts/gen/identity/v1"
	socialv1 "chimera/contracts/gen/social/v1"
	"chimera/platform/env"
	"chimera/platform/grpcx"
	"chimera/platform/health"
	"chimera/platform/kafka"
	"chimera/platform/logger"
	"chimera/platform/pg"
	"chimera/platform/worker"
	"chimera/social/internal/config"
	"chimera/social/internal/infra/catalog"
	"chimera/social/internal/infra/identity"
	"chimera/social/internal/infra/postgres"
	grpcapi "chimera/social/internal/transport/grpc"
	"chimera/social/internal/transport/events"
	"chimera/social/internal/usecase"
	"chimera/social/migrations"
)

const service = "social"

func main() {
	root := context.Background()
	healthcheck := flag.Bool("healthcheck", false, "probe the local gRPC health service and exit")
	flag.Parse()
	env.LoadDotEnv(".")

	cfg, err := config.Load()
	if err != nil {
		slog.ErrorContext(root, "load config", "error", err)
		os.Exit(1)
	}
	if *healthcheck {
		if err := grpcx.Probe(cfg.GRPC.Addr, cfg.Health.Timeout); err != nil {
			os.Exit(1)
		}
		return
	}

	log := logger.New(service, cfg.Base.Mode, cfg.Base.LogLevel)
	ctx, stop := signal.NotifyContext(root, os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, log, cfg); err != nil {
		log.ErrorContext(ctx, "social stopped", "error", err)
		os.Exit(1)
	}
	log.InfoContext(root, "graceful shutdown")
}

func run(ctx context.Context, log *slog.Logger, cfg config.Config) error {
	pool, err := pg.Connect(ctx, log, cfg.Postgres, cfg.Health)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pg.Migrate(pool, migrations.FS, "."); err != nil {
		return err
	}

	catalogConn, err := grpcx.Dial(cfg.Upstreams.CatalogAddr, cfg.Upstreams.Timeout)
	if err != nil {
		return err
	}
	defer catalogConn.Close()
	identityConn, err := grpcx.Dial(cfg.Upstreams.IdentityAddr, cfg.Upstreams.Timeout)
	if err != nil {
		return err
	}
	defer identityConn.Close()

	tracks := catalog.New(catalogv1.NewCatalogServiceClient(catalogConn))
	users := identity.New(identityv1.NewIdentityServiceClient(identityConn))
	likeService := usecase.NewLikeService(tracks, postgres.NewTrackLikeRepository(pool))
	followService := usecase.NewFollowService(users, postgres.NewFollowRepository(pool))
	playlistService := usecase.NewPlaylistService(postgres.NewPlaylistRepository(pool), tracks, users)
	handlers := events.NewHandlers(usecase.NewCleanupService(postgres.NewCleanupRepository(pool)))

	consumer, err := kafka.NewConsumer(cfg.Kafka, service, []string{topics.TopicUser, topics.TopicTrack}, handlers.Handle, log)
	if err != nil {
		return err
	}
	defer consumer.Close()

	srv := grpcx.NewServer(log)
	socialv1.RegisterSocialServiceServer(srv, grpcapi.NewServer(likeService, followService, playlistService))

	// Only the own database gates readiness. A Catalog or Identity outage
	// fails the calls that need them with 503 instead of taking Social down.
	checks := []health.Check{{Name: "postgres", Ping: pool.Ping}}
	workers := worker.New(ctx, len(checks))
	defer workers.Wait()
	monitor := health.NewMonitor(log, cfg.Health.Interval, cfg.Health.Timeout, workers, checks)
	monitor.Watch(srv.SetReady)
	go monitor.Run(ctx)
	go consumer.Run(ctx)

	log.InfoContext(ctx, "listening", "addr", cfg.GRPC.Addr)
	return srv.Serve(ctx, cfg.GRPC.Addr, cfg.GRPC.ShutdownTimeout)
}
