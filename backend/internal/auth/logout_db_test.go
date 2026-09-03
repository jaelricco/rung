package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"calisthenics/api/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Signing out is where "forget my API key when I sign out" is actually
// honoured, so the hook has to run, has to name the right athlete, and must
// never be able to strand someone in a session they asked to leave.
func TestLogoutRunsTheSignOutHookForTheRightAthlete(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run the sign-out hook test")
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
		values (gen_random_uuid()::text || '@test.invalid', 'x', 'Test')
		returning id`).Scan(&userID); err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `delete from users where id = $1`, userID)
	})

	svc := New(pool, false, OAuthConfig{})

	// A hook that fails is still a successful sign-out: the cookie goes, the
	// session row goes, and the athlete is out.
	var seen string
	svc.OnSignOut(func(_ context.Context, id string) error {
		seen = id
		return errors.New("the key could not be dropped")
	})

	issued := httptest.NewRecorder()
	if err := svc.issueSession(ctx, issued, userID); err != nil {
		t.Fatalf("issueSession: %v", err)
	}
	cookie := issued.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(cookie)
	out := httptest.NewRecorder()
	svc.Logout(out, req)

	if out.Code != http.StatusOK {
		t.Fatalf("logout answered %d despite a failing hook", out.Code)
	}
	if seen != userID {
		t.Fatalf("hook ran for %q, wanted %q", seen, userID)
	}
	var sessions int
	if err := pool.QueryRow(ctx,
		`select count(*) from sessions where user_id = $1`, userID).Scan(&sessions); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if sessions != 0 {
		t.Fatalf("the session survived the sign-out (%d rows)", sessions)
	}
}

// No cookie, no athlete to run it for. The hook must not fire on a stray or
// already-expired sign-out.
func TestLogoutWithoutASessionRunsNoHook(t *testing.T) {
	svc := New(nil, false, OAuthConfig{})
	ran := false
	svc.OnSignOut(func(context.Context, string) error { ran = true; return nil })

	out := httptest.NewRecorder()
	svc.Logout(out, httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil))

	if out.Code != http.StatusOK {
		t.Fatalf("logout answered %d", out.Code)
	}
	if ran {
		t.Fatal("the sign-out hook ran with nobody signed out")
	}
}
