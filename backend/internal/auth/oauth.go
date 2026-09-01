package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"calisthenics/api/internal/httpx"

	"github.com/jackc/pgx/v5"
)

// Signing in with an identity provider answers one question: who is this. It
// is not a way to reach a model. Both Anthropic and OpenAI bill inference to
// an API key and neither lets a third-party site spend a signed-in consumer
// account's plan, so an athlete who signs in with Google still connects their
// own key under Settings. The two are separate on purpose, and the login page
// says so.
//
// Everything here is the plain authorization-code flow with PKCE, written
// against net/http the same way the model transports are: one provider, four
// URLs, no library.

const (
	oauthCookie   = "cali_oauth"
	oauthLifetime = 10 * time.Minute
)

// Provider is one identity provider, as the login page needs to describe it.
type Provider struct {
	ID    string `json:"id"`
	Label string `json:"label"`

	authURL     string
	tokenURL    string
	userInfoURL string
	scopes      []string

	clientID     string
	clientSecret string
}

// OAuthConfig carries the client credentials for whichever providers the
// operator has registered. A provider with no client ID is simply not offered.
//
// Google is the only one here for now, and not for lack of trying: "Sign in
// with ChatGPT" still ships only inside OpenAI's own Codex tooling, with no
// public client registration, and Anthropic offers no consumer sign-in for
// third-party sites at all. Both slot in as another entry in providers() the
// day they open up.
type OAuthConfig struct {
	GoogleClientID     string
	GoogleClientSecret string
	// AppOrigin is the public https origin of this site. The redirect URI is
	// built from it, and it must match what is registered with the provider.
	AppOrigin string
}

func (s *Service) providers() []Provider {
	var out []Provider
	if s.oauth.GoogleClientID != "" && s.oauth.GoogleClientSecret != "" {
		out = append(out, s.at(Provider{
			ID:           "google",
			Label:        "Google",
			authURL:      "https://accounts.google.com/o/oauth2/v2/auth",
			tokenURL:     "https://oauth2.googleapis.com/token",
			userInfoURL:  "https://openidconnect.googleapis.com/v1/userinfo",
			scopes:       []string{"openid", "email", "profile"},
			clientID:     s.oauth.GoogleClientID,
			clientSecret: s.oauth.GoogleClientSecret,
		}))
	}
	return out
}

// at points a provider at another host. Only the tests set endpointBase; in
// production every provider knows its own endpoints.
func (s *Service) at(p Provider) Provider {
	if s.endpointBase == "" {
		return p
	}
	p.authURL = s.endpointBase + "/authorize"
	p.tokenURL = s.endpointBase + "/token"
	p.userInfoURL = s.endpointBase + "/userinfo"
	return p
}

func (s *Service) provider(id string) (Provider, bool) {
	for _, p := range s.providers() {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}

func (s *Service) redirectURI(p Provider) string {
	return strings.TrimSuffix(s.oauth.AppOrigin, "/") + "/api/v1/auth/oauth/" + p.ID + "/callback"
}

// Providers lists what the login page may offer. It is public: the page has to
// know before anyone has signed in.
func (s *Service) Providers(w http.ResponseWriter, r *http.Request) {
	list := s.providers()
	if list == nil {
		list = []Provider{}
	}
	httpx.JSON(w, http.StatusOK, list)
}

// ---------- the flow ----------

func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// OAuthStart sends the browser to the provider. The state and the PKCE
// verifier ride along in a short-lived cookie, which is what the callback
// checks its answer against.
func (s *Service) OAuthStart(w http.ResponseWriter, r *http.Request) {
	p, ok := s.provider(r.PathValue("provider"))
	if !ok {
		s.failLogin(w, r, "That sign-in method isn't available here.")
		return
	}
	if s.oauth.AppOrigin == "" {
		s.failLogin(w, r, "This server has no APP_ORIGIN set, so it cannot complete a sign-in redirect.")
		return
	}

	state, err := randomString(24)
	if err != nil {
		s.failLogin(w, r, "Couldn't start the sign-in. Try again.")
		return
	}
	verifier, err := randomString(48)
	if err != nil {
		s.failLogin(w, r, "Couldn't start the sign-in. Try again.")
		return
	}
	challenge := sha256.Sum256([]byte(verifier))

	http.SetCookie(w, &http.Cookie{
		Name:     oauthCookie,
		Value:    state + ":" + verifier,
		Path:     "/api/v1/auth/oauth",
		MaxAge:   int(oauthLifetime.Seconds()),
		HttpOnly: true,
		Secure:   s.secure,
		// Lax, not Strict: the provider sends the browser back with a
		// top-level GET, which Lax allows and Strict would strip the cookie
		// from — leaving every sign-in to fail its own state check.
		SameSite: http.SameSiteLaxMode,
	})

	query := url.Values{
		"client_id":             {p.clientID},
		"redirect_uri":          {s.redirectURI(p)},
		"response_type":         {"code"},
		"scope":                 {strings.Join(p.scopes, " ")},
		"state":                 {state},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"code_challenge_method": {"S256"},
		// Ask every time rather than silently reusing whichever account the
		// browser happens to be signed in to.
		"prompt": {"select_account"},
	}
	http.Redirect(w, r, p.authURL+"?"+query.Encode(), http.StatusFound)
}

// OAuthCallback finishes the flow. Started while signed in, it links the
// provider to that account; started signed out, it signs in or creates one.
func (s *Service) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	p, ok := s.provider(r.PathValue("provider"))
	if !ok {
		s.failLogin(w, r, "That sign-in method isn't available here.")
		return
	}

	// Whatever happens next, this cookie has done its job.
	stored := ""
	if c, err := r.Cookie(oauthCookie); err == nil {
		stored = c.Value
	}
	http.SetCookie(w, &http.Cookie{
		Name: oauthCookie, Value: "", Path: "/api/v1/auth/oauth", MaxAge: -1,
		HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode,
	})

	// A person who changes their mind at the provider is not an error worth
	// shouting about.
	if reason := r.URL.Query().Get("error"); reason != "" {
		if reason == "access_denied" {
			s.finish(w, r, "/login")
			return
		}
		s.failLogin(w, r, "The sign-in was refused: "+reason)
		return
	}

	state, verifier, found := strings.Cut(stored, ":")
	given := r.URL.Query().Get("state")
	if !found || state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(given)) != 1 {
		s.failLogin(w, r, "That sign-in link has expired. Try again.")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		s.failLogin(w, r, "The provider sent no authorization code back.")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	info, err := s.exchange(ctx, p, code, verifier)
	if err != nil {
		s.failLogin(w, r, err.Error())
		return
	}
	if info.Subject == "" {
		s.failLogin(w, r, "The provider returned no account id.")
		return
	}
	// An unverified address would let anyone who can claim it at the provider
	// walk into an account here that already uses it.
	if !info.EmailVerified || info.Email == "" {
		s.failLogin(w, r, "Your "+p.Label+" account has no verified email address, so it can't be used to sign in here.")
		return
	}

	// Signed in already means this is a link, not a sign-in.
	var current string
	if c, err := r.Cookie(cookieName); err == nil && c.Value != "" {
		if u, err := s.userForToken(ctx, c.Value); err == nil {
			current = u.ID
		}
	}

	user, err := s.resolveIdentity(ctx, p, info, current)
	if err != nil {
		if current != "" {
			s.finish(w, r, "/settings?error="+url.QueryEscape(err.Error()))
			return
		}
		s.failLogin(w, r, err.Error())
		return
	}

	if current != "" {
		s.finish(w, r, "/settings")
		return
	}
	if err := s.issueSession(ctx, w, user.ID); err != nil {
		s.failLogin(w, r, "Signed in with "+p.Label+", but starting the session failed. Try again.")
		return
	}
	s.finish(w, r, "/")
}

// failLogin puts the reason on the login page rather than answering a browser
// redirect with a JSON error nobody will see.
func (s *Service) failLogin(w http.ResponseWriter, r *http.Request, message string) {
	s.finish(w, r, "/login?error="+url.QueryEscape(message))
}

// finish redirects within this site only: the path is always one this code
// wrote, never anything that came in with the request.
func (s *Service) finish(w http.ResponseWriter, r *http.Request, path string) {
	http.Redirect(w, r, path, http.StatusFound)
}

type identity struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

// exchange trades the code for a token and asks who it belongs to. Both calls
// go straight to the provider over TLS, so the answer needs no signature check
// of its own — nothing in between could have written it.
func (s *Service) exchange(ctx context.Context, p Provider, code, verifier string) (identity, error) {
	var info identity

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {s.redirectURI(p)},
		"client_id":     {p.clientID},
		"client_secret": {p.clientSecret},
		"code_verifier": {verifier},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return info, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := s.http.Do(req)
	if err != nil {
		return info, fmt.Errorf("reaching %s failed. Try again", p.Label)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return info, err
	}
	var token struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if err := json.Unmarshal(raw, &token); err != nil || resp.StatusCode != http.StatusOK {
		if token.Description != "" {
			return info, fmt.Errorf("%s refused the sign-in: %s", p.Label, token.Description)
		}
		return info, fmt.Errorf("%s refused the sign-in", p.Label)
	}
	if token.AccessToken == "" {
		return info, fmt.Errorf("%s returned no access token", p.Label)
	}

	userReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.userInfoURL, nil)
	if err != nil {
		return info, err
	}
	userReq.Header.Set("Authorization", "Bearer "+token.AccessToken)
	userReq.Header.Set("Accept", "application/json")

	userResp, err := s.http.Do(userReq)
	if err != nil {
		return info, fmt.Errorf("reaching %s failed. Try again", p.Label)
	}
	defer userResp.Body.Close()

	userRaw, err := io.ReadAll(io.LimitReader(userResp.Body, 1<<20))
	if err != nil {
		return info, err
	}
	if userResp.StatusCode != http.StatusOK {
		return info, fmt.Errorf("%s wouldn't say who you are", p.Label)
	}
	if err := json.Unmarshal(userRaw, &info); err != nil {
		return info, fmt.Errorf("%s sent an answer this server couldn't read", p.Label)
	}
	return info, nil
}

// resolveIdentity finds the account this identity belongs to, linking or
// creating one as needed. It runs in a transaction so two sign-ins racing each
// other cannot make two accounts for one person.
func (s *Service) resolveIdentity(ctx context.Context, p Provider, info identity, currentUserID string) (User, error) {
	var user User

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return user, errors.New("Couldn't complete the sign-in. Try again.")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var linked string
	err = tx.QueryRow(ctx,
		`select user_id from user_identities where provider = $1 and subject = $2`,
		p.ID, info.Subject).Scan(&linked)

	switch {
	case err == nil:
		if currentUserID != "" && linked != currentUserID {
			return user, fmt.Errorf("That %s account is already linked to another account here.", p.Label)
		}
		user, err = loadUser(ctx, tx, linked)

	case errors.Is(err, pgx.ErrNoRows):
		owner := currentUserID
		if owner == "" {
			// A verified address that already has an account here is the same
			// person: link rather than making them a second one.
			err = tx.QueryRow(ctx,
				`select id from users where lower(email) = lower($1)`, info.Email).Scan(&owner)
			if errors.Is(err, pgx.ErrNoRows) {
				owner = ""
			} else if err != nil {
				return user, errors.New("Couldn't complete the sign-in. Try again.")
			}
		}
		if owner == "" {
			name := strings.TrimSpace(info.Name)
			if name == "" {
				name = strings.SplitN(info.Email, "@", 2)[0]
			}
			// No password: this account is reached through its provider.
			err = tx.QueryRow(ctx, `
				insert into users (email, password_hash, display_name)
				values ($1, null, $2) returning id`, info.Email, name).Scan(&owner)
			if err != nil {
				return user, errors.New("Couldn't create an account for you. Try again.")
			}
		}
		if _, err = tx.Exec(ctx, `
			insert into user_identities (provider, subject, user_id, email)
			values ($1, $2, $3, $4)`, p.ID, info.Subject, owner, info.Email); err != nil {
			return user, fmt.Errorf("Couldn't link your %s account. Try again.", p.Label)
		}
		user, err = loadUser(ctx, tx, owner)

	default:
		return user, errors.New("Couldn't complete the sign-in. Try again.")
	}

	if err != nil {
		return user, errors.New("Couldn't complete the sign-in. Try again.")
	}
	if err := tx.Commit(ctx); err != nil {
		return user, errors.New("Couldn't complete the sign-in. Try again.")
	}
	return user, nil
}

func loadUser(ctx context.Context, tx pgx.Tx, id string) (User, error) {
	var u User
	err := tx.QueryRow(ctx,
		`select id, email, display_name, bodyweight_kg from users where id = $1`, id).
		Scan(&u.ID, &u.Email, &u.DisplayName, &u.BodyweightKg)
	return u, err
}

// ---------- what the athlete sees about their own sign-in ----------

type linkedIdentity struct {
	Provider  string    `json:"provider"`
	Label     string    `json:"label"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type loginMethods struct {
	// HasPassword is false for an account that has only ever signed in
	// through a provider.
	HasPassword bool             `json:"has_password"`
	Identities  []linkedIdentity `json:"identities"`
	Available   []Provider       `json:"available"`
}

// LoginMethods reports how this account can be signed in to, and what else it
// could be linked to.
func (s *Service) LoginMethods(w http.ResponseWriter, r *http.Request) {
	me := MustUser(r.Context())

	var hasPassword bool
	if err := s.pool.QueryRow(r.Context(),
		`select password_hash is not null from users where id = $1`, me.ID).Scan(&hasPassword); err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't read your sign-in methods.")
		return
	}

	rows, err := s.pool.Query(r.Context(), `
		select provider, email, created_at from user_identities
		where user_id = $1 order by created_at`, me.ID)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't read your sign-in methods.")
		return
	}
	defer rows.Close()

	out := loginMethods{HasPassword: hasPassword, Identities: []linkedIdentity{}, Available: s.providers()}
	for rows.Next() {
		var it linkedIdentity
		if err := rows.Scan(&it.Provider, &it.Email, &it.CreatedAt); err != nil {
			continue
		}
		if p, ok := s.provider(it.Provider); ok {
			it.Label = p.Label
		} else {
			it.Label = it.Provider
		}
		out.Identities = append(out.Identities, it)
	}
	httpx.JSON(w, http.StatusOK, out)
}

// Unlink removes one provider from this account, unless it is the only way in.
func (s *Service) Unlink(w http.ResponseWriter, r *http.Request) {
	me := MustUser(r.Context())
	provider := r.PathValue("provider")

	var hasPassword bool
	var linked int
	err := s.pool.QueryRow(r.Context(), `
		select (select password_hash is not null from users where id = $1),
		       (select count(*) from user_identities where user_id = $1)`,
		me.ID).Scan(&hasPassword, &linked)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't read your sign-in methods.")
		return
	}
	// Removing the last one would lock the athlete out of their own account.
	if !hasPassword && linked <= 1 {
		httpx.Fail(w, http.StatusConflict,
			"That's the only way into this account. Set a password first, or link another provider.")
		return
	}

	tag, err := s.pool.Exec(r.Context(),
		`delete from user_identities where user_id = $1 and provider = $2`, me.ID, provider)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't unlink that account.")
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Fail(w, http.StatusNotFound, "That provider isn't linked to your account.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "unlinked"})
}

// noPasswordMessage names the button to press. An athlete who signed in with
// Google once and then types their email and a guess should be told which of
// those two things is real, not that their password is wrong.
func (s *Service) noPasswordMessage(ctx context.Context, userID string) string {
	rows, err := s.pool.Query(ctx,
		`select provider from user_identities where user_id = $1 order by created_at`, userID)
	if err != nil {
		return "This account has no password. Sign in with the provider you used to create it."
	}
	defer rows.Close()

	var labels []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		if p, ok := s.provider(id); ok {
			labels = append(labels, p.Label)
		} else {
			labels = append(labels, id)
		}
	}
	if len(labels) == 0 {
		return "This account has no password. Sign in with the provider you used to create it."
	}
	return "This account signs in with " + strings.Join(labels, " or ") + ". Use that button instead."
}
