package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"calisthenics/api/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Which account a sign-in lands on is the whole point, and it is decided by
// SQL. These run against TEST_DATABASE_URL and skip without it, so CI stays
// green on a machine with no Postgres.
func testServiceDB(t *testing.T, identities map[string]any) (*Service, *pgxpool.Pool) {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run the sign-in linking tests")
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

	// identities is keyed by authorization code, so one fake provider can
	// answer for several different people.
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_ = r.ParseForm()
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": r.PostForm.Get("code")})
		case "/userinfo":
			code := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			who, ok := identities[code]
			if !ok {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(who)
		}
	}))
	t.Cleanup(provider.Close)

	s := New(pool, false, OAuthConfig{
		GoogleClientID:     "client-id",
		GoogleClientSecret: "client-secret",
		AppOrigin:          "https://training.example.com",
	})
	s.endpointBase = provider.URL
	return s, pool
}

// signIn drives one whole callback and reports where it sent the browser.
func signIn(t *testing.T, s *Service, code string, sessionCookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/google/callback?code="+code+"&state=st", nil)
	r.SetPathValue("provider", "google")
	r.AddCookie(&http.Cookie{Name: oauthCookie, Value: "st:verifier"})
	if sessionCookie != nil {
		r.AddCookie(sessionCookie)
	}
	w := httptest.NewRecorder()
	s.OAuthCallback(w, r)
	return w
}

func sessionCookieFrom(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == cookieName && c.Value != "" {
			return c
		}
	}
	t.Fatalf("no session cookie was issued; redirect was %q", w.Header().Get("Location"))
	return nil
}

func cleanupUser(t *testing.T, pool *pgxpool.Pool, email string) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `delete from users where lower(email) = lower($1)`, email)
	})
}

func TestSigningInTwiceLandsOnOneAccount(t *testing.T) {
	const email = "oauth-once@test.invalid"
	s, pool := testServiceDB(t, map[string]any{
		"code-1": map[string]any{"sub": "google-1", "email": email, "email_verified": true, "name": "Ada"},
	})
	cleanupUser(t, pool, email)

	first := signIn(t, s, "code-1", nil)
	if got := first.Header().Get("Location"); got != "/" {
		t.Fatalf("first sign-in went to %q, want the app", got)
	}
	sessionCookieFrom(t, first)

	var name string
	if err := pool.QueryRow(context.Background(),
		`select display_name from users where lower(email) = lower($1)`, email).Scan(&name); err != nil {
		t.Fatalf("the account was not created: %v", err)
	}
	if name != "Ada" {
		t.Fatalf("display name = %q, want the provider's", name)
	}

	// Signing in again is the same person, not a second account.
	sessionCookieFrom(t, signIn(t, s, "code-1", nil))

	var accounts int
	_ = pool.QueryRow(context.Background(),
		`select count(*) from users where lower(email) = lower($1)`, email).Scan(&accounts)
	if accounts != 1 {
		t.Fatalf("%d accounts exist for one person", accounts)
	}
}

// A verified address that already has a password account here is the same
// person: link, rather than colliding with the unique email index.
func TestSigningInAdoptsAnExistingAccountWithThatVerifiedEmail(t *testing.T) {
	const email = "oauth-existing@test.invalid"
	s, pool := testServiceDB(t, map[string]any{
		"code-1": map[string]any{"sub": "google-2", "email": email, "email_verified": true, "name": "Grace"},
	})
	cleanupUser(t, pool, email)

	var existing string
	if err := pool.QueryRow(context.Background(), `
		insert into users (email, password_hash, display_name)
		values ($1, 'argon2-hash', 'Existing') returning id`, email).Scan(&existing); err != nil {
		t.Fatalf("seed: %v", err)
	}

	sessionCookieFrom(t, signIn(t, s, "code-1", nil))

	var linked string
	if err := pool.QueryRow(context.Background(),
		`select user_id from user_identities where provider = 'google' and subject = 'google-2'`).
		Scan(&linked); err != nil {
		t.Fatalf("no identity was linked: %v", err)
	}
	if linked != existing {
		t.Fatal("the sign-in made a second account instead of linking the existing one")
	}
	// Linking must not overwrite the name the athlete already chose.
	var name string
	_ = pool.QueryRow(context.Background(), `select display_name from users where id = $1`, existing).Scan(&name)
	if name != "Existing" {
		t.Fatalf("display name = %q, want the account's own", name)
	}
}

// Starting the flow while signed in links the provider to that account rather
// than signing the person into a different one.
func TestSigningInWhileSignedInLinksToThatAccount(t *testing.T) {
	const first = "oauth-link-a@test.invalid"
	const second = "oauth-link-b@test.invalid"
	s, pool := testServiceDB(t, map[string]any{
		"code-a": map[string]any{"sub": "google-a", "email": first, "email_verified": true, "name": "A"},
		"code-b": map[string]any{"sub": "google-b", "email": second, "email_verified": true, "name": "B"},
	})
	cleanupUser(t, pool, first)
	cleanupUser(t, pool, second)

	session := sessionCookieFrom(t, signIn(t, s, "code-a", nil))

	// A second, different Google account, linked while signed in as the first.
	linking := signIn(t, s, "code-b", session)
	if got := linking.Header().Get("Location"); got != "/settings" {
		t.Fatalf("linking went to %q, want the settings page", got)
	}

	var owner, subjects int
	_ = pool.QueryRow(context.Background(), `
		select count(distinct user_id), count(*) from user_identities
		where subject in ('google-a', 'google-b')`).Scan(&owner, &subjects)
	if owner != 1 || subjects != 2 {
		t.Fatalf("%d identities across %d accounts, want both on one", subjects, owner)
	}
	// The second address must not have become an account of its own.
	var strays int
	_ = pool.QueryRow(context.Background(),
		`select count(*) from users where lower(email) = lower($1)`, second).Scan(&strays)
	if strays != 0 {
		t.Fatal("linking created a second account anyway")
	}
}

// Unlinking the only way into an account would lock the athlete out of it.
func TestUnlinkRefusesToRemoveTheLastWayIn(t *testing.T) {
	const email = "oauth-lockout@test.invalid"
	s, pool := testServiceDB(t, map[string]any{
		"code-1": map[string]any{"sub": "google-lock", "email": email, "email_verified": true, "name": "Solo"},
	})
	cleanupUser(t, pool, email)

	sessionCookieFrom(t, signIn(t, s, "code-1", nil))

	var user User
	if err := pool.QueryRow(context.Background(),
		`select id, email, display_name, bodyweight_kg from users where lower(email) = lower($1)`, email).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.BodyweightKg); err != nil {
		t.Fatalf("read back: %v", err)
	}

	unlink := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodDelete, "/api/v1/me/logins/google", nil)
		r.SetPathValue("provider", "google")
		r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, user))
		w := httptest.NewRecorder()
		s.Unlink(w, r)
		return w
	}

	if got := unlink().Code; got != http.StatusConflict {
		t.Fatalf("status = %d, want 409: this is the only way in", got)
	}

	// With a password set, the same unlink is safe and must go through.
	hash, err := HashPassword("a-long-enough-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(),
		`update users set password_hash = $2 where id = $1`, user.ID, hash); err != nil {
		t.Fatal(err)
	}
	if got := unlink().Code; got != http.StatusOK {
		t.Fatalf("status = %d, want 200 once a password exists", got)
	}
}
