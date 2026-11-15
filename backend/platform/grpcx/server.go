package grpcx

import (
	"context"
	"net"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"chimera/platform/apperr"
	"chimera/platform/runctx"
)

const (
	mdRunID   = "x-run-id"
	mdRunStep = "x-run-step"
)

// LivenessService is the health service name for liveness probes. It stays SERVING
// while the process runs, so an outage of Postgres or Kafka never restarts the pod;
// the empty name is readiness and follows the dependency checks.
const LivenessService = "chimera.liveness"

type Logger interface {
	DebugContext(ctx context.Context, msg string, args ...any)
	InfoContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
}

type Server struct {
	*grpc.Server
	health *health.Server
	log    Logger
}

// NewServer returns a gRPC server with recovery, run id propagation, request
// logging and application error mapping. The standard health service starts NOT_SERVING.
func NewServer(log Logger) *Server {
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(
		recoverInterceptor(log),
		runInterceptor,
		errorInterceptor(log),
	))
	hs := health.NewServer()
	hs.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	hs.SetServingStatus(LivenessService, healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(srv, hs)
	return &Server{Server: srv, health: hs, log: log}
}

// SetReady switches the health status that Kubernetes readiness probes read.
func (s *Server) SetReady(ready bool) {
	st := healthpb.HealthCheckResponse_NOT_SERVING
	if ready {
		st = healthpb.HealthCheckResponse_SERVING
	}
	s.health.SetServingStatus("", st)
}

// Serve listens on addr until ctx ends, then stops gracefully within timeout.
func (s *Server) Serve(ctx context.Context, addr string, timeout time.Duration) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return apperr.Wrap(apperr.CodeInternal, "listen grpc", err)
	}
	errs := make(chan error, 1)
	go func() { errs <- s.Server.Serve(lis) }()
	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
	}
	s.health.Shutdown()
	done := make(chan struct{})
	go func() {
		s.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		s.Stop()
	}
	return nil
}

func recoverInterceptor(log Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.ErrorContext(ctx, "grpc panic", "method", info.FullMethod, "panic", r, "stack", string(debug.Stack()))
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return next(ctx, req)
	}
}

func runInterceptor(ctx context.Context, req any, _ *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return next(ctx, req)
	}
	id := first(md, mdRunID)
	if id == "" {
		return next(ctx, req)
	}
	return next(runctx.With(ctx, runctx.Run{ID: id, Step: first(md, mdRunStep)}), req)
}

func errorInterceptor(log Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := next(ctx, req)
		if err == nil {
			log.DebugContext(ctx, "grpc request", "method", info.FullMethod, "duration", time.Since(start))
			return resp, nil
		}
		app, ok := apperr.As(err)
		if !ok || app.Code == apperr.CodeInternal || app.Code == apperr.CodeUnavailable {
			log.ErrorContext(ctx, "grpc request failed", "method", info.FullMethod, "error", err)
		} else {
			log.DebugContext(ctx, "grpc request rejected", "method", info.FullMethod, "code", app.Code, "error", err)
		}
		return nil, ToStatus(err)
	}
}

func first(md metadata.MD, key string) string {
	if v := md.Get(key); len(v) > 0 {
		return v[0]
	}
	return ""
}
