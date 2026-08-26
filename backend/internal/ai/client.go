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

const anthropicURL = "https://api.anthropic.com/v1/messages"

// defaultMaxTokens is deliberately roomy. Current models think before they
// answer and thinking is spent out of the same budget, so a tight ceiling
// truncates the turn before any answer text exists.
const defaultMaxTokens = 16000

var ErrNotConfigured = errors.New("no Anthropic API key is configured")

type Client struct {
	key           string
	model         string
	searchVersion string
	thinking      string
	baseURL       string
	http          *http.Client
	pool          *pgxpool.Pool
}

func NewClient(pool *pgxpool.Pool, key, model, searchVersion, thinking string) *Client {
	// No client-wide timeout: a streamed plan can legitimately run for
	// minutes, and each caller sets its own deadline on the context. The
	// header timeout still catches an API that never answers at all — it is
	// generous because a web search turn, which is not streamed, sends no
	// headers until the whole turn is finished.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 240 * time.Second

	return &Client{
		key:           key,
		model:         model,
		searchVersion: searchVersion,
		thinking:      thinking,
		baseURL:       anthropicURL,
		pool:          pool,
		http:          &http.Client{Transport: transport},
	}
}

func (c *Client) Configured() bool { return c.key != "" }

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type thinkingConfig struct {
	Type string `json:"type"`
	// A summarised chain of thought is what the progress indicator reports
	// while the model is still reasoning. Without it the stream is silent
	// until the first answer token.
	Display string `json:"display,omitempty"`
}

type apiRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system,omitempty"`
	Messages  []message       `json:"messages"`
	Stream    bool            `json:"stream,omitempty"`
	Thinking  *thinkingConfig `json:"thinking,omitempty"`
}

// thinkingConfig returns what to send as the thinking parameter. Adaptive is
// right for every current model. Older models take a token budget instead and
// reject "adaptive", so ANTHROPIC_THINKING=off keeps them working.
func (c *Client) thinkingConfig() *thinkingConfig {
	switch strings.ToLower(c.thinking) {
	case "off", "disabled", "none":
		return nil
	default:
		return &thinkingConfig{Type: "adaptive", Display: "summarized"}
	}
}

// apiResponse is only ever parsed for its error: a failed request answers a
// streaming call with an ordinary JSON body.
type apiResponse struct {
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// Complete sends one prompt and returns the answer text. Every call is written
// to ai_calls, successful or not, so cost and bad output are traceable.
func (c *Client) Complete(ctx context.Context, userID, purpose, system, prompt string, maxTokens int) (string, error) {
	return c.CompleteStream(ctx, userID, purpose, system, prompt, maxTokens, nil)
}

// CompleteJSON asks for JSON and unmarshals into dst, tolerating a model that
// wraps its answer in a code fence or adds a sentence around it.
func (c *Client) CompleteJSON(ctx context.Context, userID, purpose, system, prompt string, maxTokens int, dst any) error {
	return c.CompleteJSONStream(ctx, userID, purpose, system, prompt, maxTokens, nil, dst)
}

// CompleteJSONStream is CompleteJSON with progress reporting.
func (c *Client) CompleteJSONStream(ctx context.Context, userID, purpose, system, prompt string,
	maxTokens int, onDelta func(Delta), dst any) error {

	out, err := c.CompleteStream(ctx, userID, purpose, system, prompt, maxTokens, onDelta)
	if err != nil {
		return err
	}
	cleaned := extractJSON(out)
	if err := json.Unmarshal([]byte(cleaned), dst); err != nil {
		return fmt.Errorf("the model's answer wasn't usable JSON: %w", err)
	}
	return nil
}

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
			(user_id, purpose, model, input_tokens, output_tokens, duration_ms, ok, prompt, completion)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		user, purpose, c.model, in, out, int(took.Milliseconds()), ok,
		truncate(prompt, 20000), truncate(completion, 20000))
}
