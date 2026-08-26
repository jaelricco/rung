package ai

import (
	"fmt"
	"strings"
	"time"

	"calisthenics/api/internal/httpx"
)

// Progress is one step of an AI call, as the browser sees it. Percent is a
// real fraction of the work, not a timer: while the model is reasoning it
// tracks how much reasoning has arrived, and while it is writing a plan it
// tracks how many sessions of that plan exist so far.
type Progress struct {
	Stage   string `json:"stage"`
	Label   string `json:"label"`
	Percent int    `json:"percent"`
	Detail  string `json:"detail,omitempty"`
	Done    int    `json:"done,omitempty"`
	Total   int    `json:"total,omitempty"`
	// Indeterminate marks the one phase that cannot report a fraction of
	// itself: web search is a single request that says nothing until it
	// returns. The browser sweeps the bar rather than inventing a number.
	Indeterminate bool `json:"indeterminate,omitempty"`
}

// progressSink writes Progress to an SSE stream. A nil stream makes every
// report a no-op, which is how the plain JSON responses reuse the same code.
type progressSink struct {
	stream  *httpx.Stream
	percent int
	stage   string
	last    time.Time
}

// report sends an update, skipping frames that say nothing new. Percent never
// goes backwards: a bar that retreats looks broken even when it is honest.
func (p *progressSink) report(u Progress) {
	if p == nil || p.stream == nil {
		return
	}
	if u.Percent < p.percent {
		u.Percent = p.percent
	}
	if u.Percent == p.percent && u.Stage == p.stage && time.Since(p.last) < 500*time.Millisecond {
		return
	}
	p.percent, p.stage, p.last = u.Percent, u.Stage, time.Now()
	p.stream.Send("progress", u)
}

// ---------- trackers: deltas in, Progress out ----------

// The phases run in order and each owns a slice of the bar: research, then
// reasoning, then writing. Research cannot report a fraction of itself, so it
// holds at its floor and sweeps; the two after it are measured.
const (
	researchFloor   = 4
	researchCeiling = 12
	thinkingFloor   = 13
	thinkingCeiling = 26
	writingCeiling  = 92
	// How much reasoning counts as "a lot", in characters of summary.
	thinkingFull = 5000
)

type planTracker struct {
	expected int
	thinking int
	written  int
	sessions int
	tail     string
	thought  string
}

func newPlanTracker(expected int) *planTracker {
	if expected < 1 {
		expected = 1
	}
	return &planTracker{expected: expected}
}

// sessionMarker appears exactly once per session object in the plan JSON, so
// counting it counts finished sessions as they stream in.
const sessionMarker = `"day_of_week"`

func (t *planTracker) update(d Delta) Progress {
	if d.Kind == "thinking" {
		t.thinking += len(d.Text)
		t.thought = lastLine(t.thought + d.Text)
		return Progress{
			Stage:   "thinking",
			Label:   "Working out the progression",
			Percent: span(thinkingFloor, thinkingCeiling, float64(t.thinking)/thinkingFull),
			Detail:  clip(t.thought, 160),
			Total:   t.expected,
		}
	}

	t.written += len(d.Text)
	t.sessions += countMarker(&t.tail, d.Text)

	// Two readings of how far along the writing is: sessions actually
	// emitted, and sheer volume. The larger one is used, so the bar keeps
	// moving even if the model formats the plan differently than expected.
	bySession := float64(t.sessions) / float64(t.expected)
	byVolume := float64(t.written) / float64(t.expected*320)
	done := t.sessions
	if done > t.expected {
		done = t.expected
	}

	label := "Drafting the plan"
	if t.sessions > 0 {
		label = fmt.Sprintf("Writing session %d of %d", done, t.expected)
	}
	return Progress{
		Stage:   "writing",
		Label:   label,
		Percent: span(thinkingCeiling, writingCeiling, max(bySession, byVolume)),
		Done:    done,
		Total:   t.expected,
	}
}

// proseTracker is the same idea for the answers that are prose rather than a
// plan: there is nothing to count but words, so length carries the bar.
type proseTracker struct {
	expected int
	thinking int
	written  int
	thought  string
	label    string
}

func newProseTracker(expectedChars int, label string) *proseTracker {
	if expectedChars < 1 {
		expectedChars = 2000
	}
	return &proseTracker{expected: expectedChars, label: label}
}

func (t *proseTracker) update(d Delta) Progress {
	if d.Kind == "thinking" {
		t.thinking += len(d.Text)
		t.thought = lastLine(t.thought + d.Text)
		return Progress{
			Stage:   "thinking",
			Label:   t.label,
			Percent: span(thinkingFloor, thinkingCeiling, float64(t.thinking)/thinkingFull),
			Detail:  clip(t.thought, 160),
		}
	}
	t.written += len(d.Text)
	return Progress{
		Stage:   "writing",
		Label:   "Writing it up",
		Percent: span(thinkingCeiling, writingCeiling, float64(t.written)/float64(t.expected)),
	}
}

// ---------- helpers ----------

// span maps a 0..1 fraction onto a slice of the bar.
func span(from, to int, fraction float64) int {
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	return from + int(float64(to-from)*fraction)
}

// countMarker counts occurrences of sessionMarker across delta boundaries by
// keeping just enough of the previous chunk to catch a split marker.
func countMarker(tail *string, chunk string) int {
	buf := *tail + chunk
	n := strings.Count(buf, sessionMarker)
	if keep := len(sessionMarker) - 1; len(buf) > keep {
		*tail = buf[len(buf)-keep:]
	} else {
		*tail = buf
	}
	return n
}

// lastLine keeps only the sentence the model is currently on, so the detail
// line under the bar reads as one thought instead of a growing wall.
func lastLine(s string) string {
	if i := strings.LastIndexAny(strings.TrimRight(s, "\n"), "\n"); i >= 0 {
		s = s[i+1:]
	}
	if len(s) > 400 {
		s = s[len(s)-400:]
	}
	return strings.TrimSpace(s)
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	// Trim to a rune boundary, then to the last space, so the tail is a word.
	s = strings.ToValidUTF8(s[len(s)-n:], "")
	if i := strings.IndexByte(s, ' '); i >= 0 {
		s = s[i+1:]
	}
	return "…" + s
}
