package ai

import (
	"context"
	"os"
	"testing"
	"time"

	"calisthenics/api/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEstimateUsesEachModelsOwnRate(t *testing.T) {
	// A million in and a million out, so the arithmetic is the rate itself.
	cost, ok := estimate("claude-sonnet-5", 1_000_000, 1_000_000)
	if !ok {
		t.Fatal("a model in the picker should have a price")
	}
	if cost != 12 {
		t.Fatalf("cost = %v, want 2 + 10", cost)
	}

	// A model this build has never heard of must produce no estimate rather
	// than a wrong one.
	if _, ok := estimate("some-model-from-2028", 1_000_000, 1_000_000); ok {
		t.Fatal("an unlisted model was given a price")
	}
}

// A third of a cent is the normal size of one review. Rounding it to $0.00
// would tell an athlete the opposite of the truth.
func TestMoneyDoesNotRoundSmallSpendToNothing(t *testing.T) {
	if got := money(0.003); got != "under $0.01" {
		t.Fatalf("money(0.003) = %q", got)
	}
	if got := money(0); got != "$0.00" {
		t.Fatalf("money(0) = %q, want a plain zero when nothing was spent", got)
	}
	if got := money(1.2345); got != "$1.23" {
		t.Fatalf("money(1.2345) = %q", got)
	}
}

func usageHandler(t *testing.T) (*Handler, *pgxpool.Pool, string) {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run the usage tests")
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

	var userID string
	if err := pool.QueryRow(ctx, `
		insert into users (email, password_hash, display_name)
		values (gen_random_uuid()::text || '@test.invalid', 'x', 'Usage')
		returning id`).Scan(&userID); err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `delete from users where id = $1`, userID)
	})

	return NewHandler(NewStore(pool, nil, Settings{}), pool, nil), pool, userID
}

func record(t *testing.T, pool *pgxpool.Pool, userID, model string, in, out int, when time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		insert into ai_calls (user_id, purpose, provider, model, input_tokens, output_tokens, created_at)
		values ($1, 'test', 'anthropic', $2, $3, $4, $5)`,
		userID, model, in, out, when)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
}

func TestUsageAddsUpOneAthletesOwnCallsOnly(t *testing.T) {
	h, pool, userID := usageHandler(t)
	ctx := context.Background()

	// Somebody else's spend must never appear in this athlete's total.
	var other string
	if err := pool.QueryRow(ctx, `
		insert into users (email, password_hash, display_name)
		values (gen_random_uuid()::text || '@test.invalid', 'x', 'Other')
		returning id`).Scan(&other); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `delete from users where id = $1`, other) })
	record(t, pool, other, "claude-sonnet-5", 1_000_000, 1_000_000, time.Now())

	now := time.Now().UTC()
	record(t, pool, userID, "claude-sonnet-5", 1_000_000, 100_000, now)  // $2.00 + $1.00
	record(t, pool, userID, "claude-haiku-4-5", 1_000_000, 100_000, now) // $1.00 + $0.50
	// Last month, so it counts towards the total but not the month.
	record(t, pool, userID, "claude-sonnet-5", 1_000_000, 0, now.AddDate(0, -2, 0))

	month, err := h.usageSince(ctx, userID, startOfMonth())
	if err != nil {
		t.Fatalf("usageSince: %v", err)
	}
	if month.Calls != 2 {
		t.Fatalf("this month = %d calls, want the two from this month", month.Calls)
	}
	if month.Estimated != "$4.50" {
		t.Fatalf("this month = %s, want $4.50", month.Estimated)
	}
	if !month.Priced {
		t.Fatal("every model this month has a price, so the figure is complete")
	}

	total, err := h.usageSince(ctx, userID, time.Time{})
	if err != nil {
		t.Fatalf("usageSince: %v", err)
	}
	if total.Calls != 3 {
		t.Fatalf("total = %d calls, want all three of this athlete's", total.Calls)
	}
	if total.Estimated != "$6.50" {
		t.Fatalf("total = %s, want $6.50", total.Estimated)
	}
}

// A model with no published price still happened: it counts as a call and as
// tokens, and the window says the money figure is incomplete.
func TestUsageFlagsAnIncompleteEstimate(t *testing.T) {
	h, pool, userID := usageHandler(t)

	now := time.Now().UTC()
	record(t, pool, userID, "claude-sonnet-5", 1_000_000, 0, now)
	record(t, pool, userID, "some-model-from-2028", 5_000_000, 5_000_000, now)

	month, err := h.usageSince(context.Background(), userID, startOfMonth())
	if err != nil {
		t.Fatalf("usageSince: %v", err)
	}
	if month.Calls != 2 || month.InputTokens != 6_000_000 {
		t.Fatalf("window = %+v, want both calls and all the tokens", month)
	}
	if month.Estimated != "$2.00" {
		t.Fatalf("estimate = %s, want only the priced model's share", month.Estimated)
	}
	if month.Priced {
		t.Fatal("an unpriced model in the window must mark the figure incomplete")
	}
}
