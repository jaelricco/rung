package plan

import (
	"strings"
	"testing"

	"calisthenics/api/internal/training"
)

// accessoriesOf collects every block the plan calls supporting work, which is
// what this file is about: the slots that used to be filled from a fixed list.
func accessoriesOf(p Plan) []Block {
	var out []Block
	for _, session := range p.Sessions {
		for _, block := range session.Blocks {
			if block.Intent == "accessory" || block.Intent == "conditioning" {
				out = append(out, block)
			}
		}
	}
	return out
}

// The complaint this file exists to answer: an athlete who holds a front lever
// and a straddle planche should not be handed australian rows and hollow body
// holds. Those are not light accessory work at that level, they are a warm-up
// with a set count, and the slot they take is one a real block would have had.
func TestAccessoriesLandAtTheAthletesOwnLevel(t *testing.T) {
	lib := seededLibrary(t)
	snap := advancedAthlete()

	// Things this athlete outgrew years ago. None of them is wrong in itself —
	// they are all in the library because somebody needs them — they are just
	// not this person's accessory work.
	outgrown := map[string]bool{
		"australian_row": true, "hollow_body_hold": true, "hanging_knee_raise": true,
		"plank": true, "arch_body_hold": true, "push_up": true, "band_assisted_pull_up": true,
		"negative_pull_up": true, "hollow_rock": true,
	}
	for _, goal := range []string{"front lever pull up", "planche", "front lever", "maltese"} {
		for _, days := range []int{3, 5, 6} {
			p, _ := Generate(Request{Goal: goal, Weeks: 6, DaysPerWeek: days}, snap, lib)
			for _, block := range accessoriesOf(p) {
				if outgrown[block.ExerciseSlug] {
					t.Errorf("%s/%dd: %q as accessory work for someone holding a 12s front lever",
						goal, days, block.ExerciseSlug)
				}
			}
		}
	}
}

// And the other direction, which is the one that hurts: scaling accessories up
// must never hand a beginner an elite movement. The first version of this did
// exactly that — a plan for somebody's first pull-up prescribing deficit
// handstand push-ups — because the level band closed over nothing and the
// fallback handed back the hardest entry in the pool.
func TestABeginnerIsNeverPrescribedAnEliteAccessory(t *testing.T) {
	lib := seededLibrary(t)
	beginners := []training.Snapshot{
		snapshotOf(0, 75),
		snapshotOf(4, 75, rec("push_up", 12, 0, 0), rec("australian_row", 10, 0, 0)),
		snapshotOf(6, 75, rec("pull_up", 2, 0, 0), rec("dip", 3, 0, 0)),
	}
	for _, snap := range beginners {
		for _, goal := range allGoalKeys() {
			p, _ := Generate(Request{Goal: goal, Weeks: 4, DaysPerWeek: 4}, snap, lib)
			for _, block := range accessoriesOf(p) {
				if d := lib.Exercises[block.ExerciseSlug].Difficulty; d > 4 {
					t.Errorf("%s: %q (difficulty %d) as accessory work for a beginner",
						goal, block.ExerciseSlug, d)
				}
			}
		}
	}
}

// The general rule behind both: an accessory is never harder than the athlete
// has shown in that kind of work. Movements that scale with load are exempt —
// a weighted pull-up is as hard as the plate on the belt.
func TestNoAccessoryIsHarderThanTheAthleteHasShown(t *testing.T) {
	lib := seededLibrary(t)
	for _, snap := range []training.Snapshot{
		snapshotOf(0, 75), advancedAthlete(), maltesePlacedAthlete(),
		snapshotOf(6, 75, rec("pull_up", 2, 0, 0), rec("dip", 3, 0, 0)),
	} {
		for _, goal := range allGoalKeys() {
			req := Request{Goal: goal, Weeks: 4, DaysPerWeek: 5}
			p, _ := Generate(req, snap, lib)
			b := newBuilder(req, snap, lib)
			for _, block := range accessoriesOf(p) {
				slug := block.ExerciseSlug
				if b.scalesWithLoad(slug) {
					continue
				}
				if got, allowed := lib.Exercises[slug].Difficulty, b.levelOf(slug); got > allowed {
					t.Errorf("%s: %q is difficulty %d against a demonstrated %d in that pattern",
						goal, slug, got, allowed)
				}
			}
		}
	}
}

// A prescription's rep count is capped by the movement, not by whichever slot
// asked for it. Twelve is what an accessory is worth; hand that number to a
// one-arm negative and the block reads "8-15 reps" of something nobody does
// more than three of.
//
// The cap applies where the number was guessed. A movement the athlete has
// logged is priced from their own set instead, and their number outranks the
// cap — telling somebody who logged six front lever rows that nobody does more
// than three is the app arguing with its own evidence.
func TestNoGuessedRepCountExceedsWhatTheMovementIsWorth(t *testing.T) {
	lib := seededLibrary(t)
	for _, snap := range []training.Snapshot{snapshotOf(0, 75), advancedAthlete(), maltesePlacedAthlete()} {
		logged := map[string]bool{}
		for _, r := range snap.Records {
			logged[r.Slug] = r.BestReps != nil
		}
		for _, goal := range allGoalKeys() {
			p, _ := Generate(Request{Goal: goal, Weeks: 6, DaysPerWeek: 5}, snap, lib)
			for _, session := range p.Sessions {
				for _, block := range session.Blocks {
					top, ok := topOfRepRange(block.Prescription)
					if !ok || logged[block.ExerciseSlug] {
						continue
					}
					most := repsWorthDoing(lib.Exercises[block.ExerciseSlug].Difficulty)
					if top > most {
						t.Errorf("%s: %q asks for %q with nothing logged, and nobody does more than %d of it",
							goal, block.ExerciseSlug, block.Prescription, most)
					}
				}
			}
		}
	}
}

// And the other half of that rule: a logged set is what a block is priced
// from, cap or no cap.
func TestALoggedSetOutranksTheRepCap(t *testing.T) {
	lib := seededLibrary(t)
	// Six front lever rows, which the library rates a 9 and the cap would hold
	// to three.
	snap := snapshotOf(20, 70, rec("pull_up", 20, 0, 0), rec("front_lever", 0, 0, 22),
		rec("front_lever_row", 6, 0, 0))
	p, _ := Generate(Request{Goal: "front lever pull up", Weeks: 6, DaysPerWeek: 4}, snap, lib)

	for _, session := range p.Sessions {
		for _, block := range session.Blocks {
			if block.ExerciseSlug != "front_lever_row" {
				continue
			}
			top, ok := topOfRepRange(block.Prescription)
			if ok && top <= repsWorthDoing(9) {
				t.Errorf("a logged six-rep set was priced as %q, which is the cap overruling the log",
					block.Prescription)
			}
			return
		}
	}
	t.Fatal("the plan for a front lever pull-up never prescribed a front lever row")
}

// topOfRepRange reads "5-10 reps" and "3-8 reps with +20 kg", and ignores
// everything that is not a rep range — holds, EMOMs, circuits, attempts.
func topOfRepRange(prescription string) (int, bool) {
	if !strings.Contains(prescription, " reps") || !strings.Contains(prescription, "-") {
		return 0, false
	}
	head, _, _ := strings.Cut(prescription, " reps")
	_, top, found := strings.Cut(head, "-")
	if !found {
		return 0, false
	}
	n := 0
	for _, r := range top {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n, n > 0
}

// The antagonist block balances the day. A block that trains the same thing
// the day trained balances nothing, so the goal's own line is the one thing
// that slot may not reach for.
func TestTheAntagonistIsNeverTheGoalsOwnWork(t *testing.T) {
	lib := seededLibrary(t)
	for _, snap := range []training.Snapshot{advancedAthlete(), maltesePlacedAthlete()} {
		for _, goal := range allGoalKeys() {
			req := Request{Goal: goal, Weeks: 4, DaysPerWeek: 6}
			b := newBuilder(req, snap, lib)
			for day := 1; day <= 7; day++ {
				for _, role := range []dayRole{roleSkill, roleOpposite} {
					s := &sessionBuilder{builder: b,
						week: weekSpec{Week: 1, Phase: phaseAccumulation, Fraction: 0.6},
						day:  daySpec{Day: day, Role: role, Hard: true}, used: map[string]bool{}}
					if slug := s.available(s.keepOnLine(s.balanceChain())); slug != "" && b.line[slug] {
						t.Errorf("%s day %d: the counterweight is %q, which is the goal's own work",
							goal, day, slug)
					}
				}
			}
		}
	}
}
