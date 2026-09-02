package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func testService() *Service {
	return New(nil, true, OAuthConfig{
		GoogleClientID:     "client-id",
		GoogleClientSecret: "client-secret",
		AppOrigin:          "https://training.example.com",
	})
}

func TestStartSendsThePersonToTheProviderWithPKCE(t *testing.T) {
	s := testService()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/google/start", nil)
	r.SetPathValue("provider", "google")

	s.OAuthStart(w, r)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want a redirect", w.Code)
	}
	target, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("Location is not a URL: %v", err)
	}
	query := target.Query()
	if got := query.Get("redirect_uri"); got != "https://training.example.com/api/v1/auth/oauth/google/callback" {
		t.Fatalf("redirect_uri = %q", got)
	}
	if query.Get("client_id") != "client-id" || query.Get("response_type") != "code" {
		t.Fatalf("query = %v", query)
	}
	if query.Get("code_challenge_method") != "S256" {
		t.Fatal("the flow must use PKCE with S256")
	}

	// The cookie is the only copy of the verifier, and the challenge sent to
	// the provider has to be its hash — otherwise the exchange cannot work.
	var stored string
	for _, c := range w.Result().Cookies() {
		if c.Name == oauthCookie {
			stored = c.Value
			if !c.HttpOnly || !c.Secure {
				t.Error("the state cookie must be HttpOnly and Secure")
			}
			if c.SameSite != http.SameSiteLaxMode {
				t.Error("SameSite=Lax: the provider returns through a top-level GET")
			}
		}
	}
	state, verifier, found := strings.Cut(stored, ":")
	if !found || state == "" || verifier == "" {
		t.Fatalf("cookie = %q, want state:verifier", stored)
	}
	if state != query.Get("state") {
		t.Fatal("the state in the URL does not match the one in the cookie")
	}
	sum := sha256.Sum256([]byte(verifier))
	if query.Get("code_challenge") != base64.RawURLEncoding.EncodeToString(sum[:]) {
		t.Fatal("the challenge is not the hash of the stored verifier")
	}
}

func TestStartRefusesAnUnknownProvider(t *testing.T) {
	s := testService()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/facebook/start", nil)
	r.SetPathValue("provider", "facebook")

	s.OAuthStart(w, r)
	if !strings.HasPrefix(w.Header().Get("Location"), "/login?error=") {
		t.Fatalf("Location = %q, want the login page with a reason", w.Header().Get("Location"))
	}
}

// The state check is what stops someone else's authorization code from being
// walked into this site. A callback carrying no cookie must not reach the
// provider at all.
func TestCallbackRefusesAMissingOrForgedState(t *testing.T) {
	reached := false
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
	}))
	defer provider.Close()

	for _, tc := range []struct{ name, cookie, state string }{
		{"no cookie at all", "", "anything"},
		{"a state the cookie never saw", "expected:verifier", "forged"},
		{"no state on the callback", "expected:verifier", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testService()
			s.endpointBase = provider.URL

			r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/google/callback?code=abc&state="+tc.state, nil)
			r.SetPathValue("provider", "google")
			if tc.cookie != "" {
				r.AddCookie(&http.Cookie{Name: oauthCookie, Value: tc.cookie})
			}
			w := httptest.NewRecorder()

			s.OAuthCallback(w, r)

			if !strings.HasPrefix(w.Header().Get("Location"), "/login?error=") {
				t.Fatalf("Location = %q, want the login page with a reason", w.Header().Get("Location"))
			}
			if reached {
				t.Fatal("a callback that failed its state check still called the provider")
			}
		})
	}
}

// Someone who presses Cancel at the provider has not hit an error; they have
// changed their mind, and the login page should just be the login page.
func TestCallbackTreatsACancelledSignInQuietly(t *testing.T) {
	s := testService()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/google/callback?error=access_denied", nil)
	r.SetPathValue("provider", "google")
	w := httptest.NewRecorder()

	s.OAuthCallback(w, r)

	if got := w.Header().Get("Location"); got != "/login" {
		t.Fatalf("Location = %q, want a plain /login", got)
	}
}

// An unverified address at the provider would let anyone who can claim it walk
// into an account here that already uses it.
func TestCallbackRefusesAnUnverifiedEmail(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "at"})
		case "/userinfo":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sub": "google-1", "email": "someone@example.com", "email_verified": false,
			})
		}
	}))
	defer provider.Close()

	s := testService()
	s.endpointBase = provider.URL

	r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/google/callback?code=abc&state=st", nil)
	r.SetPathValue("provider", "google")
	r.AddCookie(&http.Cookie{Name: oauthCookie, Value: "st:verifier"})
	w := httptest.NewRecorder()

	s.OAuthCallback(w, r)

	location := w.Header().Get("Location")
	if !strings.Contains(location, "verified") {
		t.Fatalf("Location = %q, want a reason naming the unverified address", location)
	}
}

// The exchange has to send the verifier back, or PKCE protects nothing.
func TestExchangeSendsTheVerifierAndTheSecret(t *testing.T) {
	var form url.Values
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_ = r.ParseForm()
			form = r.PostForm
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "at"})
		case "/userinfo":
			if r.Header.Get("Authorization") != "Bearer at" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sub": "google-1", "email": "a@example.com", "email_verified": true, "name": "A",
			})
		}
	}))
	defer provider.Close()

	s := testService()
	s.endpointBase = provider.URL
	p, _ := s.provider("google")

	info, err := s.exchange(t.Context(), p, "the-code", "the-verifier")
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if info.Subject != "google-1" || !info.EmailVerified {
		t.Fatalf("identity = %+v", info)
	}
	for key, want := range map[string]string{
		"grant_type":    "authorization_code",
		"code":          "the-code",
		"code_verifier": "the-verifier",
		"client_secret": "client-secret",
		"redirect_uri":  "https://training.example.com/api/v1/auth/oauth/google/callback",
	} {
		if got := form.Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestProviderIsOfferedOnlyWhenItIsConfigured(t *testing.T) {
	if got := len(testService().providers()); got != 1 {
		t.Fatalf("configured Google should be offered once, got %d", got)
	}
	bare := New(nil, true, OAuthConfig{AppOrigin: "https://training.example.com"})
	if len(bare.providers()) != 0 {
		t.Fatal("a provider with no client credentials must not be offered")
	}
}
