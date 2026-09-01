package plan

import (
	"strings"
	"testing"
)

// The knowledge base names slugs in Go and the database seeds them in SQL.
// Nothing in the compiler connects the two, so this is what does: a renamed or
// dropped exercise fails here rather than quietly thinning out every plan that
// used it.
func TestEverySlugInTheKnowledgeBaseExists(t *testing.T) {
	lib := seededLibrary(t)

	check := func(where string, slugs chain) {
		for _, slug := range slugs {
			if !lib.Has(slug) {
				t.Errorf("%s names %q, which is not in the exercise library", where, slug)
			}
		}
	}
	for _, goal := range Goals {
		check(goal.Key+" drills", goal.Drills)
		check(goal.Key+" accessories", goal.Accessories)
		for i, step := range goal.Ladder {
			check(goal.Key+" rung "+step.Name, step.Movement)
			check(goal.Key+" rung "+step.Name+" assist", step.Assist)
			if step.Standard <= 0 {
				t.Errorf("%s rung %d has no standard, so nothing clears it", goal.Key, i)
			}
			if step.Metric == "" {
				t.Errorf("%s rung %d has no metric", goal.Key, i)
			}
		}
		if goal.Phrase == "" || goal.Timeline == "" {
			t.Errorf("%s is missing the prose a plan is written from", goal.Key)
		}
	}

	for slug := range wristLoaded {
		if !lib.Has(slug) {
			t.Errorf("the wrist-loading list names %q, which is not in the library", slug)
		}
	}
	for slug := range extraRegions {
		if !lib.Has(slug) {
			t.Errorf("the region map names %q, which is not in the library", slug)
		}
	}
	for region, slug := range warmupFor {
		if _, ok := lib.Protocols[slug]; !ok {
			t.Errorf("the %s warm-up points at %q, which is not a protocol", region, slug)
		}
	}
	for region, slug := range rehabFor {
		if _, ok := lib.Protocols[slug]; !ok {
			t.Errorf("the %s rehab points at %q, which is not a protocol", region, slug)
		}
	}
}

// A ladder that gets easier as it goes is a ladder nobody can be placed on
// correctly, since placement walks it in order.
func TestLaddersGetHarderNotEasier(t *testing.T) {
	lib := seededLibrary(t)
	for _, goal := range Goals {
		for i := 1; i < len(goal.Ladder); i++ {
			prev, step := goal.Ladder[i-1], goal.Ladder[i]
			if prev.Metric != step.Metric || len(prev.Movement) == 0 || len(step.Movement) == 0 {
				continue
			}
			if prev.Movement[0] == step.Movement[0] {
				// The same movement twice is a longer hold or a heavier belt,
				// so its standard must rise.
				if step.Standard <= prev.Standard {
					t.Errorf("%s: %q repeats %q without asking for more (%v then %v)",
						goal.Key, step.Name, prev.Movement[0], prev.Standard, step.Standard)
				}
				continue
			}
			// Different movements: the harder one is allowed a lower number,
			// but it should not be an easier exercise.
			if lib.Exercises[step.Movement[0]].Difficulty < lib.Exercises[prev.Movement[0]].Difficulty {
				t.Errorf("%s: %q (%s, difficulty %d) comes after %q (difficulty %d)",
					goal.Key, step.Name, step.Movement[0], lib.Exercises[step.Movement[0]].Difficulty,
					prev.Movement[0], lib.Exercises[prev.Movement[0]].Difficulty)
			}
		}
	}
}

func TestValidateHoldsAPlanToTheLibrary(t *testing.T) {
	lib := seededLibrary(t)
	p := Plan{Weeks: 4, Sessions: []Session{
		{Week: 1, DayOfWeek: 1, Blocks: []Block{
			{ExerciseSlug: "pull_up", Sets: 3},
			{ExerciseSlug: "invented_move", Sets: 3},
		}, WarmupProtocols: []string{"general_warmup", "made_up_protocol"}},
		{Week: 2, DayOfWeek: 1, Blocks: []Block{{ExerciseSlug: "also_invented"}}},
		{Week: 9, DayOfWeek: 1, Blocks: []Block{{ExerciseSlug: "dip", Sets: 3}}},
		{Week: 1, DayOfWeek: 9, Blocks: []Block{{ExerciseSlug: "dip", Sets: 0, RestSeconds: 99999}}},
	}}

	warnings := Validate(&p, lib, 4)
	if len(p.Sessions) != 2 {
		t.Fatalf("kept %d sessions, want the two with usable blocks", len(p.Sessions))
	}
	if len(p.Sessions[0].Blocks) != 1 || p.Sessions[0].Blocks[0].ExerciseSlug != "pull_up" {
		t.Errorf("the invented block should have been dropped: %+v", p.Sessions[0].Blocks)
	}
	if len(p.Sessions[0].WarmupProtocols) != 1 {
		t.Errorf("the invented protocol should have been dropped: %v", p.Sessions[0].WarmupProtocols)
	}
	if p.Sessions[1].DayOfWeek != 1 || p.Sessions[1].Blocks[0].Sets != 1 || p.Sessions[1].Blocks[0].RestSeconds != 0 {
		t.Errorf("out-of-range fields should be corrected, not carried: %+v", p.Sessions[1])
	}
	for _, want := range []string{"invented_move", "past the end", "lost every block", "made_up_protocol"} {
		if !containsAny(warnings, want) {
			t.Errorf("nothing warned about %q; warnings were %v", want, warnings)
		}
	}
}

func TestTheLibraryCatalogueReadsAsOne(t *testing.T) {
	lib := seededLibrary(t)
	text := lib.Text()
	if !strings.Contains(text, "EXERCISE LIBRARY") || !strings.Contains(text, "PROTOCOL LIBRARY") {
		t.Fatal("the catalogue is missing one of its two halves")
	}
	if !strings.Contains(text, "pull_up") || !strings.Contains(text, "general_warmup") {
		t.Error("the catalogue does not list what it is supposed to list")
	}
	if lib.Text() != text {
		t.Error("the catalogue has to render the same way twice, or every prompt differs")
	}
}
