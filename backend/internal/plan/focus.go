package plan

import (
	"fmt"
	"math"
	"sort"
)

// How much of the week one skill is allowed to take.
//
// The planner has always asked two questions — what are you training for, and
// how many days do you have — and then quietly answered a third one for the
// athlete: how much of those days the new skill gets. It answered "as much as
// the session order allows", which is the wrong answer, and it is wrong in
// both directions. Someone slotting a maltese into a week they are otherwise
// happy with does not want it eating the week. Someone who has cleared their
// calendar for it does not want two sets and an apology.
//
// So it is asked, in three steps, and the steps have a ceiling none of them
// crosses. Three lines of evidence set that ceiling, and they arrive from
// different directions:
//
// Tendon adaptation saturates, and it saturates quickly. Roughly ten minutes
// of loading gives a tendon its maximum anabolic signal; past that the signal
// does not grow but the wear does, and ten minutes of load followed by six to
// eight hours of rest, repeated, produces about double the collagen response
// of one long bout. A skill day that keeps adding sets is buying tissue cost
// at full price and adaptation at none.
//
// Frequency is what actually moves a static, and it has its own ceiling: two
// to three sessions a week per maximal skill, at least 48 hours apart. More
// often and the wrists and shoulders start reporting it; less often and the
// stimulus does not land.
//
// And the athletes who own these skills do not spend their week on them. The
// maltese programme this app's floor ladder comes from runs six training days
// around three skill sessions — one planche, one maltese, one mixed. For the
// person whose entire sport is the maltese, the maltese is one day in six.
//
// Hence: three levels, and a hard ceiling of forty percent of the week's
// working sets that not even the top one crosses.

// The three levels, as they cross the wire.
const (
	FocusLight    = "light"
	FocusStandard = "standard"
	FocusHigh     = "high"
)

// hardShareCeiling is the line no level crosses, whatever the athlete picks.
// A skill that takes more than two working sets in five is not a focus, it is
// a week with one exercise in it, and the tissue that has to absorb it does
// not get a vote.
const hardShareCeiling = 0.40

// focusSpec is one position of the dial: what it allows, and what it changes.
type focusSpec struct {
	Key  string
	Name string
	// Sentence is what the athlete reads beside the option.
	Sentence string
	// Share is the most of a week's working sets this level lets the goal's
	// own ladder take. Never above hardShareCeiling.
	Share float64
	// Exposures is the most sessions a week that carry the skill at all,
	// counting the light technique day.
	Exposures int
	// Span is how many rungs of the ladder one skill session covers. One is
	// the rung alone; two adds the rung above it as an opener; three adds the
	// rung below it as the volume block — which is the shape every session in
	// the coaching material actually has.
	Span int
	// Base is the set count for the rung itself before the week's own
	// adjustments.
	Base int
	// Claim is what the focus costs the tendon budget on top of the goal's
	// own cost. A week built around a maximal static spends more of the same
	// account than a week that merely contains one.
	Claim int
}

// The dial. Ordered lightest first, which is the order it is drawn in.
var focusLevels = []focusSpec{
	{
		Key: FocusLight, Name: "Keep it in the week",
		Sentence: "One session on the skill, and the rest of your training stays as it is. " +
			"The slowest way to the skill and the cheapest way to keep everything else.",
		Share: 0.20, Exposures: 1, Span: 1, Base: 3, Claim: 0,
	},
	{
		Key: FocusStandard, Name: "Train it properly",
		Sentence: "Two sessions on the skill, opened with the rung above it. The default, " +
			"and what the coaching sources prescribe for a skill somebody is actually chasing.",
		Share: 0.30, Exposures: 2, Span: 2, Base: 4, Claim: 0,
	},
	{
		Key: FocusHigh, Name: "Build the week around it",
		Sentence: "Up to three exposures, each covering a span of the ladder, and the other " +
			"skills give way. Still never more than 40% of the week: past that you are " +
			"buying tissue cost rather than progress.",
		Share: hardShareCeiling, Exposures: 3, Span: 3, Base: 5, Claim: 1,
	},
}

// FocusLevel is the dial as the browser draws it, so the option text and the
// numbers behind it have one source rather than two that drift.
type FocusLevel struct {
	Key      string  `json:"key"`
	Name     string  `json:"name"`
	Sentence string  `json:"sentence"`
	Share    float64 `json:"share"`
	Sessions int     `json:"sessions"`
}

// FocusLevels renders the dial for the picker.
func FocusLevels() []FocusLevel {
	out := make([]FocusLevel, 0, len(focusLevels))
	for _, f := range focusLevels {
		out = append(out, FocusLevel{
			Key: f.Key, Name: f.Name, Sentence: f.Sentence,
			Share: f.Share, Sessions: f.Exposures,
		})
	}
	return out
}

// focusFor resolves whatever arrived to a level. Anything unrecognised — an
// empty field from an older client, a typo — is the middle one, because the
// middle one is what the planner did before the dial existed.
func focusFor(key string) focusSpec {
	for _, f := range focusLevels {
		if f.Key == key {
			return f
		}
	}
	return focusLevels[1]
}

// ---------- the goal's own line ----------

// A skill day belongs to one line. That is the rule the elite programmes are
// built on and the one this planner was missing: the work beside a maltese
// hold is a maltese lean or a planche lean, not an L-sit and not a back lever.
// Another skill's rung on a maximal skill day is not accessory work — it is a
// second skill, and it belongs on its own day or in nobody's week.
//
// lineOf collects everything that counts as this goal's own line: its rungs,
// the drills that build them, the assists trained beside them, its own
// accessory list, and the ladders of the skills it is built on. The last of
// those matters: the planche is not a distraction on a maltese day, it is
// what the maltese is made of.
func (b *builder) lineOf(g Goal) map[string]bool {
	line := map[string]bool{}
	add := func(slugs chain) {
		for _, slug := range slugs {
			line[slug] = true
		}
	}
	var collect func(g Goal, depth int)
	collect = func(g Goal, depth int) {
		for _, step := range g.Ladder {
			add(step.Movement)
			add(step.Assist)
		}
		add(g.Drills)
		add(g.Accessories)
		if depth == 0 {
			for _, key := range g.Feeds {
				if fed, ok := goalByKey[key]; ok {
					collect(fed, depth+1)
				}
			}
		}
	}
	collect(g, 0)
	return line
}

// fedLineOf is the ladders of the skills this goal is built on, which are on
// the goal's line for the purpose of what may appear on its day but are not
// the goal's own work for the purpose of the ceiling. A planche kept alive on
// a maltese day is maintenance of a skill the athlete already owns; charging
// it to the maltese's share would price the maltese for work it is not doing.
func (b *builder) fedLineOf(g Goal) map[string]bool {
	out := map[string]bool{}
	own := map[string]bool{}
	for _, step := range g.Ladder {
		for _, slug := range step.Movement {
			own[slug] = true
		}
	}
	for _, key := range g.Feeds {
		fed, ok := goalByKey[key]
		if !ok {
			continue
		}
		for _, step := range fed.Ladder {
			for _, slug := range step.Movement {
				if !own[slug] {
					out[slug] = true
				}
			}
		}
	}
	return out
}

// ordinaryWork is every movement this planner reaches for as general strength
// or core work, gathered from the four chains that pick them. A movement in
// here is never "somebody else's skill", whichever ladder also happens to list
// it: a row is a row on any day of the week, and banning it from a maltese day
// because a first pull-up is climbed through one would be a rule mistaking a
// slug for a meaning. The list is written out by hand, so a test walks the
// chains across a spread of athletes and fails if one of them names something
// that is not in here.
var ordinaryWork = map[string]bool{
	// pullChain, in all its branches
	"weighted_pull_up": true, "pull_up": true, "wide_pull_up": true, "australian_row": true,
	"band_assisted_pull_up": true, "negative_pull_up": true, "dead_hang": true,
	// pushChain
	"weighted_dip": true, "dip": true, "ring_dip": true, "push_up": true,
	"straight_bar_dip": true, "pike_push_up": true,
	// legChain
	"pistol_squat": true, "shrimp_squat": true, "bulgarian_split": true,
	"assisted_pistol_squat": true, "jump_squat": true, "bodyweight_squat": true,
	"single_leg_calf_raise": true,
	// coreChain
	"toes_to_bar": true, "dragon_flag_negative": true, "hanging_leg_raise": true,
	"ab_wheel_rollout": true, "hollow_rock": true, "hollow_body_hold": true,
	"hanging_knee_raise": true, "plank": true, "arch_body_hold": true,
}

// offLine is the work that means "a second skill" on a day that already has
// one: the rungs, assists and drills of every *other* skill in the catalogue,
// with this goal's own line and the ordinary work taken back out.
//
// Two narrowings matter, and both were learned the hard way.
//
// Only straight-arm work counts. The rule exists because the tendon budget is
// one account and a second maximal static spends from it — an L-sit or a back
// lever beside a maltese hold. A hard *bent-arm* movement that happens to sit
// on somebody else's ladder is not that: a typewriter pull-up is the
// antagonist a maltese day wants, and refusing it because a one-arm pull-up is
// climbed through one would leave the counterweight to a heavy push day as a
// band face pull. So the filter is straight-arm goals and static holds, not
// every ladder in the catalogue.
//
// And it is the whole line, not just the rungs. An ice cream maker is a front
// lever drill rather than a front lever, and putting one on a planche day is
// exactly the mistake this rule is about.
func (b *builder) offLineOf(g Goal) map[string]bool {
	own := b.lineOf(g)
	out := map[string]bool{}
	mark := func(slugs chain, straightArm bool) {
		for _, slug := range slugs {
			if own[slug] || ordinaryWork[slug] {
				continue
			}
			if straightArm || isHold(b.lib.Exercises[slug].Measure) {
				out[slug] = true
			}
		}
	}
	for _, other := range Goals {
		if other.Key == g.Key || other.Foundation {
			continue
		}
		for _, step := range other.Ladder {
			mark(step.Movement, other.StraightArm)
			mark(step.Assist, other.StraightArm)
		}
		mark(other.Drills, other.StraightArm)
	}
	return out
}

// isHold reports a movement measured as a static, loaded or not. A weighted
// front lever is a front lever with a belt on: it spends from the same tissue
// and belongs to the same skill, so every rule written about holds has to see
// both measures or the belt becomes a way around the rule.
func isHold(measure string) bool {
	return measure == "static_hold" || measure == "weighted_hold"
}

// keepOnLine drops another skill's straight-arm work from a candidate list. It
// is used for the supporting slots and nowhere else: the strength block is
// allowed its dips whatever ladder they also appear on.
//
// It applies on every day of the week rather than only on the skill days,
// which is a change from where this rule started. The argument for the skill
// day was rehearsal — one line per session. The argument for the rest of the
// week is the tendon budget, and it is the stronger one: a back lever on the
// day *after* a maltese day is loading the same tissue during the recovery
// that day exists to give it. Another skill is another skill whenever it shows
// up.
func (s *sessionBuilder) keepOnLine(candidates chain) chain {
	if s.goal.Foundation {
		return candidates
	}
	kept := make(chain, 0, len(candidates))
	for _, slug := range candidates {
		if !s.offLine[slug] {
			kept = append(kept, slug)
		}
	}
	return kept
}

// skillDay reports a session built around the goal's own skill, as opposed to
// the days that carry the pattern it does not train.
func (s *sessionBuilder) skillDay() bool {
	if s.goal.Foundation {
		return false
	}
	return s.isTest || s.day.Role == roleSkill || s.day.Role == roleLightSkill
}

// ---------- holding the week to the ceiling ----------

// capFocusShare is the invariant, enforced rather than hoped for.
//
// Everything above shapes the week so that the skill lands near its share:
// how many days carry it, how many rungs each of those days covers, how many
// sets the rung gets. None of that is arithmetic, so it can miss — a short
// week, an injury that removed the strength work, a deload that cut the
// accessories first. This runs afterwards, measures what the week actually
// spends on the goal's own ladder, and takes sets off the largest skill block
// until it is inside the ceiling.
//
// What it will not do is empty a session. Sets come off first, down to a floor
// of two; then the session gives up a rung of its span, drill first; and the
// rung the session is named after is never removed. A week that still cannot
// be brought under says so in the plan instead of pretending.
func (b *builder) capFocusShare(p *Plan, weeks []weekSpec) *Focus {
	spec := b.focus
	ceiling := math.Min(spec.Share, hardShareCeiling)
	out := &Focus{
		Level: spec.Key, Name: spec.Name,
		Ceiling: ceiling, Sessions: b.skillDays,
	}
	// A foundation goal has no separate skill to ration: its ladder *is* the
	// strength work, and capping it at two sets in five would cap the plan.
	if b.goal.Foundation || len(p.Sessions) == 0 {
		out.Share = 0
		out.Note = fmt.Sprintf("%s is trained as strength rather than as a skill, so there is no separate "+
			"skill share to hold to a ceiling.", capitalise(b.goal.phrase()))
		return out
	}

	// The ceiling is a rule about a training week. A deload deliberately cuts
	// the accessories and keeps the skill, and the test week is one session
	// with everything around it made easy — so in both the skill's share of a
	// smaller week rises, and it is supposed to. Measuring the ceiling there
	// would trim the one block those weeks exist to protect.
	working := map[int]bool{}
	for _, w := range weeks {
		if w.Phase != phaseDeload && w.Phase != phaseTest {
			working[w.Week] = true
		}
	}

	byWeek := map[int][]int{} // week -> indexes into p.Sessions
	for i, session := range p.Sessions {
		if working[session.Week] {
			byWeek[session.Week] = append(byWeek[session.Week], i)
		}
	}

	// Ordered, so the same plan always produces the same sentence.
	numbers := make([]int, 0, len(byWeek))
	for week := range byWeek {
		numbers = append(numbers, week)
	}
	sort.Ints(numbers)

	stuck := false
	for _, week := range numbers {
		share, tight := b.holdWeekToShare(p, byWeek[week], ceiling)
		out.Share = math.Max(out.Share, share)
		stuck = stuck || tight
	}

	out.Note = fmt.Sprintf("%s takes %d%% of the heaviest training week's working sets, against the %d%% "+
		"this focus allows. %s", capitalise(b.goal.phrase()), int(math.Round(out.Share*100)),
		int(math.Round(ceiling*100)), focusReason(spec))
	if stuck {
		out.Note += " One week could not be brought under that without taking the rung this plan is " +
			"named after out of its own session, so it sits over — the honest fix is a training day " +
			"more, not a smaller skill block."
	}
	return out
}

// holdWeekToShare trims one week onto the ceiling and reports the share it
// ended at, and whether it ran out of sets to take before it got there.
func (b *builder) holdWeekToShare(p *Plan, indexes []int, ceiling float64) (float64, bool) {
	// A block reference: which session, which block.
	type ref struct{ session, block int }
	var skill []ref
	total, spent := 0, 0

	for _, i := range indexes {
		for j, block := range p.Sessions[i].Blocks {
			// The warm-up is preparation, not training, and counting it would
			// let a plan hide a heavy skill day behind a long warm-up.
			if block.Intent == "prep" {
				continue
			}
			total += block.Sets
			if block.Intent == "skill" && b.line[block.ExerciseSlug] && !b.fedLine[block.ExerciseSlug] {
				spent += block.Sets
				skill = append(skill, ref{i, j})
			}
		}
	}
	if total == 0 {
		return 0, false
	}

	// Order for the second pass: the blocks a session can give up entirely go
	// from the end of its span backwards.
	sort.SliceStable(skill, func(x, y int) bool {
		return p.Sessions[skill[x].session].Blocks[skill[x].block].Sets >
			p.Sessions[skill[y].session].Blocks[skill[y].block].Sets
	})

	touched := map[int]bool{}
	over := func() bool { return float64(spent) > ceiling*float64(total) }

	// First pass: take sets off, whichever block is currently largest, down to
	// a floor of two. Below two a block is not a smaller dose of the same
	// thing, it is a gesture, and a gesture in a plan is worse than an
	// absence. Re-finding the largest each time is what keeps the two skill
	// days of a week the same size as each other, rather than emptying Monday
	// to protect Thursday.
	for over() {
		var biggest *Block
		for _, r := range skill {
			block := &p.Sessions[r.session].Blocks[r.block]
			if block.Sets > 2 && (biggest == nil || block.Sets > biggest.Sets) {
				biggest = block
			}
		}
		if biggest == nil {
			break
		}
		biggest.Sets--
		biggest.Prescription = renumber(biggest.Prescription, biggest.Sets)
		spent, total = spent-1, total-1
		for _, r := range skill {
			if &p.Sessions[r.session].Blocks[r.block] == biggest {
				touched[r.session] = true
			}
		}
	}

	// Second pass: when every block is at the floor and the week is still
	// over, the session gives up a rung of its span rather than shaving sets
	// that cannot be shaved. They go from the end — the drill first, then the
	// rung below, then the opener — because that is the order they add value
	// in. The rung the session exists for is never one of them: a maltese day
	// without the maltese is not a lighter maltese day.
	if over() {
		for i := len(skill) - 1; i >= 0 && over(); i-- {
			r := skill[i]
			block := p.Sessions[r.session].Blocks[r.block]
			if b.rungSlugs[block.ExerciseSlug] {
				continue
			}
			p.Sessions[r.session].Blocks[r.block].Sets = 0 // marked for removal
			touched[r.session] = true
			spent, total = spent-block.Sets, total-block.Sets
		}
		for i := range touched {
			kept := p.Sessions[i].Blocks[:0]
			for _, block := range p.Sessions[i].Blocks {
				if block.Sets > 0 {
					kept = append(kept, block)
				}
			}
			p.Sessions[i].Blocks = kept
		}
	}

	// A session whose sets moved is a session whose length moved with them.
	for i := range touched {
		p.Sessions[i].DurationMinutes = estimateMinutes(p.Sessions[i].Blocks)
	}
	if total == 0 {
		return 0, true
	}
	return float64(spent) / float64(total), over()
}

// focusReason is the one line that says why the ceiling is where it is, in
// the terms the athlete can act on rather than as a rule handed down.
func focusReason(spec focusSpec) string {
	switch spec.Key {
	case FocusLight:
		return "You asked to keep it in the week rather than build the week around it, so it gets " +
			"one session and everything else you train keeps its place."
	case FocusHigh:
		return "This is as much as one skill gets here. Tendon adaptation saturates after about ten " +
			"minutes of loading — past that the growth signal stops rising and only the wear does — " +
			"so the way past this ceiling is another session on another day, never a longer one."
	default:
		return "Two sessions a week, 48 hours apart, is what the coaching sources prescribe for a " +
			"maximal skill: often enough for the stimulus to land, rare enough for the tissue to answer it."
	}
}
