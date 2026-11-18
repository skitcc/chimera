package testkit

import (
	"context"
	"io/fs"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"chimera/platform/pg"
)

// Open connects to the database in TEST_DATABASE_URL, applies the service
// migrations once per test binary and runs reset before and after the case.
// Each service gets its own database, so reset lists only that service's tables.
func Open(t *testing.T, migrations fs.FS, reset string) *pgxpool.Pool {
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
	if err := migrateDB(pool, migrations); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	if _, err := pool.Exec(ctx, reset); err != nil {
		t.Fatalf("reset test database: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), reset)
	})
	return pool
}

var (
	migrateMu   sync.Mutex
	migrateDone bool
)

func migrateDB(pool *pgxpool.Pool, migrations fs.FS) error {
	migrateMu.Lock()
	defer migrateMu.Unlock()
	if migrateDone {
		return nil
	}
	if err := pg.Migrate(pool, migrations, "."); err != nil {
		return err
	}
	migrateDone = true
	return nil
}
