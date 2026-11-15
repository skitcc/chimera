package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	"chimera/catalog/internal/config"
	"chimera/catalog/internal/infra/postgres"
	"chimera/catalog/internal/infra/s3"
	"chimera/catalog/internal/transport/events"
	grpcapi "chimera/catalog/internal/transport/grpc"
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
	"chimera/platform/readpref"
	"chimera/platform/telemetry"
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
		log.ErrorContext(ctx, "catalog stopped", "error", err)
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

	objects, err := connectS3(ctx, log, cfg)
	if err != nil {
		return err
	}

	producer, err := kafka.NewProducer(cfg.Kafka)
	if err != nil {
		return err
	}
	defer producer.Close()

	tracks := postgres.NewTrackRepository(pools)
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

	srv := grpcx.NewServer(log, grpcx.ServerConfig{
		Instance:         cfg.Base.Instance,
		MaxConnectionAge: cfg.GRPC.MaxConnectionAge,
		Interceptors:     []grpc.UnaryServerInterceptor{readpref.Interceptor(grpcapi.ReplicaMethods...)},
	})
	catalogv1.RegisterCatalogServiceServer(srv, grpcapi.NewServer(trackService))

	// Kafka and the replica are not readiness dependencies: the outbox and consumer
	// offsets carry the work over a broker outage, and reads fall back to the primary.
	checks := []health.Check{
		{Name: "postgres", Ping: primary.Ping},
		{Name: "s3", Ping: objects.Ping},
	}
	workers := worker.New(ctx, len(checks))
	defer workers.Wait()
	monitor := health.NewMonitor(log, cfg.Health.Interval, cfg.Health.Timeout, workers, checks)
	monitor.Watch(srv.SetReady)
	go monitor.Run(ctx)

	relay := outbox.NewRelay(pools.Write(), producer, log, service, cfg.Kafka.OutboxPollInterval, cfg.Kafka.OutboxBatchSize)
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
