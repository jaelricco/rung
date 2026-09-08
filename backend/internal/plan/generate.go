package plan

import (
	"fmt"
	"math"
	"sort"
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
	b.applyEquipment()
	b.applyInjuries()
	// Entry first, because it can replace the ladder outright, and the budget
	// after it, because what the week costs depends on which ladder won.
	b.checkEntry()
	b.weighLoad()
	b.place()

	b.rungSlugs = map[string]bool{}
	for _, slug := range b.currentStep().Movement {
		b.rungSlugs[slug] = true
	}

	weeks := b.schedule()
	shape := weekShape(b.req.DaysPerWeek, b.focus.Exposures)
	for _, day := range shape {
		if day.Role == roleSkill || day.Role == roleLightSkill {
			b.skillDays++
		}
	}

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

	// The algorithm is written against a library that a migration could change
	// underneath it, so its own output goes through the same check the model's
	// does. It runs before the focus ceiling is measured, so the ceiling is
	// measured on the week that survived the check rather than on the one that
	// was written.
	warnings := Validate(&p, lib, b.req.Weeks)

	// The ceiling is enforced on the finished week rather than assumed from
	// the way it was built, and finish reports what it measured.
	b.share = b.capFocusShare(&p, weeks)
	b.finish(&p, weeks)

	// If the check left nothing at all, say so in the plan rather than handing
	// back a shape with no training in it.
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
	// Focus is how much of the week the goal is allowed to take: one of the
	// three levels in focus.go. Empty is the middle one, which is what the
	// planner did before the dial existed — so an older client that does not
	// send it gets exactly the plan it used to get.
	Focus string
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
	r.Focus = focusFor(strings.TrimSpace(r.Focus)).Key
}

// ---------- the builder ----------

type builder struct {
	req     Request
	lib     Library
	snap    training.Snapshot
	rec     records
	goal    Goal
	matched bool
	ladder  []Step
	rung    int
	banned  map[string]bool
	// substitute redirects a movement onto the version this athlete can train
	// today — the ring planche for the floor planche when the wrist is angry.
	// It is a swap rather than a ban, which is the difference between training
	// around an injury and stopping.
	substitute map[string]string
	injured    map[string]bool
	// spared is a region being trained around rather than cleared. It does not
	// ban anything — that is what the substitutions are for — but its warm-up
	// still gives way to its rehab protocol, because preparing a joint for
	// load it is not going to take is theatre.
	spared   map[string]bool
	owned    map[string]bool
	answered bool
	rehab    []string
	volume   float64
	// bonusCap limits how fast the weeks climb. Volume is a multiplier on set
	// counts of three to six, where a ten percent correction rounds away to
	// nothing — so a readiness problem that deserves less than a whole step
	// down slows the climb instead of shrinking week one.
	bonusCap int
	readines string

	// The elite end of the catalogue: whether the goal is open, what is
	// missing if not, and what the week costs the tendons.
	entryMet bool
	gaps     []Gap
	// heldBack is the rung the athlete's records reach but whose own gate they
	// have not cleared. It is a different answer from "not ready for this
	// skill": they are on the ladder, one rung below where they could be.
	heldBack string
	load     *Load

	// How much of the week the goal gets, and the movements that are on its
	// own line rather than on somebody else's. Both decide the shape of a
	// skill day: how many rungs it spans, and what is allowed to fill the
	// slots beside them.
	focus focusSpec
	share *Focus
	line  map[string]bool
	// hardest is the difficulty of the hardest movement the athlete has on
	// record, as the library rates it, overall and per category. It is what
	// places their accessory work, so that someone holding a front lever is
	// not handed an australian row to balance a session.
	hardest   int
	hardestIn map[string]int
	// rungSlugs is the movement of the rung being trained. The share cap may
	// take sets off it but never takes it out: a session that has lost the
	// thing it is named after is not a lighter session, it is a different one.
	rungSlugs map[string]bool
	fedLine   map[string]bool
	offLine   map[string]bool
	skillDays int

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
		banned: map[string]bool{}, substitute: map[string]string{}, injured: map[string]bool{},
		spared:       map[string]bool{},
		volume:       1,
		bonusCap:     2,
		restrictions: []string{},
	}
	// The catalogue is package state shared by every request, so the ladder
	// this plan may trim is a copy of it.
	b.owned, b.answered = ownedEquipment(snap.Equipment)
	b.hardest, b.hardestIn = b.gaugeLevel()
	b.focus = focusFor(req.Focus)
	b.line = b.lineOf(goal)
	b.fedLine = b.fedLineOf(goal)
	b.offLine = b.offLineOf(goal)
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
	declared := 0
	if b.snap.TrainsPerWeek != nil {
		declared = *b.snap.TrainsPerWeek
	}

	switch {
	case b.snap.SessionsLast28 >= 8:
		b.readines = fmt.Sprintf("%d sessions logged in the last four weeks — enough history to programme at full volume.",
			b.snap.SessionsLast28)
	case b.snap.SessionsLast28 > 0:
		b.volume = 0.85
		b.readines = fmt.Sprintf("%d sessions logged in the last four weeks, so the volume starts a step below full.",
			b.snap.SessionsLast28)

	// Nothing logged here yet. That is not the same as not training — most
	// people arrive mid-way through a training life — so what they told us
	// about their week stands in until a log exists to replace it.
	case declared >= 3:
		b.volume = 0.85
		b.readines = fmt.Sprintf("Nothing logged here yet, so this is built on the %d sessions a week you told us "+
			"you train. It starts one step below full volume until your log can confirm it.", declared)
	case declared >= 1:
		b.volume = 0.75
		b.readines = fmt.Sprintf("Nothing logged here yet, and %s a week to build on, so this starts light.",
			plural(declared, "session"))
	default:
		b.volume = 0.7
		b.readines = "Nothing logged and no training history given, so this starts deliberately light. " +
			"Fill in your baseline, or log a couple of sessions, and the numbers here get sharper."
	}

	// Short sleep raises injury risk by roughly a third, and the only part of
	// that this app controls is how much work it asks for. So the plan starts
	// where it would have started and climbs one step more slowly, which is a
	// correction the athlete can feel without it costing them week one.
	if b.snap.SleepHours != nil && *b.snap.SleepHours < 7 {
		b.bonusCap = 1
		b.notes = append(b.notes, fmt.Sprintf(
			"You put your sleep at %.1f hours a night. Athletes sleeping under about eight are injured a good "+
				"deal more often than those who are not, so this plan adds volume half as fast as it otherwise "+
				"would. Sleep is the cheapest thing on this page to fix, and the plan speeds up when it changes.",
			*b.snap.SleepHours))
	}

	// Whether the asked-for frequency is supported by anything.
	if got := b.snap.SessionsLast28; got > 0 && b.req.DaysPerWeek*4 > got*2 {
		b.notes = append(b.notes, fmt.Sprintf(
			"You asked for %d days a week but logged %d sessions in the last four weeks. The plan is built at "+
				"the frequency you asked for and at a volume your history supports; if the first fortnight feels "+
				"like too much, drop the last accessory block rather than a whole session.",
			b.req.DaysPerWeek, got))
	} else if b.snap.SessionsLast28 == 0 && declared > 0 && b.req.DaysPerWeek > declared+1 {
		b.notes = append(b.notes, fmt.Sprintf(
			"You asked for %d days a week and train %d. Going up by more than one day at a time is where most "+
				"plans come apart; if this one does, drop back to %d and keep everything else.",
			b.req.DaysPerWeek, declared, declared+1))
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
		// A mild wrist is the one case where clearing the region is the wrong
		// answer: it would delete the whole sport for a complaint that half of
		// all hand-balancers carry. That one moves the load off the palm
		// instead. Everything else, and anything severe, clears the region.
		if region == regionWrist && injury.Severity <= 2 {
			b.spareTheWrist(injury.Severity)
			b.volume = math.Min(b.volume, 0.9)
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

// applyEquipment takes off the table what the athlete has nothing to perform
// it on. Unlike an injury this is never a reason to stop training, so it
// removes movements and says so, and the plan is built from what is left —
// which, for someone with a floor and nothing else, is still a plan.
func (b *builder) applyEquipment() {
	if !b.answered {
		return
	}
	blocked := map[string][]string{}
	for slug := range b.lib.Exercises {
		if performable(slug, b.owned) {
			continue
		}
		b.banned[slug] = true
		for _, item := range missingFor(slug, b.owned) {
			blocked[item] = append(blocked[item], slug)
		}
	}
	if len(blocked) == 0 {
		return
	}

	// Ordered, so the same answer always produces the same sentence.
	missing := make([]string, 0, len(blocked))
	for item := range blocked {
		missing = append(missing, item)
	}
	sort.Strings(missing)

	parts := make([]string, 0, len(missing))
	for _, item := range missing {
		parts = append(parts, fmt.Sprintf("%s (%s)", item, plural(len(blocked[item]), "movement")))
	}
	b.restrictions = append(b.restrictions, fmt.Sprintf(
		"Built for the equipment you have. Left out for what you do not: %s. Tick the kit off on your baseline "+
			"page when you get it and the next plan reaches for it.", humanList(parts)))
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
	b.rung = b.placeOn(b.ladder)

	// A rung can carry its own prerequisites, and where it does they cap the
	// placement rather than the ladder. This is the difference the coaching
	// material insists on: leaning into a maltese with a band is beginner
	// accessory work, holding one is not, and a planner that gates the whole
	// skill gets the first half wrong.
	for i := 0; i <= b.rung && i < len(b.ladder); i++ {
		if unmet := b.unmetGate(b.ladder[i]); len(unmet) > 0 {
			if i == 0 {
				b.rung = 0
			} else {
				b.rung = i - 1
			}
			b.gaps = append(b.gaps, unmet...)
			b.heldBack = b.ladder[i].Name
			break
		}
	}

	b.stepDownToWhatIsTrainable()
}

// stepDownToWhatIsTrainable moves the athlete to the highest rung an injury or
// their equipment has not taken away.
//
// The planner used to place them and then discover, block by block, that the
// rung's movement was banned — at which point the session stopped being about
// the skill at all and said so. That is the right answer when the whole ladder
// is gone and the wrong one when it is not, and the hefesto is what made the
// difference obvious: its first two rungs are a german hang and a back lever,
// neither of which touches a wrist, while everything above them finishes with
// bodyweight on the palms behind the body. A sore wrist should cost that
// athlete the top of the ladder, not the skill.
func (b *builder) stepDownToWhatIsTrainable() {
	if b.trainable(b.currentStep()) {
		return
	}
	for i := b.rung - 1; i >= 0; i-- {
		if !b.trainable(b.ladder[i]) {
			continue
		}
		b.restrictions = append(b.restrictions, fmt.Sprintf(
			"%s is where your records put you, and nothing on it can be trained around what you have "+
				"open, so this plan works at %s instead. That is a lower rung, not a smaller goal: the "+
				"ladder is climbed from wherever you can stand on it.",
			b.ladder[b.rung].Name, strings.ToLower(b.ladder[i].Name)))
		b.rung = i
		return
	}
}

// trainable reports whether any movement this rung is measured on survives the
// injury and equipment filters, once substitutions have been applied.
func (b *builder) trainable(step Step) bool {
	for _, slug := range step.Movement {
		slug = b.substituteFor(slug)
		if b.lib.Has(slug) && !b.banned[slug] {
			return true
		}
	}
	return false
}

// unmetGate reports the rung's own prerequisites that this athlete has not
// demonstrated, with their figures beside the standards.
func (b *builder) unmetGate(step Step) []Gap {
	var out []Gap
	for _, req := range step.Gate {
		have := b.rec.best(req.Slug, req.Metric)
		if have >= req.Standard {
			continue
		}
		out = append(out, Gap{
			Name:     b.exerciseName(req.Slug),
			Standard: measure(req.Standard, req.Metric),
			Have:     b.haveText(req.Slug, req.Metric, have),
			Why:      req.Why,
		})
	}
	return out
}

// placeOn is the placement rule on its own, so the same reasoning can locate
// the athlete on a skill they are only maintaining rather than chasing.
func (b *builder) placeOn(ladder []Step) int {
	if len(ladder) == 0 {
		return 0
	}
	floor := 0
	for i, step := range ladder {
		if b.cleared(step) {
			floor = max(floor, i+1)
		}
		if b.logged(step) {
			floor = max(floor, i)
		}
	}
	return min(floor, len(ladder)-1)
}

// maintains returns the rungs of the skills this goal is built on, at the
// athlete's own level. These stay in the week at maintenance volume: a planche
// parked to chase a maltese takes the maltese down with it.
func (b *builder) maintains() []Step {
	out := make([]Step, 0, len(b.goal.Feeds))
	for _, key := range b.goal.Feeds {
		fed, ok := goalByKey[key]
		if !ok || len(fed.Ladder) == 0 {
			continue
		}
		out = append(out, fed.Ladder[b.placeOn(fed.Ladder)])
	}
	return out
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

// source says where this movement's best figure came from, so a block that
// quotes a number can say whether it was performed or claimed.
func (r records) source(slug string) string {
	if rec, ok := r[slug]; ok {
		return rec.Source
	}
	return ""
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
			slug = b.substituteFor(slug)
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

// The core chain used to be three tiers keyed on a hanging leg raise and an
// L-sit, which is how an athlete whose log is full of levers and empty of leg
// raises ended up prescribed planks. It is now the pool at their own level:
// the evidence is the hardest thing they have done, whatever movement that
// happened to be.
func (b *builder) coreChain() chain {
	return b.keepAtLevel(accessoryPools[patternCore], accessoryGap, 0)
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

// weekShape places the sessions across the week. Three rules decide every
// entry: 48 hours between hard sessions of the same pattern, no more than four
// hard sessions in a week however many days are trained, and no more skill
// exposures than the focus level allows. Above four days the extra days are
// technique and recovery, which is what the fifth and sixth day of a week are
// actually good for.
//
// The focus is what turns the same six days into three different weeks. It
// does not add days the 48-hour rule would not allow — a three-day week gets
// two skill days at every level above the lowest, because a third would land
// inside the recovery of the second — it only ever takes them away.
func weekShape(days int, exposures int) []daySpec {
	return holdExposures(baseShape(days), exposures)
}

// holdExposures demotes skill days beyond what the focus allows, from the end
// of the week backwards and the light technique days first. A demoted hard day
// still trains — it becomes the opposite pattern, which is what the week
// needed anyway — and a demoted light day becomes recovery.
func holdExposures(shape []daySpec, exposures int) []daySpec {
	if exposures < 1 {
		exposures = 1
	}
	count := func() int {
		n := 0
		for _, d := range shape {
			if d.Role == roleSkill || d.Role == roleLightSkill {
				n++
			}
		}
		return n
	}
	demote := func(role dayRole, to dayRole, hard bool) bool {
		for i := len(shape) - 1; i >= 0; i-- {
			if shape[i].Role == role {
				shape[i].Role, shape[i].Hard = to, hard
				return true
			}
		}
		return false
	}
	// The light day is a top-up; the hard day is the point of the plan. So the
	// top-ups go first, and only then does a real skill day become the
	// opposite pattern.
	for count() > exposures {
		if demote(roleLightSkill, roleRecovery, false) {
			continue
		}
		if !demote(roleSkill, roleOpposite, true) {
			break
		}
	}
	return space(shape)
}

// space is the 48-hour rule applied after the demotions, because a demotion
// can break it. Turning Thursday's skill day into a second push day when
// Friday is already one puts two hard sessions for the same pattern back to
// back, which is the one thing the week shape exists to prevent. The later of
// the pair stays in the week and stops being hard.
func space(shape []daySpec) []daySpec {
	for i := 1; i < len(shape); i++ {
		previous, day := shape[i-1], shape[i]
		if day.Day == previous.Day+1 && day.Hard && previous.Hard && day.Role == previous.Role {
			shape[i].Hard = false
		}
	}
	return shape
}

func baseShape(days int) []daySpec {
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
