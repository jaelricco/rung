package training

import (
	"context"
	"os"
	"testing"

	"calisthenics/api/internal/auth"
	"calisthenics/api/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The snapshot is what the planner sees, and everything the athlete tells us
// about themselves has to survive the trip into it. Learning did not: the
// picker stored the skills, the tendon budget counted them, and the query in
// between selected every other baseline column but that one — so the budget
// weighed one skill instead of three and never named anything to park. The
// planner's own tests set Learning directly, which is why nothing caught it.
//
// Runs against TEST_DATABASE_URL and skips without it.
func TestSnapshotCarriesWhatTheBaselineStored(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run the snapshot test")
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

	var id string
	if err := pool.QueryRow(ctx, `
		insert into users (email, password_hash, display_name, trains_per_week,
		                   sleep_hours, equipment, learning)
		values ($1, 'x', 'Snapshot', 4, 7, $2, $3)
		returning id`,
		"snapshot+"+t.Name()+"@example.test",
		[]string{"pull_up_bar", "parallettes"},
		[]string{"maltese", "iron_cross", "planche_press"},
	).Scan(&id); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "delete from users where id = $1", id) })

	snap, err := New(pool).BuildSnapshot(ctx, auth.User{ID: id})
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}

	if got := len(snap.Learning); got != 3 {
		t.Errorf("snapshot carries %d skills being learned, stored 3: %v", got, snap.Learning)
	}
	if len(snap.Equipment) != 2 {
		t.Errorf("snapshot carries %d pieces of equipment, stored 2", len(snap.Equipment))
	}
	if snap.TrainsPerWeek == nil || *snap.TrainsPerWeek != 4 {
		t.Errorf("snapshot lost trains_per_week: %v", snap.TrainsPerWeek)
	}
	if snap.SleepHours == nil || *snap.SleepHours != 7 {
		t.Errorf("snapshot lost sleep_hours: %v", snap.SleepHours)
	}
}
