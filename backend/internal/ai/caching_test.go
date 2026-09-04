package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// What a cached turn actually puts on the wire.
type sentMessage struct {
	Role    string            `json:"role"`
	Content []json.RawMessage `json:"content"`
}

type sentBlock struct {
	Type         string `json:"type"`
	Text         string `json:"text"`
	CacheControl *struct {
		Type string `json:"type"`
	} `json:"cache_control"`
}

func captureRequest(t *testing.T, body string) (*Client, *string) {
	t.Helper()
	var (
		mu   sync.Mutex
		seen string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = string(raw)
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return clientAt(server.URL, ProviderAnthropic, "claude-sonnet-5", "adaptive"), &seen
}

// The breakpoint has to fall after the stable half and before the volatile
// one. Prefix matching means a single changing byte in front of the catalogue
// would make the entry unreadable on the next request, so the order is the
// whole feature.
func TestTheCachedHalfIsSentFirstAndMarked(t *testing.T) {
	c, seen := captureRequest(t, textFrames("fine"))

	if _, err := c.CompleteCachedStream(context.Background(), "", "review", "standing brief",
		"EXERCISE LIBRARY\n- pull_up", "ATHLETE SNAPSHOT\n{}", 4000, nil); err != nil {
		t.Fatalf("CompleteCachedStream: %v", err)
	}

	var sent struct {
		System   string        `json:"system"`
		Messages []sentMessage `json:"messages"`
	}
	if err := json.Unmarshal([]byte(*seen), &sent); err != nil {
		t.Fatalf("request body: %v", err)
	}
	if len(sent.Messages) != 1 || len(sent.Messages[0].Content) != 2 {
		t.Fatalf("expected one message of two blocks, got %+v", sent.Messages)
	}

	var first, second sentBlock
	_ = json.Unmarshal(sent.Messages[0].Content[0], &first)
	_ = json.Unmarshal(sent.Messages[0].Content[1], &second)

	if !strings.Contains(first.Text, "EXERCISE LIBRARY") {
		t.Errorf("the first block is not the catalogue: %q", first.Text)
	}
	if first.CacheControl == nil || first.CacheControl.Type != "ephemeral" {
		t.Error("the catalogue block carries no cache_control")
	}
	if !strings.Contains(second.Text, "ATHLETE SNAPSHOT") {
		t.Errorf("the second block is not the athlete: %q", second.Text)
	}
	if second.CacheControl != nil {
		t.Error("the volatile block was marked for caching, which can never be read back")
	}
	// The system prompt sits in front of the breakpoint and is cached with it.
	if sent.System != "standing brief" {
		t.Errorf("system = %q", sent.System)
	}
}

// A turn with nothing worth caching must not pay the write premium. A cache
// write costs 1.25x base input; marking a one-off prompt is a pure loss.
func TestAnUncachedTurnSendsAPlainPrompt(t *testing.T) {
	c, seen := captureRequest(t, textFrames("fine"))

	if _, err := c.CompleteStream(context.Background(), "", "review", "sys", "prompt", 4000, nil); err != nil {
		t.Fatalf("CompleteStream: %v", err)
	}
	if strings.Contains(*seen, "cache_control") {
		t.Error("an ordinary turn asked for a cache write it can never read back")
	}

	var sent struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(*seen), &sent); err != nil {
		t.Fatalf("the content is not a plain string: %v", err)
	}
	if sent.Messages[0].Content != "prompt" {
		t.Errorf("content = %q", sent.Messages[0].Content)
	}
}

// A repair rewrites one document that will never be asked for again, so it
// must not open a cache entry either.
func TestARepairTurnIsNotCached(t *testing.T) {
	broken := `{"skill":"pull-up","summary":"a "quoted" phrase"}`
	fixed := `{"skill":"pull-up","summary":"a \"quoted\" phrase"}`
	c, seen := scripted(t, textFrames(broken), textFrames(fixed))

	var out SkillResearch
	if err := c.CompleteJSONStream(context.Background(), "", "skill_research", "sys", "", "prompt",
		4000, researchSchema, nil, &out); err != nil {
		t.Fatalf("CompleteJSONStream: %v", err)
	}
	if strings.Contains((*seen)[1], "cache_control") {
		t.Error("the repair turn opened a cache entry for a one-off document")
	}
}

// The search cap is the sharpest cost control here, so it is clamped rather
// than trusted: every retrieved page is billed as input.
func TestSearchCountIsClampedBothWays(t *testing.T) {
	if got := clampSearches(0, 5); got != 5 {
		t.Errorf("unset should fall back to the default, got %d", got)
	}
	if got := clampSearches(-3, 5); got != 5 {
		t.Errorf("a negative count should fall back to the default, got %d", got)
	}
	if got := clampSearches(3, 5); got != 3 {
		t.Errorf("an explicit lower count should be kept, got %d", got)
	}
	if got := clampSearches(500, 5); got != maxSearchesEver {
		t.Errorf("a runaway count should be capped at %d, got %d", maxSearchesEver, got)
	}
}

// The default was lowered deliberately after a measured turn spent 87,000
// input tokens on seven searches. A future edit that raises it back should
// have to say so here.
func TestResearchAsksForFewerSearchesThanItUsedTo(t *testing.T) {
	const measuredAtSeven = 7
	if researchSearches >= measuredAtSeven {
		t.Errorf("researchSearches is %d; seven was measured at 87k input tokens, "+
			"roughly half the price of a whole plan", researchSearches)
	}
	if researchSearches < 3 {
		t.Errorf("researchSearches is %d, too few to find a ladder, its standards "+
			"and its injuries", researchSearches)
	}
}

// The cap reaches the search request, not just the Go struct.
func TestTheSearchCapReachesTheProvider(t *testing.T) {
	var seen string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		seen = string(raw)
		_, _ = w.Write([]byte(`{
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 100, "output_tokens": 10},
			"content": [{"type": "text", "text": "{\"skill\":\"x\",\"summary\":\"y\"}"}]
		}`))
	}))
	defer server.Close()

	c := clientAt(server.URL, ProviderAnthropic, "claude-sonnet-5", "adaptive")
	var out SkillResearch
	if _, err := c.SearchJSON(context.Background(), "", "skill_research", "sys", "prompt", 14000,
		SearchOptions{MaxSearches: 4, Schema: researchSchema}, &out); err != nil {
		t.Fatalf("SearchJSON: %v", err)
	}

	var sent struct {
		Tools []struct {
			MaxUses int `json:"max_uses"`
		} `json:"tools"`
	}
	if err := json.Unmarshal([]byte(seen), &sent); err != nil {
		t.Fatalf("request body: %v", err)
	}
	if len(sent.Tools) != 1 || sent.Tools[0].MaxUses != 4 {
		t.Fatalf("the cap did not reach the tool: %+v", sent.Tools)
	}
}
