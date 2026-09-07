package plan

import "sort"

// Accessories at the athlete's own level.
//
// The planner picked its supporting work from fixed lists, and fixed lists
// have one failure mode that gets worse the better the athlete is: they keep
// prescribing the movement that used to be hard. An athlete who holds a front
// lever and is training the front lever pull-up does not need an australian
// row or a hollow body hold in their week. Those are not light accessory work
// for them, they are a warm-up wearing a set count — and a set that costs time
// and buys nothing is worse than no set, because it displaces one that would
// have.
//
// The fix is not more tiers. Tiers were what broke: the core chain had three
// of them and every one keyed on a hanging leg raise or an L-sit, so somebody
// whose log is full of levers and planches and empty of leg raises fell to the
// bottom of it and got planks. What the library already has is a difficulty
// rating on every movement, and what the athlete already has is a record on
// the hardest thing they can do. Those two numbers are the tiering, and they
// need nothing kept in step.
//
// So: one ordered pool per pattern, hardest first, and a band around the
// athlete's own ceiling decides which entry of it they get.

// accessoryGap is how far below the hardest thing on record an accessory may
// sit and still be worth a set. Three points of the library's ten-point scale
// is roughly one rung of a ladder either side of where the athlete is — close
// enough to be worth doing, far enough not to be a second main lift.
const accessoryGap = 3

// conditioningGap is wider, because a conditioning piece is *supposed* to be
// submaximal: the clock caps the rest and the movement has to stay clean under
// fatigue. It still should not be the movement the athlete outgrew years ago.
const conditioningGap = 5

// beginnerCeiling is where an athlete with nothing on record is assumed to be.
// It resolves the pools to push-ups, rows and squats, which is both the right
// answer for a beginner and the safe direction to be wrong in.
const beginnerCeiling = 2

// The accessory pools, hardest first. One list per pattern rather than a tier
// per level: the athlete's ceiling picks the entry, so there is nothing to
// keep in step and a new movement joins by being added once.
//
// No weighted movement is in here. A belt turns an accessory into a lift, and
// these blocks are prescribed at accessory dosage. Where a goal genuinely
// wants loaded work beside it — the front lever pull-up does — it says so in
// its own accessory list, which is consulted first.
var accessoryPools = map[string]chain{
	patternPull: {
		"front_lever_row", "ice_cream_maker", "front_lever_raise", "one_arm_negative",
		"typewriter_pull_up", "archer_pull_up", "l_sit_pull_up", "tuck_front_lever_row",
		"wide_pull_up", "chin_up", "pull_up", "band_assisted_pull_up", "australian_row",
	},
	patternPush: {
		"deficit_hspu", "russian_dip", "handstand_push_up", "wall_hspu", "negative_hspu",
		"pseudo_planche_push_up", "ring_dip", "elevated_pike_push_up", "straight_bar_dip",
		"pike_push_up", "dip", "push_up",
	},
	patternCore: {
		"dragon_flag", "dragon_flag_negative", "toes_to_bar", "copenhagen_plank",
		"ab_wheel_rollout", "hanging_leg_raise", "hollow_rock", "hanging_knee_raise",
		"hollow_body_hold", "plank",
	},
	patternLegs: {
		"nordic_curl", "shrimp_squat", "pistol_squat", "reverse_nordic",
		"assisted_pistol_squat", "bulgarian_split", "jump_squat", "bodyweight_squat",
	},
}

// A movement is compared against the athlete's level in its own kind of work,
// not against their level overall. Someone holding a maltese is not therefore
// ready for a one-arm pull-up, and someone with a front lever is not therefore
// a runner. Statics and anything unmapped fall back to the overall figure,
// which is what the ladders are rated against anyway.
var categoryLevel = map[string]string{
	"pull": patternPull, "dynamic": patternPull,
	"push": patternPush,
	"core": patternCore,
	"legs": patternLegs,
}

// gaugeLevel finds the hardest thing the athlete has on record, overall and
// per category, as the library rates difficulty. Declared figures count, on
// the same rule as everywhere else: a stated number stands until a logged one
// replaces it.
//
// A hold of a second or two is not a hold, so it does not count; a rep or a
// kilo does, because you either did it or you did not.
func (b *builder) gaugeLevel() (int, map[string]int) {
	overall, byCategory := 0, map[string]int{}
	for slug, record := range b.rec {
		exercise, ok := b.lib.Exercises[slug]
		if !ok {
			continue
		}
		shown := (record.BestReps != nil && *record.BestReps >= 1) ||
			(record.BestWeight != nil && *record.BestWeight > 0) ||
			(record.BestHold != nil && *record.BestHold >= 3)
		if !shown {
			continue
		}
		overall = max(overall, exercise.Difficulty)
		byCategory[exercise.Category] = max(byCategory[exercise.Category], exercise.Difficulty)
	}
	if overall == 0 {
		overall = beginnerCeiling
	}
	return overall, byCategory
}

// levelOf is the ceiling this movement is measured against: the hardest thing
// the athlete has shown in the same kind of work.
//
// A pattern they have simply not logged does not make them a beginner in it —
// somebody with a full planche has been training for years whatever their
// pull-up column says — so the floor under a category is the overall figure
// less two, which is one useful step down rather than a reset.
func (b *builder) levelOf(slug string) int {
	exercise, ok := b.lib.Exercises[slug]
	if !ok {
		return b.hardest
	}
	pattern, mapped := categoryLevel[exercise.Category]
	if !mapped {
		return b.hardest
	}
	best := 0
	for category, pat := range categoryLevel {
		if pat == pattern {
			best = max(best, b.hardestIn[category])
		}
	}
	// The belt and the bar are the same pattern, so weighted work counts.
	if pattern == patternPull || pattern == patternPush {
		best = max(best, b.hardestIn["weighted"])
	}
	// Never below one. A beginner's ceiling has to leave the bottom of the
	// library reachable, or the band closes over nothing and the fallback
	// hands them the hardest movement in the pool — which is how a plan for
	// somebody's first pull-up ended up prescribing deficit handstand
	// push-ups the first time this was written.
	return max(best, b.hardest-2, 1)
}

// scalesWithLoad reports a movement whose difficulty is not the number in the
// library. A weighted pull-up is as hard as the plate on the belt, and a band
// face pull is prescribed for what it does to a shoulder rather than for how
// hard it is. Neither is ever "too easy for this athlete", so neither is held
// to the floor.
func (b *builder) scalesWithLoad(slug string) bool {
	exercise, ok := b.lib.Exercises[slug]
	if !ok {
		return true
	}
	return exercise.Measure == "weighted_reps" || exercise.Measure == "weighted_hold" ||
		exercise.Category == "mobility"
}

// keepAtLevel narrows a candidate list to the movements worth a set for this
// athlete: nothing harder than they have shown, and nothing so far below it
// that the set is a formality.
//
// It never returns nothing. If the band is empty — a narrow pool, a library an
// injury filter has thinned — it falls back to dropping only what is too hard,
// and then to the list as it arrived. An accessory at the wrong level is a bad
// block; no block at all is a missing counterweight, and on a hard day that is
// the worse of the two.
func (b *builder) keepAtLevel(candidates chain, gap, ease int) chain {
	var known, novel, notTooHard chain
	for _, slug := range candidates {
		exercise, ok := b.lib.Exercises[slug]
		if !ok {
			continue
		}
		level := max(b.levelOf(slug)-ease, 1)
		if exercise.Difficulty > level && !b.scalesWithLoad(slug) {
			continue
		}
		notTooHard = append(notTooHard, slug)
		if exercise.Difficulty < level-gap && !b.scalesWithLoad(slug) {
			continue
		}
		// Inside the band, something they have actually done comes first. The
		// point of scaling accessories up is to stop prescribing work they
		// outgrew, not to fill their week with movements nobody has a number
		// for — a block priced from a rung's standard is a guess, and a guess
		// is fine for one block and wrong for three.
		if b.rec.source(slug) != "" {
			known = append(known, slug)
			continue
		}
		novel = append(novel, slug)
	}
	switch {
	case len(known)+len(novel) > 0:
		return append(known, novel...)
	case len(notTooHard) > 0:
		return notTooHard
	default:
		// Nothing in the list is at this athlete's level at all. The pools are
		// written hardest first, so handing back what arrived would hand back
		// the hardest thing in it; easiest first is the only safe direction to
		// be wrong in.
		out := append(chain{}, candidates...)
		sort.SliceStable(out, func(i, j int) bool {
			return b.lib.Exercises[out[i]].Difficulty < b.lib.Exercises[out[j]].Difficulty
		})
		return out
	}
}

// poolFor is the pattern's pool at this athlete's level, rotated by the day so
// a six-day week does not do the same accessory six times.
//
// The rotation happens first and the level filter second, which is the way
// round that matters: the filter puts what the athlete has a record for at the
// front, and rotating afterwards would shuffle that back out again.
func (s *sessionBuilder) poolFor(pattern string, day, ease int) chain {
	return s.keepAtLevel(rotate(accessoryPools[pattern], day), accessoryGap, ease)
}

// supportChain is the supporting work for the day, at this athlete's level.
//
// On a skill day it starts from what the goal names for itself, because a goal
// knows what it wants beside it — but only while that list still fits: once an
// athlete has outgrown every entry on it, the pattern's pool takes over rather
// than the list being prescribed anyway. On any other day it follows the day
// rather than the goal, which is what stops a pull goal putting heavy pulling
// on the push day that is meant to be 48 hours away from it.
func (s *sessionBuilder) supportChain() chain {
	pattern, out := s.goal.Pattern, chain{}
	if s.skillDay() {
		out = append(out, s.keepAtLevel(rotate(append(chain{}, s.goal.Accessories...),
			s.day.Day-1), accessoryGap, 0)...)
	} else {
		pattern = s.oppositePattern()
	}
	out = append(out, s.poolFor(pattern, s.day.Day-1, 0)...)
	out = append(out, "band_face_pull")
	// The goal's own list unfiltered, last, so a session still gets a block
	// when an injury or the equipment answer has taken everything above away.
	return append(out, s.goal.Accessories...)
}

// offGoalLine drops the goal's own work from a candidate list, for the one
// slot where it does not belong: the antagonist. Everything else in a session
// is allowed to serve the skill; this block exists to balance it, and a block
// that trains the same thing balances nothing.
func (s *sessionBuilder) offGoalLine(candidates chain) chain {
	kept := make(chain, 0, len(candidates))
	for _, slug := range candidates {
		if !s.line[slug] {
			kept = append(kept, slug)
		}
	}
	return kept
}
