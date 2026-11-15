package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"chimera/catalog/internal/config"
	"chimera/catalog/internal/infra/postgres"
	"chimera/catalog/internal/infra/s3"
	grpcapi "chimera/catalog/internal/transport/grpc"
	"chimera/catalog/internal/transport/events"
	"chimera/catalog/internal/usecase"
	"chimera/catalog/migrations"
	topics "chimera/contracts/events"
	catalogv1 "chimera/contracts/gen/catalog/v1"
	"chimera/platform/env"
	"chimera/platform/grpcx"
	"chimera/platform/health"
	"chimera/platform/kafka"
	"chimera/platform/logger"
	"chimera/platform/outbox"
	"chimera/platform/pg"
	"chimera/platform/worker"
)

const (
	service = "catalog"
	// objectsGroup reads Catalog's own track events to delete audio objects
	// outside the request path.
	objectsGroup = "catalog-objects"
)

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
		log.ErrorContext(ctx, "catalog stopped", "error", err)
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

	objects, err := connectS3(ctx, log, cfg)
	if err != nil {
		return err
	}

	producer, err := kafka.NewProducer(cfg.Kafka)
	if err != nil {
		return err
	}
	defer producer.Close()

	tracks := postgres.NewTrackRepository(pool)
	trackService := usecase.NewTrackService(tracks, objects, cfg.TrackMaxBytes)
	handlers := events.NewHandlers(usecase.NewCleanupService(tracks, objects))

	userConsumer, err := kafka.NewConsumer(cfg.Kafka, service, []string{topics.TopicUser}, handlers.UserEvents, log)
	if err != nil {
		return err
	}
	defer userConsumer.Close()
	objectConsumer, err := kafka.NewConsumer(cfg.Kafka, objectsGroup, []string{topics.TopicTrack}, handlers.TrackObjects, log)
	if err != nil {
		return err
	}
	defer objectConsumer.Close()

	srv := grpcx.NewServer(log)
	catalogv1.RegisterCatalogServiceServer(srv, grpcapi.NewServer(trackService))

	// Kafka is not a readiness dependency: the outbox and consumer offsets
	// carry the work over a broker outage.
	checks := []health.Check{
		{Name: "postgres", Ping: pool.Ping},
		{Name: "s3", Ping: objects.Ping},
	}
	workers := worker.New(ctx, len(checks))
	defer workers.Wait()
	monitor := health.NewMonitor(log, cfg.Health.Interval, cfg.Health.Timeout, workers, checks)
	monitor.Watch(srv.SetReady)
	go monitor.Run(ctx)

	relay := outbox.NewRelay(pool, producer, log, service, cfg.Kafka.OutboxPollInterval, cfg.Kafka.OutboxBatchSize)
	go relay.Run(ctx)
	go userConsumer.Run(ctx)
	go objectConsumer.Run(ctx)

	log.InfoContext(ctx, "listening", "addr", cfg.GRPC.Addr)
	return srv.Serve(ctx, cfg.GRPC.Addr, cfg.GRPC.ShutdownTimeout)
}

func connectS3(ctx context.Context, log *slog.Logger, cfg config.Config) (*s3.Store, error) {
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
