package postgres

import (
	"context"
	"embed"

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
