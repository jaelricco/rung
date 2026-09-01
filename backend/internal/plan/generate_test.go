package plan

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"calisthenics/api/internal/training"
)

func TestMatchGoalTakesTheLongestAlias(t *testing.T) {
	cases := []struct {
		typed string
		want  string
	}{
		{"front lever", "front_lever"},
		{"Full front lever", "front_lever"},
		{"FRONTLEVER", "front_lever"},
		{"muscle up", "muscle_up"},
		{"ring muscle up", "ring_muscle_up"},       // not the bar muscle-up
		{"weighted pull up", "weighted_pull_up"},   // not the first pull-up
		{"one arm pull up", "one_arm_pull_up"},     // nor that
		{"handstand push up", "handstand_push_up"}, // not the handstand
		{"20 kg weighted pull-up", "weighted_pull_up"},
		{"erster Klimmzug", "pull_up"},
		{"Menschliche Flagge", "human_flag"},
		{"gewichtete Klimmzüge", "weighted_pull_up"},
		{"einbeinige Kniebeuge", "pistol_squat"},
	}
	for _, c := range cases {
		got, matched := MatchGoal(c.typed)
		if !matched || got.Key != c.want {
			t.Errorf("MatchGoal(%q) = %q (matched %v), want %q", c.typed, got.Key, matched, c.want)
		}
	}

	// Anything unrecognised is balanced strength, and says so rather than
	// failing. A goal the app has never heard of is still a person who wants
	// to train on Monday.
	for _, typed := range []string{"", "become awesome", "ich will stark werden", "🏋"} {
		got, matched := MatchGoal(typed)
		if matched || got.Key != "general" {
			t.Errorf("MatchGoal(%q) = %q (matched %v), want the general track", typed, got.Key, matched)
		}
	}
}

func TestANamedTargetTrimsTheLadderToIt(t *testing.T) {
	lib := seededLibrary(t)
	snap := snapshotOf(12, 70, rec("pull_up", 12, 0, 0))

	p, _ := Generate(Request{Goal: "20 kg weighted pull-up", Weeks: 8, DaysPerWeek: 3}, snap, lib)
	last := p.Method.Ladder[len(p.Method.Ladder)-1]
	if !strings.Contains(last.Standard, "+20 kg") {
		t.Errorf("the ladder should end at the 20 kg the athlete asked for, ended at %q", last.Standard)
	}

	// Without a named target it runs to the catalogue's own last rung.
	full, _ := Generate(Request{Goal: "weighted pull-up", Weeks: 8, DaysPerWeek: 3}, snap, lib)
	if len(full.Method.Ladder) <= len(p.Method.Ladder) {
		t.Errorf("naming a target should shorten the ladder: %d rungs vs %d",
			len(p.Method.Ladder), len(full.Method.Ladder))
	}
}

func TestPlacementFollowsTheLogAndNeverGoesBackwards(t *testing.T) {
	lib := seededLibrary(t)

	// Cleared the tuck, part-way into the advanced tuck, and has never logged
	// the inverted hang three rungs below. The plan trains the advanced tuck.
	snap := snapshotOf(12, 72,
		rec("tuck_front_lever", 0, 0, 25),
		rec("adv_tuck_front_lever", 0, 0, 9))
	p, _ := Generate(Request{Goal: "front lever", Weeks: 8, DaysPerWeek: 3}, snap, lib)
	if got := currentRung(p); got != "Advanced tuck" {
		t.Errorf("placed on %q, want the advanced tuck — an unlogged rung below a cleared one is not a reason to go back", got)
	}

	// Nothing logged at all is a beginner, whatever they typed.
	empty, _ := Generate(Request{Goal: "front lever", Weeks: 8, DaysPerWeek: 3}, training.Snapshot{}, lib)
	if got := currentRung(empty); got != "Inversion and body line" {
		t.Errorf("with an empty log the plan should start at the bottom of the ladder, started at %q", got)
	}

	// The goal rung is never "cleared": you maintain a skill, you do not
	// outgrow it.
	strong := snapshotOf(20, 72, rec("front_lever", 0, 0, 25))
	held, _ := Generate(Request{Goal: "front lever", Weeks: 8, DaysPerWeek: 3}, strong, lib)
	if got := currentRung(held); got != "Full front lever" {
		t.Errorf("an athlete past the last standard should stay on it, got %q", got)
	}
}

func TestAnOpenInjuryRemovesEveryMovementThatLoadsIt(t *testing.T) {
	lib := seededLibrary(t)
	snap := snapshotOf(12, 70, rec("pull_up", 12, 0, 0), rec("dip", 15, 0, 0))
	snap.OpenInjuries = []training.Injury{{Region: "wrist", Severity: 3}}

	p, _ := Generate(Request{Goal: "planche", Weeks: 6, DaysPerWeek: 4}, snap, lib)
	if len(p.Sessions) == 0 {
		t.Fatal("an injured wrist is not a reason to answer with no plan")
	}
	for _, session := range p.Sessions {
		for _, block := range session.Blocks {
			if wristLoaded[block.ExerciseSlug] {
				t.Errorf("week %d: %q loads the injured wrist and should have been removed",
					session.Week, block.ExerciseSlug)
			}
		}
		for _, slug := range session.WarmupProtocols {
			if p, ok := lib.Protocols[slug]; ok && p.Purpose == "warmup" && p.Region == "wrist" {
				t.Errorf("week %d: the wrist warm-up should be replaced by its rehab protocol, not run alongside it", session.Week)
			}
		}
	}
	if !containsAny(p.Restrictions, "wrist") {
		t.Errorf("the plan has to say what it removed and why; restrictions were %v", p.Restrictions)
	}
	if !containsAny(p.Restrictions, "assessment") {
		t.Errorf("an injury restriction has to point at a clinician; restrictions were %v", p.Restrictions)
	}

	// The rehab protocol is in every warm-up, not just the first one.
	for _, session := range p.Sessions {
		if !contains(session.WarmupProtocols, "wrist_rehab_light") {
			t.Fatalf("week %d day %d dropped the rehab protocol", session.Week, session.DayOfWeek)
		}
	}
}

func TestEveryUpperBodyMovementGoesWithAShoulderInjury(t *testing.T) {
	lib := seededLibrary(t)
	snap := snapshotOf(12, 70, rec("pull_up", 12, 0, 0))
	snap.OpenInjuries = []training.Injury{{Region: "shoulder", Severity: 4}}

	// Almost everything this sport does loads the shoulder, so this is the
	// case that most easily produces an empty plan. It must not.
	p, _ := Generate(Request{Goal: "front lever", Weeks: 8, DaysPerWeek: 3}, snap, lib)
	if len(p.Sessions) == 0 {
		t.Fatal("a shoulder injury left no plan at all")
	}
	for _, session := range p.Sessions {
		if len(session.Blocks) == 0 {
			t.Fatal("a session with no blocks reached the calendar")
		}
	}
	if !containsAny(p.Restrictions, "ladder") {
		t.Errorf("when the skill itself is removed the plan has to say so; restrictions were %v", p.Restrictions)
	}
}

func TestDeloadsAndTheTestWeekLandWhereTheEvidenceSaysTheyShould(t *testing.T) {
	lib := seededLibrary(t)
	snap := snapshotOf(12, 72, rec("pull_up", 12, 0, 0))

	cases := []struct {
		weeks   int
		deloads []int
		test    int
	}{
		{2, nil, 0},               // too short for either
		{4, nil, 4},               // too short to need a deload
		{6, []int{4}, 6},          // one lighter week, mid-plan
		{8, []int{4, 7}, 8},       // and a taper into the test
		{12, []int{4, 8, 11}, 12}, // every fourth week, taper before the test
	}
	for _, c := range cases {
		p, _ := Generate(Request{Goal: "front lever", Weeks: c.weeks, DaysPerWeek: 3}, snap, lib)
		got := weeksWithLoad(p, "deload")
		if fmt.Sprint(got) != fmt.Sprint(c.deloads) {
			t.Errorf("%d weeks: deloads in %v, want %v", c.weeks, got, c.deloads)
		}
		if c.test > 0 && !hasTestSession(p, c.test) {
			t.Errorf("%d weeks: no test session in week %d", c.weeks, c.test)
		}
		if c.test == 0 && hasTestSession(p, c.weeks) {
			t.Errorf("%d weeks: too short to be worth testing, but a test was scheduled", c.weeks)
		}
	}
}

func TestHardSessionsForOnePatternStay48HoursApart(t *testing.T) {
	lib := seededLibrary(t)
	snap := snapshotOf(12, 72, rec("pull_up", 12, 0, 0), rec("dip", 14, 0, 0))

	for days := 1; days <= 7; days++ {
		p, _ := Generate(Request{Goal: "front lever", Weeks: 4, DaysPerWeek: days}, snap, lib)
		byDay := map[int]Session{}
		for _, s := range p.Sessions {
			if s.Week == 1 {
				byDay[s.DayOfWeek] = s
			}
		}
		for day, session := range byDay {
			next, ok := byDay[day+1]
			if !ok || session.Load != "hard" || next.Load != "hard" {
				continue
			}
			if sameFocus(session, next) {
				t.Errorf("%d days a week: %q on day %d is followed by %q on day %d",
					days, session.Title, day, next.Title, day+1)
			}
		}
		if hard := countLoad(p, 1, "hard"); hard > 4 {
			t.Errorf("%d days a week: %d hard sessions in week 1, which is more than anyone recovers from", days, hard)
		}
	}
}

func TestWeeklyHardSetsStayInsideTheEvidenceBand(t *testing.T) {
	lib := seededLibrary(t)
	snap := snapshotOf(16, 72, rec("pull_up", 12, 0, 0), rec("dip", 14, 0, 0), rec("tuck_front_lever", 0, 0, 25))

	for days := 3; days <= 6; days++ {
		p, _ := Generate(Request{Goal: "front lever", Weeks: 8, DaysPerWeek: days}, snap, lib)
		worst := 0
		for week := 1; week <= 8; week++ {
			sets := map[string]int{}
			for _, s := range p.Sessions {
				if s.Week != week {
					continue
				}
				for _, b := range s.Blocks {
					if b.Intent == "prep" || b.Intent == "conditioning" {
						continue
					}
					sets[lib.Exercises[b.ExerciseSlug].Category] += b.Sets
				}
			}
			for _, n := range sets {
				if n > worst {
					worst = n
				}
			}
		}
		// 12 to 20 weekly sets per pattern is the band the dose-response work
		// supports; the ceiling here allows the top of a build week plus the
		// accessory that shares the category.
		if worst > 30 {
			t.Errorf("%d days a week peaks at %d sets on one category in a week, which is past useful", days, worst)
		}
	}
}

func TestPrescriptionsMatchHowTheMovementIsMeasured(t *testing.T) {
	lib := seededLibrary(t)
	snap := snapshotOf(12, 72, rec("pull_up", 12, 0, 0), rec("dip", 14, 0, 0))

	for _, goal := range []string{"front lever", "l-sit", "handstand", "planche", "human flag"} {
		p, _ := Generate(Request{Goal: goal, Weeks: 6, DaysPerWeek: 4}, snap, lib)
		for _, session := range p.Sessions {
			for _, block := range session.Blocks {
				measure := lib.Exercises[block.ExerciseSlug].Measure
				hold := strings.Contains(block.Prescription, "hold") || strings.Contains(block.Prescription, "maximum hold")
				format := strings.Contains(block.Prescription, "EMOM") || strings.Contains(block.Prescription, "AMRAP") ||
					strings.Contains(block.Prescription, "circuit") || strings.Contains(block.Prescription, "controlled reps") ||
					strings.Contains(block.Prescription, "per position") || strings.Contains(block.Prescription, "attempt")
				if measure == "static_hold" && !hold && !format {
					t.Errorf("%s: %q is measured in seconds but was prescribed as %q",
						goal, block.ExerciseSlug, block.Prescription)
				}
				if measure == "reps" && hold {
					t.Errorf("%s: %q is measured in reps but was prescribed as a hold: %q",
						goal, block.ExerciseSlug, block.Prescription)
				}
				if measure == "weighted_reps" && !strings.Contains(block.Prescription, "kg") {
					t.Errorf("%s: %q takes added load but was prescribed without any: %q",
						goal, block.ExerciseSlug, block.Prescription)
				}
			}
		}
	}
}

func TestAddedLoadIsBuiltFromBodyweightAndComesDownWithoutIt(t *testing.T) {
	lib := seededLibrary(t)

	withWeight := snapshotOf(16, 80, rec("pull_up", 15, 0, 0))
	p, _ := Generate(Request{Goal: "weighted pull-up", Weeks: 6, DaysPerWeek: 3}, withWeight, lib)
	block := findBlock(p, "weighted_pull_up")
	if block == nil {
		t.Fatal("a weighted pull-up plan for a 15-rep athlete has to contain weighted pull-ups")
	}
	// Epley on total load: 80 kg at 15 reps is a 120 kg max, so a six-rep set
	// is 100 kg total — twenty on the belt, give or take a plate.
	if !strings.Contains(block.Prescription, "+20 kg") {
		t.Errorf("expected roughly +20 kg for a 15-rep athlete at 80 kg, got %q", block.Prescription)
	}
	if !strings.Contains(block.Intensity, "bodyweight included") {
		t.Errorf("the block should say what the load was calculated from, said %q", block.Intensity)
	}

	// No bodyweight on file means no honest percentage, so it says so instead
	// of inventing one.
	noWeight := snapshotOf(16, 0, rec("pull_up", 15, 0, 0))
	q, _ := Generate(Request{Goal: "weighted pull-up", Weeks: 6, DaysPerWeek: 3}, noWeight, lib)
	if block := findBlock(q, "weighted_pull_up"); block == nil ||
		!strings.Contains(block.Intensity, "bodyweight in your profile") {
		t.Errorf("without a bodyweight the plan should ask for one rather than guess: %+v", block)
	}
}

func TestEveryBlockIsUsableAndEverySessionIsWorthDoing(t *testing.T) {
	lib := seededLibrary(t)
	snap := snapshotOf(10, 74, rec("pull_up", 9, 0, 0), rec("dip", 11, 0, 0), rec("l_sit", 0, 0, 18))

	for _, goal := range allGoalKeys() {
		for _, weeks := range []int{1, 3, 6, 12, 24} {
			for days := 1; days <= 7; days++ {
				p, warnings := Generate(Request{Goal: goal, Weeks: weeks, DaysPerWeek: days}, snap, lib)
				where := fmt.Sprintf("%s/%dw/%dd", goal, weeks, days)

				if len(p.Sessions) == 0 {
					t.Fatalf("%s: produced no sessions at all", where)
				}
				if len(warnings) > 0 {
					t.Errorf("%s: warned about its own output: %v", where, warnings)
				}
				if p.Title == "" || p.Summary == "" || p.Test == "" {
					t.Errorf("%s: a plan without a title, a summary or a test is not finished", where)
				}
				if len(p.Phases) == 0 || len(p.ProgressionRules) == 0 {
					t.Errorf("%s: no phases or no progression rules", where)
				}
				for _, session := range p.Sessions {
					if session.Week < 1 || session.Week > weeks {
						t.Errorf("%s: a session landed in week %d", where, session.Week)
					}
					if len(session.Blocks) == 0 {
						t.Errorf("%s: empty session in week %d", where, session.Week)
					}
					if session.Title == "" || session.Focus == "" || session.DurationMinutes <= 0 {
						t.Errorf("%s: session in week %d is missing its heading", where, session.Week)
					}
					for _, block := range session.Blocks {
						if !lib.Has(block.ExerciseSlug) {
							t.Errorf("%s: %q is not in the library", where, block.ExerciseSlug)
						}
						if block.Sets < 1 || block.Prescription == "" {
							t.Errorf("%s: %q has no dosage", where, block.ExerciseSlug)
						}
						// Rule 9 of the app's own coaching brief: prescribe
						// intensity, not only volume. And rule 10: say what
						// changes next week.
						if block.Intensity == "" || block.Progression == "" {
							t.Errorf("%s: %q says how much but not how hard or what changes: %+v",
								where, block.ExerciseSlug, block)
						}
					}
				}
			}
		}
	}
}

func TestTheSameRequestAlwaysProducesTheSamePlan(t *testing.T) {
	lib := seededLibrary(t)
	snap := snapshotOf(11, 71, rec("pull_up", 10, 0, 0), rec("tuck_planche", 0, 0, 12))
	snap.OpenInjuries = []training.Injury{{Region: "elbow", Severity: 2}}
	req := Request{Goal: "planche", Weeks: 12, DaysPerWeek: 5, Notes: "rings at home"}

	first, _ := Generate(req, snap, lib)
	for i := 0; i < 25; i++ {
		next, _ := Generate(req, snap, lib)
		if !sameJSON(t, first, next) {
			t.Fatalf("run %d differed from the first one; two athletes with the same log must get the same plan", i)
		}
	}
}

func TestGenerateSurvivesEverythingItCanBeAsked(t *testing.T) {
	lib := seededLibrary(t)
	full := snapshotOf(12, 70, rec("pull_up", 10, 0, 0))
	everythingHurts := full
	for _, region := range []string{"wrist", "elbow", "shoulder", "chest", "back", "core", "hip", "knee", "ankle", "other"} {
		everythingHurts.OpenInjuries = append(everythingHurts.OpenInjuries,
			training.Injury{Region: region, Severity: 5})
	}

	requests := []Request{
		{},
		{Goal: "front lever", Weeks: 0, DaysPerWeek: 0},
		{Goal: "front lever", Weeks: -5, DaysPerWeek: -1},
		{Goal: "front lever", Weeks: 100000, DaysPerWeek: 99},
		{Goal: strings.Repeat("planche ", 500), Weeks: 24, DaysPerWeek: 7},
		{Goal: "\x00\x01 front lever", Weeks: 8, DaysPerWeek: 3},
		{Goal: "999999999 kg weighted pull-up", Weeks: 8, DaysPerWeek: 3},
		{Goal: "0 kg weighted pull-up", Weeks: 8, DaysPerWeek: 3},
	}
	snapshots := []training.Snapshot{{}, full, everythingHurts}
	libraries := []Library{lib, {}, {Exercises: map[string]training.Exercise{}, Protocols: map[string]training.Protocol{}}}

	for i, req := range requests {
		for j, snap := range snapshots {
			for k, library := range libraries {
				p, _ := Generate(req, snap, library)
				if p.Weeks < 1 || p.Weeks > 24 {
					t.Errorf("req %d/snap %d/lib %d: %d weeks is not a plan anyone asked for", i, j, k, p.Weeks)
				}
				if p.Summary == "" {
					t.Errorf("req %d/snap %d/lib %d: answered without saying anything", i, j, k)
				}
				// An empty library is the one case with nothing legal to
				// prescribe. It still has to answer, and say why.
				if len(library.Exercises) == 0 && !strings.Contains(p.Summary, "empty") {
					t.Errorf("req %d/snap %d/lib %d: an empty library should be explained, said %q", i, j, k, p.Summary)
				}
			}
		}
	}
}

func TestASkillGoalPutsItsSkillWorkFirst(t *testing.T) {
	lib := seededLibrary(t)
	snap := snapshotOf(12, 72, rec("pull_up", 12, 0, 0), rec("tuck_front_lever", 0, 0, 22))

	p, _ := Generate(Request{Goal: "front lever", Weeks: 8, DaysPerWeek: 4}, snap, lib)
	order := map[string]int{"prep": 0, "skill": 1, "strength": 2, "accessory": 3, "conditioning": 4}
	for _, session := range p.Sessions {
		last := -1
		for _, block := range session.Blocks {
			rank, ok := order[block.Intent]
			if !ok {
				t.Fatalf("unknown intent %q", block.Intent)
			}
			if rank < last {
				t.Errorf("week %d day %d: %s work came after %s work",
					session.Week, session.DayOfWeek, block.Intent, invert(order, last))
			}
			last = rank
		}
	}

	// Straight-arm work never appears without elbow preparation in front of
	// it. That rule has no exceptions, which is why it is tested.
	for _, session := range p.Sessions {
		straightArm := false
		for _, block := range session.Blocks {
			if block.Intent == "skill" {
				straightArm = true
			}
		}
		if !straightArm {
			continue
		}
		if !contains(session.WarmupProtocols, "straight_arm_warmup") {
			t.Errorf("week %d day %d has straight-arm work and no elbow preparation",
				session.Week, session.DayOfWeek)
		}
	}
}

func TestConditioningNeverLandsOnAHardDay(t *testing.T) {
	lib := seededLibrary(t)
	snap := snapshotOf(14, 72, rec("pull_up", 12, 0, 0))

	for days := 1; days <= 7; days++ {
		p, _ := Generate(Request{Goal: "muscle up", Weeks: 8, DaysPerWeek: days}, snap, lib)
		for _, session := range p.Sessions {
			for _, block := range session.Blocks {
				if block.Intent == "conditioning" && session.Load == "hard" {
					t.Errorf("%d days: a clock on a hard day (week %d, %q)", days, session.Week, session.Title)
				}
			}
		}
	}
}

// ---------- helpers ----------

func allGoalKeys() []string {
	out := make([]string, 0, len(Goals))
	for _, g := range Goals {
		out = append(out, g.Key)
	}
	return out
}

func currentRung(p Plan) string {
	if p.Method == nil {
		return ""
	}
	return p.Method.Rung
}

func weeksWithLoad(p Plan, load string) []int {
	seen := map[int]bool{}
	var out []int
	for _, s := range p.Sessions {
		if s.Load == load && !seen[s.Week] {
			seen[s.Week] = true
			out = append(out, s.Week)
		}
	}
	return out
}

func countLoad(p Plan, week int, load string) int {
	n := 0
	for _, s := range p.Sessions {
		if s.Week == week && s.Load == load {
			n++
		}
	}
	return n
}

func hasTestSession(p Plan, week int) bool {
	for _, s := range p.Sessions {
		if s.Week == week && strings.HasPrefix(s.Title, "Test:") {
			return true
		}
	}
	return false
}

func sameFocus(a, b Session) bool { return a.Title == b.Title }

func findBlock(p Plan, slug string) *Block {
	for _, s := range p.Sessions {
		for i, b := range s.Blocks {
			if b.ExerciseSlug == slug {
				return &s.Blocks[i]
			}
		}
	}
	return nil
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func containsAny(list []string, substring string) bool {
	for _, item := range list {
		if strings.Contains(item, substring) {
			return true
		}
	}
	return false
}

func invert(order map[string]int, rank int) string {
	for name, value := range order {
		if value == rank {
			return name
		}
	}
	return "?"
}

func sameJSON(t *testing.T, a, b Plan) bool {
	t.Helper()
	left, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	right, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return string(left) == string(right)
}
