package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

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
	"chimera/platform/readpref"
	"chimera/platform/telemetry"
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

	log := logger.New(service, cfg.Base)
	ctx, stop := signal.NotifyContext(root, os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, log, cfg); err != nil {
		log.ErrorContext(ctx, "identity stopped", "error", err)
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

	key, err := auth.LoadPrivateKey(cfg.Auth.PrivateKeyFile)
	if err != nil {
		return err
	}

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

	producer, err := kafka.NewProducer(cfg.Kafka)
	if err != nil {
		return err
	}
	defer producer.Close()

	users := postgres.NewUserRepository(pools)
	hasher := auth.NewBcryptHasher()
	userService := usecase.NewUserService(users, users, hasher)
	authService := usecase.NewAuthService(users, hasher, auth.NewJWT(key, cfg.Auth.TTL))

	srv := grpcx.NewServer(log, grpcx.ServerConfig{
		Instance:         cfg.Base.Instance,
		MaxConnectionAge: cfg.GRPC.MaxConnectionAge,
		Interceptors:     []grpc.UnaryServerInterceptor{readpref.Interceptor(grpcapi.ReplicaMethods...)},
	})
	identityv1.RegisterIdentityServiceServer(srv, grpcapi.NewServer(userService, authService))

	// Kafka and the replica are not readiness dependencies: the outbox holds events
	// while the broker is down, and reads fall back to the primary.
	checks := []health.Check{{Name: "postgres", Ping: primary.Ping}}
	workers := worker.New(ctx, len(checks))
	defer workers.Wait()
	monitor := health.NewMonitor(log, cfg.Health.Interval, cfg.Health.Timeout, workers, checks)
	monitor.Watch(srv.SetReady)
	go monitor.Run(ctx)

	relay := outbox.NewRelay(pools.Write(), producer, log, service, cfg.Kafka.OutboxPollInterval, cfg.Kafka.OutboxBatchSize)
	go relay.Run(ctx)

	log.InfoContext(ctx, "listening", "addr", cfg.GRPC.Addr)
	return srv.Serve(ctx, cfg.GRPC.Addr, cfg.GRPC.ShutdownTimeout)
}
