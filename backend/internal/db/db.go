package db

import (
	"context"
	"embed"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Connect opens a pool and waits for Postgres to accept connections. On a fresh
// compose stack the database is often still initialising, so this retries.
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 15 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt < 30; attempt++ {
		lastErr = pool.Ping(ctx)
		if lastErr == nil {
			return pool, nil
		}
		select {
		case <-ctx.Done():
			pool.Close()
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	pool.Close()
	return nil, fmt.Errorf("database unreachable after 30s: %w", lastErr)
}

// migrationLock is the advisory lock every migrator queues behind. The number
// is arbitrary and only has to be the same one everywhere.
const migrationLock = 8613427

// Migrate applies any embedded .sql file that has not run yet, in filename
// order, each inside its own transaction.
//
// Only one process migrates at a time. Two that start together both read
// schema_migrations before either writes to it, so both decide the same file
// still needs applying and the second one fails on whatever the first created —
// a duplicate type, a duplicate extension, a column that already exists. That
// is not only a test-suite problem: it is what two API containers starting at
// once would do to each other. An advisory lock is the right shape for it,
// because the work being serialised is a whole sequence of transactions rather
// than one, and the lock is released whatever happens to the connection.
//
// Note: pgx sends statements without bind parameters over the simple protocol,
// which is what lets a migration file contain several statements at once.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire a connection to migrate on: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `select pg_advisory_lock($1)`, migrationLock); err != nil {
		return fmt.Errorf("take the migration lock: %w", err)
	}
	defer func() {
		// A fresh context: ctx may already be cancelled by the time this runs,
		// and an unreleased lock would hold up the next process to start.
		unlock, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if _, err := conn.Exec(unlock, `select pg_advisory_unlock($1)`, migrationLock); err != nil {
			log.Printf("warning: could not release the migration lock: %v", err)
		}
	}()

	// Everything below runs on the pool rather than the locked connection: the
	// lock is held by that session for as long as it is checked out, and the
	// work itself does not care which connection it travels on.
	_, err = pool.Exec(ctx, `create table if not exists schema_migrations (
		version    text primary key,
		applied_at timestamptz not null default now()
	)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var applied bool
		err := pool.QueryRow(ctx,
			`select exists(select 1 from schema_migrations where version = $1)`, name).Scan(&applied)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		body, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %s failed: %w", name, err)
		}
		if _, err := tx.Exec(ctx,
			`insert into schema_migrations (version) values ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
		log.Printf("migration applied: %s", name)
	}
	return nil
}
