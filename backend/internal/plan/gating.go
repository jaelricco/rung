package plan

import (
	"fmt"
	"sort"
	"strings"
)

// The three questions the elite end of the catalogue forced this planner to
// ask, and which the rungs below it never needed.
//
// May this athlete start this skill at all? What else is already spending from
// the same tissue? And what does a wrist that has hurt for months mean for a
// sport that is played on the hands? None of the three has a good answer in
// terms of sets and reps alone, which is why they live here rather than in the
// dosage code.

// ---------- may this ladder be entered ----------

// checkEntry compares the goal's prerequisites against what the athlete has
// actually demonstrated. Where they are not met the ladder is replaced by the
// gaps themselves, so the plan trains what is missing rather than a skill that
// is not yet safe to start — and says which numbers it wants and what it saw.
//
// This is the one place the planner tells an athlete no. It does it with their
// own figures rather than with an opinion, because "you are not ready" is only
// useful when it comes with what ready would look like.
func (b *builder) checkEntry() {
	if len(b.goal.Entry) == 0 {
		b.entryMet = true
		return
	}

	for _, req := range b.goal.Entry {
		have := b.rec.best(req.Slug, req.Metric)
		if have >= req.Standard {
			continue
		}
		b.gaps = append(b.gaps, Gap{
			Name:     b.exerciseName(req.Slug),
			Standard: measure(req.Standard, req.Metric),
			Have:     b.haveText(req.Slug, req.Metric, have),
			Why:      req.Why,
		})
	}

	b.entryMet = len(b.gaps) == 0
	if b.entryMet {
		return
	}

	// The gaps become the ladder. Everything downstream — placement, dosage,
	// the test in the final week — then works unchanged, and the athlete gets
	// a plan that ends with them holding the entry standard rather than one
	// that pretends the skill was available.
	ladder := make([]Step, 0, len(b.goal.Entry))
	for _, req := range b.goal.Entry {
		ladder = append(ladder, Step{
			Name:     "Entry standard: " + b.exerciseName(req.Slug),
			Movement: chain{req.Slug},
			Metric:   req.Metric,
			Standard: req.Standard,
			Assist:   b.goal.Drills,
			Typical:  "what opens the ladder",
		})
	}
	// Easiest gap first, so the plan closes the nearest one rather than the
	// most distant.
	sort.SliceStable(ladder, func(i, j int) bool {
		return b.shortfall(ladder[i]) < b.shortfall(ladder[j])
	})
	b.ladder = ladder

	missing := make([]string, 0, len(b.gaps))
	for _, gap := range b.gaps {
		missing = append(missing, strings.ToLower(gap.Name))
	}
	b.notes = append(b.notes, fmt.Sprintf(
		"%s is not open yet: %s. So this plan trains those rather than the skill. That is not a smaller goal — "+
			"it is the same goal with the part that gets you hurt taken out.",
		capitalise(b.goal.phrase()), humanList(missing)+" "+pluralVerb(len(b.gaps))+" short of the entry standard"))
}

// shortfall is how far off a requirement the athlete is, as a fraction, so the
// nearest gap can be closed first.
func (b *builder) shortfall(step Step) float64 {
	have := 0.0
	for _, slug := range step.Movement {
		if v := b.rec.best(slug, step.Metric); v > have {
			have = v
		}
	}
	if step.Standard <= 0 {
		return 0
	}
	return 1 - have/step.Standard
}

func (b *builder) haveText(slug, metric string, have float64) string {
	if have <= 0 {
		return "nothing logged or declared"
	}
	return measure(have, metric)
}

func pluralVerb(n int) string {
	if n == 1 {
		return "is"
	}
	return "are"
}

// ---------- what else is spending from the same tissue ----------

// tendonCeiling is how many units of maximal straight-arm work one athlete
// carries in a week. It is a deliberately blunt number: the literature has no
// dose-response curve for elite statics, and what coaching sources agree on is
// only the shape — that new skills layer load onto tissue the old ones have
// not prepared, and that almost every straight-arm injury follows a spike
// rather than a plateau.
const tendonCeiling = 5

// weighLoad adds up what the athlete is already learning and what they are
// asking for. Over the ceiling, the plan does not refuse — it names what to
// park, cheapest-to-park first, and keeps the goal that everything else feeds.
func (b *builder) weighLoad() {
	// The focus is part of the bill. A week built around a maximal static
	// spends more of the same tendon account than a week that merely contains
	// one — the sets are on the same tissue and the tissue does not know which
	// dial produced them — so the top level of the dial claims a unit more and
	// the other skills give way sooner. That is not a side effect of asking
	// for focus; it is what asking for focus means.
	claim := b.goal.Units() + b.focus.Claim
	load := &Load{Ceiling: tendonCeiling, Spent: claim}

	type other struct {
		goal Goal
		cost int
	}
	var others []other
	for _, key := range b.snap.Learning {
		g, ok := goalByKey[strings.TrimSpace(key)]
		if !ok || g.Key == b.goal.Key {
			continue
		}
		cost := g.Units()
		others = append(others, other{g, cost})
		load.Spent += cost
		load.Learning = append(load.Learning, g.Name)
	}

	if load.Spent <= load.Ceiling {
		if len(others) > 0 {
			load.Note = fmt.Sprintf(
				"%d of %d units of maximal straight-arm work, counting what you are already learning%s. "+
					"That is inside what one athlete recovers from.", load.Spent, load.Ceiling, b.focusCost())
		}
		b.load = load
		return
	}

	// Park the cheapest first: dropping a two-unit skill twice costs less than
	// dropping the four-unit one, and the goal being asked for is never the
	// thing that gets parked.
	sort.SliceStable(others, func(i, j int) bool { return others[i].cost < others[j].cost })
	spent := load.Spent
	for _, o := range others {
		if spent <= load.Ceiling {
			break
		}
		// A skill the goal is built on is not parked. Parking the planche to
		// chase a maltese loses both.
		if b.feeds(o.goal.Key) {
			continue
		}
		load.Parked = append(load.Parked, o.goal.Name)
		spent -= o.cost
	}

	load.Note = fmt.Sprintf(
		"%d units of maximal straight-arm work against a ceiling of %d%s. Every one of these loads the same "+
			"tendons, and they do not know which goal a set belonged to.", load.Spent, load.Ceiling, b.focusCost())
	b.load = load

	if len(load.Parked) > 0 {
		b.restrictions = append(b.restrictions, fmt.Sprintf(
			"Park %s for the length of this plan. Adding a skill without removing one is the shape almost every "+
				"straight-arm injury has: the load lands on tissue that the previous skill did not prepare. "+
				"They come back when %s is holding.",
			humanList(load.Parked), b.goal.phrase()))
		// A week that is still over budget after parking is a week that starts
		// smaller, because there is nothing left to remove.
		if spent > load.Ceiling {
			b.bonusCap = min(b.bonusCap, 1)
			b.volume = minFloat(b.volume, 0.85)
		}
	}
}

// focusCost names the unit the focus level added, where it added one, so a
// budget the athlete can see does not silently gain a unit they did not ask
// for.
func (b *builder) focusCost() string {
	if b.focus.Claim <= 0 {
		return ""
	}
	return fmt.Sprintf(" (%d of them the extra cost of building the week around %s rather than fitting it in)",
		b.focus.Claim, b.goal.phrase())
}

// feeds reports whether the goal being planned is built on this other skill,
// in which case it stays in the week whatever the budget says.
func (b *builder) feeds(key string) bool {
	for _, fed := range b.goal.Feeds {
		if fed == key {
			return true
		}
	}
	return false
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// ---------- a wrist that has hurt for months ----------

// Half of the people who train handstands regularly report chronic wrist pain,
// and in the one survey that asked, it was not associated with weekly training
// hours, warm-up routines, braces or grip devices. That finding is what this
// code is built on, and it points somewhere specific: for a wrist, the useful
// lever is the *position* the load arrives in, not the amount of it.
//
// So a mild wrist complaint no longer deletes the sport. It moves the work off
// the palm — rings, parallettes, fists, fingertips — and only removes what has
// nowhere else to go. A severe one still clears the region, because at that
// point the question is medical rather than programmatic.

// neutralWrist maps a movement loaded through an extended wrist onto the
// version of it that is not. These are real substitutions, not consolations:
// the ring planche is harder than the floor planche, it simply asks nothing of
// the wrist.
var neutralWrist = map[string]string{
	"planche_lean":           "ring_planche_lean",
	"tuck_planche":           "ring_tuck_planche",
	"straddle_planche":       "ring_straddle_planche",
	"full_planche":           "ring_planche",
	"pseudo_planche_push_up": "ring_planche_lean",
	"dip":                    "ring_dip",
	"straight_bar_dip":       "ring_dip",
	"push_up":                "knuckle_plank",
	"plank":                  "knuckle_plank",
	"l_sit":                  "ring_support_hold",
	"tuck_l_sit":             "ring_support_hold",
	"handstand":              "fingertip_hold",
	"wall_handstand":         "fingertip_hold",
	"crow_pose":              "knuckle_plank",
	"frog_stand":             "knuckle_plank",

	// The maltese has one neutral-wrist form and it is the banded rings
	// version, which the coaching material itself offers at the top of the
	// floor ladder. Everything else on that ladder — the wide planche, the
	// elevators, the presses — is floor-specific and simply comes out.
	// The SAT's neutral-wrist form is the victorian: the same position on
	// rings, where the ring turns with the forearm instead of the forearm
	// being pressed against a bar. It is not an easier skill — it is harder —
	// it simply asks nothing of the wrist.
	"box_victorian": "tuck_victorian",
	"tuck_sat":      "tuck_victorian",
	"adv_tuck_sat":  "tuck_victorian",
	"straddle_sat":  "victorian",
	"sat":           "victorian",
	"band_sat":      "tuck_victorian",

	"lean_maltese":     "band_maltese",
	"maltese":          "band_maltese",
	"planche_kicks":    "ring_planche_lean",
	"l_sit_to_planche": "ring_tuck_planche",
}

// spareTheWrist swaps what it can and bans what it cannot. It runs only for a
// complaint mild enough that training around it is the right call; anything
// worse goes through the ordinary regional filter.
func (b *builder) spareTheWrist(severity int) {
	swapped, removed := 0, 0
	for slug, exercise := range b.lib.Exercises {
		if !loadedRegions(exercise)[regionWrist] {
			continue
		}
		if alt, ok := neutralWrist[slug]; ok && b.lib.Has(alt) && !b.banned[alt] {
			b.substitute[slug] = alt
			swapped++
			continue
		}
		b.banned[slug] = true
		removed++
	}

	b.spared[regionWrist] = true
	b.rehab = appendUnique(b.rehab, "wrist_rehab_light")
	b.restrictions = append(b.restrictions, fmt.Sprintf(
		"Wrist (severity %d): the work moves off the palm rather than off the plan. %s with a neutral-wrist "+
			"version were swapped onto it — rings, parallettes, fists, fingertips — and the %s without one were "+
			"dropped. Chronic wrist pain is reported by more than half of everyone who trains handstands "+
			"regularly, and in the survey that asked it tracked neither weekly hours nor warm-ups; what it tracks "+
			"is how much load arrives through an extended wrist. So the lever here is the position, not the volume.",
		severity, capitalise(plural(swapped, "movement")), plural(removed, "movement")))
	b.notes = append(b.notes, "Pain that has lasted months, wakes you at night, or comes with numbness or "+
		"clicking on rotation is a wrist that needs imaging and an in-person assessment, not a programming "+
		"change. This plan trains around it; it does not treat it.")
}

// substituteFor resolves a movement through the injury substitutions, so every
// caller gets the version this athlete can actually train today.
func (b *builder) substituteFor(slug string) string {
	if alt, ok := b.substitute[slug]; ok {
		return alt
	}
	return slug
}
