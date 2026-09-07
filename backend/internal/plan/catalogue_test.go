package plan

import (
	"testing"

	"calisthenics/api/internal/training"
)

// The reference tables carry foreign keys onto exercises, which means a slug
// in any of these maps that is not in the library stops the app at boot rather
// than producing a quieter plan. That is a better failure than the old one —
// but only if it is caught here, before it reaches a server.
func TestEverySlugTheCatalogueProjectsIsReal(t *testing.T) {
	lib := seededLibrary(t)

	check := func(where, slug string) {
		if !lib.Has(slug) {
			t.Errorf("%s names %q, which is not in the exercise library — the projection's "+
				"foreign key would refuse it at startup", where, slug)
		}
	}

	for slug, target := range neutralWrist {
		check("the wrist substitution map", slug)
		check("the wrist substitution target for "+slug, target)
		if slug == target {
			t.Errorf("%q substitutes for itself", slug)
		}
		// A substitution has to actually spare the wrist, or it is just a
		// different exercise wearing the same excuse.
		if wristLoaded[target] {
			t.Errorf("%q substitutes onto %q, which also loads the wrist", slug, target)
		}
	}

	for _, r := range training.Rubrics() {
		check("the level rubric table", r.Slug)
		if len(r.Cuts) != 5 {
			t.Errorf("rubric for %q has %d cut points, want 5", r.Slug, len(r.Cuts))
		}
		for i := 1; i < len(r.Cuts); i++ {
			if r.Cuts[i] <= r.Cuts[i-1] {
				t.Errorf("rubric for %q does not rise: %v", r.Slug, r.Cuts)
			}
		}
	}

	for _, goal := range Goals {
		for _, req := range goal.Entry {
			check(goal.Key+" entry requirement", req.Slug)
		}
		for _, step := range goal.Ladder {
			for _, req := range step.Gate {
				check(goal.Key+" gate on "+step.Name, req.Slug)
			}
		}
		for _, key := range goal.Feeds {
			if _, ok := goalByKey[key]; !ok {
				t.Errorf("%s feeds from %q, which is not a skill", goal.Key, key)
			}
		}
	}
}

// The projection writes one row per skill and per rung, keyed by position, so
// duplicate keys or names would collide on insert.
func TestTheCatalogueProjectsCleanly(t *testing.T) {
	seen := map[string]bool{}
	for _, goal := range Goals {
		if seen[goal.Key] {
			t.Errorf("two skills share the key %q", goal.Key)
		}
		seen[goal.Key] = true

		if goal.Pattern == "" || goal.Name == "" {
			t.Errorf("%s is missing a name or a pattern, both of which are not null", goal.Key)
		}
		if goal.Cost < 0 || goal.Cost > 5 {
			t.Errorf("%s costs %d, which is outside anything the budget can price", goal.Key, goal.Cost)
		}

		steps := map[string]bool{}
		for _, step := range goal.Ladder {
			if steps[step.Name] {
				t.Errorf("%s has two rungs called %q", goal.Key, step.Name)
			}
			steps[step.Name] = true
			if len(step.Movement) == 0 {
				t.Errorf("%s rung %q has no movement, so nothing can be prescribed for it",
					goal.Key, step.Name)
			}
		}
	}

	for _, region := range training.Regions {
		if region.Key == "" || region.Label == "" {
			t.Errorf("an injury region is missing its key or label: %+v", region)
		}
	}
	for _, item := range Equipment {
		if item.Key == "" || item.Label == "" {
			t.Errorf("an equipment entry is missing its key or label: %+v", item)
		}
	}
}

// The projection has to write the same rows in the same order on every boot,
// or a restart looks like a change.
func TestTheProjectionIsOrdered(t *testing.T) {
	first := sortedKeys(neutralWrist)
	for i := 0; i < 20; i++ {
		if next := sortedKeys(neutralWrist); len(next) != len(first) {
			t.Fatal("key ordering is not stable")
		} else {
			for j := range next {
				if next[j] != first[j] {
					t.Fatalf("key ordering changed between runs at %d: %q vs %q", j, next[j], first[j])
				}
			}
		}
	}
}
