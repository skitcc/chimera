package pg

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"chimera/platform/readpref"
)

type fakeRow struct{ err error }

func (r fakeRow) Scan(...any) error { return r.err }

// fakePool answers QueryRow with rowErr and Query with queryErr, and counts calls.
type fakePool struct {
	name     string
	rowErr   error
	queryErr error
	pingErr  error
	calls    int
}

func (f *fakePool) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	f.calls++
	return pgconn.CommandTag{}, nil
}

func (f *fakePool) Query(context.Context, string, ...any) (pgx.Rows, error) {
	f.calls++
	return nil, f.queryErr
}

func (f *fakePool) QueryRow(context.Context, string, ...any) pgx.Row {
	f.calls++
	return fakeRow{err: f.rowErr}
}

func (f *fakePool) Begin(context.Context) (pgx.Tx, error) { return nil, nil }
func (f *fakePool) Ping(context.Context) error            { return f.pingErr }
func (f *fakePool) Close()                                {}

type nopLogger struct{}

func (nopLogger) InfoContext(context.Context, string, ...any)  {}
func (nopLogger) ErrorContext(context.Context, string, ...any) {}

func scan(ctx context.Context, p *Pools) error {
	return p.Read(ctx).QueryRow(ctx, "SELECT 1").Scan()
}

func TestPoolsRead(t *testing.T) {
	refused := &pgconn.ConnectError{}
	shutdown := &pgconn.PgError{Code: "57P01"}
	tests := []struct {
		name         string
		ctx          context.Context
		replica      *fakePool
		noReplica    bool
		wantErr      error
		wantPrimary  int
		wantReplica  int
		wantDegraded bool
	}{
		{name: "replica serves reads", ctx: context.Background(), replica: &fakePool{}, wantReplica: 1},
		{name: "readpref sends reads to primary", ctx: readpref.Primary(context.Background()), replica: &fakePool{}, wantPrimary: 1},
		{name: "no replica reads primary", ctx: context.Background(), noReplica: true, wantPrimary: 1},
		{name: "refused replica falls back", ctx: context.Background(), replica: &fakePool{rowErr: refused}, wantPrimary: 1, wantReplica: 1, wantDegraded: true},
		{name: "standby shutdown falls back", ctx: context.Background(), replica: &fakePool{rowErr: shutdown}, wantPrimary: 1, wantReplica: 1, wantDegraded: true},
		{name: "statement error is returned", ctx: context.Background(), replica: &fakePool{rowErr: &pgconn.PgError{Code: "22P02"}}, wantErr: &pgconn.PgError{}, wantReplica: 1},
		{name: "missing row is returned", ctx: context.Background(), replica: &fakePool{rowErr: pgx.ErrNoRows}, wantErr: pgx.ErrNoRows, wantReplica: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			primary := &fakePool{name: "primary"}
			var p *Pools
			if tt.noReplica {
				p = NewPools(primary, nil, nopLogger{})
			} else {
				p = NewPools(primary, tt.replica, nopLogger{})
			}
			err := scan(tt.ctx, p)
			assertErr(t, err, tt.wantErr)
			if primary.calls != tt.wantPrimary {
				t.Fatalf("primary calls = %d, want %d", primary.calls, tt.wantPrimary)
			}
			if tt.replica != nil && tt.replica.calls != tt.wantReplica {
				t.Fatalf("replica calls = %d, want %d", tt.replica.calls, tt.wantReplica)
			}
			if degraded := !tt.noReplica && !p.healthy.Load(); degraded != tt.wantDegraded {
				t.Fatalf("degraded = %v, want %v", degraded, tt.wantDegraded)
			}
		})
	}
}

func TestPoolsQueryFallsBackOnBrokenReplica(t *testing.T) {
	primary := &fakePool{}
	replica := &fakePool{queryErr: &pgconn.ConnectError{}}
	p := NewPools(primary, replica, nopLogger{})

	if _, err := p.Read(context.Background()).Query(context.Background(), "SELECT 1"); err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if primary.calls != 1 || replica.calls != 1 {
		t.Fatalf("calls primary=%d replica=%d, want 1 and 1", primary.calls, replica.calls)
	}
	if _, err := p.Read(context.Background()).Query(context.Background(), "SELECT 1"); err != nil {
		t.Fatalf("second Query() error = %v", err)
	}
	if replica.calls != 1 {
		t.Fatalf("replica out of rotation got %d calls, want 1", replica.calls)
	}
}

func TestPoolsFindRepeatsMissingRowOnPrimary(t *testing.T) {
	tests := []struct {
		name        string
		ctx         context.Context
		primaryErr  error
		wantErr     error
		wantPrimary int
	}{
		{name: "row only on primary", ctx: context.Background(), wantPrimary: 1},
		{name: "row nowhere", ctx: context.Background(), primaryErr: pgx.ErrNoRows, wantErr: pgx.ErrNoRows, wantPrimary: 1},
		{name: "primary read is not repeated", ctx: readpref.Primary(context.Background()), primaryErr: pgx.ErrNoRows, wantErr: pgx.ErrNoRows, wantPrimary: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			primary := &fakePool{rowErr: tt.primaryErr}
			replica := &fakePool{rowErr: pgx.ErrNoRows}
			p := NewPools(primary, replica, nopLogger{})

			err := p.Find(tt.ctx, func(db DB) error { return db.QueryRow(tt.ctx, "SELECT 1").Scan() })
			assertErr(t, err, tt.wantErr)
			if primary.calls != tt.wantPrimary {
				t.Fatalf("primary calls = %d, want %d", primary.calls, tt.wantPrimary)
			}
		})
	}
}

func TestReplicaFailed(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "connect error", err: &pgconn.ConnectError{}, want: true},
		{name: "connection exception", err: &pgconn.PgError{Code: "08006"}, want: true},
		{name: "admin shutdown", err: &pgconn.PgError{Code: "57P01"}, want: true},
		{name: "recovery conflict", err: &pgconn.PgError{Code: "40001"}, want: true},
		{name: "unique violation", err: &pgconn.PgError{Code: "23505"}, want: false},
		{name: "read only transaction", err: &pgconn.PgError{Code: "25006"}, want: false},
		{name: "no rows", err: pgx.ErrNoRows, want: false},
		{name: "caller deadline", err: context.DeadlineExceeded, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ReplicaFailed(tt.err); got != tt.want {
				t.Fatalf("ReplicaFailed(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func assertErr(t *testing.T, err, want error) {
	t.Helper()
	var pgErr *pgconn.PgError
	switch {
	case want == nil && err != nil:
		t.Fatalf("error = %v, want nil", err)
	case want == nil:
	case errors.As(want, &pgErr):
		if !errors.As(err, &pgErr) {
			t.Fatalf("error = %v, want a PgError", err)
		}
	case !errors.Is(err, want):
		t.Fatalf("error = %v, want %v", err, want)
	}
}
