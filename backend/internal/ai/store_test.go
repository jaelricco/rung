package ai

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"calisthenics/api/internal/db"
	"calisthenics/api/internal/secret"

	"github.com/jackc/pgx/v5/pgxpool"
)

// These exercise the part of the change a unit test cannot reach: the
// migration, the sealed column and the queries over it. They run against
// TEST_DATABASE_URL and skip without it, so CI stays green on a machine with
// no Postgres.
func testStore(t *testing.T, provider http.Handler) (*Store, *pgxpool.Pool, string) {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run the credential store tests")
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
	err = pool.QueryRow(ctx, `
		insert into users (email, password_hash, display_name)
		values (gen_random_uuid()::text || '@test.invalid', 'x', 'Test')
		returning id`).Scan(&userID)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `delete from users where id = $1`, userID)
	})

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	box, err := secret.New(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatalf("keystore: %v", err)
	}

	server := httptest.NewServer(provider)
	t.Cleanup(server.Close)

	store := NewStore(pool, box, Settings{Thinking: "adaptive"})
	store.baseURL = server.URL
	return store, pool, userID
}

// happyProvider answers the verification lookup and then one short stream.
func happyProvider() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"id":"claude-sonnet-5","type":"model"}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: content_block_delta\n" +
			`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"ok"}}` + "\n\n"))
	})
}

func TestConnectStoresASealedKeyAndHandsBackAWorkingClient(t *testing.T) {
	store, pool, userID := testStore(t, happyProvider())
	ctx := context.Background()

	conn, err := store.Connect(ctx, userID, ProviderAnthropic, "sk-ant-api03-secret-key-9876", "claude-sonnet-5")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if conn.KeyHint != "9876" || conn.Model != "claude-sonnet-5" {
		t.Fatalf("connection = %+v", conn)
	}

	// What lands in the table must not be the key.
	var sealed []byte
	if err := pool.QueryRow(ctx,
		`select key_sealed from user_ai_credentials where user_id = $1`, userID).Scan(&sealed); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if strings.Contains(string(sealed), "secret-key") {
		t.Fatal("the key was stored in the clear")
	}

	client, err := store.ClientFor(ctx, userID)
	if err != nil {
		t.Fatalf("ClientFor: %v", err)
	}
	out, err := client.CompleteStream(ctx, userID, "test", "sys", "prompt", 500, nil)
	if err != nil || out != "ok" {
		t.Fatalf("call on the stored key: %q, %v", out, err)
	}

	// The call is billed to this athlete, so it is recorded against them.
	var provider, model string
	if err := pool.QueryRow(ctx,
		`select provider, model from ai_calls where user_id = $1`, userID).Scan(&provider, &model); err != nil {
		t.Fatalf("ai_calls: %v", err)
	}
	if provider != ProviderAnthropic || model != "claude-sonnet-5" {
		t.Fatalf("recorded %s/%s", provider, model)
	}

	if err := store.Disconnect(ctx, userID); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	if _, err := store.ClientFor(ctx, userID); !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("after disconnect: %v", err)
	}
}

// A key the provider rejects must not be stored: an athlete should find out on
// the settings page, not ten minutes into a plan.
func TestConnectStoresNothingWhenTheProviderRejectsTheKey(t *testing.T) {
	store, _, userID := testStore(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid x-api-key"}}`))
	}))
	ctx := context.Background()

	_, err := store.Connect(ctx, userID, ProviderAnthropic, "sk-ant-api03-wrong", "claude-sonnet-5")
	if err == nil || !strings.Contains(err.Error(), "invalid x-api-key") {
		t.Fatalf("expected the provider's own message, got: %v", err)
	}
	if _, ok, _ := store.Connection(ctx, userID); ok {
		t.Fatal("a rejected key was stored anyway")
	}
}

func TestUseModelKeepsTheKeyAndSwitchesTheModel(t *testing.T) {
	store, _, userID := testStore(t, happyProvider())
	ctx := context.Background()

	if _, err := store.Connect(ctx, userID, ProviderAnthropic, "sk-ant-api03-secret-key-9876", "claude-opus-5"); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	conn, err := store.UseModel(ctx, userID, "claude-sonnet-5")
	if err != nil {
		t.Fatalf("UseModel: %v", err)
	}
	if conn.Model != "claude-sonnet-5" || conn.KeyHint != "9876" {
		t.Fatalf("connection = %+v", conn)
	}
}

// Without a sealing key nothing can be stored, and nothing already stored can
// be read: the failure has to name the server's missing secret rather than
// blame the athlete for not connecting an account.
func TestAStoreWithNoKeystoreRefusesEverything(t *testing.T) {
	store := NewStore(nil, nil, Settings{})
	if store.Ready() {
		t.Fatal("a store with no sealing key called itself ready")
	}
	if _, err := store.ClientFor(context.Background(), "someone"); !errors.Is(err, ErrNoKeystore) {
		t.Fatalf("ClientFor: %v", err)
	}
	if _, err := store.Connect(context.Background(), "someone", ProviderAnthropic, "sk-ant-x", ""); !errors.Is(err, ErrNoKeystore) {
		t.Fatalf("Connect: %v", err)
	}
}

// Switching the connector off has to stop the spending, and it has to do it
// before the key is unsealed — a paused connection should not have its key in
// memory at all. Switching it back on must return the same connection, not
// ask for the key again.
func TestPausingHoldsTheKeyWithoutSpendingIt(t *testing.T) {
	store, pool, userID := testStore(t, happyProvider())
	ctx := context.Background()

	if _, err := store.Connect(ctx, userID, ProviderAnthropic, "sk-ant-api03-secret-key-9876", "claude-sonnet-5"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	off := true
	conn, err := store.SetSwitches(ctx, userID, &off, nil)
	if err != nil {
		t.Fatalf("SetSwitches: %v", err)
	}
	if !conn.Paused {
		t.Fatalf("connection = %+v", conn)
	}

	if _, err := store.ClientFor(ctx, userID); !errors.Is(err, ErrPaused) {
		t.Fatalf("a paused connector still handed out a client: %v", err)
	}
	// Paused is not disconnected: the key is still here.
	var stored int
	if err := pool.QueryRow(ctx,
		`select count(*) from user_ai_credentials where user_id = $1`, userID).Scan(&stored); err != nil {
		t.Fatalf("count: %v", err)
	}
	if stored != 1 {
		t.Fatalf("pausing deleted the key (%d rows)", stored)
	}

	on := false
	if _, err := store.SetSwitches(ctx, userID, &on, nil); err != nil {
		t.Fatalf("SetSwitches back on: %v", err)
	}
	if _, err := store.ClientFor(ctx, userID); err != nil {
		t.Fatalf("after switching back on: %v", err)
	}
}

// Each switch moves on its own. Sending one must not quietly reset the other,
// which is the whole reason the request takes pointers.
func TestOneSwitchLeavesTheOtherAlone(t *testing.T) {
	store, _, userID := testStore(t, happyProvider())
	ctx := context.Background()

	if _, err := store.Connect(ctx, userID, ProviderAnthropic, "sk-ant-api03-secret-key-9876", "claude-sonnet-5"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	yes := true
	if _, err := store.SetSwitches(ctx, userID, nil, &yes); err != nil {
		t.Fatalf("set forget: %v", err)
	}
	conn, err := store.SetSwitches(ctx, userID, &yes, nil)
	if err != nil {
		t.Fatalf("set paused: %v", err)
	}
	if !conn.Paused || !conn.ForgetOnLogout {
		t.Fatalf("connection = %+v", conn)
	}

	// Reconnecting is an athlete saying "make this work", so it clears the
	// pause — but it is not a change of mind about the logout preference.
	conn, err = store.Connect(ctx, userID, ProviderAnthropic, "sk-ant-api03-secret-key-9876", "claude-sonnet-5")
	if err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	if conn.Paused {
		t.Fatal("reconnecting left the connector switched off")
	}
	if !conn.ForgetOnLogout {
		t.Fatal("reconnecting forgot the logout preference")
	}
}

// Signing out drops the key only for the athlete who asked for that, and it
// must be harmless for everyone else — it runs on every sign-out.
func TestSignOutForgetsOnlyTheKeysThatAskedToBeForgotten(t *testing.T) {
	store, _, userID := testStore(t, happyProvider())
	ctx := context.Background()

	if _, err := store.Connect(ctx, userID, ProviderAnthropic, "sk-ant-api03-secret-key-9876", "claude-sonnet-5"); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Switch off, not set: signing out leaves it be.
	if err := store.ForgetOnSignOut(ctx, userID); err != nil {
		t.Fatalf("ForgetOnSignOut: %v", err)
	}
	if _, ok, _ := store.Connection(ctx, userID); !ok {
		t.Fatal("a key that had not asked to be forgotten was deleted at sign-out")
	}

	yes := true
	if _, err := store.SetSwitches(ctx, userID, nil, &yes); err != nil {
		t.Fatalf("set forget: %v", err)
	}
	if err := store.ForgetOnSignOut(ctx, userID); err != nil {
		t.Fatalf("ForgetOnSignOut: %v", err)
	}
	if _, ok, _ := store.Connection(ctx, userID); ok {
		t.Fatal("the key survived a sign-out it was meant to be dropped at")
	}
	// And it stays harmless once there is nothing left to forget.
	if err := store.ForgetOnSignOut(ctx, userID); err != nil {
		t.Fatalf("ForgetOnSignOut with no row: %v", err)
	}
}

// Nothing to switch is a plain refusal, not a row conjured into existence.
func TestSwitchesNeedAConnectionToActOn(t *testing.T) {
	store, _, userID := testStore(t, happyProvider())
	yes := true
	if _, err := store.SetSwitches(context.Background(), userID, &yes, nil); !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("SetSwitches with no connection: %v", err)
	}
}
