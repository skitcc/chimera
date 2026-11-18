// Package readpref carries the read preference of a request: replica or primary.
package readpref

import (
	"context"

	"google.golang.org/grpc"
)

type primaryKey struct{}

// Primary marks ctx so every read in it goes to the primary database.
func Primary(ctx context.Context) context.Context {
	return context.WithValue(ctx, primaryKey{}, true)
}

func IsPrimary(ctx context.Context) bool {
	v, _ := ctx.Value(primaryKey{}).(bool)
	return v
}

// Interceptor sends every RPC outside replicaMethods to the primary. A method that
// writes, or reads before it writes, therefore never sees a lagging replica, and a
// new RPC is safe until someone lists it here on purpose.
func Interceptor(replicaMethods ...string) grpc.UnaryServerInterceptor {
	replica := make(map[string]bool, len(replicaMethods))
	for _, m := range replicaMethods {
		replica[m] = true
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		if !replica[info.FullMethod] {
			ctx = Primary(ctx)
		}
		return next(ctx, req)
	}
}
