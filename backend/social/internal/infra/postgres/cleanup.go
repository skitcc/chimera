package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	"chimera/platform/pg"
	"chimera/social/internal/domain"
)

type CleanupRepository struct {
	db pg.TxDB
}

func NewCleanupRepository(db pg.TxDB) *CleanupRepository {
	return &CleanupRepository{db: db}
}

// DeleteUser drops the user's likes, follows in both directions and playlists,
// and blocks new rows for the user.
func (r *CleanupRepository) DeleteUser(ctx context.Context, d domain.UserDeletion) error {
	return r.apply(ctx, d.EventID, []statement{
		{`INSERT INTO deleted_users (user_id) VALUES ($1::uuid) ON CONFLICT DO NOTHING`, "record deleted user"},
		{`DELETE FROM track_likes WHERE user_id = $1::uuid`, "delete user likes"},
		{`DELETE FROM user_follows WHERE follower_id = $1::uuid OR following_id = $1::uuid`, "delete user follows"},
		{`DELETE FROM playlists WHERE owner_id = $1::uuid`, "delete user playlists"},
	}, d.UserID)
}

// DeleteTrack drops likes and playlist entries of the track. The playlist
// trigger closes the position gaps.
func (r *CleanupRepository) DeleteTrack(ctx context.Context, d domain.TrackDeletion) error {
	return r.apply(ctx, d.EventID, []statement{
		{`INSERT INTO deleted_tracks (track_id) VALUES ($1::uuid) ON CONFLICT DO NOTHING`, "record deleted track"},
		{`DELETE FROM track_likes WHERE track_id = $1::uuid`, "delete track likes"},
		{`DELETE FROM playlist_tracks WHERE track_id = $1::uuid`, "delete playlist entries"},
	}, d.TrackID)
}

type statement struct {
	sql string
	op  string
}

func (r *CleanupRepository) apply(ctx context.Context, eventID string, stmts []statement, id string) error {
	return pg.InTx(ctx, r.db, func(tx pgx.Tx) error {
		fresh, err := pg.MarkProcessed(ctx, tx, eventID)
		if err != nil || !fresh {
			return err
		}
		for _, s := range stmts {
			if _, err := tx.Exec(ctx, s.sql, id); err != nil {
				return mapError(err, s.op)
			}
		}
		return nil
	})
}
