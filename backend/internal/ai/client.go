package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The two accounts an athlete can bring. Nothing here is the server's own: a
// call is always spent on the key of the person who asked for it.
const (
	ProviderAnthropic = "anthropic"
	ProviderOpenAI    = "openai"
)

// defaultMaxTokens is deliberately roomy. Current models think before they
// answer and thinking is spent out of the same budget, so a tight ceiling
// truncates the turn before any answer text exists.
const defaultMaxTokens = 16000

var ErrNotConfigured = errors.New("this account has no model provider connected")

// Credentials are one athlete's model account: which API to call, with which
// key, on which model.
type Credentials struct {
	Provider string
	Key      string
	Model    string
}

// Client is one athlete's connection to their own provider. It is built per
// request from stored credentials, so it is cheap to make and never shared.
type Client struct {
	provider string
	model    string
	key      string
	pool     *pgxpool.Pool
	api      transport
}

// transport is what one provider's HTTP API looks like from in here. Both
// implementations answer in the same normalised shape so that everything
// above this line — progress, ceilings, refusals, recording — is written once.
type transport interface {
	// Verify proves the key works and the model exists, without spending a turn.
	Verify(ctx context.Context) error
	// Stream runs one turn, calling onDelta (which may be nil) as it arrives.
	Stream(ctx context.Context, t turn, onDelta func(Delta)) (outcome, error)
	// Search runs one turn with the provider's own server-side web search.
	Search(ctx context.Context, t turn, opts SearchOptions) (outcome, error)
	// useBaseURL points the transport at another host. Tests use it.
	useBaseURL(string)
}

// turn is one request to a model, in the shape both providers understand.
type turn struct {
	System    string
	Prompt    string
	MaxTokens int
	// Schema is the JSON Schema the answer must satisfy, empty when the turn
	// is prose. A transport whose provider can enforce it sends it and the
	// answer is guaranteed to parse; one that cannot ignores it, and the
	// repair turn in CompleteJSONStream and SearchJSON is what catches the
	// difference.
	Schema json.RawMessage
}

// outcome is a finished turn, normalised across providers.
type outcome struct {
	Text         string
	InputTokens  int
	OutputTokens int
	// StopReason is "", "max_tokens" or "refusal": the three the caller acts on.
	StopReason    string
	StopNote      string
	ThinkingChars int
	Searches      int
	Sources       []Source
}

// Settings are the server-wide knobs that are not anyone's account: which web
// search tool version to ask Anthropic for, and whether to ask for thinking.
type Settings struct {
	SearchToolVersion string
	// Thinking is "adaptive" (every current model) or "off" for a model old
	// enough to reject the parameter.
	Thinking string
}

// NewClient builds a client for one athlete's credentials.
func NewClient(pool *pgxpool.Pool, creds Credentials, settings Settings) *Client {
	model := strings.TrimSpace(creds.Model)
	if model == "" {
		model = defaultModel(creds.Provider)
	}

	c := &Client{
		provider: normaliseProvider(creds.Provider),
		model:    model,
		key:      strings.TrimSpace(creds.Key),
		pool:     pool,
	}
	if c.provider == ProviderOpenAI {
		c.api = newOpenAI(c.key, model, settings)
	} else {
		c.api = newAnthropic(c.key, model, settings)
	}
	return c
}

func normaliseProvider(p string) string {
	if strings.EqualFold(strings.TrimSpace(p), ProviderOpenAI) {
		return ProviderOpenAI
	}
	return ProviderAnthropic
}

// newHTTPClient is shared by both transports.
//
// No client-wide timeout: a streamed plan can legitimately run for minutes,
// and each caller sets its own deadline on the context. The header timeout
// still catches an API that never answers at all — it is generous because a
// web search turn, which is not streamed, sends no headers until the whole
// turn is finished.
func newHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 240 * time.Second
	return &http.Client{Transport: transport}
}

func (c *Client) Provider() string { return c.provider }
func (c *Client) Model() string    { return c.model }

// Verify checks the credentials against the provider. It costs no tokens, so
// it is what the settings page calls before storing a key.
func (c *Client) Verify(ctx context.Context) error { return c.api.Verify(ctx) }

// Complete sends one prompt and returns the answer text. Every call is written
// to ai_calls, successful or not, so cost and bad output are traceable.
func (c *Client) Complete(ctx context.Context, userID, purpose, system, prompt string, maxTokens int) (string, error) {
	return c.CompleteStream(ctx, userID, purpose, system, prompt, maxTokens, nil)
}

// CompleteStream sends one prompt and returns the answer text, calling onDelta
// (which may be nil) for every increment as it arrives.
//
// Every completion is streamed, for two reasons.
//
// The first is correctness. Current models think before they answer, and
// thinking tokens are spent out of the ceiling. A plan asked for in one
// non-streaming call with a small ceiling can hit that ceiling while still
// thinking: the API answers 200 with reasoning and no text at all, which reads
// downstream as "the model returned an empty response". A stream tells us
// which phase the tokens went to and why the turn ended, so the failure names
// itself.
//
// The second is the progress bar. The deltas are the only honest source of
// "how far along is it".
func (c *Client) CompleteStream(ctx context.Context, userID, purpose, system, prompt string,
	maxTokens int, onDelta func(Delta)) (string, error) {

	return c.completeStream(ctx, userID, purpose, system, prompt, maxTokens, nil, onDelta)
}

// completeStream is CompleteStream with the shape the answer must satisfy.
func (c *Client) completeStream(ctx context.Context, userID, purpose, system, prompt string,
	maxTokens int, schema json.RawMessage, onDelta func(Delta)) (string, error) {

	if !c.Configured() {
		return "", ErrNotConfigured
	}
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}

	started := time.Now()
	res, err := c.api.Stream(ctx,
		turn{System: system, Prompt: prompt, MaxTokens: maxTokens, Schema: schema}, onDelta)
	if err != nil {
		c.record(ctx, userID, purpose, prompt, err.Error(), res.InputTokens, res.OutputTokens, time.Since(started), false)
		return res.Text, err
	}

	ok := strings.TrimSpace(res.Text) != ""
	c.record(ctx, userID, purpose, prompt, res.Text, res.InputTokens, res.OutputTokens, time.Since(started), ok)

	// A turn that ended at the ceiling is not an answer even when some of it
	// arrived: half a plan is not half usable.
	if res.StopReason == "max_tokens" {
		return res.Text, ceilingError(ok, maxTokens)
	}
	if !ok {
		return "", emptyAnswerError(res.StopReason, res.StopNote, res.ThinkingChars)
	}
	return res.Text, nil
}

// CompleteJSON asks for JSON and unmarshals into dst, tolerating a model that
// wraps its answer in a code fence or adds a sentence around it.
func (c *Client) CompleteJSON(ctx context.Context, userID, purpose, system, prompt string,
	maxTokens int, schema json.RawMessage, dst any) error {
	return c.CompleteJSONStream(ctx, userID, purpose, system, prompt, maxTokens, schema, nil, dst)
}

// CompleteJSONStream is CompleteJSON with progress reporting. schema may be
// empty, in which case the answer is only asked for in the prompt.
func (c *Client) CompleteJSONStream(ctx context.Context, userID, purpose, system, prompt string,
	maxTokens int, schema json.RawMessage, onDelta func(Delta), dst any) error {

	out, err := c.completeStream(ctx, userID, purpose, system, prompt, maxTokens, schema, onDelta)
	if err != nil {
		return err
	}
	return c.parseOrRepair(ctx, userID, purpose, out, schema, dst)
}

// Configured is false only for a client built without a key, which the store
// never does; it is the last guard before a pointless request.
func (c *Client) Configured() bool { return c.key != "" && c.api != nil }

func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if fence := strings.Index(s, "```"); fence >= 0 {
		rest := s[fence+3:]
		if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
			rest = rest[nl+1:]
		}
		if end := strings.Index(rest, "```"); end >= 0 {
			rest = rest[:end]
		}
		s = strings.TrimSpace(rest)
	}
	start := strings.IndexAny(s, "{[")
	if start < 0 {
		return s
	}
	end := strings.LastIndexAny(s, "}]")
	if end < start {
		return s
	}
	return s[start : end+1]
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// ceilingError names the ceiling and the fix. Both halves matter: the ceiling
// covers reasoning as well as the answer, so the number in the message is the
// one to raise.
func ceilingError(hadText bool, maxTokens int) error {
	if hadText {
		return fmt.Errorf("the model ran out of room partway through the answer, at its %d-token ceiling. "+
			"Ask for less at a time, or raise the ceiling for this call", maxTokens)
	}
	return fmt.Errorf("the model spent its whole %d-token ceiling reasoning and never wrote an answer. "+
		"Ask for less at a time, or raise the ceiling for this call", maxTokens)
}

// emptyAnswerError explains a turn that produced no text for some other reason.
func emptyAnswerError(stopReason, stopNote string, thinkingChars int) error {
	if stopReason == "refusal" {
		if stopNote != "" {
			return fmt.Errorf("the model declined to answer (%s)", stopNote)
		}
		return errors.New("the model declined to answer this request")
	}
	if thinkingChars > 0 {
		return fmt.Errorf("the model reasoned but wrote no answer (stop reason: %s)", orUnknown(stopReason))
	}
	return fmt.Errorf("the model returned an empty response (stop reason: %s)", orUnknown(stopReason))
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func (c *Client) record(ctx context.Context, userID, purpose, prompt, completion string,
	in, out int, took time.Duration, ok bool) {
	if c.pool == nil {
		return
	}
	// Logging must never block or fail the request it describes.
	logCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	var user any
	if userID != "" {
		user = userID
	}
	_, _ = c.pool.Exec(logCtx, `
		insert into ai_calls
			(user_id, purpose, provider, model, input_tokens, output_tokens, duration_ms, ok, prompt, completion)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		user, purpose, c.provider, c.model, in, out, int(took.Milliseconds()), ok,
		truncate(prompt, 20000), truncate(completion, 20000))
}
