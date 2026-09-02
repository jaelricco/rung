package secret

import (
	"context"
	"encoding/base64"
	"os"
	"sync"
	"testing"

	"calisthenics/api/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The sealing key is the difference between an athlete's stored API key being
// readable and being ciphertext, so where it comes from is worth a test with a
// real database behind it. Runs against TEST_DATABASE_URL, skips without it.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run the sealing key tests")
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
	// Start from nothing, so the first call in each test really is the first.
	if _, err := pool.Exec(ctx, `delete from server_secrets where name = $1`, sealingKeyName); err != nil {
		t.Fatalf("reset: %v", err)
	}
	return pool
}

// A key generated on first boot has to be the same key on every boot after,
// or every stored credential becomes unreadable on restart.
func TestGeneratedKeyIsKeptAndReused(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	first, generated, err := Keystore(ctx, pool, "")
	if err != nil {
		t.Fatalf("Keystore: %v", err)
	}
	if !generated {
		t.Fatal("with no key configured, the server should have made one")
	}

	sealed, err := first.Seal("sk-ant-api03-secret")
	if err != nil {
		t.Fatal(err)
	}

	// A second boot, as far as this package is concerned.
	second, _, err := Keystore(ctx, pool, "")
	if err != nil {
		t.Fatalf("Keystore on restart: %v", err)
	}
	opened, err := second.Open(sealed)
	if err != nil {
		t.Fatalf("a restart could not read what the first boot sealed: %v", err)
	}
	if opened != "sk-ant-api03-secret" {
		t.Fatalf("opened %q", opened)
	}
}

// An operator who does set a key keeps the stronger arrangement: it is used,
// and nothing is written to the database.
func TestConfiguredKeyWinsAndIsNotStored(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	box, generated, err := Keystore(ctx, pool, base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatalf("Keystore: %v", err)
	}
	if generated {
		t.Fatal("a configured key should not be reported as generated")
	}
	if box == nil {
		t.Fatal("no box")
	}

	var rows int
	_ = pool.QueryRow(ctx,
		`select count(*) from server_secrets where name = $1`, sealingKeyName).Scan(&rows)
	if rows != 0 {
		t.Fatal("a configured key was written to the database anyway")
	}
}

// Two API containers starting at once must agree on one key. If they each
// stored their own, half the athletes' credentials would stop opening.
func TestConcurrentFirstBootsAgreeOnOneKey(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	const boots = 6
	sealedBy := make([]*Box, boots)
	var wait sync.WaitGroup
	for i := range boots {
		wait.Add(1)
		go func() {
			defer wait.Done()
			box, _, err := Keystore(ctx, pool, "")
			if err != nil {
				t.Errorf("Keystore: %v", err)
				return
			}
			sealedBy[i] = box
		}()
	}
	wait.Wait()

	var keys int
	_ = pool.QueryRow(ctx,
		`select count(*) from server_secrets where name = $1`, sealingKeyName).Scan(&keys)
	if keys != 1 {
		t.Fatalf("%d keys stored, want exactly one", keys)
	}

	// Every boot must be able to open what any other one sealed.
	sealed, err := sealedBy[0].Seal("shared")
	if err != nil {
		t.Fatal(err)
	}
	for i, box := range sealedBy {
		if box == nil {
			t.Fatalf("boot %d produced no box", i)
		}
		if _, err := box.Open(sealed); err != nil {
			t.Fatalf("boot %d could not open what boot 0 sealed: %v", i, err)
		}
	}
}
