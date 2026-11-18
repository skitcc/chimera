package grpcx

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc/filters"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"

	"chimera/platform/apperr"
	"chimera/platform/env"
	"chimera/platform/runctx"
)

// roundRobin spreads calls over every address the resolver returns. With a
// dns:/// target on a headless Service that is one subchannel per pod.
const roundRobin = `{"loadBalancingConfig":[{"round_robin":{}}]}`

// Dial opens a lazy client connection inside the cluster network. A call carries
// the run id and returns application errors instead of gRPC statuses; each attempt
// gets cfg.Timeout unless the caller set an earlier deadline, and a failed attempt
// is repeated as retrier decides. repeatUnsafe names the methods not to repeat
// after a timeout (see chimera/contracts/rpc).
func Dial(addr string, cfg env.GRPCClient, log Logger, repeatUnsafe ...string) (*grpc.ClientConn, error) {
	retry := newRetrier(log, cfg.MaxAttempts, cfg.Backoff, repeatUnsafe)
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(roundRobin),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler(otelgrpc.WithFilter(filters.Not(filters.HealthCheck())))),
		grpc.WithChainUnaryInterceptor(
			appErrorInterceptor,
			retry.intercept,
			attemptTimeout(cfg.Timeout),
			upstreamInterceptor,
		),
	)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInternal, "dial "+addr, err)
	}
	return conn, nil
}

func appErrorInterceptor(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	if run, ok := runctx.From(ctx); ok {
		ctx = metadata.AppendToOutgoingContext(ctx, mdRunID, run.ID, mdRunStep, run.Step)
	}
	return FromError(invoker(ctx, method, req, reply, cc, opts...))
}

func attemptTimeout(timeout time.Duration) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// HealthPing checks that the peer behind conn reports SERVING.
func HealthPing(conn *grpc.ClientConn) func(context.Context) error {
	client := healthpb.NewHealthClient(conn)
	return func(ctx context.Context) error {
		resp, err := client.Check(ctx, &healthpb.HealthCheckRequest{})
		if err != nil {
			return err
		}
		if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
			return apperr.Unavailable("not serving")
		}
		return nil
	}
}

// Probe runs one health check against addr. Container health checks call the
// service binary with -healthcheck, which exits with this result.
func Probe(addr string, timeout time.Duration) error {
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	conn, err := Dial(addr, env.GRPCClient{Timeout: timeout, MaxAttempts: 1}, slog.New(slog.DiscardHandler))
	if err != nil {
		return err
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return HealthPing(conn)(ctx)
}
