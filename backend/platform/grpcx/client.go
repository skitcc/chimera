package grpcx

import (
	"context"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"

	"chimera/platform/apperr"
	"chimera/platform/runctx"
)

// Dial opens a lazy client connection inside the cluster network. Every call gets
// timeout unless the caller set an earlier deadline, carries the run id, and
// returns application errors instead of gRPC statuses.
func Dial(addr string, timeout time.Duration) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(clientInterceptor(timeout)),
	)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInternal, "dial "+addr, err)
	}
	return conn, nil
}

func clientInterceptor(timeout time.Duration) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if _, ok := ctx.Deadline(); !ok {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
		if run, ok := runctx.From(ctx); ok {
			ctx = metadata.AppendToOutgoingContext(ctx, mdRunID, run.ID, mdRunStep, run.Step)
		}
		return FromError(invoker(ctx, method, req, reply, cc, opts...))
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
	conn, err := Dial(addr, timeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return HealthPing(conn)(ctx)
}
