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

const openAIBase = "https://api.openai.com"

// The cheaper tier, for the same reason Anthropic's default is Sonnet: the
// bill is the athlete's. The full model is one click away.
const defaultOpenAIModel = "gpt-5-mini"

// Everything here speaks the Responses API rather than chat completions. It is
// the only OpenAI surface that carries the two things this app depends on:
// reasoning summaries, which is what the progress bar reports while the model
// is still thinking, and server-side web search, which is what the research
// and event-discovery passes are built on.
type openAIAPI struct {
	key   string
	model string
	// thinking mirrors ANTHROPIC_THINKING: "off" turns off the reasoning
	// summary for a model that has no reasoning to summarise.
	thinking string
	base     string
	http     *http.Client
}

func newOpenAI(key, model string, settings Settings) *openAIAPI {
	return &openAIAPI{
		key:      key,
		model:    model,
		thinking: settings.Thinking,
		base:     openAIBase,
		http:     newHTTPClient(),
	}
}

func (o *openAIAPI) useBaseURL(u string) { o.base = u }

func (o *openAIAPI) request(ctx context.Context, method, path string, body []byte) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, o.base+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.key)
	return req, nil
}

// reasons reports whether this model thinks before it answers. The reasoning
// parameter is rejected outright by the models that do not, so it cannot just
// be sent to everything.
func (o *openAIAPI) reasons() bool {
	switch strings.ToLower(o.thinking) {
	case "off", "disabled", "none":
		return false
	}
	model := strings.ToLower(o.model)
	for _, family := range []string{"gpt-5", "o1", "o3", "o4"} {
		if strings.HasPrefix(model, family) {
			return true
		}
	}
	return false
}

func (o *openAIAPI) Verify(ctx context.Context) error {
	req, err := o.request(ctx, http.MethodGet, "/v1/models/"+o.model, nil)
	if err != nil {
		return err
	}
	resp, err := o.http.Do(req)
	if err != nil {
		return fmt.Errorf("reaching OpenAI failed: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("your OpenAI account has no model called %q", o.model)
	}
	return errors.New(openAIErrorMessage(raw, resp.Status))
}

func openAIErrorMessage(raw []byte, fallback string) string {
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

type openAIReasoning struct {
	// Summary is what the progress indicator reads while the model thinks.
	Summary string `json:"summary,omitempty"`
}

type openAITool struct {
	Type         string              `json:"type"`
	Filters      *openAIToolFilters  `json:"filters,omitempty"`
	UserLocation *openAIToolLocation `json:"user_location,omitempty"`
}

type openAIToolFilters struct {
	AllowedDomains []string `json:"allowed_domains,omitempty"`
}

type openAIToolLocation struct {
	Type     string `json:"type"`
	City     string `json:"city,omitempty"`
	Region   string `json:"region,omitempty"`
	Country  string `json:"country,omitempty"`
	Timezone string `json:"timezone,omitempty"`
}

type openAIRequest struct {
	Model           string           `json:"model"`
	Instructions    string           `json:"instructions,omitempty"`
	Input           string           `json:"input"`
	MaxOutputTokens int              `json:"max_output_tokens,omitempty"`
	Stream          bool             `json:"stream,omitempty"`
	Reasoning       *openAIReasoning `json:"reasoning,omitempty"`
	Tools           []openAITool     `json:"tools,omitempty"`
	// The athlete's prompts are their training history. There is no reason to
	// leave a copy of them sitting in their OpenAI dashboard.
	Store bool `json:"store"`
}

func (o *openAIAPI) newRequestBody(t turn, stream bool, tools []openAITool) ([]byte, error) {
	body := openAIRequest{
		Model:           o.model,
		Instructions:    t.System,
		Input:           t.Prompt,
		MaxOutputTokens: t.MaxTokens,
		Stream:          stream,
		Tools:           tools,
		Store:           false,
	}
	if o.reasons() {
		body.Reasoning = &openAIReasoning{Summary: "auto"}
	}
	return json.Marshal(body)
}

// openAIResponse is the finished object, which arrives whole from a
// non-streaming call and inside the last frame of a streamed one.
type openAIResponse struct {
	Status            string `json:"status"`
	IncompleteDetails *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Output []openAIOutputItem `json:"output"`
}

type openAIOutputItem struct {
	Type    string `json:"type"`
	Content []struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		Refusal     string `json:"refusal"`
		Annotations []struct {
			Type  string `json:"type"`
			URL   string `json:"url"`
			Title string `json:"title"`
		} `json:"annotations"`
	} `json:"content"`
}

// applyTo folds a finished response into the outcome: how the turn ended, what
// it cost, and — for a search — which pages the API actually read.
func (r openAIResponse) applyTo(res *outcome) {
	if r.Usage.InputTokens > 0 {
		res.InputTokens = r.Usage.InputTokens
	}
	if r.Usage.OutputTokens > 0 {
		res.OutputTokens = r.Usage.OutputTokens
	}
	// The ceiling has the same meaning here as on Anthropic: reasoning and
	// answer are spent from one budget, and hitting it truncates the answer.
	if r.IncompleteDetails != nil && r.IncompleteDetails.Reason == "max_output_tokens" {
		res.StopReason = "max_tokens"
	}
	for _, item := range r.Output {
		if item.Type == "web_search_call" {
			res.Searches++
		}
		for _, part := range item.Content {
			if part.Type == "refusal" && part.Refusal != "" {
				res.StopReason = "refusal"
				res.StopNote = part.Refusal
			}
		}
	}
}

// sources collects every page the search tool actually retrieved. Unlike
// Anthropic, the annotation carries no quoted passage, only the page it backs,
// which is all the citation allowlist downstream needs.
func (r openAIResponse) sources() []Source {
	var out []Source
	seen := map[string]bool{}
	for _, item := range r.Output {
		for _, part := range item.Content {
			for _, note := range part.Annotations {
				if note.Type != "url_citation" || note.URL == "" {
					continue
				}
				key := normaliseURL(note.URL)
				if seen[key] {
					continue
				}
				seen[key] = true
				out = append(out, Source{URL: note.URL, Title: note.Title})
			}
		}
	}
	return out
}

func (r openAIResponse) text() string {
	var text strings.Builder
	for _, item := range r.Output {
		if item.Type != "message" {
			continue
		}
		for _, part := range item.Content {
			if part.Type == "output_text" {
				text.WriteString(part.Text)
			}
		}
	}
	return text.String()
}

// streamFrame is one SSE payload from the Responses API. Every frame names its
// own type; the ones not listed here are progress bookkeeping we do not need.
type streamFrame struct {
	Type     string          `json:"type"`
	Delta    string          `json:"delta"`
	Response *openAIResponse `json:"response"`
	Message  string          `json:"message"`
	Code     string          `json:"code"`
}

func (o *openAIAPI) Stream(ctx context.Context, t turn, onDelta func(Delta)) (outcome, error) {
	var res outcome

	body, err := o.newRequestBody(t, true, nil)
	if err != nil {
		return res, err
	}
	req, err := o.request(ctx, http.MethodPost, "/v1/responses", body)
	if err != nil {
		return res, err
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := o.http.Do(req)
	if err != nil {
		return res, fmt.Errorf("reaching the model failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return res, fmt.Errorf("model returned an error: %s", openAIErrorMessage(raw, resp.Status))
	}

	var text strings.Builder
	reader := bufio.NewReaderSize(resp.Body, 64<<10)
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			if payload, ok := sseData(line); ok && payload != "[DONE]" {
				var frame streamFrame
				if json.Unmarshal([]byte(payload), &frame) != nil {
					continue // a frame we do not model; keep reading
				}
				switch frame.Type {
				case "response.output_text.delta":
					text.WriteString(frame.Delta)
					if onDelta != nil {
						onDelta(Delta{Kind: "text", Text: frame.Delta})
					}
				case "response.reasoning_summary_text.delta":
					res.ThinkingChars += len(frame.Delta)
					if onDelta != nil {
						onDelta(Delta{Kind: "thinking", Text: frame.Delta})
					}
				case "response.completed", "response.incomplete":
					if frame.Response != nil {
						frame.Response.applyTo(&res)
					}
				case "response.failed":
					res.Text = text.String()
					msg := "the model gave up on the turn"
					if frame.Response != nil && frame.Response.Error != nil {
						msg = frame.Response.Error.Message
					}
					return res, fmt.Errorf("model returned an error: %s", msg)
				case "error":
					res.Text = text.String()
					return res, fmt.Errorf("model returned an error: %s", orUnknown(frame.Message))
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

func (o *openAIAPI) Search(ctx context.Context, t turn, opts SearchOptions) (outcome, error) {
	var res outcome

	// The tool has no cap on how many searches it runs, so MaxSearches is
	// advisory here in a way it is not on Anthropic; blocked domains have no
	// equivalent at all and are simply not enforceable through this API.
	tool := openAITool{Type: "web_search"}
	if len(opts.AllowedDomains) > 0 {
		tool.Filters = &openAIToolFilters{AllowedDomains: opts.AllowedDomains}
	}
	if loc := opts.UserLocation; loc != nil {
		tool.UserLocation = &openAIToolLocation{
			Type:     "approximate",
			City:     loc.City,
			Region:   loc.Region,
			Country:  loc.Country,
			Timezone: loc.Timezone,
		}
	}

	body, err := o.newRequestBody(t, false, []openAITool{tool})
	if err != nil {
		return res, err
	}
	req, err := o.request(ctx, http.MethodPost, "/v1/responses", body)
	if err != nil {
		return res, err
	}

	resp, err := o.http.Do(req)
	if err != nil {
		return res, fmt.Errorf("reaching the model failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return res, err
	}
	if resp.StatusCode != http.StatusOK {
		return res, fmt.Errorf("model returned an error: %s", openAIErrorMessage(raw, resp.Status))
	}

	var parsed openAIResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return res, fmt.Errorf("unexpected response from the model: %s", truncate(string(raw), 200))
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return res, fmt.Errorf("model returned an error: %s", parsed.Error.Message)
	}

	parsed.applyTo(&res)
	res.Text = parsed.text()
	res.Sources = parsed.sources()
	return res, nil
}
