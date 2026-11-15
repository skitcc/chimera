package pg

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"chimera/platform/readpref"
)

// Pool is a connection pool that can be probed and closed.
type Pool interface {
	TxDB
	Ping(context.Context) error
	Close()
}

type Logger interface {
	InfoContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
}

// Pools routes statements: writes, transactions and reads marked by readpref go to
// the primary, other reads to a replica. While the replica fails, reads use the
// primary, so a lost replica costs latency and not errors.
type Pools struct {
	write   Pool
	read    Pool
	log     Logger
	healthy atomic.Bool
}

// NewPools routes reads to read. Without a replica (read nil) both roles use write.
func NewPools(write, read Pool, log Logger) *Pools {
	p := &Pools{write: write, read: read, log: log}
	p.healthy.Store(read != nil)
	return p
}

// Single serves both roles from db. Tests and stands without a replica use it.
func Single(db TxDB) *Pools {
	return &Pools{write: singlePool{db}}
}

type singlePool struct{ TxDB }

func (singlePool) Ping(context.Context) error { return nil }

func (singlePool) Close() {}

func (p *Pools) Write() TxDB { return p.write }

// Read returns the replica unless ctx asks for the primary or the replica is down.
func (p *Pools) Read(ctx context.Context) DB {
	db, _ := p.route(ctx)
	return db
}

// Find runs a lookup by key. A row the replica does not have yet is looked up
// again on the primary, so replication lag never turns a fresh entity into not_found.
func (p *Pools) Find(ctx context.Context, fn func(DB) error) error {
	db, replica := p.route(ctx)
	err := fn(db)
	if replica && errors.Is(err, pgx.ErrNoRows) {
		return fn(p.write)
	}
	return err
}

func (p *Pools) route(ctx context.Context) (DB, bool) {
	if p.read == nil || readpref.IsPrimary(ctx) || !p.healthy.Load() {
		return p.write, false
	}
	return replicaDB{p: p}, true
}

// WatchReplica probes the replica every interval and puts it back into rotation
// after a failure. It returns when ctx ends.
func (p *Pools) WatchReplica(ctx context.Context, interval, timeout time.Duration) {
	if p.read == nil {
		return
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		pctx, cancel := context.WithTimeout(ctx, timeout)
		err := p.read.Ping(pctx)
		cancel()
		if err != nil {
			p.markDown(ctx, err)
			continue
		}
		if p.healthy.CompareAndSwap(false, true) {
			p.log.InfoContext(ctx, "postgres replica back in rotation")
		}
	}
}

func (p *Pools) Close() {
	p.write.Close()
	if p.read != nil {
		p.read.Close()
	}
}

func (p *Pools) markDown(ctx context.Context, err error) {
	if p.healthy.CompareAndSwap(true, false) {
		p.log.ErrorContext(ctx, "postgres replica out of rotation, reading from primary", "error", err)
	}
}

// replicaDB reads from the replica and repeats a statement on the primary when the
// replica itself failed. Only reads arrive here, so repeating them is safe.
type replicaDB struct{ p *Pools }

func (r replicaDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return r.p.read.Exec(ctx, sql, args...)
}

func (r replicaDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := r.p.read.Query(ctx, sql, args...)
	if err == nil || !ReplicaFailed(err) {
		return rows, err
	}
	r.p.markDown(ctx, err)
	return r.p.write.Query(ctx, sql, args...)
}

func (r replicaDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return fallbackRow{
		row: r.p.read.QueryRow(ctx, sql, args...),
		retry: func(err error) pgx.Row {
			r.p.markDown(ctx, err)
			return r.p.write.QueryRow(ctx, sql, args...)
		},
	}
}

type fallbackRow struct {
	row   pgx.Row
	retry func(error) pgx.Row
}

func (f fallbackRow) Scan(dest ...any) error {
	err := f.row.Scan(dest...)
	if err == nil || !ReplicaFailed(err) {
		return err
	}
	return f.retry(err).Scan(dest...)
}

// ReplicaFailed reports errors of the server or the connection, not of the statement:
// refused or broken connections, a standby shutting down or cancelling a query that
// conflicts with recovery.
func ReplicaFailed(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return strings.HasPrefix(pgErr.Code, "08") || strings.HasPrefix(pgErr.Code, "57P") || pgErr.Code == "40001"
	}
	var connErr *pgconn.ConnectError
	var netErr net.Error
	return errors.As(err, &connErr) || errors.As(err, &netErr) ||
		errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || pgconn.SafeToRetry(err)
}
