// Package pg opens the pgx pool, applies a service's embedded migrations and runs transactions.
package pg

import (
	"context"
	"errors"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"chimera/platform/apperr"
	"chimera/platform/env"
	"chimera/platform/health"
)

// DB is the part of pgxpool.Pool and pgx.Tx that repositories use.
type DB interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// TxDB is a DB that can also open a transaction.
type TxDB interface {
	DB
	Begin(context.Context) (pgx.Tx, error)
}

func NewPool(ctx context.Context, cfg env.Postgres) (*pgxpool.Pool, error) {
	pcfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInternal, "parse postgres dsn", err)
	}
	pcfg.MinConns = cfg.MinConns
	pcfg.MaxConns = cfg.MaxConns
	pcfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	pcfg.MaxConnLifetime = cfg.MaxConnLifetime
	pcfg.HealthCheckPeriod = cfg.HealthCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInternal, "open postgres", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, apperr.Wrap(apperr.CodeInternal, "ping postgres", err)
	}
	return pool, nil
}

// Connect retries NewPool until Postgres answers or ctx ends.
func Connect(ctx context.Context, log health.Logger, cfg env.Postgres, h env.Health) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	err := health.Wait(ctx, log, "postgres", h.Interval, h.Timeout, func(ctx context.Context) error {
		p, err := NewPool(ctx, cfg)
		if err != nil {
			return err
		}
		pool = p
		return nil
	})
	if err != nil {
		return nil, err
	}
	return pool, nil
}

// Migrate applies every pending migration in dir of fsys. Concurrent replicas
// wait on the advisory lock golang-migrate takes, so only one applies a version.
func Migrate(pool *pgxpool.Pool, fsys fs.FS, dir string) error {
	src, err := iofs.New(fsys, dir)
	if err != nil {
		return apperr.Wrap(apperr.CodeInternal, "read migrations", err)
	}
	db := stdlib.OpenDBFromPool(pool)
	drv, err := pgxmigrate.WithInstance(db, &pgxmigrate.Config{})
	if err != nil {
		_ = db.Close()
		return apperr.Wrap(apperr.CodeInternal, "open migration driver", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "pgx5", drv)
	if err != nil {
		_ = db.Close()
		return apperr.Wrap(apperr.CodeInternal, "open migrations", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return apperr.Wrap(apperr.CodeInternal, "apply migrations", err)
	}
	return nil
}

// InTx runs fn in one transaction and commits when fn returns nil.
func InTx(ctx context.Context, db TxDB, fn func(pgx.Tx) error) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return apperr.Wrap(apperr.CodeInternal, "begin transaction", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return apperr.Wrap(apperr.CodeInternal, "commit transaction", err)
	}
	return nil
}

// MarkProcessed records eventID in processed_events inside tx. It reports false
// when the event was already handled, so a redelivered event changes nothing.
func MarkProcessed(ctx context.Context, tx pgx.Tx, eventID string) (bool, error) {
	tag, err := tx.Exec(ctx,
		`INSERT INTO processed_events (event_id) VALUES ($1::uuid) ON CONFLICT DO NOTHING`, eventID)
	if err != nil {
		return false, apperr.Wrap(apperr.CodeInternal, "mark event processed", err)
	}
	return tag.RowsAffected() == 1, nil
}
