package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"calisthenics/api/internal/secret"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Nobody's coaching runs on the server's money. An athlete connects their own
// Claude or ChatGPT account, the key is sealed into the database, and every
// call they make afterwards is billed to them by their own provider.
var (
	ErrNoCredentials = errors.New("connect your own Claude or ChatGPT key before using the coaching features")
	ErrNoKeystore    = errors.New("this server cannot seal provider keys right now, so none can be stored")
	// ErrPaused is the athlete's own doing, and reads differently from the
	// other two because of it: the key is here and it works, they have simply
	// switched the connector off. Nothing to fix, one toggle to flip.
	ErrPaused = errors.New("your AI connector is switched off, so nothing was sent to a model")
)

// Provider is one account an athlete can connect, as the settings page needs
// to describe it.
type Provider struct {
	ID string `json:"id"`
	// Label is the product an athlete recognises; Vendor is the company that
	// bills them. The settings page shows both, on two lines.
	Label        string   `json:"label"`
	Vendor       string   `json:"vendor"`
	DefaultModel string   `json:"default_model"`
	Models       []string `json:"models"`
	// KeysURL is where the athlete goes to make a key.
	KeysURL string `json:"keys_url"`
	// KeyPrefix is a shape check, not a secret check: it catches a pasted
	// password or a key from the other provider before a request is spent.
	KeyPrefix string `json:"key_prefix"`
}

// Providers is the catalogue, in the order the settings page offers them.
var Providers = []Provider{
	{
		ID:           ProviderAnthropic,
		Label:        "Claude",
		Vendor:       "Anthropic",
		DefaultModel: defaultAnthropicModel,
		// Cheapest useful first, dearest last. That ordering is the honest one
		// now that the app writes the plan itself and the model only improves
		// it: the difference between the tiers shows up in the polish, and the
		// difference in the bill lands on the athlete.
		Models:    []string{"claude-sonnet-5", "claude-haiku-4-5-20251001", "claude-opus-5"},
		KeysURL:   "https://console.anthropic.com/settings/keys",
		KeyPrefix: "sk-ant-",
	},
	{
		ID:           ProviderOpenAI,
		Label:        "ChatGPT",
		Vendor:       "OpenAI",
		DefaultModel: defaultOpenAIModel,
		Models:       []string{"gpt-5-mini", "gpt-5", "gpt-4.1"},
		KeysURL:      "https://platform.openai.com/api-keys",
		KeyPrefix:    "sk-",
	},
}

func providerByID(id string) (Provider, bool) {
	for _, p := range Providers {
		if strings.EqualFold(strings.TrimSpace(id), p.ID) {
			return p, true
		}
	}
	return Provider{}, false
}

func defaultModel(provider string) string {
	if p, ok := providerByID(provider); ok {
		return p.DefaultModel
	}
	return defaultAnthropicModel
}

// Connection is what the athlete is shown about their own connection. It never
// carries the key: once stored, the key only ever leaves the database on its
// way to the provider.
type Connection struct {
	Provider   string     `json:"provider"`
	Label      string     `json:"label"`
	Model      string     `json:"model"`
	KeyHint    string     `json:"key_hint"`
	UpdatedAt  time.Time  `json:"updated_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	// Paused holds the key without spending it. ForgetOnLogout drops it at
	// the next sign-out. Both are the athlete's switches, not the server's.
	Paused         bool `json:"paused"`
	ForgetOnLogout bool `json:"forget_on_logout"`
}

// Store is the per-user credential table plus the sealing key that makes it
// safe to hold. It is also where clients come from: every model call in this
// app starts by asking the store for the caller's own client.
type Store struct {
	pool     *pgxpool.Pool
	box      *secret.Box
	settings Settings
	// baseURL redirects every client this store builds at another host. Only
	// the tests set it; in production both transports know their own API.
	baseURL string
}

func NewStore(pool *pgxpool.Pool, box *secret.Box, settings Settings) *Store {
	return &Store{pool: pool, box: box, settings: settings}
}

// Ready is false when the server was started without a sealing key. Coaching
// is then off for everyone, and the message says so rather than blaming the
// athlete for not connecting an account.
func (s *Store) Ready() bool { return s != nil && s.box != nil }

// Connection returns what the athlete has connected, if anything.
func (s *Store) Connection(ctx context.Context, userID string) (Connection, bool, error) {
	var (
		conn    Connection
		hint    *string
		lastUse *time.Time
	)
	err := s.pool.QueryRow(ctx, `
		select provider, model, key_hint, updated_at, last_used_at, paused, forget_on_logout
		from user_ai_credentials where user_id = $1`, userID).
		Scan(&conn.Provider, &conn.Model, &hint, &conn.UpdatedAt, &lastUse,
			&conn.Paused, &conn.ForgetOnLogout)
	if errors.Is(err, pgx.ErrNoRows) {
		return Connection{}, false, nil
	}
	if err != nil {
		return Connection{}, false, err
	}
	if hint != nil {
		conn.KeyHint = *hint
	}
	conn.LastUsedAt = lastUse
	if p, ok := providerByID(conn.Provider); ok {
		conn.Label = p.Label
	}
	if conn.Model == "" {
		conn.Model = defaultModel(conn.Provider)
	}
	return conn, true, nil
}

// Connect verifies the credentials against the provider and, only if they
// work, seals and stores them. Verifying first means a mistyped key is a
// message on the settings page rather than a failed plan ten minutes later.
func (s *Store) Connect(ctx context.Context, userID, providerID, key, model string) (Connection, error) {
	if !s.Ready() {
		return Connection{}, ErrNoKeystore
	}

	provider, ok := providerByID(providerID)
	if !ok {
		return Connection{}, errors.New("pick either Claude or ChatGPT")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return Connection{}, errors.New("paste the API key from your provider")
	}
	if err := checkKeyShape(provider, key); err != nil {
		return Connection{}, err
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = provider.DefaultModel
	}

	client := s.client(Credentials{Provider: provider.ID, Key: key, Model: model})
	verifyCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := client.Verify(verifyCtx); err != nil {
		return Connection{}, err
	}

	sealed, err := s.box.Seal(key)
	if err != nil {
		return Connection{}, err
	}

	_, err = s.pool.Exec(ctx, `
		insert into user_ai_credentials (user_id, provider, key_sealed, key_hint, model)
		values ($1, $2, $3, $4, $5)
		on conflict (user_id) do update
		set provider   = excluded.provider,
		    key_sealed = excluded.key_sealed,
		    key_hint   = excluded.key_hint,
		    model      = excluded.model,
		    -- Pasting a key and pressing Connect means you want it working.
		    -- forget_on_logout is a standing preference, so it survives.
		    paused     = false,
		    updated_at = now()`,
		userID, provider.ID, sealed, keyHint(key), model)
	if err != nil {
		return Connection{}, err
	}

	conn, _, err := s.Connection(ctx, userID)
	return conn, err
}

// UseModel switches the model without asking for the key again.
func (s *Store) UseModel(ctx context.Context, userID, model string) (Connection, error) {
	creds, err := s.credential(ctx, userID)
	if err != nil {
		return Connection{}, err
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = defaultModel(creds.Provider)
	}
	creds.Model = model

	// The key still has to work with the new model, and the provider is the
	// only thing that can say whether it does.
	verifyCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := s.client(creds).Verify(verifyCtx); err != nil {
		return Connection{}, err
	}

	if _, err := s.pool.Exec(ctx,
		`update user_ai_credentials set model = $2, updated_at = now() where user_id = $1`,
		userID, model); err != nil {
		return Connection{}, err
	}
	conn, _, err := s.Connection(ctx, userID)
	return conn, err
}

// Disconnect forgets the key. Nothing is kept for later.
func (s *Store) Disconnect(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `delete from user_ai_credentials where user_id = $1`, userID)
	return err
}

// SetSwitches flips either of the athlete's own switches. Both are optional:
// a nil leaves that one alone, so the settings page can send one toggle
// without having to restate the other.
//
// Neither switch touches the key, and neither needs the provider's agreement,
// so unlike Connect and UseModel this makes no network call at all.
func (s *Store) SetSwitches(ctx context.Context, userID string, paused, forgetOnLogout *bool) (Connection, error) {
	tag, err := s.pool.Exec(ctx, `
		update user_ai_credentials
		set paused           = coalesce($2, paused),
		    forget_on_logout = coalesce($3, forget_on_logout),
		    updated_at       = now()
		where user_id = $1`, userID, paused, forgetOnLogout)
	if err != nil {
		return Connection{}, err
	}
	if tag.RowsAffected() == 0 {
		return Connection{}, ErrNoCredentials
	}
	conn, _, err := s.Connection(ctx, userID)
	return conn, err
}

// ForgetOnSignOut drops the key of an athlete who asked for it to go when they
// sign out. It is wired into logout, and it is deliberately quiet: signing out
// must succeed whether or not there was ever a key to forget.
func (s *Store) ForgetOnSignOut(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx,
		`delete from user_ai_credentials where user_id = $1 and forget_on_logout`, userID)
	return err
}

// ClientFor builds the caller's own client. Every coaching path starts here,
// which is what guarantees no call is ever made on anyone else's account.
func (s *Store) ClientFor(ctx context.Context, userID string) (*Client, error) {
	creds, err := s.credential(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Best effort: knowing when a key was last spent is how an athlete tells
	// a connection that works from one that is merely stored.
	markCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, _ = s.pool.Exec(markCtx,
		`update user_ai_credentials set last_used_at = now() where user_id = $1`, userID)

	return s.client(creds), nil
}

func (s *Store) client(creds Credentials) *Client {
	c := NewClient(s.pool, creds, s.settings)
	if s.baseURL != "" {
		c.api.useBaseURL(s.baseURL)
	}
	return c
}

// credential unseals one athlete's stored key. It is the only path back to
// plaintext, and nothing outside this package can reach it.
func (s *Store) credential(ctx context.Context, userID string) (Credentials, error) {
	if !s.Ready() {
		return Credentials{}, ErrNoKeystore
	}

	var (
		creds  Credentials
		sealed []byte
		paused bool
	)
	err := s.pool.QueryRow(ctx, `
		select provider, key_sealed, model, paused
		from user_ai_credentials where user_id = $1`, userID).
		Scan(&creds.Provider, &sealed, &creds.Model, &paused)
	if errors.Is(err, pgx.ErrNoRows) {
		return Credentials{}, ErrNoCredentials
	}
	if err != nil {
		return Credentials{}, err
	}
	// Refuse before unsealing. A switched-off connector should not have its
	// key in memory at all, let alone on its way to a provider.
	if paused {
		return Credentials{}, ErrPaused
	}

	creds.Key, err = s.box.Open(sealed)
	if err != nil {
		return Credentials{}, err
	}
	return creds, nil
}

// checkKeyShape catches the two mistakes worth catching before a request is
// spent: a key that is not one at all, and a key pasted under the wrong
// provider. Anthropic keys begin sk-ant-, which is also a valid OpenAI prefix,
// so the more specific prefix has to win.
func checkKeyShape(provider Provider, key string) error {
	best := provider
	for _, other := range Providers {
		if strings.HasPrefix(key, other.KeyPrefix) && len(other.KeyPrefix) > len(best.KeyPrefix) {
			best = other
		}
	}
	if best.ID != provider.ID {
		return fmt.Errorf("that looks like a %s key, not a %s one", best.Label, provider.Label)
	}
	if !strings.HasPrefix(key, provider.KeyPrefix) {
		return fmt.Errorf("that doesn't look like a %s key: they start with %s", provider.Label, provider.KeyPrefix)
	}
	return nil
}

// keyHint is the last four characters, which is what the settings page shows
// so an athlete can tell which of their keys is in here.
func keyHint(key string) string {
	if len(key) <= 4 {
		return key
	}
	return key[len(key)-4:]
}
