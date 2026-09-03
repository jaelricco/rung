package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const anthropicBase = "https://api.anthropic.com"

// Three web search tool versions exist. The basic one is the default here
// because it returns a flat block list; later versions run search inside code
// execution by default, which nests the blocks. If you move to a later
// version, keep allowed_callers as "direct" or teach parseBlocks about the
// nesting.
const defaultSearchToolVersion = "web_search_20250305"

// Sonnet rather than Opus. The model's job here is to improve a plan the app
// has already written and to put a few hundred words of review together — not
// to invent training from nothing — and the athlete pays for it out of their
// own pocket. Opus costs two and a half times as much per token and is one
// click away in the picker for anyone who wants it.
const defaultAnthropicModel = "claude-sonnet-5"

type anthropicAPI struct {
	key           string
	model         string
	searchVersion string
	thinking      string
	base          string
	http          *http.Client
}

func newAnthropic(key, model string, settings Settings) *anthropicAPI {
	version := settings.SearchToolVersion
	if version == "" {
		version = defaultSearchToolVersion
	}
	return &anthropicAPI{
		key:           key,
		model:         model,
		searchVersion: version,
		thinking:      settings.Thinking,
		base:          anthropicBase,
		http:          newHTTPClient(),
	}
}

func (a *anthropicAPI) useBaseURL(u string) { a.base = u }

func (a *anthropicAPI) request(ctx context.Context, method, path string, body []byte) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.base+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.key)
	req.Header.Set("anthropic-version", "2023-06-01")
	return req, nil
}

// Verify asks for the model's own record. It proves the key is live and the
// model name is one this account may use, and it costs nothing.
func (a *anthropicAPI) Verify(ctx context.Context) error {
	req, err := a.request(ctx, http.MethodGet, "/v1/models/"+a.model, nil)
	if err != nil {
		return err
	}
	resp, err := a.http.Do(req)
	if err != nil {
		return fmt.Errorf("reaching Anthropic failed: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("your Anthropic account has no model called %q", a.model)
	}
	return errors.New(anthropicErrorMessage(raw, resp.Status))
}

func anthropicErrorMessage(raw []byte, fallback string) string {
	var parsed struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &parsed) == nil && parsed.Error != nil && parsed.Error.Message != "" {
		return parsed.Error.Message
	}
	return fallback
}

// ---------- streaming ----------

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

type anthropicRequest struct {
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
func (a *anthropicAPI) thinkingConfig() *thinkingConfig {
	switch strings.ToLower(a.thinking) {
	case "off", "disabled", "none":
		return nil
	default:
		return &thinkingConfig{Type: "adaptive", Display: "summarized"}
	}
}

type streamEvent struct {
	Type    string `json:"type"`
	Message struct {
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
	ContentBlock struct {
		Type string `json:"type"`
	} `json:"content_block"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		Thinking    string `json:"thinking"`
		StopReason  string `json:"stop_reason"`
		StopDetails *struct {
			Type        string `json:"type"`
			Category    string `json:"category"`
			Explanation string `json:"explanation"`
		} `json:"stop_details"`
	} `json:"delta"`
	Usage struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (a *anthropicAPI) Stream(ctx context.Context, t turn, onDelta func(Delta)) (outcome, error) {
	var res outcome

	body, err := json.Marshal(anthropicRequest{
		Model:     a.model,
		MaxTokens: t.MaxTokens,
		System:    t.System,
		Messages:  []message{{Role: "user", Content: t.Prompt}},
		Stream:    true,
		Thinking:  a.thinkingConfig(),
	})
	if err != nil {
		return res, err
	}

	req, err := a.request(ctx, http.MethodPost, "/v1/messages", body)
	if err != nil {
		return res, err
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := a.http.Do(req)
	if err != nil {
		return res, fmt.Errorf("reaching the model failed: %w", err)
	}
	defer resp.Body.Close()

	// An error answers the streaming request with an ordinary JSON body.
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return res, fmt.Errorf("model returned an error: %s", anthropicErrorMessage(raw, resp.Status))
	}

	var text strings.Builder
	reader := bufio.NewReaderSize(resp.Body, 64<<10)
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			payload, ok := sseData(line)
			if ok {
				var ev streamEvent
				if json.Unmarshal([]byte(payload), &ev) != nil {
					continue // a frame we do not model; keep reading
				}
				switch ev.Type {
				case "message_start":
					res.InputTokens = ev.Message.Usage.InputTokens
				case "content_block_delta":
					switch ev.Delta.Type {
					case "text_delta":
						text.WriteString(ev.Delta.Text)
						if onDelta != nil {
							onDelta(Delta{Kind: "text", Text: ev.Delta.Text})
						}
					case "thinking_delta":
						res.ThinkingChars += len(ev.Delta.Thinking)
						if onDelta != nil {
							onDelta(Delta{Kind: "thinking", Text: ev.Delta.Thinking})
						}
					}
				case "message_delta":
					if ev.Delta.StopReason != "" {
						res.StopReason = ev.Delta.StopReason
					}
					if d := ev.Delta.StopDetails; d != nil {
						res.StopNote = strings.TrimSpace(d.Category + " " + d.Explanation)
					}
					if ev.Usage.OutputTokens > 0 {
						res.OutputTokens = ev.Usage.OutputTokens
					}
				case "error":
					if ev.Error != nil {
						res.Text = text.String()
						return res, fmt.Errorf("model returned an error: %s", ev.Error.Message)
					}
				}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			res.Text = text.String()
			return res, fmt.Errorf("the model's answer was cut off: %w", err)
		}
	}

	res.Text = text.String()
	return res, nil
}

// sseData pulls the payload out of one "data: ..." line, ignoring the event
// name lines, comments and blank separators around it.
func sseData(line string) (string, bool) {
	line = strings.TrimRight(line, "\r\n")
	if !strings.HasPrefix(line, "data:") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(line, "data:")), true
}

// ---------- web search ----------

// Web search runs server-side: one HTTP request, the API performs the searches
// and returns the answer with citations attached. The only client-side loop we
// need is for pause_turn, where a long search turn is handed back to us to
// resume.

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
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system,omitempty"`
	Messages  []anyMessage    `json:"messages"`
	Tools     []searchTool    `json:"tools"`
	Thinking  *thinkingConfig `json:"thinking,omitempty"`
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

func (a *anthropicAPI) Search(ctx context.Context, t turn, opts SearchOptions) (outcome, error) {
	var res outcome

	tool := searchTool{
		Type:           a.searchVersion,
		Name:           "web_search",
		MaxUses:        opts.MaxSearches,
		AllowedDomains: opts.AllowedDomains,
		BlockedDomains: opts.BlockedDomains,
		UserLocation:   opts.UserLocation,
	}

	messages := []anyMessage{{Role: "user", Content: t.Prompt}}
	var text strings.Builder
	seen := map[string]bool{}

	// The API may pause a long search turn; resume by sending the assistant
	// message back untouched. Bounded so a pathological turn cannot loop.
	for continuation := 0; continuation < 6; continuation++ {
		body, err := json.Marshal(searchRequest{
			Model:     a.model,
			MaxTokens: t.MaxTokens,
			System:    t.System,
			Messages:  messages,
			Tools:     []searchTool{tool},
			Thinking:  a.thinkingConfig(),
		})
		if err != nil {
			return res, err
		}

		parsed, err := a.postMessages(ctx, body)
		if err != nil {
			return res, err
		}

		res.Searches += parsed.Usage.ServerToolUse.WebSearchRequests
		res.InputTokens = parsed.Usage.InputTokens
		res.OutputTokens = parsed.Usage.OutputTokens
		blockText, sources := parseBlocks(parsed.Content)
		text.WriteString(blockText)
		for _, s := range sources {
			key := normaliseURL(s.URL) + "|" + s.CitedText
			if !seen[key] {
				seen[key] = true
				res.Sources = append(res.Sources, s)
			}
		}

		if parsed.StopReason != "pause_turn" {
			res.Text = text.String()
			if parsed.StopReason == "max_tokens" {
				res.StopReason = "max_tokens"
			}
			return res, nil
		}

		// Resume: the assistant's blocks must go back byte-for-byte, because
		// encrypted_content is validated on the next request.
		raw, err := json.Marshal(parsed.Content)
		if err != nil {
			return res, err
		}
		messages = append(messages, anyMessage{Role: "assistant", Content: json.RawMessage(raw)})
	}

	res.Text = text.String()
	return res, errors.New("the search turn did not finish after six continuations")
}

func (a *anthropicAPI) postMessages(ctx context.Context, body []byte) (searchResponse, error) {
	var parsed searchResponse

	req, err := a.request(ctx, http.MethodPost, "/v1/messages", body)
	if err != nil {
		return parsed, err
	}

	resp, err := a.http.Do(req)
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
