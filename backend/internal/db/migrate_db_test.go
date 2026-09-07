package db

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Two processes starting together both read schema_migrations before either
// writes to it, so both decide the same file still needs applying and the
// second fails on whatever the first created. That is what two API containers
// starting at once would do to each other, and it is what made the database
// tests here unrunnable in one `go test ./...`.
//
// The check has to run against a database nothing has migrated yet, or every
// goroutine skips every file and proves nothing — so it makes its own and drops
// it afterwards. Runs against TEST_DATABASE_URL and skips without it.
func TestMigrateIsSafeToRunConcurrently(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run the concurrent migration test")
	}
	ctx := context.Background()

	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer admin.Close()

	const name = "rung_migrate_race_test"
	if _, err := admin.Exec(ctx, "drop database if exists "+name); err != nil {
		t.Fatalf("drop: %v", err)
	}
	if _, err := admin.Exec(ctx, "create database "+name); err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "drop database if exists "+name+" with (force)")
	})

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	cfg.ConnConfig.Database = name

	const racers = 6
	var wg sync.WaitGroup
	errs := make(chan error, racers)
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pool, err := pgxpool.NewWithConfig(ctx, cfg.Copy())
			if err != nil {
				errs <- err
				return
			}
			defer pool.Close()
			if err := Migrate(ctx, pool); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)

	failed := 0
	for err := range errs {
		failed++
		if failed == 1 {
			t.Errorf("a concurrent migration failed: %v", err)
		}
	}
	if failed > 1 {
		t.Errorf("%d of %d concurrent migrations failed", failed, racers)
	}
}
