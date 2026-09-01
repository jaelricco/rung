package plan

import (
	"fmt"
	"math"
	"strings"

	"calisthenics/api/internal/training"
)

// Generate writes a plan from the athlete's own records and nothing else.
//
// It is the app's default, not its fallback. A model writes a better plan on a
// good day, but it needs an account, a network, a provider that is up and a
// budget that has not run out — and a training plan is not a thing that should
// stop existing because one of those is missing. So the algorithm runs first,
// always, and the model's job is to improve on something that already works.
//
// The contract that makes that worth relying on: Generate never fails. There
// is no error return, no partial answer and no input that makes it panic — not
// an empty snapshot, not a goal typed in a language it has never seen, not a
// library a migration emptied out. Whatever it cannot do it says in the plan's
// own words, and hands back a week the athlete can train tomorrow.
func Generate(req Request, snap training.Snapshot, lib Library) (Plan, []string) {
	b := newBuilder(req, snap, lib)
	b.applyInjuries()
	b.place()

	weeks := b.schedule()
	shape := weekShape(b.req.DaysPerWeek)

	p := Plan{
		Weeks:        b.req.Weeks,
		Restrictions: b.restrictions,
		Sessions:     make([]Session, 0, len(weeks)*len(shape)),
	}
	for _, week := range weeks {
		// The test goes on the week's first skill day, and everything around
		// it stays light, so the answer is about the training rather than
		// about how tired the athlete was on the day.
		tested := false
		for _, day := range shape {
			isTest := week.Phase == phaseTest && !tested && day.Role == roleSkill
			tested = tested || isTest

			session := b.session(week, day, isTest)
			// A session that lost every block to the injury filter is a rest
			// day, not an empty appointment.
			if len(session.Blocks) > 0 {
				p.Sessions = append(p.Sessions, session)
			}
		}
	}

	b.finish(&p, weeks)

	// The algorithm is written against a library that a migration could change
	// underneath it, so its own output goes through the same check the model's
	// does. If that leaves nothing at all, say so in the plan rather than
	// handing back a shape with no training in it.
	warnings := Validate(&p, lib, b.req.Weeks)
	if len(p.Sessions) == 0 {
		p.Summary = "This plan could not be built: the exercise library the app prescribes from came back " +
			"empty, so there was nothing legal to put in a session. Nothing is wrong with your training — " +
			"this is a fault on our side. Try again shortly."
		warnings = append(warnings, "No session survived the exercise library check, so the plan is empty.")
	}
	return p, warnings
}

// Request is what the athlete asked for. Everything else the planner needs it
// works out for itself.
type Request struct {
	Goal        string
	Weeks       int
	DaysPerWeek int
	Notes       string
}

// clamp brings a request into the range the rest of the planner assumes.
// Out-of-range values are corrected rather than rejected: a plan asked for
// with 400 weeks is a slip, not a reason to answer with nothing.
func (r *Request) clamp() {
	if r.Weeks < 1 || r.Weeks > 24 {
		r.Weeks = 8
	}
	if r.DaysPerWeek < 1 || r.DaysPerWeek > 7 {
		r.DaysPerWeek = 3
	}
	r.Goal = strings.TrimSpace(r.Goal)
}

// ---------- the builder ----------

type builder struct {
	req      Request
	lib      Library
	snap     training.Snapshot
	rec      records
	goal     Goal
	matched  bool
	ladder   []Step
	rung     int
	banned   map[string]bool
	injured  map[string]bool
	rehab    []string
	volume   float64
	readines string

	restrictions []string
	notes        []string
	// progressionExtras collects rules the session assembly discovers as it
	// goes — the grease-the-groove note a light day earns, for instance — so
	// they reach the plan's own rules rather than being buried in one block.
	progressionExtras []string
}

func newBuilder(req Request, snap training.Snapshot, lib Library) *builder {
	req.clamp()
	goal, matched := MatchGoal(req.Goal)

	b := &builder{
		req: req, lib: lib, snap: snap,
		rec: recordsOf(snap), goal: goal, matched: matched,
		banned: map[string]bool{}, injured: map[string]bool{},
		volume:       1,
		restrictions: []string{},
	}
	// The catalogue is package state shared by every request, so the ladder
	// this plan may trim is a copy of it.
	b.ladder = append([]Step(nil), goal.Ladder...)
	b.aimAtNamedTarget()
	b.gaugeReadiness()
	if !matched && strings.TrimSpace(req.Goal) != "" {
		b.notes = append(b.notes, fmt.Sprintf(
			"%q isn't a skill the planner has a ladder for, so this is a balanced strength plan. "+
				"Naming a known skill — front lever, planche, muscle-up, handstand, pistol squat, "+
				"weighted pull-up — gets you its progression instead.", req.Goal))
	}
	return b
}

// aimAtNamedTarget honours a number in the goal text — "20 kg weighted
// pull-up", "10s front lever" — by trimming the ladder to end where the
// athlete asked it to end.
func (b *builder) aimAtNamedTarget() {
	if len(b.ladder) == 0 {
		return
	}
	value, metric, ok := namedTarget(b.req.Goal)
	if !ok || metric != b.ladder[len(b.ladder)-1].Metric {
		return
	}
	for i, step := range b.ladder {
		if step.Metric == metric && step.Standard >= value {
			b.ladder = b.ladder[:i+1]
			b.ladder[i].Standard = value
			return
		}
	}
	b.ladder[len(b.ladder)-1].Standard = value
}

// gaugeReadiness sets how much volume this athlete is actually ready for. The
// number that decides it is what they have logged, not what they have asked
// for: someone with four sessions in the last month does not get a twenty-set
// week because they typed 5 into the days field.
func (b *builder) gaugeReadiness() {
	switch {
	case b.snap.SessionsLast28 == 0:
		b.volume = 0.7
		b.readines = "Nothing logged in the last four weeks, so this starts deliberately light. " +
			"Log your sessions and the numbers here get sharper."
	case b.snap.SessionsLast28 < 8:
		b.volume = 0.85
		b.readines = fmt.Sprintf("%d sessions logged in the last four weeks, so the volume starts a step below full.",
			b.snap.SessionsLast28)
	default:
		b.readines = fmt.Sprintf("%d sessions logged in the last four weeks — enough history to programme at full volume.",
			b.snap.SessionsLast28)
	}
	if want, got := b.req.DaysPerWeek, b.snap.SessionsLast28; got > 0 && want*4 > got*2 {
		b.notes = append(b.notes, fmt.Sprintf(
			"You asked for %d days a week but logged %d sessions in the last four weeks. The plan is built at "+
				"the frequency you asked for and at a volume your history supports; if the first fortnight feels "+
				"like too much, drop the last accessory block rather than a whole session.",
			want, got))
	}
}

// applyInjuries turns open injuries into a hard filter on the movement list.
// This is the one place the planner is deliberately blunt: an injured region
// takes every movement that loads it off the table for the length of the plan,
// whatever the severity, and the plan says what it removed. Training around an
// injury too carefully costs a fortnight. Not doing it costs a season.
func (b *builder) applyInjuries() {
	if len(b.snap.OpenInjuries) == 0 {
		return
	}
	for _, injury := range b.snap.OpenInjuries {
		region := strings.ToLower(strings.TrimSpace(injury.Region))
		if region == "" || region == "other" {
			b.restrictions = append(b.restrictions,
				"An open injury is recorded without a body region, so nothing could be removed for it "+
					"automatically. Skip anything in here that loads it.")
			continue
		}
		b.injured[region] = true
		if slug, ok := rehabFor[region]; ok {
			b.rehab = appendUnique(b.rehab, slug)
		} else if slug, ok := warmupFor[region]; ok {
			b.rehab = appendUnique(b.rehab, slug)
		}
		b.restrictions = append(b.restrictions, fmt.Sprintf(
			"Open %s injury (severity %d): every movement that loads the %s is out of this plan, and %s is in "+
				"every warm-up in its place. Persistent or worsening pain needs an in-person assessment — this "+
				"plan is not one and cannot become one.",
			region, injury.Severity, region, humanList(b.protocolTitles())))

		if injury.Severity >= 4 {
			b.volume = math.Min(b.volume, 0.55)
		} else if injury.Severity >= 3 {
			b.volume = math.Min(b.volume, 0.7)
		} else {
			b.volume = math.Min(b.volume, 0.85)
		}
	}

	for slug, exercise := range b.lib.Exercises {
		for region := range loadedRegions(exercise) {
			if b.injured[region] {
				b.banned[slug] = true
				break
			}
		}
	}
}

// protocolTitles names the rehab protocols the way the athlete sees them on
// the protocol page, rather than by their slug.
func (b *builder) protocolTitles() []string {
	out := make([]string, 0, len(b.rehab))
	for _, slug := range b.rehab {
		if p, ok := b.lib.Protocols[slug]; ok && p.Title != "" {
			out = append(out, "the "+strings.ToLower(p.Title)+" protocol")
			continue
		}
		out = append(out, slug)
	}
	return out
}

// place puts the athlete on the ladder, from their log and nothing else.
//
// Two facts decide it, and both are needed. The highest rung they have
// *cleared* sets the floor: you do not go back down a ladder you have climbed.
// The highest rung they have *logged at all* also sets a floor, because
// someone with a nine-second advanced tuck is training the advanced tuck even
// though they have never logged the inverted hang three rungs below it — and a
// planner that only looked for the lowest unlogged rung would send them back
// to hang upside down for eight weeks.
//
// Where neither says anything, the answer is the bottom of the ladder. An
// unlogged athlete is a beginner, which is the safe direction to be wrong in.
func (b *builder) place() {
	if len(b.ladder) == 0 {
		return
	}
	floor := 0
	for i, step := range b.ladder {
		if b.cleared(step) {
			floor = max(floor, i+1)
		}
		if b.logged(step) {
			floor = max(floor, i)
		}
	}
	b.rung = min(floor, len(b.ladder)-1)
}

// logged reports whether the athlete has ever recorded a set of this rung's
// own movement. It is weaker evidence than clearing it, and it is used only to
// stop the planner sending someone backwards.
func (b *builder) logged(step Step) bool {
	for _, slug := range step.Movement {
		if rec, ok := b.rec[slug]; ok && rec.TotalSets > 0 {
			return true
		}
	}
	return false
}

func (b *builder) cleared(step Step) bool {
	// The goal itself is never "cleared": you do not outgrow the thing you
	// came for, you maintain it.
	if step.Metric == metricAttempt {
		return false
	}
	best := 0.0
	for _, slug := range step.Movement {
		best = math.Max(best, b.rec.best(slug, step.Metric))
	}
	return best >= step.Standard
}

// currentStep is the rung being trained, or a zero Step for a goal with no
// ladder.
func (b *builder) currentStep() Step {
	if b.rung < len(b.ladder) {
		return b.ladder[b.rung]
	}
	return Step{}
}

// ---------- records ----------

type records map[string]training.Record

func recordsOf(snap training.Snapshot) records {
	out := make(records, len(snap.Records))
	for _, r := range snap.Records {
		out[r.Slug] = r
	}
	return out
}

func (r records) best(slug, metric string) float64 {
	rec, ok := r[slug]
	if !ok {
		return 0
	}
	switch metric {
	case metricHold:
		if rec.BestHold != nil {
			return *rec.BestHold
		}
	case metricReps:
		if rec.BestReps != nil {
			return float64(*rec.BestReps)
		}
	case metricAdded:
		if rec.BestWeight != nil {
			return *rec.BestWeight
		}
	}
	return 0
}

func (r records) reps(slug string) float64  { return r.best(slug, metricReps) }
func (r records) hold(slug string) float64  { return r.best(slug, metricHold) }
func (r records) added(slug string) float64 { return r.best(slug, metricAdded) }

// ---------- picking a movement ----------

// pick resolves a chain of candidates to the first slug that exists in the
// library and survives the injury filter. It returns "" when nothing in the
// chain is available, and every caller is written to cope with that: a block
// that cannot be filled is a block that is not written, not a plan that fails.
func (b *builder) pick(candidates ...chain) string {
	for _, group := range candidates {
		for _, slug := range group {
			if b.lib.Has(slug) && !b.banned[slug] {
				return slug
			}
		}
	}
	return ""
}

// The strength chains are conditioned on what the athlete has demonstrated, so
// nothing is prescribed more than one clear step above their logged level. A
// belt only appears once the strict movement is owned; a pull-up only appears
// once one exists.
func (b *builder) pullChain() chain {
	switch {
	case b.rec.reps("pull_up") >= 10:
		return chain{"weighted_pull_up", "pull_up", "wide_pull_up", "australian_row"}
	case b.rec.reps("pull_up") >= 3:
		return chain{"pull_up", "band_assisted_pull_up", "australian_row"}
	case b.rec.reps("pull_up") >= 1 || b.rec.reps("negative_pull_up") >= 3 || b.rec.reps("australian_row") >= 10:
		return chain{"negative_pull_up", "band_assisted_pull_up", "australian_row"}
	default:
		return chain{"australian_row", "band_assisted_pull_up", "negative_pull_up", "dead_hang"}
	}
}

func (b *builder) pushChain() chain {
	switch {
	case b.rec.reps("dip") >= 12:
		return chain{"weighted_dip", "dip", "ring_dip", "push_up"}
	case b.rec.reps("dip") >= 3:
		return chain{"dip", "straight_bar_dip", "push_up"}
	case b.rec.reps("push_up") >= 15:
		return chain{"straight_bar_dip", "dip", "pike_push_up", "push_up"}
	default:
		return chain{"push_up", "australian_row", "straight_bar_dip"}
	}
}

func (b *builder) legChain() chain {
	switch {
	case b.rec.reps("pistol_squat") >= 3:
		return chain{"pistol_squat", "shrimp_squat", "bulgarian_split"}
	case b.rec.reps("bodyweight_squat") >= 25 || b.rec.reps("bulgarian_split") >= 8:
		return chain{"bulgarian_split", "assisted_pistol_squat", "jump_squat", "bodyweight_squat"}
	default:
		return chain{"bodyweight_squat", "bulgarian_split", "single_leg_calf_raise"}
	}
}

func (b *builder) coreChain() chain {
	switch {
	case b.rec.reps("hanging_leg_raise") >= 10 || b.rec.hold("l_sit") >= 20:
		return chain{"toes_to_bar", "dragon_flag_negative", "hanging_leg_raise", "ab_wheel_rollout"}
	case b.rec.reps("hanging_knee_raise") >= 8 || b.rec.hold("hollow_body_hold") >= 30:
		return chain{"hanging_leg_raise", "hollow_rock", "ab_wheel_rollout", "hollow_body_hold"}
	default:
		return chain{"hollow_body_hold", "hanging_knee_raise", "plank", "arch_body_hold"}
	}
}

// ---------- the week and the block of weeks ----------

// Phases of a training block.
const (
	phaseAccumulation  = "accumulation"
	phaseIntensifation = "intensification"
	phaseDeload        = "deload"
	phaseTest          = "test"
)

type weekSpec struct {
	Week     int
	Phase    string
	Load     string // hard, moderate, easy, deload — what the calendar colours
	SetBonus int
	Fraction float64 // of the athlete's best hold
	Effort   string  // how close to failure, in reps in reserve
}

// schedule lays the weeks out. Deloads land every fourth week, which is the
// middle of the 4-to-6-week range surveyed strength athletes actually use, and
// one lands the week before a test so the test is taken fresh. A plan shorter
// than six weeks gets none: there is not enough fatigue yet to be worth a week.
func (b *builder) schedule() []weekSpec {
	weeks := b.req.Weeks
	test := weeks >= 3

	deload := map[int]bool{}
	if weeks >= 6 {
		for w := 4; w <= weeks; w += 4 {
			deload[w] = true
		}
		delete(deload, weeks) // never deload the week you test in
		if weeks >= 8 && test {
			deload[weeks-1] = true // taper into the test
		}
	}

	// The last week that is not the test week is where intensification ends.
	working := weeks
	if test {
		working = weeks - 1
	}
	switchAt := (working + 1) / 2

	out := make([]weekSpec, 0, weeks)
	sinceReset := 0
	for w := 1; w <= weeks; w++ {
		spec := weekSpec{Week: w}
		switch {
		case test && w == weeks:
			spec.Phase, spec.Load, spec.Fraction = phaseTest, "moderate", 0.9
			spec.Effort = "Take the test at full effort. Everything around it stays easy."
		case deload[w]:
			spec.Phase, spec.Load, spec.Fraction = phaseDeload, "deload", 0.5
			spec.Effort = "3 to 4 reps in reserve. Half the sets, same movements, same quality."
			sinceReset = 0
		case w <= switchAt:
			spec.Phase, spec.Load, spec.Fraction = phaseAccumulation, "hard", 0.55
			spec.Effort = "2 to 3 reps in reserve. This block is about sets that stay clean."
			spec.SetBonus = min(sinceReset, 2)
			sinceReset++
		default:
			spec.Phase, spec.Load, spec.Fraction = phaseIntensifation, "hard", 0.65
			spec.Effort = "1 to 2 reps in reserve. Fewer, harder, longer rests."
			spec.SetBonus = min(sinceReset, 2)
			sinceReset++
		}
		out = append(out, spec)
	}
	return out
}

// A day's job in the week.
type dayRole int

const (
	roleSkill dayRole = iota
	roleOpposite
	roleLightSkill
	roleRecovery
)

type daySpec struct {
	Day  int // 1 = Monday
	Role dayRole
	Hard bool
}

// weekShape places the sessions across the week. Two rules decide every entry:
// 48 hours between hard sessions of the same pattern, and no more than four
// hard sessions in a week however many days are trained. Above four days the
// extra days are technique and recovery, which is what the fifth and sixth
// day of a week are actually good for.
func weekShape(days int) []daySpec {
	switch days {
	case 1:
		return []daySpec{{1, roleSkill, true}}
	case 2:
		return []daySpec{{1, roleSkill, true}, {4, roleOpposite, true}}
	case 3:
		return []daySpec{{1, roleSkill, true}, {3, roleOpposite, true}, {5, roleSkill, true}}
	case 4:
		return []daySpec{{1, roleSkill, true}, {2, roleOpposite, true}, {4, roleSkill, true}, {5, roleOpposite, true}}
	case 5:
		return []daySpec{{1, roleSkill, true}, {2, roleOpposite, true}, {3, roleLightSkill, false},
			{5, roleSkill, true}, {6, roleOpposite, true}}
	case 6:
		return []daySpec{{1, roleSkill, true}, {2, roleOpposite, true}, {3, roleLightSkill, false},
			{4, roleSkill, true}, {5, roleOpposite, true}, {6, roleRecovery, false}}
	default:
		return []daySpec{{1, roleSkill, true}, {2, roleOpposite, true}, {3, roleLightSkill, false},
			{4, roleSkill, true}, {5, roleOpposite, true}, {6, roleRecovery, false}, {7, roleRecovery, false}}
	}
}

// oppositePattern is what gets trained on the days the goal does not.
func (b *builder) oppositePattern() string {
	switch b.goal.Pattern {
	case patternPush:
		return patternPull
	case patternLegs, patternCore:
		return patternPull
	default:
		return patternPush
	}
}
