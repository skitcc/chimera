package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"chimera/contracts/events"
	eventsv1 "chimera/contracts/gen/events/v1"
	"chimera/identity/internal/domain"
	"chimera/platform/idempotency"
	"chimera/platform/outbox"
	"chimera/platform/pg"
)

const producer = "identity"

// Pools routes each statement to the primary or a replica.
type Pools interface {
	Write() pg.TxDB
	Read(ctx context.Context) pg.DB
	Find(ctx context.Context, fn func(pg.DB) error) error
}

// UserRepository reads profiles from a replica. Login and every write use the
// primary, so a new account or a changed password works at once.
type UserRepository struct {
	db Pools
}

func NewUserRepository(db Pools) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) List(ctx context.Context) ([]domain.User, error) {
	rows, err := r.db.Read(ctx).Query(ctx, `SELECT id::text, email, name FROM users ORDER BY created_at, id`)
	if err != nil {
		return nil, mapError(err, "list users")
	}
	return collectUsers(rows, "list users")
}

func (r *UserRepository) GetMany(ctx context.Context, ids []string) ([]domain.User, error) {
	rows, err := r.db.Read(ctx).Query(ctx,
		`SELECT id::text, email, name FROM users WHERE id = ANY($1::uuid[]) ORDER BY created_at, id`, ids)
	if err != nil {
		return nil, mapError(err, "get users")
	}
	return collectUsers(rows, "get users")
}

func collectUsers(rows pgx.Rows, op string) ([]domain.User, error) {
	defer rows.Close()

	users := make([]domain.User, 0)
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name); err != nil {
			return nil, mapError(err, "scan user")
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err, op)
	}
	return users, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (domain.User, error) {
	var u domain.User
	err := r.db.Find(ctx, func(db pg.DB) error {
		var err error
		u, err = scanUser(ctx, db, id)
		return err
	})
	return u, userError(err, "get user")
}

func getUser(ctx context.Context, db pg.DB, id string) (domain.User, error) {
	u, err := scanUser(ctx, db, id)
	return u, userError(err, "get user")
}

func scanUser(ctx context.Context, db pg.DB, id string) (domain.User, error) {
	var u domain.User
	err := db.QueryRow(ctx, `SELECT id::text, email, name FROM users WHERE id = $1::uuid`, id).
		Scan(&u.ID, &u.Email, &u.Name)
	return u, err
}

func userError(err error, op string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NotFound("user not found")
	}
	if err != nil {
		return mapError(err, op)
	}
	return nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (domain.AuthUser, error) {
	var acc domain.AuthUser
	err := r.db.Write().QueryRow(ctx,
		`SELECT id::text, email, name, password_hash FROM users WHERE email = $1`, email,
	).Scan(&acc.User.ID, &acc.User.Email, &acc.User.Name, &acc.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AuthUser{}, domain.NotFound("user not found")
	}
	if err != nil {
		return domain.AuthUser{}, mapError(err, "get user by email")
	}
	return acc, nil
}

// opCreateUser scopes idempotency keys of registrations. The password is not part
// of the fingerprint, so no hash of it is stored next to the key.
const opCreateUser = "identity.CreateUser"

// Create inserts the account. With an idempotency key in ctx a repeat of the same
// registration returns the account the first call created.
func (r *UserRepository) Create(ctx context.Context, u domain.User, passwordHash string) (domain.User, error) {
	if idempotency.From(ctx) == "" {
		return insertUser(ctx, r.db.Write(), u, passwordHash)
	}
	var created domain.User
	err := pg.InTx(ctx, r.db.Write(), func(tx pgx.Tx) error {
		replay, err := idempotency.Claim(ctx, tx, opCreateUser, "", u.Email, u.Name)
		if err != nil {
			return err
		}
		if replay != "" {
			created, err = getUser(ctx, tx, replay)
			return err
		}
		created, err = insertUser(ctx, tx, u, passwordHash)
		if err != nil {
			return err
		}
		return idempotency.Complete(ctx, tx, opCreateUser, "", created.ID)
	})
	return created, err
}

func insertUser(ctx context.Context, db pg.DB, u domain.User, passwordHash string) (domain.User, error) {
	err := db.QueryRow(ctx,
		`INSERT INTO users (email, name, password_hash) VALUES ($1, $2, $3)
		 RETURNING id::text, email, name`,
		u.Email, u.Name, passwordHash,
	).Scan(&u.ID, &u.Email, &u.Name)
	if err != nil {
		return domain.User{}, mapError(err, "create user")
	}
	return u, nil
}

func (r *UserRepository) Update(ctx context.Context, u domain.User) (domain.User, error) {
	err := r.db.Write().QueryRow(ctx,
		`UPDATE users SET email = $2, name = $3 WHERE id = $1::uuid
		 RETURNING id::text, email, name`,
		u.ID, u.Email, u.Name,
	).Scan(&u.ID, &u.Email, &u.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.NotFound("user not found")
	}
	if err != nil {
		return domain.User{}, mapError(err, "update user")
	}
	return u, nil
}

func (r *UserRepository) UpdateProfile(ctx context.Context, u domain.User, passwordHash *string) (domain.User, error) {
	err := r.db.Write().QueryRow(ctx,
		`UPDATE users
		 SET email = $2, name = $3, password_hash = COALESCE($4, password_hash)
		 WHERE id = $1::uuid
		 RETURNING id::text, email, name`,
		u.ID, u.Email, u.Name, passwordHash,
	).Scan(&u.ID, &u.Email, &u.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.NotFound("user not found")
	}
	if err != nil {
		return domain.User{}, mapError(err, "update profile")
	}
	return u, nil
}

// Delete removes the user and writes UserDeleted to the outbox in the same
// transaction. Catalog and Social drop the user's data when the event arrives.
func (r *UserRepository) Delete(ctx context.Context, id string) error {
	return pg.InTx(ctx, r.db.Write(), func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM users WHERE id = $1::uuid`, id)
		if err != nil {
			return mapError(err, "delete user")
		}
		if tag.RowsAffected() == 0 {
			return domain.NotFound("user not found")
		}
		event := outbox.NewEvent(producer, events.TypeUserDeleted)
		event.Payload = &eventsv1.Envelope_UserDeleted{UserDeleted: &eventsv1.UserDeleted{UserId: id}}
		return outbox.Write(ctx, tx, events.TopicUser, id, event)
	})
}
