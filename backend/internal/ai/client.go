package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const anthropicURL = "https://api.anthropic.com/v1/messages"

var ErrNotConfigured = errors.New("no Anthropic API key is configured")

type Client struct {
	key           string
	model         string
	searchVersion string
	http          *http.Client
	pool          *pgxpool.Pool
}

func NewClient(pool *pgxpool.Pool, key, model, searchVersion string) *Client {
	return &Client{
		key:           key,
		model:         model,
		searchVersion: searchVersion,
		pool:          pool,
		// Search turns are slower than plain completions.
		http: &http.Client{Timeout: 240 * time.Second},
	}
}

func (c *Client) Configured() bool { return c.key != "" }

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type apiRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []message `json:"messages"`
}

type apiResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// Complete sends one prompt and returns the concatenated text. Every call is
// written to ai_calls, successful or not, so cost and bad output are traceable.
func (c *Client) Complete(ctx context.Context, userID, purpose, system, prompt string, maxTokens int) (string, error) {
	if !c.Configured() {
		return "", ErrNotConfigured
	}

	body, err := json.Marshal(apiRequest{
		Model:     c.model,
		MaxTokens: maxTokens,
		System:    system,
		Messages:  []message{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.key)
	req.Header.Set("anthropic-version", "2023-06-01")

	started := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		c.record(ctx, userID, purpose, prompt, "", 0, 0, time.Since(started), false)
		return "", fmt.Errorf("reaching the model failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		c.record(ctx, userID, purpose, prompt, "", 0, 0, time.Since(started), false)
		return "", err
	}

	var parsed apiResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		c.record(ctx, userID, purpose, prompt, string(raw), 0, 0, time.Since(started), false)
		return "", fmt.Errorf("unexpected response from the model: %s", truncate(string(raw), 200))
	}
	if resp.StatusCode != http.StatusOK {
		msg := resp.Status
		if parsed.Error != nil {
			msg = parsed.Error.Message
		}
		c.record(ctx, userID, purpose, prompt, msg, 0, 0, time.Since(started), false)
		return "", fmt.Errorf("model returned an error: %s", msg)
	}

	var sb strings.Builder
	for _, block := range parsed.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	out := sb.String()
	c.record(ctx, userID, purpose, prompt, out,
		parsed.Usage.InputTokens, parsed.Usage.OutputTokens, time.Since(started), true)

	if strings.TrimSpace(out) == "" {
		return "", errors.New("the model returned an empty response")
	}
	return out, nil
}

// CompleteJSON asks for JSON and unmarshals into dst, tolerating a model that
// wraps its answer in a code fence or adds a sentence around it.
func (c *Client) CompleteJSON(ctx context.Context, userID, purpose, system, prompt string, maxTokens int, dst any) error {
	out, err := c.Complete(ctx, userID, purpose, system, prompt, maxTokens)
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
