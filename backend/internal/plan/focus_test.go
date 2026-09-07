package plan

import (
	"fmt"
	"strings"
	"testing"

	"calisthenics/api/internal/training"
)

// An athlete deep enough into the catalogue for the focus dial to have
// anything to move: a planche that is held, so the maltese ladder is open.
func maltesePlacedAthlete() training.Snapshot {
	return snapshotOf(16, 72,
		rec("pull_up", 12, 0, 0), rec("dip", 14, 0, 0),
		rec("straddle_planche", 0, 0, 12), rec("full_planche", 0, 0, 11),
		rec("wide_planche_hold", 0, 0, 6), rec("lean_maltese", 0, 0, 14))
}

// skillShare is the fraction of a week's working sets that went to the goal's
// own ladder, computed the way the plan claims to compute it: warm-ups out,
// the fed skill's maintenance not charged to the goal.
func skillShare(t *testing.T, b *builder, p Plan, week int) float64 {
	t.Helper()
	total, spent := 0, 0
	for _, session := range p.Sessions {
		if session.Week != week {
			continue
		}
		for _, block := range session.Blocks {
			if block.Intent == "prep" {
				continue
			}
			total += block.Sets
			if block.Intent == "skill" && b.line[block.ExerciseSlug] && !b.fedLine[block.ExerciseSlug] {
				spent += block.Sets
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(spent) / float64(total)
}

// The rule the whole dial exists to enforce, checked on every skill in the
// catalogue at every level and every week length: no skill takes more than
// two working sets in five, and none takes more than the level asked for.
func TestNoSkillTakesMoreOfTheWeekThanItsFocusAllows(t *testing.T) {
	lib := seededLibrary(t)
	snap := maltesePlacedAthlete()

	for _, goal := range allGoalKeys() {
		for _, level := range []string{FocusLight, FocusStandard, FocusHigh} {
			for days := 1; days <= 7; days++ {
				req := Request{Goal: goal, Weeks: 8, DaysPerWeek: days, Focus: level}
				p, _ := Generate(req, snap, lib)
				b := newBuilder(req, snap, lib)
				where := fmt.Sprintf("%s/%s/%dd", goal, level, days)

				focus := p.Method.Focus
				if focus == nil {
					t.Fatalf("%s: the plan does not say what share the skill took", where)
				}
				if focus.Ceiling > hardShareCeiling+1e-9 {
					t.Errorf("%s: a ceiling of %.2f is past the %.2f nothing crosses",
						where, focus.Ceiling, hardShareCeiling)
				}
				if goalByKey[goalKeyOf(goal)].Foundation {
					continue
				}
				for week := 1; week <= 8; week++ {
					share := skillShare(t, b, p, week)
					if share > hardShareCeiling+1e-9 && !deloadOrTest(p, week) {
						t.Errorf("%s week %d: the skill took %.0f%% of the week, past the 40%% ceiling",
							where, week, share*100)
					}
					if share > focus.Ceiling+1e-9 && !deloadOrTest(p, week) &&
						!strings.Contains(focus.Note, "could not be brought under") {
						t.Errorf("%s week %d: took %.0f%% against a ceiling of %.0f%%, and said nothing about it",
							where, week, share*100, focus.Ceiling*100)
					}
				}
			}
		}
	}
}

func deloadOrTest(p Plan, week int) bool {
	for _, s := range p.Sessions {
		if s.Week == week && (s.Load == "deload" || strings.HasPrefix(s.Title, "Test:")) {
			return true
		}
	}
	return false
}

func goalKeyOf(text string) string {
	g, _ := MatchGoal(text)
	return g.Key
}

// The dial has to be worth having: each step up gives the skill more of the
// week than the step below it, in sets and in sessions.
func TestEachStepOfTheDialGivesTheSkillMoreOfTheWeek(t *testing.T) {
	lib := seededLibrary(t)
	snap := maltesePlacedAthlete()

	for _, days := range []int{3, 5, 6} {
		var lastSets, lastDays int
		for i, level := range []string{FocusLight, FocusStandard, FocusHigh} {
			req := Request{Goal: "maltese", Weeks: 8, DaysPerWeek: days, Focus: level}
			p, _ := Generate(req, snap, lib)

			sets, skillDays := 0, 0
			for _, session := range p.Sessions {
				if session.Week != 2 {
					continue
				}
				onSkill := false
				for _, block := range session.Blocks {
					if block.Intent == "skill" {
						sets += block.Sets
						onSkill = true
					}
				}
				if onSkill {
					skillDays++
				}
			}
			if i > 0 && sets <= lastSets {
				t.Errorf("%d days: %s gave the maltese %d skill sets, no more than the level below (%d)",
					days, level, sets, lastSets)
			}
			if i > 0 && skillDays < lastDays {
				t.Errorf("%d days: %s put the maltese on %d days, fewer than the level below (%d)",
					days, level, skillDays, lastDays)
			}
			lastSets, lastDays = sets, skillDays
		}
	}
}

// The rule the maltese made obvious: a day built around one skill carries that
// skill's line and nobody else's. The work beside a maltese hold is a lean, a
// wide planche, a planche — not an L-sit and not a back lever, which are other
// skills wearing an accessory's clothes.
func TestASkillDayCarriesOneLineAndNobodyElses(t *testing.T) {
	lib := seededLibrary(t)
	snap := maltesePlacedAthlete()

	for _, goal := range []string{"maltese", "planche", "front lever", "iron cross", "one-arm handstand"} {
		for _, level := range []string{FocusLight, FocusStandard, FocusHigh} {
			req := Request{Goal: goal, Weeks: 6, DaysPerWeek: 5, Focus: level}
			p, _ := Generate(req, snap, lib)
			b := newBuilder(req, snap, lib)

			for _, session := range p.Sessions {
				// A skill day is one that trains the skill at all.
				skillDay := false
				for _, block := range session.Blocks {
					if block.Intent == "skill" {
						skillDay = true
					}
				}
				if !skillDay {
					continue
				}
				for _, block := range session.Blocks {
					if block.Intent != "accessory" && block.Intent != "conditioning" {
						continue
					}
					if b.offLine[block.ExerciseSlug] {
						t.Errorf("%s/%s day %d: %q is another skill's rung and has no business on this day",
							goal, level, session.DayOfWeek, block.ExerciseSlug)
					}
				}
			}
		}
	}
}

// The maltese case the athlete actually asked about, spelled out: at the top
// of the dial the session spans the ladder, and the block under the main rung
// is the rung under it — a lean, not somebody else's skill.
func TestTheTopOfTheDialSpansTheLadderAndTheBottomDoesNot(t *testing.T) {
	lib := seededLibrary(t)
	snap := maltesePlacedAthlete()

	slugsOn := func(level string) []string {
		p, _ := Generate(Request{Goal: "maltese", Weeks: 6, DaysPerWeek: 5, Focus: level}, snap, lib)
		for _, session := range p.Sessions {
			if session.Week != 2 || session.DayOfWeek != 1 {
				continue
			}
			var out []string
			for _, block := range session.Blocks {
				if block.Intent == "skill" {
					out = append(out, block.ExerciseSlug)
				}
			}
			return out
		}
		return nil
	}

	light, high := slugsOn(FocusLight), slugsOn(FocusHigh)
	if len(light) == 0 || len(high) == 0 {
		t.Fatalf("the maltese day lost its skill work: light %v, high %v", light, high)
	}
	if len(high) <= len(light) {
		t.Errorf("the top of the dial should span more rungs than the bottom: light %v, high %v", light, high)
	}
	// The rung under the one being trained is what fills the volume block, and
	// for this athlete that is the wide planche hold.
	if !containsSlug(high, "wide_planche_hold") {
		t.Errorf("the high-focus maltese day should carry the rung below it for volume, got %v", high)
	}
	for _, slug := range append(append([]string{}, light...), high...) {
		if slug == "l_sit" || slug == "back_lever" || slug == "front_lever" {
			t.Errorf("%q is a different skill and should not be on a maltese day: %v / %v", slug, light, high)
		}
	}
}

func containsSlug(list []string, want string) bool {
	for _, slug := range list {
		if slug == want {
			return true
		}
	}
	return false
}

// Focus costs tissue, and the budget has to see it: a week built around a
// maximal static parks what a week that merely contains one can keep.
func TestBuildingTheWeekAroundASkillSpendsMoreOfTheTendonBudget(t *testing.T) {
	lib := seededLibrary(t)
	snap := maltesePlacedAthlete()
	snap.Learning = []string{"maltese", "front_lever", "one_arm_handstand"}

	light, _ := Generate(Request{Goal: "maltese", Weeks: 8, DaysPerWeek: 5, Focus: FocusLight}, snap, lib)
	high, _ := Generate(Request{Goal: "maltese", Weeks: 8, DaysPerWeek: 5, Focus: FocusHigh}, snap, lib)

	if light.Method.Load == nil || high.Method.Load == nil {
		t.Fatal("a plan for someone learning three skills has to show the budget")
	}
	if high.Method.Load.Spent <= light.Method.Load.Spent {
		t.Errorf("building the week around the maltese should cost more than fitting it in: %d against %d",
			high.Method.Load.Spent, light.Method.Load.Spent)
	}
	if len(high.Method.Load.Parked) <= len(light.Method.Load.Parked) {
		t.Errorf("the extra cost should park something: high parked %v, light parked %v",
			high.Method.Load.Parked, light.Method.Load.Parked)
	}
	// The skill the goal is built on is never what gets parked.
	for _, name := range high.Method.Load.Parked {
		if strings.EqualFold(name, "Planche") {
			t.Error("the planche is what drives the maltese and must not be parked for it")
		}
	}
}

// A field the browser has not learned to send yet must not change the plan a
// browser used to get.
func TestAnUnknownFocusIsTheOneThePlannerAlwaysUsed(t *testing.T) {
	lib := seededLibrary(t)
	snap := maltesePlacedAthlete()

	for _, level := range []string{"", "  ", "obsessive", "LIGHT"} {
		p, _ := Generate(Request{Goal: "front lever", Weeks: 6, DaysPerWeek: 4, Focus: level}, snap, lib)
		if p.Method.Focus.Level != FocusStandard {
			t.Errorf("focus %q resolved to %q, not the middle of the dial", level, p.Method.Focus.Level)
		}
	}
	// And the dial the browser draws is the dial the planner applies.
	levels := FocusLevels()
	if len(levels) != 3 {
		t.Fatalf("the dial has three positions, the picker was given %d", len(levels))
	}
	for _, level := range levels {
		if level.Share > hardShareCeiling+1e-9 {
			t.Errorf("%q offers %.0f%% of the week, past the ceiling nothing crosses", level.Key, level.Share*100)
		}
		if level.Name == "" || level.Sentence == "" {
			t.Errorf("%q is offered to the athlete without saying what it does", level.Key)
		}
	}
}

// Whatever the dial is set to, the session still has to be worth doing: the
// trim takes sets, not the point of the session.
func TestTheShareCapNeverEmptiesASkillSession(t *testing.T) {
	lib := seededLibrary(t)
	snap := maltesePlacedAthlete()

	for _, goal := range allGoalKeys() {
		for _, level := range []string{FocusLight, FocusStandard, FocusHigh} {
			for _, days := range []int{1, 3, 6} {
				p, _ := Generate(Request{Goal: goal, Weeks: 6, DaysPerWeek: days, Focus: level}, snap, lib)
				for _, session := range p.Sessions {
					working := 0
					for _, block := range session.Blocks {
						if block.Intent != "prep" {
							working++
						}
						if block.Sets < 1 {
							t.Errorf("%s/%s/%dd day %d: %q was left with %d sets",
								goal, level, days, session.DayOfWeek, block.ExerciseSlug, block.Sets)
						}
					}
					if working == 0 {
						t.Errorf("%s/%s/%dd day %d is a warm-up and nothing else",
							goal, level, days, session.DayOfWeek)
					}
					if session.DurationMinutes < 20 {
						t.Errorf("%s/%s/%dd day %d claims to take %d minutes",
							goal, level, days, session.DayOfWeek, session.DurationMinutes)
					}
				}
			}
		}
	}
}

// The one-line rule reads "another skill's rung", and it decides that by
// subtracting the movements this planner treats as ordinary work. That list is
// written out by hand in focus.go, so this holds it to the chains it claims to
// come from: a movement the planner would prescribe as general strength must
// never be filtered off a skill day as somebody else's skill.
func TestTheOrdinaryWorkListMatchesTheChainsItCameFrom(t *testing.T) {
	lib := seededLibrary(t)

	// A spread of athletes wide enough to reach every branch of every chain,
	// from someone with nothing logged to someone with a belt on.
	athletes := []training.Snapshot{
		snapshotOf(0, 70),
		snapshotOf(8, 70, rec("pull_up", 1, 0, 0), rec("push_up", 20, 0, 0),
			rec("bodyweight_squat", 30, 0, 0), rec("hanging_knee_raise", 10, 0, 0)),
		snapshotOf(12, 70, rec("pull_up", 5, 0, 0), rec("dip", 5, 0, 0),
			rec("bulgarian_split", 10, 0, 0), rec("hollow_body_hold", 0, 0, 40)),
		snapshotOf(20, 70, rec("pull_up", 14, 0, 0), rec("dip", 16, 0, 0),
			rec("pistol_squat", 6, 0, 0), rec("hanging_leg_raise", 14, 0, 0)),
	}
	for _, snap := range athletes {
		b := newBuilder(Request{Goal: "planche", Weeks: 6, DaysPerWeek: 3}, snap, lib)
		for _, group := range []chain{b.pullChain(), b.pushChain(), b.legChain(), b.coreChain()} {
			for _, slug := range group {
				if !ordinaryWork[slug] {
					t.Errorf("%q is prescribed as general strength but is missing from ordinaryWork, "+
						"so a skill day would refuse it as another skill's rung", slug)
				}
			}
		}
	}
	// And the other direction, because a typo in that list is silent: it would
	// simply stop filtering something and nothing would ever say so.
	for slug := range ordinaryWork {
		if !lib.Has(slug) {
			t.Errorf("ordinaryWork names %q, which is not an exercise the app has", slug)
		}
	}
}
