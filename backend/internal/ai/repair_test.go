package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// textFrames is one streamed answer, in the shape the Anthropic transport reads.
func textFrames(text string) string {
	payload, _ := json.Marshal(text)
	return "event: message_start\n" +
		`data: {"type":"message_start","message":{"usage":{"input_tokens":10}}}` + "\n\n" +
		"event: content_block_delta\n" +
		fmt.Sprintf(`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":%s}}`, payload) +
		"\n\n" +
		"event: message_delta\n" +
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":20}}` + "\n\n"
}

// scripted answers each successive request with the next body, and remembers
// every request it was sent so a test can assert what went on the wire.
func scripted(t *testing.T, bodies ...string) (*Client, *[]string) {
	t.Helper()
	var (
		mu   sync.Mutex
		seen []string
		n    int
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = append(seen, string(body))
		answer := bodies[len(bodies)-1]
		if n < len(bodies) {
			answer = bodies[n]
		}
		n++
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(server.Close)
	return clientAt(server.URL, ProviderAnthropic, "claude-sonnet-5", "adaptive"), &seen
}

// The exact failure that cost a real research turn: the model put a quoted
// phrase inside a JSON string without escaping it. One repair turn recovers
// the call instead of losing everything that was paid to produce it.
func TestAnUnescapedQuoteIsRepairedRatherThanLost(t *testing.T) {
	broken := `{"skill":"pull-up","summary":"more than lack of "back strength.""}`
	fixed := `{"skill":"pull-up","summary":"more than lack of \"back strength.\""}`

	c, seen := scripted(t, textFrames(broken), textFrames(fixed))

	var out SkillResearch
	if err := c.CompleteJSONStream(context.Background(), "", "skill_research", "sys", "", "prompt",
		4000, researchSchema, nil, &out); err != nil {
		t.Fatalf("CompleteJSONStream: %v", err)
	}
	if out.Skill != "pull-up" {
		t.Fatalf("parsed %+v", out)
	}
	if !strings.Contains(out.Summary, `"back strength."`) {
		t.Fatalf("summary lost its quotes: %q", out.Summary)
	}
	if len(*seen) != 2 {
		t.Fatalf("expected the answer and one repair, got %d requests", len(*seen))
	}

	// The repair must not drag the expensive half of the call along with it:
	// what made the answer costly was the retrieved pages, and re-sending them
	// would cost as much again.
	repair := (*seen)[1]
	if strings.Contains(repair, "prompt") {
		t.Error("the repair turn re-sent the original prompt")
	}
	if !strings.Contains(repair, "back strength") {
		t.Error("the repair turn did not carry the broken document")
	}
}

// A repair that fails must report the model's original mistake, not the
// repair's — the first error is the one that describes what went wrong.
func TestAFailedRepairReportsTheOriginalComplaint(t *testing.T) {
	broken := `{"skill":"pull-up","summary":"x" "y"}`
	c, seen := scripted(t, textFrames(broken), textFrames(`still not json`))

	var out SkillResearch
	err := c.CompleteJSONStream(context.Background(), "", "skill_research", "sys", "", "prompt",
		4000, researchSchema, nil, &out)
	if err == nil {
		t.Fatal("a document that never parsed was accepted")
	}
	if !strings.Contains(err.Error(), "wasn't usable JSON") {
		t.Fatalf("error = %v", err)
	}
	if len(*seen) != 2 {
		t.Fatalf("expected exactly one repair attempt, got %d requests", len(*seen))
	}
}

// An answer that parses first time must not spend a second turn.
func TestGoodJSONCostsOnlyOneTurn(t *testing.T) {
	c, seen := scripted(t, textFrames(`{"skill":"pull-up","summary":"fine"}`))

	var out SkillResearch
	if err := c.CompleteJSONStream(context.Background(), "", "skill_research", "sys", "", "prompt",
		4000, researchSchema, nil, &out); err != nil {
		t.Fatalf("CompleteJSONStream: %v", err)
	}
	if len(*seen) != 1 {
		t.Fatalf("a clean answer cost %d requests", len(*seen))
	}
}

// The schema has to reach the provider on the plain turn — a schema that
// stays in Go enforces nothing.
func TestTheSchemaReachesTheProviderOnAStreamedTurn(t *testing.T) {
	c, seen := scripted(t, textFrames(`{"skill":"pull-up","summary":"fine"}`))

	var out SkillResearch
	if err := c.CompleteJSONStream(context.Background(), "", "skill_research", "sys", "", "prompt",
		4000, researchSchema, nil, &out); err != nil {
		t.Fatalf("CompleteJSONStream: %v", err)
	}

	var sent struct {
		OutputConfig *struct {
			Format *struct {
				Type   string          `json:"type"`
				Schema json.RawMessage `json:"schema"`
			} `json:"format"`
		} `json:"output_config"`
	}
	if err := json.Unmarshal([]byte((*seen)[0]), &sent); err != nil {
		t.Fatalf("request body: %v", err)
	}
	if sent.OutputConfig == nil || sent.OutputConfig.Format == nil {
		t.Fatal("the streamed turn carried no output_config")
	}
	if sent.OutputConfig.Format.Type != "json_schema" {
		t.Fatalf("format type = %q", sent.OutputConfig.Format.Type)
	}
	var got, want any
	_ = json.Unmarshal(sent.OutputConfig.Format.Schema, &got)
	_ = json.Unmarshal(researchSchema, &want)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Error("the schema on the wire is not the one that was asked for")
	}
}

// A prose turn must stay prose: no schema, no output_config, nothing that
// would constrain a four-week review into JSON.
func TestAProseTurnCarriesNoSchema(t *testing.T) {
	c, seen := scripted(t, textFrames("Your last four weeks looked strong."))

	if _, err := c.CompleteStream(context.Background(), "", "review", "sys", "prompt", 4000, nil); err != nil {
		t.Fatalf("CompleteStream: %v", err)
	}
	if strings.Contains((*seen)[0], "output_config") {
		t.Error("a prose turn carried an output_config")
	}
}

func TestRepairTokensLeaveRoomForTheWholeDocumentBack(t *testing.T) {
	if got := repairTokens("tiny"); got != defaultMaxTokens {
		t.Errorf("a short document got %d, wanted the ordinary ceiling %d", got, defaultMaxTokens)
	}
	// A research answer is around 9kB; the repair rewrites all of it.
	big := strings.Repeat("x", 90_000)
	if got := repairTokens(big); got <= defaultMaxTokens {
		t.Errorf("a 90kB document got only %d tokens to come back in", got)
	}
	if got := repairTokens(strings.Repeat("x", 10_000_000)); got != 64000 {
		t.Errorf("the ceiling is not capped: %d", got)
	}
}

// The ceiling regression. A measured six-session plan spent 11,900 output
// tokens and was still cut off mid-block under the old 8000+650/session rule,
// which gave exactly 11,900. Whatever the formula becomes, it has to leave
// real room above what a real plan was observed to need.
func TestPlanCeilingClearsWhatARealPlanNeeded(t *testing.T) {
	const measuredAndStillTruncated = 11900

	if got := planTokens(6); got <= measuredAndStillTruncated {
		t.Fatalf("planTokens(6) = %d, which a real six-session plan already overran", got)
	}
	// Every session must buy meaningfully more room than the old rule gave,
	// up to the point where the cap takes over.
	for _, sessions := range []int{3, 6, 12} {
		perSession := (planTokens(sessions) - planTokens(0)) / sessions
		if perSession < 2600 {
			t.Errorf("planTokens gives %d tokens a session at %d sessions; a measured "+
				"session needed about 2600", perSession, sessions)
		}
	}
	// Past the cap the clock binds before the ceiling does, and a plan that
	// long needs writing in parts rather than a larger number here.
	if planTokens(1000) != 64000 {
		t.Errorf("the ceiling is not capped: %d", planTokens(1000))
	}
}

// And on the search turn, which is the expensive one and therefore the one it
// most matters for: the retrieved pages are billed as input, so a research
// answer that will not parse is the costliest thing this app can produce.
func TestTheSchemaReachesTheProviderOnASearchTurn(t *testing.T) {
	var (
		mu   sync.Mutex
		seen []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = append(seen, string(body))
		mu.Unlock()
		_, _ = w.Write([]byte(`{
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 83000, "output_tokens": 5000},
			"content": [{"type": "text", "text": "{\"skill\":\"pull-up\",\"summary\":\"fine\"}"}]
		}`))
	}))
	defer server.Close()

	c := clientAt(server.URL, ProviderAnthropic, "claude-sonnet-5", "adaptive")
	var out SkillResearch
	if _, err := c.SearchJSON(context.Background(), "", "skill_research", "sys", "prompt", 24000,
		SearchOptions{MaxSearches: 7, Schema: researchSchema}, &out); err != nil {
		t.Fatalf("SearchJSON: %v", err)
	}
	if out.Skill != "pull-up" {
		t.Fatalf("parsed %+v", out)
	}

	var sent struct {
		Tools        []map[string]any `json:"tools"`
		OutputConfig *struct {
			Format *struct {
				Type string `json:"type"`
			} `json:"format"`
		} `json:"output_config"`
	}
	if err := json.Unmarshal([]byte(seen[0]), &sent); err != nil {
		t.Fatalf("request body: %v", err)
	}
	if sent.OutputConfig == nil || sent.OutputConfig.Format == nil {
		t.Fatal("the search turn carried no output_config")
	}
	if sent.OutputConfig.Format.Type != "json_schema" {
		t.Fatalf("format type = %q", sent.OutputConfig.Format.Type)
	}
	// The schema constrains the answer; the tool still has to be there to
	// produce one worth constraining.
	if len(sent.Tools) != 1 {
		t.Fatalf("the search turn carried %d tools", len(sent.Tools))
	}
}
