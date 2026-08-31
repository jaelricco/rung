package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIStreamCollectsTextAndReportsPhases(t *testing.T) {
	c := replay(t, ProviderOpenAI, "gpt-5", "adaptive", `event: response.reasoning_summary_text.delta
data: {"type":"response.reasoning_summary_text.delta","delta":"Weighing the shoulder injury"}

event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"{\"sessions\":"}

event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"[]}"}

event: response.completed
data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":40,"output_tokens":12}}}

`)

	var kinds []string
	out, err := c.CompleteStream(context.Background(), "", "test", "sys", "prompt", 1000,
		func(d Delta) { kinds = append(kinds, d.Kind) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != `{"sessions":[]}` {
		t.Fatalf("text = %q", out)
	}
	if strings.Join(kinds, ",") != "thinking,text,text" {
		t.Fatalf("delta kinds = %v", kinds)
	}
}

// The ceiling means the same thing on both providers, and has to fail the same
// way: an answer cut off at max_output_tokens is not an answer.
func TestOpenAIStreamNamesTheCeiling(t *testing.T) {
	c := replay(t, ProviderOpenAI, "gpt-5", "adaptive", `event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"{\"sessions\":[{\"week\":1,"}

event: response.incomplete
data: {"type":"response.incomplete","response":{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"usage":{"output_tokens":4000}}}

`)

	_, err := c.CompleteStream(context.Background(), "", "test", "sys", "prompt", 4000, nil)
	if err == nil || !strings.Contains(err.Error(), "4000-token ceiling") {
		t.Fatalf("a truncated answer should name the ceiling, got: %v", err)
	}
}

func TestOpenAIStreamSurfacesAPIErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"Incorrect API key provided"}}`))
	}))
	defer server.Close()

	c := clientAt(server.URL, ProviderOpenAI, "gpt-5", "adaptive")
	_, err := c.CompleteStream(context.Background(), "", "test", "sys", "prompt", 1000, nil)
	if err == nil || !strings.Contains(err.Error(), "Incorrect API key") {
		t.Fatalf("error should carry the API's message, got: %v", err)
	}
}

// The reasoning parameter is rejected outright by the models that do not
// reason, so it cannot simply be sent to everything.
func TestOpenAIAsksForReasoningOnlyWhereItExists(t *testing.T) {
	server, body := recorder(t, "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n")

	thinker := clientAt(server.URL, ProviderOpenAI, "gpt-5", "adaptive")
	_, _ = thinker.CompleteStream(context.Background(), "", "test", "sys", "prompt", 1000, nil)
	for _, want := range []string{`"stream":true`, `"reasoning":{"summary":"auto"}`, `"store":false`} {
		if !strings.Contains(*body, want) {
			t.Errorf("request body missing %s: %s", want, *body)
		}
	}

	plain := clientAt(server.URL, ProviderOpenAI, "gpt-4.1", "adaptive")
	_, _ = plain.CompleteStream(context.Background(), "", "test", "sys", "prompt", 1000, nil)
	if strings.Contains(*body, "reasoning") {
		t.Errorf("a model that does not reason should not be sent the parameter: %s", *body)
	}

	off := clientAt(server.URL, ProviderOpenAI, "gpt-5", "off")
	_, _ = off.CompleteStream(context.Background(), "", "test", "sys", "prompt", 1000, nil)
	if strings.Contains(*body, "reasoning") {
		t.Errorf("thinking off should omit the parameter: %s", *body)
	}
}

// Only pages the tool actually retrieved may be trusted downstream, so the
// annotations are what the source list has to be built from.
func TestOpenAISearchKeepsOnlyRetrievedPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"status": "completed",
			"usage": {"input_tokens": 900, "output_tokens": 120},
			"output": [
				{"type": "web_search_call"},
				{"type": "web_search_call"},
				{"type": "message", "content": [{
					"type": "output_text",
					"text": "{\"events\":[]}",
					"annotations": [
						{"type": "url_citation", "url": "https://calisthenics.test/cup", "title": "Cup"},
						{"type": "url_citation", "url": "https://www.calisthenics.test/cup/", "title": "Cup again"}
					]
				}]}
			]
		}`))
	}))
	defer server.Close()

	c := clientAt(server.URL, ProviderOpenAI, "gpt-5", "adaptive")
	var payload struct {
		Events []string `json:"events"`
	}
	result, err := c.SearchJSON(context.Background(), "", "test", "sys", "prompt", 4000,
		SearchOptions{MaxSearches: 4}, &payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SearchCount != 2 {
		t.Fatalf("search count = %d, want 2", result.SearchCount)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("kept %d sources, want 1 after collapsing the duplicate", len(result.Sources))
	}
	if _, ok := result.SourceURLs()["calisthenics.test/cup"]; !ok {
		t.Fatalf("the retrieved page is missing from the allowlist: %+v", result.Sources)
	}
}
