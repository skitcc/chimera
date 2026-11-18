package grpcx

import (
	"context"
	"slices"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// Upstreams collects the instances that served the gRPC calls of one request.
type Upstreams struct {
	mu    sync.Mutex
	names []string
}

type upstreamsKey struct{}

// WithUpstreams starts collecting the instances that answer calls made with ctx.
func WithUpstreams(ctx context.Context) (context.Context, *Upstreams) {
	u := &Upstreams{}
	return context.WithValue(ctx, upstreamsKey{}, u), u
}

func (u *Upstreams) add(name string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if !slices.Contains(u.names, name) {
		u.names = append(u.names, name)
	}
}

// String lists the instances in call order, comma separated.
func (u *Upstreams) String() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return strings.Join(u.names, ",")
}

func upstreamInterceptor(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	u, ok := ctx.Value(upstreamsKey{}).(*Upstreams)
	if !ok {
		return invoker(ctx, method, req, reply, cc, opts...)
	}
	var header metadata.MD
	err := invoker(ctx, method, req, reply, cc, append(opts, grpc.Header(&header))...)
	for _, name := range header.Get(MDInstance) {
		u.add(name)
	}
	return err
}
