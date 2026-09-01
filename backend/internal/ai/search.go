package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Web search runs inside the provider: one request, the API performs the
// searches itself and returns the answer with citations attached. Both
// providers offer it, and both bill it to the account whose key was used —
// which, here, is always the athlete's own.

type UserLocation struct {
	Type     string `json:"type"`
	City     string `json:"city,omitempty"`
	Region   string `json:"region,omitempty"`
	Country  string `json:"country,omitempty"`
	Timezone string `json:"timezone,omitempty"`
}

type SearchOptions struct {
	MaxSearches int
	// Use one or the other, never both: the Anthropic API rejects requests
	// with both, and OpenAI can only express the allowlist.
	AllowedDomains []string
	BlockedDomains []string
	UserLocation   *UserLocation
}

// Source is a page the API actually retrieved. Only URLs that appear here are
// ever trusted downstream; anything else the model writes is treated as invented.
type Source struct {
	URL       string `json:"url"`
	Title     string `json:"title"`
	CitedText string `json:"cited_text,omitempty"`
	PageAge   string `json:"page_age,omitempty"`
}

type SearchResult struct {
	Text        string   `json:"text"`
	Sources     []Source `json:"sources"`
	SearchCount int      `json:"search_count"`
}

// SourceURLs is the allowlist used to reject fabricated citations.
func (r SearchResult) SourceURLs() map[string]Source {
	out := make(map[string]Source, len(r.Sources))
	for _, s := range r.Sources {
		out[normaliseURL(s.URL)] = s
	}
	return out
}

func normaliseURL(u string) string {
	u = strings.TrimSpace(strings.ToLower(u))
	u = strings.TrimSuffix(u, "/")
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	return strings.TrimPrefix(u, "www.")
}

// Search asks the model a question with web search enabled and returns its
// answer alongside every source the API actually retrieved.
func (c *Client) Search(ctx context.Context, userID, purpose, system, prompt string,
	maxTokens int, opts SearchOptions) (SearchResult, error) {

	var result SearchResult
	if !c.Configured() {
		return result, ErrNotConfigured
	}
	if opts.MaxSearches <= 0 {
		opts.MaxSearches = 6
	}
	if len(opts.AllowedDomains) > 0 && len(opts.BlockedDomains) > 0 {
		return result, errors.New("set allowed_domains or blocked_domains, not both")
	}
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}

	started := time.Now()
	res, err := c.api.Search(ctx, turn{System: system, Prompt: prompt, MaxTokens: maxTokens}, opts)
	result = SearchResult{Text: res.Text, Sources: res.Sources, SearchCount: res.Searches}
	if err != nil {
		c.record(ctx, userID, purpose, prompt, err.Error(), res.InputTokens, res.OutputTokens, time.Since(started), false)
		return result, err
	}

	answered := strings.TrimSpace(result.Text) != ""
	c.record(ctx, userID, purpose, prompt, result.Text,
		res.InputTokens, res.OutputTokens, time.Since(started), answered)

	// Same failure as a plan that never got written: reasoning and search
	// results are spent from the same ceiling as the answer.
	if res.StopReason == "max_tokens" {
		return result, ceilingError(answered, maxTokens)
	}
	if res.StopReason == "refusal" {
		return result, emptyAnswerError(res.StopReason, res.StopNote, res.ThinkingChars)
	}
	return result, nil
}

// SearchJSON runs a search and unmarshals the JSON the model was asked for,
// returning the sources alongside so callers can validate against them.
func (c *Client) SearchJSON(ctx context.Context, userID, purpose, system, prompt string,
	maxTokens int, opts SearchOptions, dst any) (SearchResult, error) {

	result, err := c.Search(ctx, userID, purpose, system, prompt, maxTokens, opts)
	if err != nil {
		return result, err
	}
	if strings.TrimSpace(result.Text) == "" {
		return result, errors.New("the search returned no answer")
	}
	if err := json.Unmarshal([]byte(extractJSON(result.Text)), dst); err != nil {
		return result, fmt.Errorf("the model's answer wasn't usable JSON: %w", err)
	}
	return result, nil
}
