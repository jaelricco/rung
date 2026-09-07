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

func TestTheMalteseIsGatedOnWhatIsActuallyMissing(t *testing.T) {
	lib := seededLibrary(t)
	p, warnings := Generate(Request{Goal: "maltese", Weeks: 12, DaysPerWeek: 5}, eliteAthlete(), lib)

	if len(warnings) > 0 {
		t.Errorf("the plan warned about its own output: %v", warnings)
	}
	if p.Method.EntryMet {
		t.Fatal("this athlete has no back lever and no ring support on record; the ladder should not be open")
	}

	// The full planche is met at 12s, so it must not be reported as a gap.
	// The back lever and ring support are not on record, so they must be.
	gapped := map[string]Gap{}
	for _, g := range p.Method.Gaps {
		gapped[g.Name] = g
	}
	if _, ok := gapped["Full planche"]; ok {
		t.Error("a 12-second full planche clears the 10-second entry standard and is not a gap")
	}
	for _, want := range []string{"Back lever", "Ring support hold"} {
		if _, ok := gapped[want]; !ok {
			t.Errorf("%q is an unmet entry standard and should be listed; got %v", want, keysOf(gapped))
		}
	}
	for _, g := range p.Method.Gaps {
		if g.Why == "" || g.Standard == "" || g.Have == "" {
			t.Errorf("a gap has to say what it wants, what you have and why: %+v", g)
		}
	}

	// And the plan trains the gaps rather than the skill.
	if findBlock(p, "maltese") != nil || findBlock(p, "straddle_maltese") != nil {
		t.Error("a gated maltese should not appear in the sessions")
	}
	if findBlock(p, "back_lever") == nil && findBlock(p, "ring_support_hold") == nil {
		t.Error("the plan should train the gaps it named")
	}
	if !containsAny(p.Notes, "not open yet") {
		t.Errorf("the athlete should be told plainly: %v", p.Notes)
	}
}

func TestTheMalteseOpensOnceTheStandardsAreThere(t *testing.T) {
	lib := seededLibrary(t)
	snap := eliteAthlete()
	snap.Records = append(snap.Records,
		declared("back_lever", 0, 0, 15),
		declared("ring_support_hold", 0, 0, 45))

	p, _ := Generate(Request{Goal: "maltese", Weeks: 12, DaysPerWeek: 5}, snap, lib)
	if !p.Method.EntryMet {
		t.Fatalf("every entry standard is met now; gaps were %+v", p.Method.Gaps)
	}
	if p.Method.Rung != "Rings, and shoulders that tolerate them" && p.Method.Rung != "Maltese lean" {
		t.Errorf("placed on %q, want the bottom of the maltese ladder", p.Method.Rung)
	}

	// A movement with nothing behind it starts small, however strong he is.
	first := findBlock(p, "ring_support_hold")
	if first == nil {
		first = findBlock(p, "maltese_lean")
	}
	if first == nil {
		t.Fatal("the maltese ladder should be trained")
	}
	if first.Sets > 3 {
		t.Errorf("a brand-new movement got %d sets; a first exposure is capped", first.Sets)
	}
	if !strings.Contains(first.Notes, "Nothing logged on this movement yet") {
		t.Errorf("and it should say why it is small: %q", first.Notes)
	}
	if !strings.Contains(first.Notes, "finding out") {
		t.Errorf("a movement with no number behind it should ask for one: %q", first.Notes)
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
