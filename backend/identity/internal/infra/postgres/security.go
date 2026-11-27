package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"chimera/identity/internal/domain"
	"chimera/platform/pg"
)

func (r *UserRepository) WithAccount(ctx context.Context, l domain.SecurityLookup, fn func(*domain.SecurityAccount) error) error {
	return pg.InTx(ctx, r.db.Write(), func(tx pgx.Tx) error {
		a := domain.SecurityAccount{Challenges: map[string]domain.Challenge{}}
		// Lock only the account first. Every auth operation takes this same lock,
		// including reads/writes of its challenge rows, in a consistent order.
		var locked *time.Time
		err := tx.QueryRow(ctx, `SELECT id::text,email,name,password_hash,auth_version,auth_failures,locked_until
   FROM users WHERE ($1 <> '' AND email=$1) OR ($2 <> '' AND id::text=$2)
   OR ($1 = '' AND $2 = '' AND $3 <> '' AND id=(SELECT user_id FROM auth_challenges WHERE id=$3)) FOR UPDATE`,
			l.Email, l.UserID, l.ChallengeID).Scan(&a.User.ID, &a.User.Email, &a.User.Name, &a.PasswordHash, &a.User.AuthVersion, &a.Failures, &locked)
		if err != nil {
			return userError(err, "lock auth account")
		}
		if locked != nil {
			a.LockedUntil = *locked
		}
		rows, err := tx.Query(ctx, `SELECT purpose,id,digest,version,expires_at,sent_at FROM auth_challenges WHERE user_id=$1::uuid`, a.User.ID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var purpose string
			var c domain.Challenge
			if err = rows.Scan(&purpose, &c.ID, &c.Digest, &c.Version, &c.ExpiresAt, &c.SentAt); err != nil {
				rows.Close()
				return err
			}
			a.Challenges[purpose] = c
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if err = fn(&a); err != nil {
			return err
		}
		var until *time.Time
		if !a.LockedUntil.IsZero() {
			until = &a.LockedUntil
		}
		if _, err = tx.Exec(ctx, `UPDATE users SET password_hash=$2,auth_version=$3,auth_failures=$4,locked_until=$5 WHERE id=$1::uuid`, a.User.ID, a.PasswordHash, a.User.AuthVersion, a.Failures, until); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM auth_challenges WHERE user_id=$1::uuid`, a.User.ID); err != nil {
			return err
		}
		for purpose, c := range a.Challenges {
			digest := c.Digest
			if digest == nil {
				digest = []byte{}
			}
			if _, err = tx.Exec(ctx, `INSERT INTO auth_challenges(user_id,purpose,id,digest,version,expires_at,sent_at) VALUES($1::uuid,$2,$3,$4,$5,$6,$7)`, a.User.ID, purpose, c.ID, digest, c.Version, c.ExpiresAt, c.SentAt); err != nil {
				return err
			}
		}
		return nil
	})
}
func (r *UserRepository) Version(ctx context.Context, id string) (int64, error) {
	var version int64
	err := r.db.Write().QueryRow(ctx, `SELECT auth_version FROM users WHERE id::text=$1`, id).Scan(&version)
	if err != nil {
		return 0, userError(err, "validate session")
	}
	return version, nil
}

var _ domain.SecurityStore = (*UserRepository)(nil)
