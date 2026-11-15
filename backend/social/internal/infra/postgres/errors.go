package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"chimera/social/internal/domain"
)

func mapError(err error, fallback string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			switch pgErr.ConstraintName {
			case "track_likes_pkey":
				return domain.Conflict("track already liked")
			case "playlist_tracks_pkey":
				return domain.Conflict("track already in playlist")
			case "playlist_tracks_playlist_position_key":
				return domain.Conflict("playlist position already occupied")
			default:
				return domain.Conflict("unique constraint violated")
			}
		case "23503":
			return domain.NotFound("not found")
		case "22P02":
			return domain.Invalid("invalid id")
		}
	}
	return domain.Wrap(domain.CodeInternal, fallback, err)
}

type scanner interface {
	Scan(dest ...any) error
}
