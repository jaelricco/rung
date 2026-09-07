package plan

import (
	"fmt"
	"strings"
	"testing"

	"calisthenics/api/internal/training"
)

// The athlete this tier was built for: twelve seconds of full planche, three
// full planche push-ups, a twenty-second front lever, three skills already in
// flight, a wrist that has hurt for a while, and a maltese he has read he is
// ready for. Every branch the elite tier added is exercised by one person.
func eliteAthlete() training.Snapshot {
	snap := snapshotOf(16, 71,
		declared("full_planche", 0, 0, 12),
		declared("planche_push_up", 3, 0, 0),
		declared("front_lever", 0, 0, 20),
		declared("straddle_planche", 0, 0, 25),
		declared("handstand", 0, 0, 40),
		declared("pull_up", 20, 0, 0),
		declared("dip", 25, 0, 0),
	)
	trains, sleep := 5, 6.5
	snap.TrainsPerWeek, snap.SleepHours = &trains, &sleep
	snap.Equipment = []string{EquipBar, EquipDipBars, EquipParallettes, EquipRings, EquipBands, EquipBelt}
	snap.Learning = []string{"one_arm_handstand", "front_lever_pull_up", "planche_press"}
	snap.OpenInjuries = []training.Injury{{Region: "wrist", Severity: 2, Description: "aches on the floor, months now"}}
	return snap
}

// The same athlete with a wrist that does not hurt, for the assertions that
// are about the ladder rather than about training around an injury.
func healthyElite() training.Snapshot {
	snap := eliteAthlete()
	snap.OpenInjuries = nil
	return snap
}

func TestTheMalteseLadderOpensAtTheBottomForThisAthlete(t *testing.T) {
	lib := seededLibrary(t)
	p, warnings := Generate(Request{Goal: "maltese", Weeks: 12, DaysPerWeek: 5}, healthyElite(), lib)

	if len(warnings) > 0 {
		t.Errorf("the plan warned about its own output: %v", warnings)
	}

	// The floor tradition asks for none of the rings-gymnastics entry: leaning
	// into a maltese with a band is accessory work that appears in beginner
	// programmes, so an athlete with a twelve-second planche is not turned away
	// for want of a back lever he has never needed.
	if !p.Method.EntryMet {
		t.Fatalf("the maltese ladder has no goal-level gate any more; gaps were %+v", p.Method.Gaps)
	}
	if p.Method.Rung != "Lean maltese" {
		t.Errorf("placed on %q, want the bottom of the ladder — he has never trained the position", p.Method.Rung)
	}
	if findBlock(p, "lean_maltese") == nil {
		t.Error("the rung he is on should be in the sessions")
	}
	for _, unwanted := range []string{"back_lever", "ring_support_hold", "band_iron_cross"} {
		if findBlock(p, unwanted) != nil {
			t.Errorf("%q belongs to the rings ladder, not this one", unwanted)
		}
	}

	// The opener is the rung above, because its own gate is met: a 25-second
	// straddle planche clears the wide planche's ten.
	opener := findBlock(p, "wide_planche_hold")
	if opener == nil {
		t.Fatal("the wide planche is the rung above and should open the session")
	}
	if opener.Sets > 3 {
		t.Errorf("the opener is %d sets; it is a look at the next position, not the work", opener.Sets)
	}
	if opener.RestSeconds < restMaximal {
		t.Errorf("the opener rests %ds; maximal straight-arm work rests minutes", opener.RestSeconds)
	}

	// And the prescriptions are ranges, the way the coaching material writes
	// them, rather than a single number that is wrong on two days out of three.
	for _, session := range p.Sessions {
		// The test week prescribes a maximum, not a dosage, so it has no range.
		if strings.HasPrefix(session.Title, "Test:") {
			continue
		}
		for _, block := range session.Blocks {
			if block.Intent != "skill" {
				continue
			}
			if !strings.Contains(block.Prescription, "-") {
				t.Errorf("skill work should be prescribed as a range: %q", block.Prescription)
			}
		}
	}
}

// With a wrist that has hurt for months, the floor maltese ladder is not
// available at all — every rung on it is bodyweight through an open palm. The
// one form that survives is the banded rings version, which is what the
// coaching material itself reaches for at the top of the floor ladder.
func TestASoreWristMovesTheMalteseOntoRings(t *testing.T) {
	lib := seededLibrary(t)
	p, _ := Generate(Request{Goal: "maltese", Weeks: 12, DaysPerWeek: 5}, eliteAthlete(), lib)

	if findBlock(p, "band_maltese") == nil {
		t.Error("the maltese should move to its banded rings form rather than disappearing")
	}
	for _, floor := range []string{"lean_maltese", "wide_planche_hold", "maltese", "zanetti", "maltese_press"} {
		if findBlock(p, floor) != nil {
			t.Errorf("%q is floor work on an open palm and should be out while the wrist hurts", floor)
		}
	}
}

func TestARungGateHoldsAnAthleteOneRungBelowTheirRecords(t *testing.T) {
	lib := seededLibrary(t)

	// Records that reach the wide planche press, on a planche that does not
	// support it. The gate caps the placement without closing the ladder.
	snap := snapshotOf(16, 71,
		declared("full_planche", 0, 0, 4),
		declared("straddle_planche", 0, 0, 14),
		declared("lean_maltese", 0, 0, 18),
		declared("wide_planche_hold", 0, 0, 8),
		declared("wide_planche_press", 3, 0, 0))

	p, _ := Generate(Request{Goal: "maltese", Weeks: 8, DaysPerWeek: 4}, snap, lib)
	if p.Method.Rung != "Wide planche hold" {
		t.Errorf("placed on %q, want to be held at the wide planche hold by the press's own gate", p.Method.Rung)
	}
	if len(p.Method.Gaps) == 0 {
		t.Fatal("being held back has to come with the number that would release it")
	}
	found := false
	for _, gap := range p.Method.Gaps {
		if gap.Name == "Full planche" && gap.Standard == "5s" && gap.Have == "4s" {
			found = true
		}
	}
	if !found {
		t.Errorf("the gap should name the planche, the standard and his own figure: %+v", p.Method.Gaps)
	}
	if findBlock(p, "wide_planche_press") != nil {
		t.Error("the gated rung should not be trained")
	}
}

func TestPlancheIsMaintainedWhileTheMalteseIsLearned(t *testing.T) {
	lib := seededLibrary(t)
	snap := eliteAthlete()
	snap.Records = append(snap.Records,
		declared("back_lever", 0, 0, 15),
		declared("ring_support_hold", 0, 0, 45))

	p, _ := Generate(Request{Goal: "maltese", Weeks: 12, DaysPerWeek: 5}, snap, lib)

	// The planche feeds the maltese, so it keeps its place in the week — and
	// because the wrist is sore, it appears in its neutral-wrist version.
	planche := findBlock(p, "ring_planche")
	if planche == nil {
		planche = findBlock(p, "full_planche")
	}
	if planche == nil {
		t.Fatal("the planche is what drives the maltese; parking it loses both")
	}
	if !strings.Contains(planche.Progression, "base the new skill is built on") {
		t.Errorf("the maintenance block should say what it is for: %q", planche.Progression)
	}
	if planche.Sets > 3 {
		t.Errorf("maintenance is %d sets; it is held, not pushed", planche.Sets)
	}
}

func TestThreeSkillsInFlightPlusAFourthIsOverBudget(t *testing.T) {
	lib := seededLibrary(t)
	snap := eliteAthlete()
	snap.Records = append(snap.Records,
		declared("back_lever", 0, 0, 15),
		declared("ring_support_hold", 0, 0, 45))

	p, _ := Generate(Request{Goal: "maltese", Weeks: 12, DaysPerWeek: 5}, snap, lib)
	load := p.Method.Load
	if load == nil {
		t.Fatal("a plan for a maximal skill has to price the week")
	}
	// maltese 3 + one-arm handstand 2 + front lever pull-up 2 + planche press 3
	if load.Spent != 10 || load.Ceiling != tendonCeiling {
		t.Errorf("spent %d of %d, want 10 of %d", load.Spent, load.Ceiling, tendonCeiling)
	}
	if len(load.Parked) == 0 {
		t.Fatal("ten units against a ceiling of five has to name something to park")
	}
	if !containsAny(p.Restrictions, "Park ") {
		t.Errorf("and say so in the restrictions: %v", p.Restrictions)
	}
	// The planche is never parked: it is what the maltese is built on.
	for _, parked := range load.Parked {
		if parked == "Planche" {
			t.Error("parking the planche to chase a maltese loses both")
		}
	}
}

func TestASoreWristMovesTheWorkRatherThanDeletingIt(t *testing.T) {
	lib := seededLibrary(t)
	snap := eliteAthlete()
	snap.Records = append(snap.Records,
		declared("back_lever", 0, 0, 15),
		declared("ring_support_hold", 0, 0, 45))

	p, _ := Generate(Request{Goal: "maltese", Weeks: 12, DaysPerWeek: 5}, snap, lib)
	if len(p.Sessions) == 0 {
		t.Fatal("a sore wrist is not a reason for no plan")
	}

	// Nothing loaded through an extended wrist survives...
	for _, session := range p.Sessions {
		for _, block := range session.Blocks {
			if _, hasNeutral := neutralWrist[block.ExerciseSlug]; hasNeutral {
				t.Errorf("%q has a neutral-wrist version and should have been swapped for it", block.ExerciseSlug)
			}
		}
	}
	// ...but the sport survives: there is still real straight-arm work in it.
	if findBlock(p, "ring_planche") == nil && findBlock(p, "ring_planche_lean") == nil &&
		findBlock(p, "ring_support_hold") == nil {
		t.Error("the whole point is that ring work keeps the training going")
	}
	if !containsAny(p.Restrictions, "off the palm") {
		t.Errorf("the plan should say what it did and why: %v", p.Restrictions)
	}
	if !containsAny(p.Restrictions, "position, not the volume") {
		t.Errorf("and name the lever it actually pulled: %v", p.Restrictions)
	}
	if !containsAny(p.Notes, "in-person assessment") {
		t.Errorf("months of wrist pain needs a clinician named: %v", p.Notes)
	}

	// A severe wrist still clears the region outright — that is a medical
	// question, not a programming one.
	severe := snap
	severe.OpenInjuries = []training.Injury{{Region: "wrist", Severity: 4}}
	hard, _ := Generate(Request{Goal: "maltese", Weeks: 8, DaysPerWeek: 4}, severe, lib)
	for _, session := range hard.Sessions {
		for _, block := range session.Blocks {
			if wristLoaded[block.ExerciseSlug] {
				t.Errorf("severity 4 should clear the region; %q survived", block.ExerciseSlug)
			}
		}
	}
}

func TestShortSleepAndFiveDaysStillProducesATrainableWeek(t *testing.T) {
	lib := seededLibrary(t)
	snap := eliteAthlete()
	p, _ := Generate(Request{Goal: "maltese", Weeks: 12, DaysPerWeek: 5}, snap, lib)

	if !containsAny(p.Notes, "sleep") {
		t.Errorf("6.5 hours is worth a word: %v", p.Notes)
	}
	for _, session := range p.Sessions {
		if len(session.Blocks) == 0 {
			t.Fatal("empty session")
		}
		if session.DurationMinutes > 120 {
			t.Errorf("a %d-minute session is not a session", session.DurationMinutes)
		}
	}
	if n := countLoad(p, 1, "hard"); n > 4 {
		t.Errorf("%d hard sessions in week 1", n)
	}
}

func keysOf(m map[string]Gap) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// Prints the plan this athlete actually gets, for reading rather than asserting.
func TestScenarioOutline(t *testing.T) {
	lib := seededLibrary(t)
	snap := eliteAthlete()
	p, _ := Generate(Request{Goal: "maltese", Weeks: 12, DaysPerWeek: 5}, snap, lib)

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n%s\n\n", p.Title, p.Summary)
	fmt.Fprintf(&b, "ENTRY MET: %v\n", p.Method.EntryMet)
	for _, g := range p.Method.Gaps {
		fmt.Fprintf(&b, "  gap  %-20s want %-8s have %-24s %s\n", g.Name, g.Standard, g.Have, g.Why)
	}
	if p.Method.Load != nil {
		fmt.Fprintf(&b, "LOAD: %d/%d  learning=%v parked=%v\n", p.Method.Load.Spent,
			p.Method.Load.Ceiling, p.Method.Load.Learning, p.Method.Load.Parked)
	}
	for _, r := range p.Restrictions {
		fmt.Fprintf(&b, "RESTRICTION: %s\n", r)
	}
	for _, n := range p.Notes {
		fmt.Fprintf(&b, "NOTE: %s\n", n)
	}
	fmt.Fprintf(&b, "\nWEEK 1\n")
	for _, s := range p.Sessions {
		if s.Week != 1 {
			continue
		}
		fmt.Fprintf(&b, "  D%d %-8s %-38s %3dmin %s\n", s.DayOfWeek, s.Load, s.Title, s.DurationMinutes,
			strings.Join(s.WarmupProtocols, "+"))
		for _, bl := range s.Blocks {
			fmt.Fprintf(&b, "       %-13s %-26s %s\n", bl.Intent, bl.ExerciseSlug, bl.Prescription)
		}
	}
	fmt.Fprintf(&b, "\nTEST: %s\n", p.Test)
	t.Log("\n" + b.String())
}
