package grpcx

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const unsafeMethod = "/chimera.test.v1.Test/Unsafe"

type discardLog struct{}

func (discardLog) DebugContext(context.Context, string, ...any) {}
func (discardLog) InfoContext(context.Context, string, ...any)  {}
func (discardLog) ErrorContext(context.Context, string, ...any) {}

func TestRetrierRepeatsUnavailableAndStopsAtTheLimit(t *testing.T) {
	r := newRetrier(discardLog{}, 3, time.Millisecond, nil)
	calls := 0
	err := r.intercept(context.Background(), "/chimera.test.v1.Test/Get", nil, nil, nil,
		func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
			calls++
			return status.Error(codes.Unavailable, "down")
		})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("code = %s", status.Code(err))
	}
	if calls != 3 {
		t.Fatalf("attempts = %d, want 3", calls)
	}
}

func TestRetrierRepeatsATimeoutOnlyForASafeMethod(t *testing.T) {
	safe := newRetrier(discardLog{}, 3, time.Millisecond, []string{unsafeMethod})
	calls := 0
	err := safe.intercept(context.Background(), "/chimera.test.v1.Test/Get", nil, nil, nil,
		func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
			calls++
			if calls == 1 {
				return status.Error(codes.DeadlineExceeded, "slow")
			}
			return nil
		})
	if err != nil || calls != 2 {
		t.Fatalf("safe: err=%v attempts=%d", err, calls)
	}

	unsafe := newRetrier(discardLog{}, 3, time.Millisecond, []string{unsafeMethod})
	calls = 0
	err = unsafe.intercept(context.Background(), unsafeMethod, nil, nil, nil,
		func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
			calls++
			return status.Error(codes.DeadlineExceeded, "slow")
		})
	if status.Code(err) != codes.DeadlineExceeded || calls != 1 {
		t.Fatalf("unsafe: code=%s attempts=%d", status.Code(err), calls)
	}
}

func TestRetrierLeavesAnApplicationErrorAlone(t *testing.T) {
	r := newRetrier(discardLog{}, 3, time.Millisecond, nil)
	calls := 0
	err := r.intercept(context.Background(), "/chimera.test.v1.Test/Get", nil, nil, nil,
		func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
			calls++
			return status.Error(codes.InvalidArgument, "bad")
		})
	if status.Code(err) != codes.InvalidArgument || calls != 1 {
		t.Fatalf("code=%s attempts=%d", status.Code(err), calls)
	}
}

func TestRetrierBudgetStopsARetryStorm(t *testing.T) {
	r := newRetrier(discardLog{}, 20, time.Millisecond, nil)
	calls := 0
	_ = r.intercept(context.Background(), "/chimera.test.v1.Test/Get", nil, nil, nil,
		func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
			calls++
			return status.Error(codes.Unavailable, "down")
		})
	// Ten tokens, retries stop once half are spent: five retries and the call
	// that finds the budget empty.
	if calls != 6 {
		t.Fatalf("attempts = %d, want 6", calls)
	}
}

func TestRetrierDoesNotRepeatAfterTheCallerGivesUp(t *testing.T) {
	r := newRetrier(discardLog{}, 3, time.Second, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	err := r.intercept(ctx, "/chimera.test.v1.Test/Get", nil, nil, nil,
		func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
			calls++
			return status.Error(codes.Unavailable, "down")
		})
	if status.Code(err) != codes.Unavailable || calls != 1 {
		t.Fatalf("code=%s attempts=%d", status.Code(err), calls)
	}
}
