package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"chimera/identity/internal/domain"
)

func mapError(err error, fallback string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			if pgErr.ConstraintName == "users_email_key" {
				return domain.Conflict("email already exists")
			}
			return domain.Conflict("unique constraint violated")
		case "23503":
			return domain.NotFound("not found")
		case "22P02":
			return domain.Invalid("invalid id")
		case "25006", "42501":
			// A write reached a hot standby or a read-only role.
			return domain.Forbidden("read only")
		}
	}
	return domain.Wrap(domain.CodeInternal, fallback, err)
}
