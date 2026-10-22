// Package runctx carries the id of a test run from an incoming request to
// every log record written while that request is handled.
package runctx

import "context"

type Run struct {
	ID   string
	Step string
}

type key struct{}

func With(ctx context.Context, run Run) context.Context {
	return context.WithValue(ctx, key{}, run)
}

func From(ctx context.Context) (Run, bool) {
	run, ok := ctx.Value(key{}).(Run)
	return run, ok
}
