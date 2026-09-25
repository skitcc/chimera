//go:build integration

package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"chimera/internal/domain"
)

type postgresFixture struct {
	ctx    context.Context
	pool   *pgxpool.Pool
	users  *UserRepository
	tracks *TrackRepository
	likes  *TrackLikeRepository
}

func newPostgresFixture(t *testing.T) *postgresFixture {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping test database: %v", err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE track_likes, tracks, users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("reset test database: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `TRUNCATE track_likes, tracks, users RESTART IDENTITY CASCADE`)
	})
	return &postgresFixture{
		ctx:    ctx,
		pool:   pool,
		users:  NewUserRepository(pool),
		tracks: NewTrackRepository(pool),
		likes:  NewTrackLikeRepository(pool),
	}
}

func (f *postgresFixture) seedUser(t *testing.T) domain.User {
	t.Helper()
	var user domain.User
	err := f.pool.QueryRow(f.ctx,
		`INSERT INTO users (email, name, password_hash)
		 VALUES ('fixture@example.com', 'Fixture', 'hash')
		 RETURNING id::text, email, name`,
	).Scan(&user.ID, &user.Email, &user.Name)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return user
}

func (f *postgresFixture) seedTrack(t *testing.T, userID string) domain.Track {
	t.Helper()
	var track domain.Track
	var status string
	err := f.pool.QueryRow(f.ctx,
		`INSERT INTO tracks (user_id, title, artist, object_key, size_bytes, status)
		 VALUES ($1::uuid, 'Fixture Track', 'Fixture Artist', 'fixture.mp3', 128, 'ready')
		 RETURNING id::text, user_id::text, title, artist, object_key, size_bytes, status`,
		userID,
	).Scan(&track.ID, &track.UserID, &track.Title, &track.Artist, &track.ObjectKey, &track.SizeBytes, &status)
	if err != nil {
		t.Fatalf("seed track: %v", err)
	}
	track.Status = domain.TrackStatus(status)
	return track
}

func TestUserRepositoryClassicIntegration(t *testing.T) {
	runCase(t, "create persists a user in PostgreSQL", func(t *testing.T) {
		// Arrange
		f := newPostgresFixture(t)
		input := domain.User{Email: "alice@example.com", Name: "Alice"}

		// Act
		created, err := f.users.Create(f.ctx, input, "hash")

		// Assert
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		var count int
		if err := f.pool.QueryRow(f.ctx, `SELECT count(*) FROM users WHERE id = $1::uuid`, created.ID).Scan(&count); err != nil {
			t.Fatalf("verify user: %v", err)
		}
		assertEqual(t, count, 1)
	})

	runCase(t, "get returns a user seeded in PostgreSQL", func(t *testing.T) {
		// Arrange
		f := newPostgresFixture(t)
		want := f.seedUser(t)

		// Act
		got, err := f.users.GetByID(f.ctx, want.ID)

		// Assert
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		assertEqual(t, got, want)
	})

	runCase(t, "list reads users from PostgreSQL", func(t *testing.T) {
		// Arrange
		f := newPostgresFixture(t)
		want := f.seedUser(t)

		// Act
		got, err := f.users.List(f.ctx)

		// Assert
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		assertEqual(t, len(got), 1)
		assertEqual(t, got[0], want)
	})

	runCase(t, "delete removes a user from PostgreSQL", func(t *testing.T) {
		// Arrange
		f := newPostgresFixture(t)
		user := f.seedUser(t)

		// Act
		err := f.users.Delete(f.ctx, user.ID)

		// Assert
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		var count int
		if err := f.pool.QueryRow(f.ctx, `SELECT count(*) FROM users WHERE id = $1::uuid`, user.ID).Scan(&count); err != nil {
			t.Fatalf("verify deletion: %v", err)
		}
		assertEqual(t, count, 0)
	})
}

func TestTrackRepositoryClassicIntegration(t *testing.T) {
	runCase(t, "create persists a track in PostgreSQL", func(t *testing.T) {
		// Arrange
		f := newPostgresFixture(t)
		user := f.seedUser(t)
		input := domain.Track{
			UserID: user.ID, Title: "Created", Artist: "Artist",
			SizeBytes: 256, Status: domain.TrackPending,
		}

		// Act
		created, err := f.tracks.Create(f.ctx, input)

		// Assert
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		var count int
		if err := f.pool.QueryRow(f.ctx, `SELECT count(*) FROM tracks WHERE id = $1::uuid`, created.ID).Scan(&count); err != nil {
			t.Fatalf("verify track: %v", err)
		}
		assertEqual(t, count, 1)
	})

	runCase(t, "get returns a track seeded in PostgreSQL", func(t *testing.T) {
		// Arrange
		f := newPostgresFixture(t)
		want := f.seedTrack(t, f.seedUser(t).ID)

		// Act
		got, err := f.tracks.GetByID(f.ctx, want.ID)

		// Assert
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		assertEqual(t, got, want)
	})

	runCase(t, "list filters tracks stored in PostgreSQL", func(t *testing.T) {
		// Arrange
		f := newPostgresFixture(t)
		want := f.seedTrack(t, f.seedUser(t).ID)

		// Act
		got, err := f.tracks.List(f.ctx, domain.TrackFilter{Artist: want.Artist, Status: domain.TrackReady})

		// Assert
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		assertEqual(t, len(got), 1)
		assertEqual(t, got[0], want)
	})

	runCase(t, "delete removes a track from PostgreSQL", func(t *testing.T) {
		// Arrange
		f := newPostgresFixture(t)
		track := f.seedTrack(t, f.seedUser(t).ID)

		// Act
		err := f.tracks.Delete(f.ctx, track.ID)

		// Assert
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		var count int
		if err := f.pool.QueryRow(f.ctx, `SELECT count(*) FROM tracks WHERE id = $1::uuid`, track.ID).Scan(&count); err != nil {
			t.Fatalf("verify deletion: %v", err)
		}
		assertEqual(t, count, 0)
	})
}

func TestTrackLikeRepositoryClassicIntegration(t *testing.T) {
	runCase(t, "add persists a like in PostgreSQL", func(t *testing.T) {
		// Arrange
		f := newPostgresFixture(t)
		user := f.seedUser(t)
		track := f.seedTrack(t, user.ID)

		// Act
		err := f.likes.Add(f.ctx, user.ID, track.ID)

		// Assert
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		var count int
		if err := f.pool.QueryRow(f.ctx,
			`SELECT count(*) FROM track_likes WHERE user_id = $1::uuid AND track_id = $2::uuid`,
			user.ID, track.ID,
		).Scan(&count); err != nil {
			t.Fatalf("verify like: %v", err)
		}
		assertEqual(t, count, 1)
	})

	runCase(t, "list returns a liked ready track from PostgreSQL", func(t *testing.T) {
		// Arrange
		f := newPostgresFixture(t)
		user := f.seedUser(t)
		want := f.seedTrack(t, user.ID)
		if _, err := f.pool.Exec(f.ctx,
			`INSERT INTO track_likes (user_id, track_id) VALUES ($1::uuid, $2::uuid)`,
			user.ID, want.ID,
		); err != nil {
			t.Fatalf("seed like: %v", err)
		}

		// Act
		got, err := f.likes.ListReadyByUser(f.ctx, user.ID)

		// Assert
		if err != nil {
			t.Fatalf("ListReadyByUser() error = %v", err)
		}
		assertEqual(t, len(got), 1)
		assertEqual(t, got[0], want)
	})

	runCase(t, "remove deletes a like from PostgreSQL", func(t *testing.T) {
		// Arrange
		f := newPostgresFixture(t)
		user := f.seedUser(t)
		track := f.seedTrack(t, user.ID)
		if _, err := f.pool.Exec(f.ctx,
			`INSERT INTO track_likes (user_id, track_id) VALUES ($1::uuid, $2::uuid)`,
			user.ID, track.ID,
		); err != nil {
			t.Fatalf("seed like: %v", err)
		}

		// Act
		err := f.likes.Remove(f.ctx, user.ID, track.ID)

		// Assert
		if err != nil {
			t.Fatalf("Remove() error = %v", err)
		}
		var count int
		if err := f.pool.QueryRow(f.ctx,
			`SELECT count(*) FROM track_likes WHERE user_id = $1::uuid AND track_id = $2::uuid`,
			user.ID, track.ID,
		).Scan(&count); err != nil {
			t.Fatalf("verify unlike: %v", err)
		}
		assertEqual(t, count, 0)
	})
}
