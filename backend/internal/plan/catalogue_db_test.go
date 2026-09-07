package plan

import (
	"context"
	"os"
	"testing"

	"calisthenics/api/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The projection is the one part of the catalogue the compiler cannot check.
// It shipped broken: every array column is NOT NULL, a nil Go slice encodes as
// NULL, and the first goal with nothing to maintain aborted the transaction —
// so all eleven tables stayed empty, /skills answered with an empty catalogue,
// and the picker had nothing to show. Nothing failed loudly, because a failed
// projection warns and lets the app serve.
//
// This runs against TEST_DATABASE_URL and skips without it, like the other
// database tests here.
func TestSyncCataloguePopulatesEveryTable(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run the catalogue projection test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Twice, because it is a boot-time rewrite: the second run has to clear
	// what the first wrote rather than colliding with it.
	for run := 1; run <= 2; run++ {
		if err := SyncCatalogue(ctx, pool); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
	}

	for _, table := range []string{
		"skills", "skill_steps", "skill_requirements", "injury_regions", "protocols",
		"equipment", "exercise_equipment", "exercise_regions", "category_regions",
		"exercise_substitutes", "level_rubrics",
	} {
		var rows int
		if err := pool.QueryRow(ctx, "select count(*) from "+table).Scan(&rows); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if rows == 0 {
			t.Errorf("%s is empty after the projection ran", table)
		}
	}

	var skills int
	if err := pool.QueryRow(ctx, "select count(*) from skills").Scan(&skills); err != nil {
		t.Fatalf("count skills: %v", err)
	}
	if skills != len(Goals) {
		t.Errorf("projected %d skills, the catalogue has %d", skills, len(Goals))
	}

	// No skill is free. A zero here under-counts the tendon budget the picker
	// exists to show, and reads as "0 units" beside the skill's name.
	var unpriced int
	if err := pool.QueryRow(ctx, "select count(*) from skills where cost < 1").Scan(&unpriced); err != nil {
		t.Fatalf("count unpriced: %v", err)
	}
	if unpriced != 0 {
		t.Errorf("%d skills projected a cost below one unit", unpriced)
	}
}
