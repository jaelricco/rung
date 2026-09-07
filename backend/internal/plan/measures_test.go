package plan

import (
	"strings"
	"testing"
)

// The invariant that would have caught this before it shipped.
//
// A rung says what it is measured in and an exercise says what a set of it
// looks like, and the planner prescribes from the second while placement reads
// the first. When they disagree, nothing errors: the block is simply written
// in the wrong units, and the number the rung is about disappears. That is
// exactly what happened to the weighted front lever — the rung asked for
// kilos, the exercise claimed seconds, and every plan past a twenty-second
// front lever prescribed a hold with no mention of the belt.
func TestEveryLadderMetricMatchesItsExercisesMeasure(t *testing.T) {
	lib := seededLibrary(t)

	// What a metric may be measured on. Added load is the one with two: a
	// weighted pull-up is reps with a belt and a weighted front lever is
	// seconds with a belt, and both are "how many kilos" to a ladder.
	allowed := map[string][]string{
		metricHold:    {"static_hold", "weighted_hold"},
		metricReps:    {"reps", "weighted_reps"},
		metricAdded:   {"weighted_reps", "weighted_hold"},
		metricAttempt: {"skill_attempt"},
	}
	ok := func(metric, measure string) bool {
		for _, want := range allowed[metric] {
			if want == measure {
				return true
			}
		}
		return false
	}

	check := func(where, slug, metric string) {
		exercise, found := lib.Exercises[slug]
		if !found {
			return // TestEverySlugInTheKnowledgeBaseExists owns that failure
		}
		if !ok(metric, exercise.Measure) {
			t.Errorf("%s is measured in %q but %q is a %q exercise, so the block would be written "+
				"in the wrong units and the number the rung is about would vanish",
				where, metric, slug, exercise.Measure)
		}
	}

	for _, goal := range Goals {
		for _, step := range goal.Ladder {
			for _, slug := range step.Movement {
				check(goal.Key+" rung "+step.Name, slug, step.Metric)
			}
			for _, req := range step.Gate {
				check(goal.Key+" gate on "+step.Name, req.Slug, req.Metric)
			}
		}
		for _, req := range goal.Entry {
			check(goal.Key+" entry standard", req.Slug, req.Metric)
		}
	}
}

// And the other half: every measure the library uses has a prescription path.
// A measure the session builder does not know about falls through to the
// default, which writes reps — silently, and for a static hold that is
// nonsense rather than an error.
func TestEveryMeasureInTheLibraryIsPrescribable(t *testing.T) {
	lib := seededLibrary(t)
	snap := snapshotOf(20, 72, rec("pull_up", 18, 0, 0), rec("front_lever", 0, 0, 35),
		rec("weighted_front_lever", 0, 12, 14))

	seen := map[string]bool{}
	for _, e := range lib.Exercises {
		seen[e.Measure] = true
	}
	for measure := range seen {
		switch measure {
		case "reps", "weighted_reps", "static_hold", "weighted_hold", "skill_attempt":
		default:
			t.Errorf("the library uses the measure %q, which the session builder has no case for "+
				"and would prescribe as plain reps", measure)
		}
	}

	// The one that was broken, end to end.
	p, _ := Generate(Request{Goal: "front lever", Weeks: 8, DaysPerWeek: 4}, snap, lib)
	block := findBlock(p, "weighted_front_lever")
	if block == nil {
		t.Fatal("a front lever plan for someone with a 35s lever has to reach the weighted rung")
	}
	if !strings.Contains(block.Prescription, "hold") {
		t.Errorf("a weighted front lever is held, and the block says %q", block.Prescription)
	}
	if !strings.Contains(block.Prescription, "kg") {
		t.Errorf("the belt is the whole point of this rung, and the block says %q", block.Prescription)
	}
	if !strings.Contains(block.Intensity, "logged on it") {
		t.Errorf("the block should say where its load came from, said %q", block.Intensity)
	}
	if !strings.Contains(p.Test, "held") || !strings.Contains(p.Test, "kg") {
		t.Errorf("the test week should be passed or failed on seconds and kilos, said %q", p.Test)
	}
}

// With nothing logged on the belt, the load comes from the rung's own standard
// and the block says so rather than inventing a number.
func TestAnUnloadedAthleteGetsAStartingLoadAndIsToldWhereItCameFrom(t *testing.T) {
	lib := seededLibrary(t)
	snap := snapshotOf(20, 72, rec("pull_up", 18, 0, 0), rec("front_lever", 0, 0, 32))

	p, _ := Generate(Request{Goal: "front lever", Weeks: 8, DaysPerWeek: 4}, snap, lib)
	block := findBlock(p, "weighted_front_lever")
	if block == nil {
		t.Fatal("a 32s front lever should reach the weighted rung")
	}
	if !strings.Contains(block.Prescription, "kg") {
		t.Errorf("even a first weighted session needs a load: %q", block.Prescription)
	}
	if !strings.Contains(block.Intensity, "half of the") {
		t.Errorf("the block should say the load is half the rung's standard, said %q", block.Intensity)
	}
}
