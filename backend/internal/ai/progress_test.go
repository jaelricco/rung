package ai

import "testing"

func TestPlanTrackerCountsSessionsAcrossDeltaBoundaries(t *testing.T) {
	tracker := newPlanTracker(4)

	// The marker is split across two deltas, which is the normal case: the
	// model streams a few characters at a time.
	chunks := []string{
		`{"sessions":[{"week":1,"day_`, `of_week":1,"blocks":[]},`,
		`{"week":1,"day_of_week":3,"blocks":[]},`,
	}
	var last Progress
	for _, chunk := range chunks {
		last = tracker.update(Delta{Kind: "text", Text: chunk})
	}

	if tracker.sessions != 2 {
		t.Fatalf("counted %d sessions, want 2", tracker.sessions)
	}
	if last.Done != 2 || last.Total != 4 {
		t.Fatalf("progress said %d of %d, want 2 of 4", last.Done, last.Total)
	}
	if last.Percent <= thinkingCeiling || last.Percent >= writingCeiling {
		t.Fatalf("percent %d should sit inside the writing span", last.Percent)
	}
}

func TestProgressStaysInsideItsSpans(t *testing.T) {
	tracker := newPlanTracker(2)

	thinking := tracker.update(Delta{Kind: "thinking", Text: "reasoning"})
	if thinking.Percent < thinkingFloor || thinking.Percent > thinkingCeiling {
		t.Fatalf("thinking percent %d outside %d..%d", thinking.Percent, thinkingFloor, thinkingCeiling)
	}

	// Far more output than expected must not push the bar past its span.
	for i := 0; i < 50; i++ {
		tracker.update(Delta{Kind: "text", Text: `{"day_of_week":1}`})
	}
	last := tracker.update(Delta{Kind: "text", Text: "}"})
	if last.Percent > writingCeiling {
		t.Fatalf("percent %d ran past the writing ceiling", last.Percent)
	}
	if last.Done > last.Total {
		t.Fatalf("reported %d of %d sessions", last.Done, last.Total)
	}
}

func TestPlanTokensCoversReasoningAndCaps(t *testing.T) {
	if got := planTokens(24); got <= 8000 {
		t.Fatalf("an 8-week plan got %d tokens, too tight for reasoning plus answer", got)
	}
	if got := planTokens(168); got != 48000 {
		t.Fatalf("the ceiling should cap at 48000, got %d", got)
	}
}
