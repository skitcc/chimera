package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"chimera/internal/config"
	"chimera/internal/domain"
)

func NewPool(ctx context.Context, cfg config.Postgres) (*pgxpool.Pool, error) {
	pcfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, domain.Wrap(domain.CodeInternal, "parse postgres dsn", err)
	}
	pcfg.MinConns = cfg.MinConns
	pcfg.MaxConns = cfg.MaxConns
	pcfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	pcfg.MaxConnLifetime = cfg.MaxConnLifetime
	pcfg.HealthCheckPeriod = cfg.HealthCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, domain.Wrap(domain.CodeInternal, "open postgres", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, domain.Wrap(domain.CodeInternal, "ping postgres", err)
	}
	return pool, nil
}

func mapError(err error, fallback string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			switch pgErr.ConstraintName {
			case "track_likes_pkey":
				return domain.Conflict("track already liked")
			default:
				return domain.Conflict("email already exists")
			}
		case "23503":
			return domain.NotFound("not found")
		case "22P02":
			return domain.Invalid("invalid id")
		}
	}
	return domain.Wrap(domain.CodeInternal, fallback, err)
}
