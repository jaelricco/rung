package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testClient points a client at a server that replays the given SSE frames.
func testClient(t *testing.T, frames string) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(frames))
	}))
	t.Cleanup(server.Close)

	c := NewClient(nil, "test-key", "claude-sonnet-5", "", "adaptive")
	c.baseURL = server.URL
	return c
}

func TestCompleteStreamCollectsTextAndReportsPhases(t *testing.T) {
	c := testClient(t, `event: message_start
data: {"type":"message_start","message":{"usage":{"input_tokens":40}}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"Checking the shoulder injury"}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"{\"sessions\":"}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"[]}"}}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":12}}

event: message_stop
data: {"type":"message_stop"}

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

// The bug this whole path exists for: the model spends the ceiling on
// reasoning and the turn ends with no text block. The old code called that
// "the model returned an empty response", which told nobody anything.
func TestCompleteStreamExplainsThinkingThatAteTheCeiling(t *testing.T) {
	c := testClient(t, `event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"Long reasoning"}}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"max_tokens"},"usage":{"output_tokens":8000}}

`)

	_, err := c.CompleteStream(context.Background(), "", "test", "sys", "prompt", 8000, nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "8000-token ceiling") {
		t.Fatalf("error should name the ceiling, got: %v", err)
	}
}

// Half a plan is not half a plan: it is unparseable JSON. Say so in terms of
// the ceiling rather than letting it surface as a JSON error.
func TestCompleteStreamRejectsATruncatedAnswer(t *testing.T) {
	c := testClient(t, `event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"{\"sessions\":[{\"week\":1,"}}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"max_tokens"},"usage":{"output_tokens":4000}}

`)

	_, err := c.CompleteStream(context.Background(), "", "test", "sys", "prompt", 4000, nil)
	if err == nil || !strings.Contains(err.Error(), "4000-token ceiling") {
		t.Fatalf("a truncated answer should name the ceiling, got: %v", err)
	}
}

func TestCompleteStreamReportsARefusal(t *testing.T) {
	c := testClient(t, `event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"refusal","stop_details":{"type":"refusal","category":"medical"}}}

`)

	_, err := c.CompleteStream(context.Background(), "", "test", "sys", "prompt", 4000, nil)
	if err == nil || !strings.Contains(err.Error(), "declined") {
		t.Fatalf("expected a refusal to be named, got: %v", err)
	}
}

func TestCompleteStreamSurfacesAPIErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`))
	}))
	defer server.Close()

	c := NewClient(nil, "bad-key", "claude-sonnet-5", "", "adaptive")
	c.baseURL = server.URL

	_, err := c.CompleteStream(context.Background(), "", "test", "sys", "prompt", 1000, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid x-api-key") {
		t.Fatalf("error should carry the API's message, got: %v", err)
	}
}

func TestRequestAsksForAdaptiveThinkingAndStreaming(t *testing.T) {
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		body = string(buf)
		_, _ = w.Write([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
	}))
	defer server.Close()

	c := NewClient(nil, "test-key", "claude-sonnet-5", "", "adaptive")
	c.baseURL = server.URL
	_, _ = c.CompleteStream(context.Background(), "", "test", "sys", "prompt", 1000, nil)

	for _, want := range []string{`"stream":true`, `"type":"adaptive"`, `"display":"summarized"`} {
		if !strings.Contains(body, want) {
			t.Errorf("request body missing %s: %s", want, body)
		}
	}

	off := NewClient(nil, "test-key", "claude-3-5-sonnet-latest", "", "off")
	off.baseURL = server.URL
	_, _ = off.CompleteStream(context.Background(), "", "test", "sys", "prompt", 1000, nil)
	if strings.Contains(body, "thinking") {
		t.Errorf("ANTHROPIC_THINKING=off should omit the parameter: %s", body)
	}
}
