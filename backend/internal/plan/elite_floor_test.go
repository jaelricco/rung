package plan

import (
	"testing"

	"calisthenics/api/internal/training"
)

// The one-arm planche is the only goal in the catalogue made of two skills
// rather than one, and its rungs say so: the push half is gated on the planche
// ladder and the balance half on the one-arm handstand ladder. An athlete with
// one and not the other is held where they are and told which number is
// missing, which is a better answer than half a plan for half a skill.
func TestTheOneArmPlancheIsGatedOnBothLaddersItIsMadeOf(t *testing.T) {
	lib := seededLibrary(t)

	// All the push, none of the balance. The lean is open; the tuck is not.
	pushOnly := snapshotOf(20, 68, rec("pull_up", 16, 0, 0), rec("dip", 22, 0, 0),
		rec("straddle_planche", 0, 0, 14), rec("full_planche", 0, 0, 8),
		rec("one_arm_planche_lean", 0, 0, 16))
	p, _ := Generate(Request{Goal: "one arm planche", Weeks: 8, DaysPerWeek: 4}, pushOnly, lib)
	if p.Method.Rung == "Tuck one-arm planche" {
		t.Error("the tuck is gated on a tuck one-arm handstand, and nothing was logged for one")
	}
	named := false
	for _, gap := range p.Method.Gaps {
		if gap.Name == "Tuck one-arm handstand" {
			named = true
		}
	}
	if !named {
		t.Errorf("the gap should name the balance half, named %+v", p.Method.Gaps)
	}

	// And both skills it is built on keep their place in the week.
	both := snapshotOf(20, 68, rec("pull_up", 16, 0, 0), rec("dip", 22, 0, 0),
		rec("straddle_planche", 0, 0, 14), rec("full_planche", 0, 0, 8),
		rec("handstand", 0, 0, 60), rec("tuck_one_arm_handstand", 0, 0, 12))
	q, _ := Generate(Request{Goal: "one arm planche", Weeks: 6, DaysPerWeek: 4}, both, lib)
	planche, balance := false, false
	for _, session := range q.Sessions {
		for _, block := range session.Blocks {
			switch block.ExerciseSlug {
			case "full_planche", "straddle_planche", "adv_tuck_planche":
				planche = true
			case "straddle_one_arm_handstand", "tuck_one_arm_handstand", "wall_one_arm_handstand",
				"one_arm_handstand", "handstand_shifts":
				balance = true
			}
		}
	}
	if !planche || !balance {
		t.Errorf("a one-arm planche plan has to keep both halves alive: planche %v, one-arm handstand %v",
			planche, balance)
	}
}

// It is also the one skill with no version that spares a wrist — a planche
// angle with one hand under it instead of two — so an angry wrist steps the
// athlete back onto two arms rather than substituting.
func TestAnAngryWristStepsTheOneArmPlancheBackToTwoArms(t *testing.T) {
	lib := seededLibrary(t)
	snap := snapshotOf(20, 68, rec("pull_up", 16, 0, 0), rec("dip", 22, 0, 0),
		rec("straddle_planche", 0, 0, 14), rec("full_planche", 0, 0, 8),
		rec("handstand", 0, 0, 60), rec("tuck_one_arm_handstand", 0, 0, 12),
		rec("one_arm_planche_lean", 0, 0, 16))
	snap.OpenInjuries = []training.Injury{{Region: regionWrist, Severity: 2}}

	p, _ := Generate(Request{Goal: "one arm planche", Weeks: 6, DaysPerWeek: 4}, snap, lib)
	if p.Method.Rung != "Straddle planche" {
		t.Errorf("a sore wrist should step the one-arm planche back to two arms, placed on %q", p.Method.Rung)
	}
	for _, slug := range []string{"one_arm_planche_lean", "tuck_one_arm_planche", "one_arm_planche"} {
		if findBlock(p, slug) != nil {
			t.Errorf("%q puts a planche angle through one wrist and has no version that does not", slug)
		}
	}
}

// The inverted cross is the one element here with a peer-reviewed strength
// benchmark behind it, and the finding is that overhead pressing correlates
// with the hold. So the accessories are overhead pressing — not because it
// looks related, and the test is here so a later tidy-up cannot quietly swap
// it for the band work every other rings ladder reaches for.
func TestTheInvertedCrossIsConditionedWithOverheadPressing(t *testing.T) {
	lib := seededLibrary(t)
	goal, ok := goalByKey["inverted_cross"]
	if !ok {
		t.Fatal("the inverted cross is not in the catalogue")
	}
	overhead := map[string]bool{
		"handstand_push_up": true, "wall_hspu": true, "negative_hspu": true,
		"deficit_hspu": true, "pike_push_up": true, "elevated_pike_push_up": true,
	}
	found := false
	for _, slug := range goal.Accessories {
		if overhead[slug] {
			found = true
		}
	}
	if !found {
		t.Errorf("the inverted cross should be conditioned with overhead pressing, accessories are %v",
			goal.Accessories)
	}

	// And it reaches the athlete: somebody on the ladder gets pressing in the
	// week, at whatever level of it they are at.
	snap := snapshotOf(20, 68, rec("pull_up", 16, 0, 0), rec("dip", 22, 0, 0),
		rec("ring_support_hold", 0, 0, 45), rec("handstand", 0, 0, 55),
		rec("ring_handstand", 0, 0, 22), rec("wall_hspu", 8, 0, 0))
	p, _ := Generate(Request{Goal: "inverted cross", Weeks: 6, DaysPerWeek: 4}, snap, lib)
	pressed := false
	for _, session := range p.Sessions {
		for _, block := range session.Blocks {
			if overhead[block.ExerciseSlug] {
				pressed = true
			}
		}
	}
	if !pressed {
		t.Error("no overhead pressing reached the week of somebody training an inverted cross")
	}
}

// Both ladders start at something an ordinary strong athlete already has, so
// neither needs an entry gate to refuse anyone: a beginner is placed on the
// base and trains it.
func TestTheLastTwoLaddersPlaceABeginnerOnTheirBase(t *testing.T) {
	lib := seededLibrary(t)
	beginner := snapshotOf(10, 74, rec("pull_up", 10, 0, 0), rec("dip", 10, 0, 0))

	for goal, want := range map[string]string{
		"one arm planche": "Straddle planche",
		"inverted cross":  "Ring support",
	} {
		p, _ := Generate(Request{Goal: goal, Weeks: 8, DaysPerWeek: 4}, beginner, lib)
		if p.Method.Rung != want {
			t.Errorf("%s for a beginner placed on %q, want %q", goal, p.Method.Rung, want)
		}
		if !p.Method.EntryMet {
			t.Errorf("%s should gate on its rungs rather than refuse the goal outright", goal)
		}
	}
}
