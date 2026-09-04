package plan

import (
	"encoding/json"
	"strings"
	"testing"
)

// The browser posts one object to whichever route the "sharpen it with AI"
// checkbox selects, and the decoder rejects unknown fields. So a field the AI
// endpoint added is a field this one has to accept, or unticking the box fails
// the request outright with "unknown field" — which is exactly what shipped.
func TestTheAlgorithmAcceptsTheBodyTheAIEndpointTakes(t *testing.T) {
	// Written as the plan page sends it, field for field.
	body := `{
		"skill": "Maltese",
		"goal": "Maltese",
		"weeks": 8,
		"days_per_week": 3,
		"starts_on": "2026-08-31",
		"notes": "slight wrist injury",
		"no_research": true,
		"save": false
	}`

	dec := json.NewDecoder(strings.NewReader(body))
	dec.DisallowUnknownFields()

	var in generateRequest
	if err := dec.Decode(&in); err != nil {
		t.Fatalf("the plan page's own body was rejected: %v", err)
	}
	if in.goal() != "Maltese" || in.Weeks != 8 || in.DaysPerWeek != 3 {
		t.Fatalf("decoded %+v", in)
	}
	if in.Notes != "slight wrist injury" {
		t.Fatalf("notes = %q", in.Notes)
	}
}

// Accepted, but it changes nothing here: this planner reads the athlete's own
// records and never searches, so there is no research for the flag to skip.
func TestNoResearchIsInertForTheAlgorithm(t *testing.T) {
	var with, without generateRequest
	for _, tc := range []struct {
		body string
		into *generateRequest
	}{
		{`{"goal":"front lever","weeks":4,"days_per_week":3,"no_research":true}`, &with},
		{`{"goal":"front lever","weeks":4,"days_per_week":3}`, &without},
	} {
		if err := json.Unmarshal([]byte(tc.body), tc.into); err != nil {
			t.Fatalf("decode: %v", err)
		}
	}

	with.NoResearch = without.NoResearch
	if with != without {
		t.Fatal("no_research changed something on the algorithm's request")
	}
}
