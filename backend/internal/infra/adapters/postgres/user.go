package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"chimera/internal/domain"
)

type UserRepository struct {
	db db
}

func NewUserRepository(db db) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) List(ctx context.Context) ([]domain.User, error) {
	rows, err := r.db.Query(ctx, `SELECT id::text, email, name FROM users ORDER BY created_at, id`)
	if err != nil {
		return nil, mapError(err, "list users")
	}
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
		return nil, mapError(err, "list users")
	}
	return users, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx, `SELECT id::text, email, name FROM users WHERE id = $1::uuid`, id).
		Scan(&u.ID, &u.Email, &u.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.NotFound("user not found")
	}
	if err != nil {
		return domain.User{}, mapError(err, "get user")
	}
	return u, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (domain.AuthUser, error) {
	var acc domain.AuthUser
	err := r.db.QueryRow(ctx,
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

func (r *UserRepository) Create(ctx context.Context, u domain.User, passwordHash string) (domain.User, error) {
	err := r.db.QueryRow(ctx,
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
	err := r.db.QueryRow(ctx,
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
	err := r.db.QueryRow(ctx,
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

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM users WHERE id = $1::uuid`, id)
	if err != nil {
		return mapError(err, "delete user")
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("user not found")
	}
	return nil
}
