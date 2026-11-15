package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"chimera/catalog/internal/domain"
)

func mapError(err error, fallback string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return domain.Conflict("unique constraint violated")
		case "23503":
			return domain.NotFound("not found")
		case "22P02":
			return domain.Invalid("invalid id")
		}
	}
	return domain.Wrap(domain.CodeInternal, fallback, err)
}
