package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

// oidcServer stands in for an identity provider: it publishes a discovery
// document pointing at its own endpoints, which is exactly how the real ones
// are found.
func oidcServer(t *testing.T, authMethod string, handler http.HandlerFunc) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var hits atomic.Int32
	mux := http.NewServeMux()
	server := httptest.NewUnstartedServer(mux)

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                server.URL,
			"authorization_endpoint":                server.URL + "/authorize",
			"token_endpoint":                        server.URL + "/token",
			"userinfo_endpoint":                     server.URL + "/userinfo",
			"token_endpoint_auth_methods_supported": []string{authMethod},
		})
	})
	if handler != nil {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			handler(w, r)
		})
	}

	server.Start()
	t.Cleanup(server.Close)
	return server, &hits
}

func testService() *Service {
	return New(nil, true, OAuthConfig{
		GoogleClientID:     "client-id",
		GoogleClientSecret: "client-secret",
		AppOrigin:          "https://training.example.com",
	})
}

func serviceAt(issuer string) *Service {
	s := testService()
	s.issuerOverride = issuer
	return s
}

func TestStartSendsThePersonToTheProviderWithPKCE(t *testing.T) {
	provider, _ := oidcServer(t, "client_secret_post", nil)
	s := serviceAt(provider.URL)

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
	// The authorize endpoint came from the discovery document, not from a
	// constant in this package.
	if got := target.Scheme + "://" + target.Host + target.Path; got != provider.URL+"/authorize" {
		t.Fatalf("redirected to %q, want the discovered authorize endpoint", got)
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

// Discovery is a network call in the middle of a sign-in, so it is asked once
// per issuer and remembered.
func TestEndpointsAreDiscoveredOnceAndReused(t *testing.T) {
	provider, hits := oidcServer(t, "client_secret_post", nil)
	s := serviceAt(provider.URL)

	for range 3 {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/google/start", nil)
		r.SetPathValue("provider", "google")
		s.OAuthStart(w, r)
		if w.Code != http.StatusFound {
			t.Fatalf("status = %d, want a redirect", w.Code)
		}
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("the discovery document was fetched %d times, want once", got)
	}
}

// An issuer that cannot be reached must fail the sign-in with something
// readable, not a blank redirect to nowhere.
func TestStartFailsReadablyWhenDiscoveryDoesNot(t *testing.T) {
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer broken.Close()

	s := serviceAt(broken.URL)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/google/start", nil)
	r.SetPathValue("provider", "google")

	s.OAuthStart(w, r)

	location := w.Header().Get("Location")
	if !strings.HasPrefix(location, "/login?error=") || !strings.Contains(location, "endpoints") {
		t.Fatalf("Location = %q, want the login page naming the problem", location)
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
// walked into this site. A callback carrying no cookie must not touch the
// provider at all — not even to look up where its endpoints are.
func TestCallbackRefusesAMissingOrForgedState(t *testing.T) {
	provider, hits := oidcServer(t, "client_secret_post", func(w http.ResponseWriter, r *http.Request) {})

	for _, tc := range []struct{ name, cookie, state string }{
		{"no cookie at all", "", "anything"},
		{"a state the cookie never saw", "expected:verifier", "forged"},
		{"no state on the callback", "expected:verifier", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := serviceAt(provider.URL)

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
		})
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("a callback that failed its state check still made %d requests to the provider", got)
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
	provider, _ := oidcServer(t, "client_secret_post", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "at"})
		case "/userinfo":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sub": "google-1", "email": "someone@example.com", "email_verified": false,
			})
		}
	})

	s := serviceAt(provider.URL)
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
	provider, _ := oidcServer(t, "client_secret_post", func(w http.ResponseWriter, r *http.Request) {
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
	})

	s := serviceAt(provider.URL)
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

// How the secret is presented is the issuer's choice. An issuer that does not
// advertise client_secret_post gets HTTP Basic, which is OIDC's default —
// sending it the other way is a 401 with nothing in it to debug.
func TestExchangeFollowsTheIssuersAuthMethod(t *testing.T) {
	var basicUser, basicPass string
	var hadBasic bool
	var bodySecret string

	provider, _ := oidcServer(t, "client_secret_basic", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			basicUser, basicPass, hadBasic = r.BasicAuth()
			_ = r.ParseForm()
			bodySecret = r.PostForm.Get("client_secret")
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "at"})
		case "/userinfo":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sub": "s", "email": "a@example.com", "email_verified": true,
			})
		}
	})

	s := serviceAt(provider.URL)
	p, _ := s.provider("google")
	if _, err := s.exchange(t.Context(), p, "code", "verifier"); err != nil {
		t.Fatalf("exchange: %v", err)
	}

	if !hadBasic {
		t.Fatal("the secret was not sent as HTTP Basic")
	}
	if basicUser != "client-id" || basicPass != "client-secret" {
		t.Fatalf("basic credentials = %q / %q", basicUser, basicPass)
	}
	if bodySecret != "" {
		t.Fatal("the secret was also put in the body, which the issuer did not ask for")
	}
}

func TestProvidersAreOfferedOnlyWhenConfigured(t *testing.T) {
	if got := len(testService().providers()); got != 1 {
		t.Fatalf("configured Google should be offered once, got %d", got)
	}

	bare := New(nil, true, OAuthConfig{AppOrigin: "https://training.example.com"})
	if len(bare.providers()) != 0 {
		t.Fatal("a provider with no client credentials must not be offered")
	}

	both := New(nil, true, OAuthConfig{
		GoogleClientID: "g", GoogleClientSecret: "gs",
		ChatGPTClientID: "c", ChatGPTClientSecret: "cs",
		AppOrigin: "https://training.example.com",
	})
	offered := both.providers()
	if len(offered) != 2 {
		t.Fatalf("both configured providers should be offered, got %d", len(offered))
	}
	chatgpt, ok := both.provider("chatgpt")
	if !ok {
		t.Fatal("ChatGPT was configured but is not offered")
	}
	if chatgpt.issuer != chatGPTIssuer {
		t.Fatalf("ChatGPT issuer = %q, want the default", chatgpt.issuer)
	}

	// The issuer is the one detail most likely to be documented in one place
	// and changed in another, so it has to be settable without a code change.
	moved := New(nil, true, OAuthConfig{
		ChatGPTClientID: "c", ChatGPTClientSecret: "cs",
		ChatGPTIssuer: "https://id.openai.example/",
		AppOrigin:     "https://training.example.com",
	})
	p, _ := moved.provider("chatgpt")
	if p.issuer != "https://id.openai.example" {
		t.Fatalf("issuer override = %q, want the configured one without its trailing slash", p.issuer)
	}
}
