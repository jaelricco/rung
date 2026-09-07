package plan

import (
	"testing"

	"calisthenics/api/internal/training"
)

// The elite tier was added push-first — maltese, iron cross, planche press —
// and the pull side of it was never added at all, which is why the baseline
// page's "maximal" group had three entries and none of them was something you
// hang from. This is the assertion that keeps it honest: the top of the
// catalogue has to have both sides of the sport in it.
func TestTheMaximalTierHasBothSidesOfTheSport(t *testing.T) {
	patterns := map[string]int{}
	for _, goal := range Goals {
		if goal.Units() >= 3 {
			patterns[goal.Pattern]++
		}
	}
	if patterns[patternPush] == 0 {
		t.Error("no maximal skill is a pushing skill")
	}
	if patterns[patternPull] == 0 {
		t.Error("every maximal skill in the catalogue is a pushing skill — the pull side of the tier " +
			"is missing, which is what this test exists to notice")
	}
}

// A gate the athlete is never asked about is a gate they cannot pass: it reads
// "nothing logged or declared" for ever and holds them a rung below where they
// are. So everything a ladder gates on has to be something the baseline form
// puts a box next to.
func TestEveryGateIsSomethingTheBaselineAsksAbout(t *testing.T) {
	lib := seededLibrary(t)
	for _, goal := range Goals {
		asked := map[string]bool{}
		for _, b := range Benchmarks(goal.Name, lib) {
			asked[b.ExerciseSlug] = true
		}
		want := func(slug, why string) {
			if !lib.Has(slug) || asked[slug] {
				return
			}
			t.Errorf("%s is gated on %q (%s) and the baseline form never asks for it",
				goal.Key, slug, why)
		}
		for _, req := range goal.Entry {
			want(req.Slug, "entry standard")
		}
		for _, step := range goal.Ladder {
			for _, req := range step.Gate {
				want(req.Slug, "gate on "+step.Name)
			}
		}
	}
}

// The two new ladders, walked end to end: an athlete with a front lever is
// placed on the rung above it, an athlete without one is placed on the front
// lever and trains that, and the gates hold back the athlete whose records
// reach a rung they have not earned.
func TestThePullTierPlacesAndGates(t *testing.T) {
	lib := seededLibrary(t)

	// No front lever yet. The first rung of both ladders is the front lever
	// itself, so this is where they belong — and there is no "you may not
	// train toward this" note, because there is nothing to refuse.
	beginner := snapshotOf(12, 74, rec("pull_up", 12, 0, 0), rec("straddle_front_lever", 0, 0, 12))
	for _, goal := range []string{"SAT", "one arm front lever"} {
		p, _ := Generate(Request{Goal: goal, Weeks: 8, DaysPerWeek: 4}, beginner, lib)
		if p.Method.Rung != "Held front lever" {
			t.Errorf("%s with no front lever was placed on %q, not on the front lever", goal, p.Method.Rung)
		}
		if !p.Method.EntryMet {
			t.Errorf("%s should gate on its rungs rather than refuse the goal outright", goal)
		}
		if findBlock(p, "front_lever") == nil {
			t.Errorf("%s for someone without a front lever has to train the front lever", goal)
		}
	}

	// A long front lever opens the rung above it on both ladders.
	held := snapshotOf(20, 70, rec("pull_up", 18, 0, 0), rec("front_lever", 0, 0, 22))
	for goal, want := range map[string]string{"SAT": "Box victorian", "one arm front lever": "Assisted one arm"} {
		p, _ := Generate(Request{Goal: goal, Weeks: 8, DaysPerWeek: 4}, held, lib)
		if p.Method.Rung != want {
			t.Errorf("%s with a 22s front lever was placed on %q, want %q", goal, p.Method.Rung, want)
		}
	}

	// And the gate with teeth: somebody who has logged a tuck SAT but has
	// never shown a front lever touch is held one rung below it, and told
	// which number is missing.
	ungated := snapshotOf(20, 70, rec("pull_up", 18, 0, 0), rec("front_lever", 0, 0, 22),
		rec("box_victorian", 0, 0, 16), rec("wide_front_lever", 0, 0, 9), rec("tuck_sat", 0, 0, 6))
	p, _ := Generate(Request{Goal: "SAT", Weeks: 8, DaysPerWeek: 4}, ungated, lib)
	if p.Method.Rung == "Tuck SAT" {
		t.Error("the tuck SAT is gated on a front lever touch, and nothing was logged for one")
	}
	if len(p.Method.Gaps) == 0 {
		t.Fatal("a held-back athlete should be told which number is missing")
	}
	found := false
	for _, gap := range p.Method.Gaps {
		if gap.Name == "Front lever touch" {
			found = true
		}
	}
	if !found {
		t.Errorf("the gap should name the front lever touch, named %+v", p.Method.Gaps)
	}
}

// Both new skills are built on the front lever, so it keeps its place in the
// week rather than being dropped for the new one — the same rule the maltese
// taught, applied to the ladder it is the mirror of.
func TestThePullTierKeepsTheFrontLeverInTheWeek(t *testing.T) {
	lib := seededLibrary(t)
	snap := snapshotOf(20, 70, rec("pull_up", 18, 0, 0), rec("front_lever", 0, 0, 22),
		rec("box_victorian", 0, 0, 16))

	for _, goal := range []string{"SAT", "one arm front lever"} {
		p, _ := Generate(Request{Goal: goal, Weeks: 6, DaysPerWeek: 4}, snap, lib)
		kept := false
		for _, session := range p.Sessions {
			for _, block := range session.Blocks {
				if block.ExerciseSlug == "front_lever" || block.ExerciseSlug == "weighted_front_lever" {
					kept = true
				}
			}
		}
		if !kept {
			t.Errorf("%s dropped the front lever it is built on", goal)
		}
	}
}

// A wrist that has hurt for months does not delete the SAT: the forearms come
// off the bar and onto rings, where the ring turns with the arm. That is the
// §8 rule applied to a position §8 predates.
func TestAnAngryWristMovesTheSatOntoRings(t *testing.T) {
	lib := seededLibrary(t)
	snap := snapshotOf(20, 70, rec("pull_up", 18, 0, 0), rec("front_lever", 0, 0, 22),
		rec("box_victorian", 0, 0, 16))
	snap.OpenInjuries = []training.Injury{{Region: regionWrist, Severity: 2}}

	p, _ := Generate(Request{Goal: "SAT", Weeks: 6, DaysPerWeek: 4}, snap, lib)
	onBar, onRings := false, false
	for _, session := range p.Sessions {
		for _, block := range session.Blocks {
			switch block.ExerciseSlug {
			case "box_victorian", "tuck_sat", "adv_tuck_sat", "straddle_sat", "sat", "band_sat":
				onBar = true
			case "tuck_victorian", "victorian":
				onRings = true
			}
		}
	}
	if onBar {
		t.Error("a mild wrist should take the forearms off the bar, not leave them on it")
	}
	if !onRings {
		t.Error("the SAT has a neutral-wrist form and the plan should have moved onto it")
	}
}
