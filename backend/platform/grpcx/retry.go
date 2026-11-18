package grpcx

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Retry throttling as in the gRPC retry design: a failure costs a token, a success
// returns a tenth of one, and retries stop while half of the tokens are spent.
// When an upstream fails as a whole, clients fall back to one attempt per call
// instead of multiplying its load.
const (
	budgetTokens = 10
	budgetRatio  = 0.1
)

type budget struct {
	mu     sync.Mutex
	tokens float64
}

func newBudget() *budget { return &budget{tokens: budgetTokens} }

func (b *budget) success() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tokens = min(budgetTokens, b.tokens+budgetRatio)
}

// failure spends a token and reports whether a retry is still allowed. Once half
// of the tokens are gone, further failures are not retried and spend nothing,
// so a recovered upstream can refill the budget with successes.
func (b *budget) failure() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.tokens <= budgetTokens/2 {
		return false
	}
	b.tokens--
	return true
}

type retrier struct {
	log         Logger
	maxAttempts int
	backoff     time.Duration
	unsafe      map[string]bool
	budget      *budget
	retries     metric.Int64Counter
}

func newRetrier(log Logger, maxAttempts int, backoff time.Duration, repeatUnsafe []string) *retrier {
	unsafe := make(map[string]bool, len(repeatUnsafe))
	for _, m := range repeatUnsafe {
		unsafe[m] = true
	}
	retries, _ := otel.Meter("chimera/platform/grpcx").Int64Counter("chimera.grpc.client.retries",
		metric.WithDescription("gRPC calls repeated after a failed attempt"))
	return &retrier{log: log, maxAttempts: maxAttempts, backoff: backoff, unsafe: unsafe, budget: newBudget(), retries: retries}
}

// intercept repeats an attempt that failed with UNAVAILABLE, or one that timed out
// on a method safe to repeat. round_robin sends each attempt to the next pod.
func (r *retrier) intercept(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	for attempt := 1; ; attempt++ {
		err := invoker(ctx, method, req, reply, cc, opts...)
		if err == nil {
			r.budget.success()
			return nil
		}
		code := status.Code(err)
		if !r.retryable(ctx, method, code) {
			return err
		}
		allowed := r.budget.failure()
		if attempt >= r.maxAttempts || !allowed {
			return err
		}
		wait := r.wait(attempt)
		attrs := []attribute.KeyValue{
			attribute.String("rpc.method", method),
			attribute.String("rpc.grpc.status_code", code.String()),
		}
		trace.SpanFromContext(ctx).AddEvent("grpc.retry", trace.WithAttributes(append(attrs,
			attribute.Int("attempt", attempt), attribute.String("wait", wait.String()))...))
		r.retries.Add(ctx, 1, metric.WithAttributes(attrs...))
		r.log.InfoContext(ctx, "grpc retry", "method", method, "code", code.String(), "attempt", attempt, "wait", wait, "error", err)
		select {
		case <-ctx.Done():
			return err
		case <-time.After(wait):
		}
	}
}

func (r *retrier) retryable(ctx context.Context, method string, code codes.Code) bool {
	if ctx.Err() != nil {
		return false
	}
	switch code {
	case codes.Unavailable:
		return true
	case codes.DeadlineExceeded:
		return !r.unsafe[method]
	}
	return false
}

// wait is the exponential backoff with jitter: between half and all of backoff*2^(attempt-1).
func (r *retrier) wait(attempt int) time.Duration {
	d := r.backoff << (attempt - 1)
	return d/2 + rand.N(d/2+1)
}
