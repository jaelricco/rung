package plan

import (
	"fmt"
	"strings"
	"testing"

	"calisthenics/api/internal/training"
)

// declared builds a record the way training.mergeBaseline does for a figure
// the athlete stated rather than performed.
func declared(slug string, reps int, added, hold float64) training.Record {
	r := rec(slug, reps, added, hold)
	r.TotalSets = 0
	r.Source = training.SourceDeclared
	return r
}

func TestADeclaredFigurePlacesTheAthleteJustLikeALoggedOne(t *testing.T) {
	lib := seededLibrary(t)

	// The case this exists for: someone who has trained for years and joined
	// yesterday. Nothing is logged, everything is declared.
	snap := snapshotOf(0, 74,
		declared("pull_up", 12, 0, 0),
		declared("dip", 15, 0, 0),
		declared("tuck_front_lever", 0, 0, 24),
		declared("adv_tuck_front_lever", 0, 0, 8))
	trains := 4
	snap.TrainsPerWeek = &trains

	p, _ := Generate(Request{Goal: "front lever", Weeks: 8, DaysPerWeek: 4}, snap, lib)
	if got := currentRung(p); got != "Advanced tuck" {
		t.Errorf("placed on %q; a declared 24-second tuck should clear the tuck rung exactly like a logged one", got)
	}

	// And it says the number is theirs rather than the app's.
	if !strings.Contains(p.Summary, "You put your") {
		t.Errorf("the summary should name a declared figure as declared: %s", p.Summary)
	}
	block := findBlock(p, "adv_tuck_front_lever")
	if block == nil {
		t.Fatal("the plan should train the rung it placed the athlete on")
	}
	if !strings.Contains(block.Intensity, "baseline") {
		t.Errorf("a prescription built on a declared figure should say so: %q", block.Intensity)
	}

	// Weighted work needs the strict base, and a declared twelve is a base.
	if findBlock(p, "weighted_pull_up") == nil {
		t.Error("twelve declared pull-ups should put the belt on the table")
	}
}

func TestAnEmptyBaselineStillSaysWhereToFixIt(t *testing.T) {
	lib := seededLibrary(t)
	p, _ := Generate(Request{Goal: "front lever", Weeks: 8, DaysPerWeek: 3}, training.Snapshot{}, lib)

	if !strings.Contains(p.Summary, "baseline") {
		t.Errorf("an athlete with nothing on record should be pointed at the baseline form: %s", p.Summary)
	}
	block := findBlock(p, "inverted_hang")
	if block == nil || !strings.Contains(block.Intensity, "nothing logged or declared") {
		t.Errorf("a prescription with nothing behind it should say so: %+v", block)
	}
}

func TestDeclaredTrainingFrequencyCarriesTheVolume(t *testing.T) {
	lib := seededLibrary(t)
	base := snapshotOf(0, 74, declared("pull_up", 12, 0, 0))

	blank, _ := Generate(Request{Goal: "front lever", Weeks: 4, DaysPerWeek: 3}, base, lib)

	trained := base
	five := 5
	trained.TrainsPerWeek = &five
	experienced, _ := Generate(Request{Goal: "front lever", Weeks: 4, DaysPerWeek: 3}, trained, lib)

	if totalSets(experienced) <= totalSets(blank) {
		t.Errorf("someone who trains five times a week should not get the same volume as someone who said nothing: %d vs %d",
			totalSets(experienced), totalSets(blank))
	}
	if !strings.Contains(experienced.Method.Readiness, "you told us") {
		t.Errorf("the plan should say the volume rests on a declaration: %q", experienced.Method.Readiness)
	}

	// A log, once it exists, outranks the declaration.
	logged := base
	logged.SessionsLast28 = 16
	logged.TrainsPerWeek = &five
	fromLog, _ := Generate(Request{Goal: "front lever", Weeks: 4, DaysPerWeek: 3}, logged, lib)
	if !strings.Contains(fromLog.Method.Readiness, "logged in the last four weeks") {
		t.Errorf("with a log to read, the plan should read it: %q", fromLog.Method.Readiness)
	}
}

func TestShortSleepTakesVolumeOffAndSaysWhy(t *testing.T) {
	lib := seededLibrary(t)
	rested := snapshotOf(16, 74, declared("pull_up", 12, 0, 0))
	short := rested
	hours := 5.5
	short.SleepHours = &hours

	full, _ := Generate(Request{Goal: "front lever", Weeks: 4, DaysPerWeek: 4}, rested, lib)
	tired, _ := Generate(Request{Goal: "front lever", Weeks: 4, DaysPerWeek: 4}, short, lib)

	if totalSets(tired) >= totalSets(full) {
		t.Errorf("five and a half hours of sleep should cost some volume: %d vs %d", totalSets(tired), totalSets(full))
	}
	if !containsAny(tired.Notes, "sleep") {
		t.Errorf("and the athlete should be told why: %v", tired.Notes)
	}

	// Eight hours changes nothing, and says nothing.
	eight := 8.0
	rested.SleepHours = &eight
	slept, _ := Generate(Request{Goal: "front lever", Weeks: 4, DaysPerWeek: 4}, rested, lib)
	if totalSets(slept) != totalSets(full) || containsAny(slept.Notes, "sleep") {
		t.Error("a normal night's sleep is not worth a note or a correction")
	}
}

func TestEquipmentIsHonouredAndNeverEmptiesThePlan(t *testing.T) {
	lib := seededLibrary(t)
	snap := snapshotOf(12, 74, declared("pull_up", 12, 0, 0), declared("dip", 14, 0, 0))

	cases := []struct {
		name      string
		equipment []string
		forbidden []string
	}{
		{"a bar and nothing else", []string{EquipBar},
			[]string{"dip", "ring_dip", "ring_muscle_up", "weighted_pull_up", "weighted_dip", "v_sit"}},
		{"no belt", []string{EquipBar, EquipDipBars, EquipRings},
			[]string{"weighted_pull_up", "weighted_dip", "weighted_muscle_up"}},
		{"no rings", []string{EquipBar, EquipDipBars, EquipBelt},
			[]string{"ring_dip", "ring_muscle_up"}},
		{"the floor and nothing else", []string{EquipFloorOnly},
			[]string{"pull_up", "dead_hang", "dip", "australian_row", "hanging_leg_raise",
				"tuck_front_lever", "muscle_up", "weighted_pull_up", "l_sit"}},
	}

	for _, c := range cases {
		snap.Equipment = c.equipment
		for _, goal := range []string{"front lever", "muscle up", "weighted pull-up", "handstand", "general"} {
			p, _ := Generate(Request{Goal: goal, Weeks: 6, DaysPerWeek: 3}, snap, lib)
			where := fmt.Sprintf("%s/%s", c.name, goal)

			if len(p.Sessions) == 0 {
				t.Fatalf("%s: no equipment answer is a reason for no plan", where)
			}
			for _, session := range p.Sessions {
				if len(session.Blocks) == 0 {
					t.Fatalf("%s: an empty session reached the calendar", where)
				}
				for _, block := range session.Blocks {
					for _, banned := range c.forbidden {
						if block.ExerciseSlug == banned {
							t.Errorf("%s: prescribed %q, which needs kit the athlete does not have",
								where, block.ExerciseSlug)
						}
					}
				}
			}
			if !containsAny(p.Restrictions, "equipment you have") {
				t.Errorf("%s: the plan should say it was cut to the available kit: %v", where, p.Restrictions)
			}
		}
	}

	// An unanswered list is not an empty one: it filters nothing, which is how
	// the app behaved before anybody was asked.
	snap.Equipment = nil
	p, _ := Generate(Request{Goal: "weighted pull-up", Weeks: 6, DaysPerWeek: 3}, snap, lib)
	if findBlock(p, "weighted_pull_up") == nil {
		t.Error("not having answered the equipment question should not take the belt away")
	}
	if containsAny(p.Restrictions, "equipment you have") {
		t.Error("an unanswered equipment question should not produce a restriction")
	}
}

func TestBenchmarksAskWhatThePlannerActuallyBranchesOn(t *testing.T) {
	lib := seededLibrary(t)

	core := Benchmarks("", lib)
	if len(core) != len(universal) {
		t.Fatalf("an unrecognised goal should ask the universal set only, asked %d", len(core))
	}
	for _, b := range core {
		if b.Scope != "core" || b.Name == "" || b.Measure == "" || b.Prompt == "" || b.Why == "" {
			t.Errorf("benchmark %q is not a question anyone can answer: %+v", b.ExerciseSlug, b)
		}
		if !lib.Has(b.ExerciseSlug) {
			t.Errorf("benchmark %q is not in the library", b.ExerciseSlug)
		}
	}

	// A goal adds its own rungs, and asks in the right units.
	lever := Benchmarks("front lever", lib)
	if len(lever) <= len(core) {
		t.Fatal("a recognised goal should add the rungs of its ladder")
	}
	seen := map[string]Benchmark{}
	for _, b := range lever {
		if _, twice := seen[b.ExerciseSlug]; twice {
			t.Errorf("%q was asked about twice", b.ExerciseSlug)
		}
		seen[b.ExerciseSlug] = b
	}
	tuck, ok := seen["tuck_front_lever"]
	if !ok {
		t.Fatal("a front lever form has to ask about the tuck")
	}
	if tuck.Scope != "front_lever" || !strings.Contains(tuck.Prompt, "seconds") {
		t.Errorf("a hold should be asked for in seconds, scoped to its goal: %+v", tuck)
	}
	if !strings.Contains(seen["pull_up"].Prompt, "set") {
		t.Errorf("reps should be asked for as reps: %q", seen["pull_up"].Prompt)
	}

	// Every goal's questions have to be answerable.
	for _, goal := range Goals {
		for _, b := range Benchmarks(goal.Name, lib) {
			if !lib.Has(b.ExerciseSlug) {
				t.Errorf("%s asks about %q, which is not in the library", goal.Key, b.ExerciseSlug)
			}
		}
	}
}

func TestEveryEquipmentRequirementNamesRealThings(t *testing.T) {
	lib := seededLibrary(t)
	known := map[string]bool{}
	for _, item := range Equipment {
		known[item.Key] = true
	}
	for slug, groups := range requires {
		if !lib.Has(slug) {
			t.Errorf("the equipment table names %q, which is not in the library", slug)
		}
		if len(groups) == 0 {
			t.Errorf("%q has an empty requirement, which should be no entry at all", slug)
		}
		for _, group := range groups {
			if len(group) == 0 {
				t.Errorf("%q has an empty requirement group", slug)
			}
			for _, item := range group {
				if !known[item] || item == EquipFloorOnly {
					t.Errorf("%q requires %q, which is not equipment anyone can tick", slug, item)
				}
			}
		}
	}
}

func totalSets(p Plan) int {
	n := 0
	for _, session := range p.Sessions {
		for _, block := range session.Blocks {
			n += block.Sets
		}
	}
	return n
}
