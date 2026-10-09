package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"chimera/internal/domain"
)

const pickCols = `id::text, user_id::text, title, generated_at`

type pickDB interface {
	db
	Begin(context.Context) (pgx.Tx, error)
}

type PickRepository struct {
	db pickDB
}

func NewPickRepository(db pickDB) *PickRepository {
	return &PickRepository{db: db}
}

func scanPick(row scanner) (domain.Pick, error) {
	var pick domain.Pick
	err := row.Scan(&pick.ID, &pick.UserID, &pick.Title, &pick.GeneratedAt)
	return pick, err
}

func (r *PickRepository) CreateRandom(ctx context.Context, userID, title string, tracks []domain.Track) (domain.PickDetail, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.PickDetail{}, mapError(err, "begin create pick")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	pick, err := scanPick(tx.QueryRow(ctx,
		`INSERT INTO picks (user_id, title)
		 VALUES ($1::uuid, $2)
		 RETURNING `+pickCols,
		userID, title,
	))
	if err != nil {
		return domain.PickDetail{}, mapError(err, "create pick")
	}

	items := make([]domain.PickTrack, 0, len(tracks))
	for position, track := range tracks {
		if _, err := tx.Exec(ctx,
			`INSERT INTO pick_tracks (pick_id, track_id, position, note)
			 VALUES ($1::uuid, $2::uuid, $3, $4)`,
			pick.ID, track.ID, position, "random",
		); err != nil {
			return domain.PickDetail{}, mapError(err, "add pick track")
		}
		items = append(items, domain.PickTrack{Position: position, Note: "random", Track: track})
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.PickDetail{}, mapError(err, "commit create pick")
	}
	return domain.PickDetail{Pick: pick, Tracks: items}, nil
}

func (r *PickRepository) ListByUser(ctx context.Context, userID string) ([]domain.Pick, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+pickCols+`
		 FROM picks
		 WHERE user_id = $1::uuid
		 ORDER BY generated_at DESC, id DESC`,
		userID,
	)
	if err != nil {
		return nil, mapError(err, "list picks")
	}
	defer rows.Close()

	picks := make([]domain.Pick, 0)
	for rows.Next() {
		pick, err := scanPick(rows)
		if err != nil {
			return nil, mapError(err, "scan pick")
		}
		picks = append(picks, pick)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err, "list picks")
	}
	return picks, nil
}

func (r *PickRepository) GetByID(ctx context.Context, id string) (domain.PickDetail, error) {
	pick, err := scanPick(r.db.QueryRow(ctx,
		`SELECT `+pickCols+` FROM picks WHERE id = $1::uuid`, id,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PickDetail{}, domain.NotFound("pick not found")
	}
	if err != nil {
		return domain.PickDetail{}, mapError(err, "get pick")
	}

	rows, err := r.db.Query(ctx,
		`SELECT pt.position, pt.note, `+trackCols+`
		 FROM pick_tracks pt
		 JOIN tracks t ON t.id = pt.track_id
		 WHERE pt.pick_id = $1::uuid
		 ORDER BY pt.position`,
		id,
	)
	if err != nil {
		return domain.PickDetail{}, mapError(err, "list pick tracks")
	}
	defer rows.Close()

	items := make([]domain.PickTrack, 0)
	for rows.Next() {
		var item domain.PickTrack
		var status string
		err := rows.Scan(
			&item.Position,
			&item.Note,
			&item.Track.ID,
			&item.Track.UserID,
			&item.Track.Title,
			&item.Track.Artist,
			&item.Track.ObjectKey,
			&item.Track.SizeBytes,
			&status,
		)
		if err != nil {
			return domain.PickDetail{}, mapError(err, "scan pick track")
		}
		item.Track.Status = domain.TrackStatus(status)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return domain.PickDetail{}, mapError(err, "list pick tracks")
	}
	return domain.PickDetail{Pick: pick, Tracks: items}, nil
}

func (r *PickRepository) UpdateTitle(ctx context.Context, id, title string) (domain.Pick, error) {
	pick, err := scanPick(r.db.QueryRow(ctx,
		`UPDATE picks SET title = $2 WHERE id = $1::uuid RETURNING `+pickCols,
		id, title,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Pick{}, domain.NotFound("pick not found")
	}
	if err != nil {
		return domain.Pick{}, mapError(err, "rename pick")
	}
	return pick, nil
}

func (r *PickRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM picks WHERE id = $1::uuid`, id)
	if err != nil {
		return mapError(err, "delete pick")
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("pick not found")
	}
	return nil
}
