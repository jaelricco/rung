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
)

// Web search runs server-side: one HTTP request, the API performs the searches
// and returns the answer with citations attached. The only client-side loop we
// need is for pause_turn, where a long search turn is handed back to us to
// resume.
//
// Three tool versions exist. The basic one is the default here because it
// returns a flat block list; later versions run search inside code execution by
// default, which nests the blocks. If you move to a later version, keep
// allowed_callers as "direct" or teach parseBlocks about the nesting.
const defaultSearchToolVersion = "web_search_20250305"

type UserLocation struct {
	Type     string `json:"type"`
	City     string `json:"city,omitempty"`
	Region   string `json:"region,omitempty"`
	Country  string `json:"country,omitempty"`
	Timezone string `json:"timezone,omitempty"`
}

type SearchOptions struct {
	MaxSearches int
	// Use one or the other, never both: the API rejects requests with both.
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

type searchTool struct {
	Type           string        `json:"type"`
	Name           string        `json:"name"`
	MaxUses        int           `json:"max_uses,omitempty"`
	AllowedDomains []string      `json:"allowed_domains,omitempty"`
	BlockedDomains []string      `json:"blocked_domains,omitempty"`
	UserLocation   *UserLocation `json:"user_location,omitempty"`
}

type anyMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type searchRequest struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	System    string       `json:"system,omitempty"`
	Messages  []anyMessage `json:"messages"`
	Tools     []searchTool `json:"tools"`
}

type searchResponse struct {
	Content    []json.RawMessage `json:"content"`
	StopReason string            `json:"stop_reason"`
	Usage      struct {
		InputTokens   int `json:"input_tokens"`
		OutputTokens  int `json:"output_tokens"`
		ServerToolUse struct {
			WebSearchRequests int `json:"web_search_requests"`
		} `json:"server_tool_use"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
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

	tool := searchTool{
		Type:           c.searchToolVersion(),
		Name:           "web_search",
		MaxUses:        opts.MaxSearches,
		AllowedDomains: opts.AllowedDomains,
		BlockedDomains: opts.BlockedDomains,
		UserLocation:   opts.UserLocation,
	}

	messages := []anyMessage{{Role: "user", Content: prompt}}
	started := time.Now()
	var text strings.Builder
	seen := map[string]bool{}

	// The API may pause a long search turn; resume by sending the assistant
	// message back untouched. Bounded so a pathological turn cannot loop.
	for turn := 0; turn < 6; turn++ {
		body, err := json.Marshal(searchRequest{
			Model:     c.model,
			MaxTokens: maxTokens,
			System:    system,
			Messages:  messages,
			Tools:     []searchTool{tool},
		})
		if err != nil {
			return result, err
		}

		parsed, err := c.postMessages(ctx, body)
		if err != nil {
			c.record(ctx, userID, purpose, prompt, err.Error(), 0, 0, time.Since(started), false)
			return result, err
		}

		result.SearchCount += parsed.Usage.ServerToolUse.WebSearchRequests
		blockText, sources := parseBlocks(parsed.Content)
		text.WriteString(blockText)
		for _, s := range sources {
			key := normaliseURL(s.URL) + "|" + s.CitedText
			if !seen[key] {
				seen[key] = true
				result.Sources = append(result.Sources, s)
			}
		}

		if parsed.StopReason != "pause_turn" {
			result.Text = text.String()
			c.record(ctx, userID, purpose, prompt, result.Text,
				parsed.Usage.InputTokens, parsed.Usage.OutputTokens, time.Since(started), true)
			return result, nil
		}

		// Resume: the assistant's blocks must go back byte-for-byte, because
		// encrypted_content is validated on the next request.
		raw, err := json.Marshal(parsed.Content)
		if err != nil {
			return result, err
		}
		messages = append(messages, anyMessage{Role: "assistant", Content: json.RawMessage(raw)})
	}

	result.Text = text.String()
	c.record(ctx, userID, purpose, prompt, result.Text, 0, 0, time.Since(started), false)
	return result, errors.New("the search turn did not finish after six continuations")
}

func (c *Client) searchToolVersion() string {
	if c.searchVersion != "" {
		return c.searchVersion
	}
	return defaultSearchToolVersion
}

func (c *Client) postMessages(ctx context.Context, body []byte) (searchResponse, error) {
	var parsed searchResponse

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicURL, bytes.NewReader(body))
	if err != nil {
		return parsed, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.key)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.http.Do(req)
	if err != nil {
		return parsed, fmt.Errorf("reaching the model failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return parsed, err
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return parsed, fmt.Errorf("unexpected response from the model: %s", truncate(string(raw), 200))
	}
	if resp.StatusCode != http.StatusOK {
		msg := resp.Status
		if parsed.Error != nil {
			msg = parsed.Error.Message
		}
		return parsed, fmt.Errorf("model returned an error: %s", msg)
	}
	return parsed, nil
}

// parseBlocks pulls the answer text out of the content blocks, and collects
// every URL the API retrieved: both raw search results and cited passages.
// A search error arrives inside a 200 response, so it is detected here.
func parseBlocks(blocks []json.RawMessage) (string, []Source) {
	var text strings.Builder
	var sources []Source

	for _, block := range blocks {
		var kind struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(block, &kind); err != nil {
			continue
		}

		switch kind.Type {
		case "text":
			var t struct {
				Text      string `json:"text"`
				Citations []struct {
					URL       string `json:"url"`
					Title     string `json:"title"`
					CitedText string `json:"cited_text"`
				} `json:"citations"`
			}
			if err := json.Unmarshal(block, &t); err != nil {
				continue
			}
			text.WriteString(t.Text)
			for _, citation := range t.Citations {
				if citation.URL != "" {
					sources = append(sources, Source{
						URL:       citation.URL,
						Title:     citation.Title,
						CitedText: citation.CitedText,
					})
				}
			}

		case "web_search_tool_result":
			// content is a list on success and a single error object on failure.
			var wrapper struct {
				Content json.RawMessage `json:"content"`
			}
			if err := json.Unmarshal(block, &wrapper); err != nil {
				continue
			}
			var results []struct {
				Type    string `json:"type"`
				URL     string `json:"url"`
				Title   string `json:"title"`
				PageAge string `json:"page_age"`
			}
			if err := json.Unmarshal(wrapper.Content, &results); err != nil {
				continue // an error object, not results
			}
			for _, r := range results {
				if r.URL != "" {
					sources = append(sources, Source{URL: r.URL, Title: r.Title, PageAge: r.PageAge})
				}
			}
		}
	}
	return text.String(), sources
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
