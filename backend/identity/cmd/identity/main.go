package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	identityv1 "chimera/contracts/gen/identity/v1"
	"chimera/identity/internal/config"
	"chimera/identity/internal/infra/auth"
	"chimera/identity/internal/infra/postgres"
	grpcapi "chimera/identity/internal/transport/grpc"
	"chimera/identity/internal/usecase"
	"chimera/identity/migrations"
	"chimera/platform/env"
	"chimera/platform/grpcx"
	"chimera/platform/health"
	"chimera/platform/kafka"
	"chimera/platform/logger"
	"chimera/platform/outbox"
	"chimera/platform/pg"
	"chimera/platform/worker"
)

const service = "identity"

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
		log.ErrorContext(ctx, "identity stopped", "error", err)
		os.Exit(1)
	}
	log.InfoContext(root, "graceful shutdown")
}

func run(ctx context.Context, log *slog.Logger, cfg config.Config) error {
	key, err := auth.LoadPrivateKey(cfg.Auth.PrivateKeyFile)
	if err != nil {
		return err
	}

	pool, err := pg.Connect(ctx, log, cfg.Postgres, cfg.Health)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pg.Migrate(pool, migrations.FS, "."); err != nil {
		return err
	}

	producer, err := kafka.NewProducer(cfg.Kafka)
	if err != nil {
		return err
	}
	defer producer.Close()

	users := postgres.NewUserRepository(pool)
	hasher := auth.NewBcryptHasher()
	userService := usecase.NewUserService(users, users, hasher)
	authService := usecase.NewAuthService(users, hasher, auth.NewJWT(key, cfg.Auth.TTL))

	srv := grpcx.NewServer(log)
	identityv1.RegisterIdentityServiceServer(srv, grpcapi.NewServer(userService, authService))

	// Kafka is not a readiness dependency: the outbox holds events while it is down.
	checks := []health.Check{{Name: "postgres", Ping: pool.Ping}}
	workers := worker.New(ctx, len(checks))
	defer workers.Wait()
	monitor := health.NewMonitor(log, cfg.Health.Interval, cfg.Health.Timeout, workers, checks)
	monitor.Watch(srv.SetReady)
	go monitor.Run(ctx)

	relay := outbox.NewRelay(pool, producer, log, service, cfg.Kafka.OutboxPollInterval, cfg.Kafka.OutboxBatchSize)
	go relay.Run(ctx)

	log.InfoContext(ctx, "listening", "addr", cfg.GRPC.Addr)
	return srv.Serve(ctx, cfg.GRPC.Addr, cfg.GRPC.ShutdownTimeout)
}
