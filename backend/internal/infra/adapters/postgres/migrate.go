package postgres

import (
	"context"
	"embed"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"chimera/internal/domain"
)

//go:embed schema.sql
var schemaFS embed.FS

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	sql, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return domain.Wrap(domain.CodeInternal, "read schema", err)
	}
	if _, err := pool.Exec(ctx, string(sql)); err != nil {
		return domain.Wrap(domain.CodeInternal, "apply schema", err)
	}
	return nil
}

// EnsureReadOnlyRole creates or updates a login role that can only read the
// current database. Identifiers and the password are quoted by Postgres format.
// CREATE ROLE cannot run inside a DO block, so the statement is executed on its own.
func EnsureReadOnlyRole(ctx context.Context, pool *pgxpool.Pool, user, password string) error {
	if strings.TrimSpace(user) == "" || password == "" {
		return domain.Internal("read only role is not configured")
	}
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)`, user).Scan(&exists); err != nil {
		return domain.Wrap(domain.CodeInternal, "ensure read only role", err)
	}
	statement := `SELECT format('CREATE ROLE %I WITH LOGIN PASSWORD %L NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT', $1::text, $2::text)`
	if exists {
		statement = `SELECT format('ALTER ROLE %I WITH LOGIN PASSWORD %L NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT', $1::text, $2::text)`
	}
	if err := execFormatted(ctx, pool, statement, user, password); err != nil {
		return domain.Wrap(domain.CodeInternal, "ensure read only role", err)
	}

	grants := []string{
		`SELECT format('GRANT CONNECT ON DATABASE %I TO %I', current_database(), $1::text)`,
		`SELECT format('GRANT USAGE ON SCHEMA public TO %I', $1::text)`,
		`SELECT format('REVOKE ALL ON ALL TABLES IN SCHEMA public FROM %I', $1::text)`,
		`SELECT format('GRANT SELECT ON ALL TABLES IN SCHEMA public TO %I', $1::text)`,
		`SELECT format('ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO %I', $1::text)`,
	}
	for _, query := range grants {
		if err := execFormatted(ctx, pool, query, user); err != nil {
			return domain.Wrap(domain.CodeInternal, "grant read only role", err)
		}
	}
	return nil
}

func execFormatted(ctx context.Context, pool *pgxpool.Pool, query string, args ...any) error {
	var stmt string
	if err := pool.QueryRow(ctx, query, args...).Scan(&stmt); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, stmt); err != nil {
		return err
	}
	return nil
}
