package testkit

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

const resetTables = `TRUNCATE track_likes, tracks, users RESTART IDENTITY CASCADE`

// Open connects to the database in TEST_DATABASE_URL and clears it for one case.
// The integration run creates that database once and shares it across cases.
func Open(t *testing.T) *pgxpool.Pool {
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
	if _, err := pool.Exec(ctx, resetTables); err != nil {
		t.Fatalf("reset test database: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), resetTables)
	})
	return pool
}
