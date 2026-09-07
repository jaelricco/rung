package plan

import (
	"strings"
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

// The SAT and the victorian are one skill on two implements, so they place an
// athlete the same way — and differ in exactly the thing the implement decides:
// the bar version loads a wrist and the rings version does not, which is why
// one is the other's substitution.
func TestTheVictorianIsTheSatOnRings(t *testing.T) {
	lib := seededLibrary(t)
	held := snapshotOf(20, 70, rec("pull_up", 18, 0, 0), rec("front_lever", 0, 0, 22))

	for _, goal := range []string{"SAT", "victorian"} {
		p, _ := Generate(Request{Goal: goal, Weeks: 8, DaysPerWeek: 4}, held, lib)
		if p.Method.Ladder[0].Name != "Held front lever" {
			t.Errorf("%s starts on %q, not on the front lever both are built from",
				goal, p.Method.Ladder[0].Name)
		}
	}

	for _, slug := range []string{"tuck_victorian", "one_leg_victorian", "straddle_victorian", "victorian"} {
		if wristLoaded[slug] {
			t.Errorf("%q is on the wrist-loading list, which would make it useless as the SAT's "+
				"neutral-wrist substitution", slug)
		}
	}
	for _, slug := range []string{"tuck_sat", "adv_tuck_sat", "straddle_sat", "sat"} {
		if !wristLoaded[slug] {
			t.Errorf("%q rests the forearms on a bar under a horizontal body and is not marked "+
				"wrist-loading", slug)
		}
		if neutralWrist[slug] == "" {
			t.Errorf("%q has no neutral-wrist version, and the rings hold it every rung", slug)
		}
	}
}

// The hefesto is the one elite skill here that is a pull, and its ladder is
// mostly the skills it passes through: a german hang, a back lever, a korean
// dip. An athlete is placed on whichever of those they are actually at.
func TestTheHefestoIsBuiltFromWhatItPassesThrough(t *testing.T) {
	lib := seededLibrary(t)

	cases := []struct {
		who  string
		snap training.Snapshot
		rung string
	}{
		{"nothing behind the back yet",
			snapshotOf(10, 74, rec("pull_up", 10, 0, 0), rec("dip", 9, 0, 0)), "Shoulder extension"},
		{"a german hang and no lever",
			snapshotOf(14, 74, rec("pull_up", 12, 0, 0), rec("dip", 16, 0, 0),
				rec("german_hang", 0, 0, 40)), "Back lever"},
		{"a back lever and dips",
			snapshotOf(18, 74, rec("pull_up", 14, 0, 0), rec("dip", 18, 0, 0),
				rec("german_hang", 0, 0, 40), rec("back_lever", 0, 0, 14)), "Korean dip"},
	}
	for _, c := range cases {
		p, _ := Generate(Request{Goal: "hefesto", Weeks: 8, DaysPerWeek: 4}, c.snap, lib)
		if p.Method.Rung != c.rung {
			t.Errorf("hefesto for %s placed on %q, want %q", c.who, p.Method.Rung, c.rung)
		}
	}

	// The korean dip is gated on a dip base and on shoulder extension, and
	// both gates have to bite. Somebody with a long german hang and nine dips
	// is not ready to put their shoulders behind them under load.
	thin := snapshotOf(18, 74, rec("pull_up", 14, 0, 0), rec("dip", 9, 0, 0),
		rec("german_hang", 0, 0, 40), rec("back_lever", 0, 0, 14), rec("korean_dip", 4, 0, 0))
	p, _ := Generate(Request{Goal: "hefesto", Weeks: 8, DaysPerWeek: 4}, thin, lib)
	if p.Method.Rung == "Korean dip" {
		t.Error("the korean dip is gated on fifteen strict dips, and nine were logged")
	}
	named := false
	for _, gap := range p.Method.Gaps {
		if gap.Name == "Dip" {
			named = true
		}
	}
	if !named {
		t.Errorf("the gap should name the dip base, named %+v", p.Method.Gaps)
	}
}

// An injury that takes the rung away should cost the athlete the top of the
// ladder, not the skill. The hefesto is the clearest case: its first two rungs
// are a german hang and a back lever, neither of which touches a wrist, while
// everything above them finishes with bodyweight on the palms behind the body.
func TestAnInjuryStepsDownTheLadderRatherThanDeletingTheSkill(t *testing.T) {
	lib := seededLibrary(t)
	snap := snapshotOf(18, 74, rec("pull_up", 14, 0, 0), rec("dip", 18, 0, 0),
		rec("german_hang", 0, 0, 40), rec("back_lever", 0, 0, 14), rec("korean_dip", 6, 0, 0))
	snap.OpenInjuries = []training.Injury{{Region: regionWrist, Severity: 2}}

	p, _ := Generate(Request{Goal: "hefesto", Weeks: 6, DaysPerWeek: 4}, snap, lib)
	if p.Method.Rung != "Back lever" {
		t.Errorf("a sore wrist should step the hefesto down to the back lever, placed on %q", p.Method.Rung)
	}
	if findBlock(p, "back_lever") == nil {
		t.Error("the rung it stepped down to should be in the plan")
	}
	for _, slug := range []string{"korean_dip", "hefesto_negative", "hefesto"} {
		if findBlock(p, slug) != nil {
			t.Errorf("%q loads the wrist and should not be in a plan written around one", slug)
		}
	}
	said := false
	for _, r := range p.Restrictions {
		if strings.Contains(r, "is where your records put you") {
			said = true
		}
	}
	if !said {
		t.Errorf("stepping an athlete down a rung has to be said out loud: %v", p.Restrictions)
	}
}

// Twenty-two skills share a vocabulary, and an alias that collides silently
// sends an athlete to the wrong ladder. "Victorian" is the rings skill;
// "straight bar victorian" is the SAT.
func TestNoTwoSkillsAnswerToTheSameName(t *testing.T) {
	// Only across skills: a goal spelling its own name three ways is
	// harmless, and "one arm pull up" and "one-arm pull-up" normalise to the
	// same string on purpose.
	owner := map[string]string{}
	for _, goal := range Goals {
		for _, alias := range goal.Aliases {
			key := normalise(alias)
			if other, taken := owner[key]; taken && other != goal.Key {
				t.Errorf("%q is an alias of both %s and %s, so one of them is unreachable",
					alias, other, goal.Key)
			}
			owner[key] = goal.Key
		}
	}
	for text, want := range map[string]string{
		"victorian":              "victorian",
		"ring victorian":         "victorian",
		"straight bar victorian": "sat",
		"bar victorian":          "sat",
		"sat":                    "sat",
		"hefesto":                "hefesto",
		"backwards muscle up":    "hefesto",
		"muscle up":              "muscle_up",
		"one arm front lever":    "one_arm_front_lever",
		"front lever":            "front_lever",
	} {
		if got, _ := MatchGoal(text); got.Key != want {
			t.Errorf("%q matched %s, want %s", text, got.Key, want)
		}
	}
}
