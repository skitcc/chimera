package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	topics "chimera/contracts/events"
	catalogv1 "chimera/contracts/gen/catalog/v1"
	identityv1 "chimera/contracts/gen/identity/v1"
	socialv1 "chimera/contracts/gen/social/v1"
	"chimera/contracts/rpc"
	"chimera/platform/env"
	"chimera/platform/grpcx"
	"chimera/platform/health"
	"chimera/platform/kafka"
	"chimera/platform/logger"
	"chimera/platform/pg"
	"chimera/platform/readpref"
	"chimera/platform/telemetry"
	"chimera/platform/worker"
	"chimera/social/internal/config"
	"chimera/social/internal/infra/catalog"
	"chimera/social/internal/infra/identity"
	"chimera/social/internal/infra/postgres"
	"chimera/social/internal/transport/events"
	grpcapi "chimera/social/internal/transport/grpc"
	"chimera/social/internal/usecase"
	"chimera/social/migrations"
)

const service = "social"

func main() {
	root := context.Background()
	healthcheck := flag.Bool("healthcheck", false, "probe the local gRPC health service and exit")
	replayDLQ := flag.Bool("replay-dlq", false, "move the records of the consumed topics' DLQs back and exit")
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

	log := logger.New(service, cfg.Base)
	ctx, stop := signal.NotifyContext(root, os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *replayDLQ {
		moved, err := kafka.ReplayDLQ(ctx, cfg.Kafka, []string{topics.TopicUser, topics.TopicTrack}, cfg.Health.Timeout, log)
		if err != nil {
			log.ErrorContext(ctx, "dlq replay failed", "moved", moved, "error", err)
			os.Exit(1)
		}
		log.InfoContext(ctx, "dlq replay finished", "moved", moved)
		return
	}

	if err := run(ctx, log, cfg); err != nil {
		log.ErrorContext(ctx, "social stopped", "error", err)
		os.Exit(1)
	}
	log.InfoContext(root, "graceful shutdown")
}

func run(ctx context.Context, log *slog.Logger, cfg config.Config) error {
	tel, err := telemetry.Start(ctx, service, cfg.Base)
	if err != nil {
		return err
	}
	defer tel.Flush(log, cfg.GRPC.ShutdownTimeout)
	tel.Serve(ctx, log, cfg.Base.MetricsAddr, cfg.Health.Timeout)

	primary, err := pg.Connect(ctx, log, cfg.Postgres, cfg.Health)
	if err != nil {
		return err
	}
	defer primary.Close()
	if err := pg.Migrate(primary, migrations.FS, "."); err != nil {
		return err
	}
	pools, err := pg.OpenPools(ctx, log, primary, cfg.Postgres, cfg.Health)
	if err != nil {
		return err
	}
	defer pools.Close()
	go pools.WatchReplica(ctx, cfg.Health.Interval, cfg.Health.Timeout)

	catalogConn, err := grpcx.Dial(cfg.Upstreams.CatalogAddr, cfg.Upstreams.Client, log, rpc.RepeatUnsafe...)
	if err != nil {
		return err
	}
	defer catalogConn.Close()
	identityConn, err := grpcx.Dial(cfg.Upstreams.IdentityAddr, cfg.Upstreams.Client, log, rpc.RepeatUnsafe...)
	if err != nil {
		return err
	}
	defer identityConn.Close()

	tracks := catalog.New(catalogv1.NewCatalogServiceClient(catalogConn))
	users := identity.New(identityv1.NewIdentityServiceClient(identityConn))
	likeService := usecase.NewLikeService(tracks, postgres.NewTrackLikeRepository(pools))
	followService := usecase.NewFollowService(users, postgres.NewFollowRepository(pools))
	playlistService := usecase.NewPlaylistService(postgres.NewPlaylistRepository(pools), tracks, users)
	handlers := events.NewHandlers(usecase.NewCleanupService(postgres.NewCleanupRepository(pools.Write())))

	consumer, err := kafka.NewConsumer(cfg.Kafka, service, []string{topics.TopicUser, topics.TopicTrack}, handlers.Handle, log)
	if err != nil {
		return err
	}
	defer consumer.Close()

	srv := grpcx.NewServer(log, grpcx.ServerConfig{
		Instance:         cfg.Base.Instance,
		MaxConnectionAge: cfg.GRPC.MaxConnectionAge,
		Interceptors:     []grpc.UnaryServerInterceptor{readpref.Interceptor(grpcapi.ReplicaMethods...)},
	})
	socialv1.RegisterSocialServiceServer(srv, grpcapi.NewServer(likeService, followService, playlistService))

	// Only the own primary gates readiness. A Catalog or Identity outage fails the
	// calls that need them with 503 instead of taking Social down; reads survive
	// a replica outage on the primary.
	checks := []health.Check{{Name: "postgres", Ping: primary.Ping}}
	workers := worker.New(ctx, len(checks))
	defer workers.Wait()
	monitor := health.NewMonitor(log, cfg.Health.Interval, cfg.Health.Timeout, workers, checks)
	monitor.Watch(srv.SetReady)
	go monitor.Run(ctx)
	go consumer.Run(ctx)

	log.InfoContext(ctx, "listening", "addr", cfg.GRPC.Addr)
	return srv.Serve(ctx, cfg.GRPC.Addr, cfg.GRPC.ShutdownTimeout)
}
