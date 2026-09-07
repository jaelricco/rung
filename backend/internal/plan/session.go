package plan

import (
	"fmt"
	"math"
	"strings"
)

// One session, assembled. The order of the blocks is the order they are
// performed in, and it is the same order every time: joint preparation, then
// skill and straight-arm work while the athlete is fresh, then heavy strength,
// then accessories, then anything on a clock. That order is not a style
// choice — a static or a release move attempted on fatigue is both a worse
// rehearsal and the version that shows up in the injury tables.

type sessionBuilder struct {
	*builder
	week   weekSpec
	day    daySpec
	isTest bool
	used   map[string]bool
	blocks []Block
	// skillGone records that the goal's own work could not be placed, which
	// only happens when an injury took every movement on the ladder. The
	// session still runs; it just stops pretending to be about the skill.
	skillGone bool
}

func (b *builder) session(week weekSpec, day daySpec, isTest bool) Session {
	s := &sessionBuilder{builder: b, week: week, day: day, isTest: isTest, used: map[string]bool{}}

	s.prep()
	switch {
	case isTest:
		s.test()
	case day.Role == roleSkill:
		s.skill(false)
		s.strength()
	case day.Role == roleOpposite:
		s.strength()
	case day.Role == roleLightSkill:
		s.skill(true)
	}
	s.accessories()
	s.conditioning()

	title, focus := s.titleAndFocus()
	load := week.Load
	if !day.Hard && load != "deload" {
		load = "easy"
	}

	return Session{
		Week:            week.Week,
		DayOfWeek:       day.Day,
		Title:           title,
		Focus:           focus,
		Load:            load,
		DurationMinutes: estimateMinutes(s.blocks),
		WarmupProtocols: s.protocols(),
		Blocks:          s.blocks,
		Cooldown:        s.cooldown(),
	}
}

// ---------- writing a block ----------

// work is a block before it has been priced: what it is for, which movement,
// and what standard it is measured against. How it is actually prescribed —
// seconds or reps or kilos — is not stated here, because that is decided by
// how the movement is measured in the library rather than by the caller's
// idea of it. That is what stops an L-sit being prescribed as three reps.
type work struct {
	Intent     string
	Candidates chain
	Standard   float64
	Light      bool
	// Base is the set count before the week's own adjustments. Three is the
	// default and four is for the block the session exists for, which is what
	// keeps a week's hard sets per pattern inside the 12-to-20 band the
	// dose-response evidence supports rather than sailing past it.
	Base int
	// HoldStandard is the seconds a static version of this block is measured
	// against, for the callers whose Standard is a rep target. Without it a
	// hollow body hold prescribed as an accessory comes out as three seconds.
	HoldStandard float64
	MaxSets      int
	Rest         int
	Notes        string
	Progression  string
}

func (s *sessionBuilder) prescribe(w work) {
	slug := s.available(w.Candidates)
	if slug == "" {
		return
	}

	if w.Base == 0 {
		w.Base = 3
	}
	block := Block{Notes: w.Notes, Progression: w.Progression, RestSeconds: w.Rest, Tempo: tempoFor(slug)}
	var sets int

	switch s.measureOf(slug) {
	case "static_hold":
		standard := w.Standard
		if w.HoldStandard > 0 {
			standard = w.HoldStandard
		}
		seconds, best, basis := s.holdWork(slug, standard)
		sets = s.setCount(w.Base + 1) // statics are short, so an extra set is cheap
		if w.Intent == "skill" && s.goal.StraightArm && w.Base >= 4 {
			// Five sets is the workhorse across every level of the coaching
			// material, and it does not scale with the athlete: what scales is
			// the difficulty of the variant. Readiness still moves it, but from
			// five rather than from three.
			sets = clampInt(sets+1, 3, 6)
		}
		if w.Light {
			sets, seconds = max(2, sets-1), max(3, seconds*2/3)
		}
		lo, hi := band(seconds, holdBands)
		block.Prescription = fmt.Sprintf("%d × %d-%ds hold", sets, lo, hi)
		block.Intensity = fmt.Sprintf("about %d%% of your best hold (%s)%s", int(s.week.Fraction*100),
			secs(best), basis)
		block.Tempo = ""
		if block.Progression == "" {
			block.Progression = fmt.Sprintf("Next week: one more second on every set, at the same quality. "+
				"The hold is trained past %s, not extended by sagging into it.", secs(standard))
		}

	case "weighted_reps":
		reps, kg, basis := s.addedWork(slug)
		sets = s.setCount(w.Base)
		if w.Light {
			sets, kg = max(2, sets-1), roundLoad(kg*0.7)
		}
		lo, hi := band(reps, repBands)
		block.Prescription = fmt.Sprintf("%d × %d-%d reps with +%s kg", sets, lo, hi, kilos(kg))
		block.Intensity = s.week.Effort + " " + basis
		block.Tempo = "3s down, drive up"
		if block.Progression == "" {
			block.Progression = "Next week: +1.25 to 2.5 kg, but only once every set has hit the rep target " +
				"cleanly. A missed target means repeating the load, not pushing through it."
		}

	case "skill_attempt":
		sets = max(3, s.setCount(w.Base))
		block.Prescription = fmt.Sprintf("%d × 2 quality attempts", sets)
		block.Intensity = "full effort on each attempt, fully rested before the next"
		block.Tempo = ""
		if block.Progression == "" {
			block.Progression = "Next week: one more attempt per set, not a longer session."
		}

	default:
		reps, best, basis := s.repWork(slug, w.Standard)
		sets = s.setCount(w.Base)
		if w.Light {
			sets, reps = max(2, sets-1), max(2, reps*2/3)
		}
		lo, hi := band(reps, repBands)
		block.Prescription = fmt.Sprintf("%d × %d-%d reps", sets, lo, hi)
		block.Intensity = fmt.Sprintf("%s Your best set is %s%s.", s.week.Effort,
			plural(int(best), "rep"), basis)
		if block.Progression == "" {
			block.Progression = "Next week: one more rep per set. When every set reaches the top of its range, " +
				"add a set or add load rather than more reps."
		}
	}

	// A movement with no history behind it starts small however strong the
	// athlete is elsewhere. Almost every straight-arm injury follows a spike,
	// and a first week at an experienced athlete's usual volume on a movement
	// their tissue has never seen is exactly that spike.
	if w.Intent == "skill" && s.novel(slug) {
		sets = min(sets, 3)
		block.Notes = "Nothing logged on this movement yet, so the numbers here are the rung's own standard " +
			"rather than yours: treat the first session as finding out. Work up to one honest effort, log it, " +
			"and everything after it scales from a real number. The volume stays deliberately below what you " +
			"could probably do — new tissue load is where straight-arm injuries come from, not the set that " +
			"felt hard. " + block.Notes
	}

	if w.MaxSets > 0 {
		sets = min(sets, w.MaxSets)
	}
	block.Prescription = renumber(block.Prescription, sets)
	s.write(w.Intent, slug, sets, block)
}

// write puts the finished block in the session. Everything about a block that
// is not the movement itself arrives filled in, because "3 sets of pull-ups"
// and nothing else is not a prescription.
func (s *sessionBuilder) write(intent, slug string, sets int, block Block) {
	s.used[slug] = true
	block.ExerciseSlug = slug
	block.Intent = intent
	block.Sets = max(1, sets)
	s.blocks = append(s.blocks, block)
}

// literal writes a block whose prescription is a format rather than a dosage —
// a warm-up circuit, an EMOM, an AMRAP — where the numbers come from the
// format and not from the athlete's records.
func (s *sessionBuilder) literal(intent string, candidates chain, sets int, prescription string, block Block) {
	slug := s.available(candidates)
	if slug == "" {
		return
	}
	block.Prescription = prescription
	s.write(intent, slug, sets, block)
}

// available is pick with one extra rule: a movement already in this session
// is not a candidate for a second block of it. Without that, a chain whose
// first entry is the warm-up's own movement silently writes nothing at all.
func (s *sessionBuilder) available(candidates ...chain) string {
	for _, group := range candidates {
		for _, slug := range group {
			slug = s.substituteFor(slug)
			if s.lib.Has(slug) && !s.banned[slug] && !s.used[slug] {
				return slug
			}
		}
	}
	return ""
}

// maintenanceNote explains a maintenance block that had to change implement,
// because "keep your planche" and "start a planche on rings" are not the same
// instruction and the athlete deserves to know which one they got.
func (s *sessionBuilder) maintenanceNote(step Step) string {
	if len(step.Movement) == 0 {
		return ""
	}
	original := step.Movement[0]
	swapped := s.substituteFor(original)
	if swapped == original {
		return ""
	}
	return fmt.Sprintf("Your %s moves to %s while the wrist settles. On rings this is its own skill and it "+
		"starts over, which is the cost of keeping it in the week at all. ",
		strings.ToLower(s.exerciseName(original)), strings.ToLower(s.exerciseName(swapped)))
}

// novel reports that the athlete has never logged or declared this movement.
func (s *sessionBuilder) novel(slug string) bool {
	rec, ok := s.rec[slug]
	if !ok {
		return true
	}
	return rec.BestReps == nil && rec.BestHold == nil && rec.BestWeight == nil
}

func (s *sessionBuilder) measureOf(slug string) string {
	if e, ok := s.lib.Exercises[slug]; ok {
		return e.Measure
	}
	return "reps"
}

// ---------- the parts of a session ----------

// protocols picks the warm-up. RAMP in the app's own vocabulary: the general
// warm-up raises, and the region protocols activate and mobilise whatever this
// session is about to load. A rehab protocol for an open injury is never
// dropped, whatever else is on.
func (s *sessionBuilder) protocols() []string {
	out := []string{"general_warmup"}
	if s.goal.StraightArm && (s.day.Role == roleSkill || s.day.Role == roleLightSkill || s.isTest) {
		out = appendUnique(out, "straight_arm_warmup")
	}
	if s.goal.Wrists {
		out = appendUnique(out, "wrist_warmup")
	}
	if s.day.Role == roleOpposite || s.goal.Pattern == patternPush {
		out = appendUnique(out, "shoulder_warmup")
	}

	// An injured region's warm-up is replaced by its rehab protocol rather
	// than run alongside it: the warm-up is written to prepare a joint for
	// load, and this one is not going to be loaded.
	kept := []string{}
	for _, slug := range out {
		if p, ok := s.lib.Protocols[slug]; ok && p.Purpose == "warmup" &&
			(s.injured[p.Region] || s.spared[p.Region]) {
			continue
		}
		kept = append(kept, slug)
	}
	// The cap applies to warm-ups. A rehab protocol for an open injury is the
	// one thing in this list that is not optional, so it is appended after the
	// trim rather than competing with the warm-ups for a place in it.
	if len(kept) > 3 {
		kept = kept[:3]
	}
	for _, slug := range s.rehab {
		kept = appendUnique(kept, slug)
	}
	return s.lib.KeepProtocols(kept, nil)
}

// prep is the joint work that has to happen before the session, not the
// mobility that would be nice afterwards. Straight-arm work gets elbow
// preparation every time it appears; that rule has no exceptions, because
// inner-elbow tendinopathy is the injury this sport is worst at treating and
// best at causing.
func (s *sessionBuilder) prep() {
	skillDay := s.day.Role == roleSkill || s.day.Role == roleLightSkill || s.isTest

	if s.goal.StraightArm && skillDay {
		s.literal("prep", chain{"elbow_prep_circuit", "band_dislocate"}, 2, "12 slow reps per position", Block{
			Intensity:   "light — this is preparation, not training",
			RestSeconds: restPrep,
			Progression: "Unchanged for the whole plan. It is the price of straight-arm work, not a thing to progress.",
			Notes:       "Elbows locked, load taken slowly. If the inner elbow is already sore before you start, that is the session telling you to skip its straight-arm work.",
		})
	}
	if s.goal.Wrists {
		s.literal("prep", chain{"wrist_prep", "wrist_extensor_curl", "scapular_push_up"}, 1, "one full circuit, 10 reps each way", Block{
			Intensity:   "pain-free range only",
			RestSeconds: 30,
			Progression: "Unchanged. Add the extensor work on days the wrists feel stiff.",
			Notes:       "If the floor position hurts, move the whole session to parallettes or a wedge rather than pushing through it.",
		})
	}

	prepChain := chain{"scapular_pull_up", "band_face_pull", "active_hang"}
	if s.pushDay() {
		prepChain = chain{"scapular_push_up", "band_face_pull", "scapular_pull_up"}
	}
	s.literal("prep", prepChain, 2, "10 controlled reps", Block{
		Intensity:   "half effort",
		RestSeconds: 45,
		Progression: "Unchanged. It is a warm-up.",
		Notes:       "Move the shoulder blade, not the elbow.",
	})
}

// balanceChain is the antagonist to what this session trained hard, kept
// deliberately light: accessory movements, not a second main lift.
func (s *sessionBuilder) balanceChain() chain {
	if s.pushDay() {
		return chain{"australian_row", "band_face_pull", "scapular_pull_up"}
	}
	return chain{"push_up", "band_face_pull", "scapular_push_up"}
}

func (s *sessionBuilder) pushDay() bool {
	if s.day.Role == roleOpposite {
		return s.oppositePattern() == patternPush
	}
	return s.goal.Pattern == patternPush
}

// skill is the point of the plan, and it goes second — after joint prep and
// before anything that fatigues it. Holds are prescribed as a fraction of the
// athlete's own best, never to failure: a static held to collapse trains the
// collapse.
func (s *sessionBuilder) skill(light bool) {
	step := s.currentStep()
	if len(step.Movement) == 0 {
		return
	}

	// A goal with no separate skill to practise — plain strength — reads as
	// strength, because calling a set of pull-ups "skill work" would be a
	// label the session does not live up to.
	intent := "skill"
	if s.goal.Foundation {
		intent = "strength"
	}

	// The opener: two sets of the next rung up, before anything has tired.
	//
	// Every session in the coaching material starts this way — the hardest
	// variant of the day first, for two or three sets of a very short hold,
	// then the main work below it. It is not extra volume; it is where the
	// hardest thing gets attempted while the athlete can still hold a shape.
	// A skill attempted at the end of a session is a skill rehearsed badly.
	if !light && s.goal.StraightArm && s.rung+1 < len(s.ladder) {
		next := s.ladder[s.rung+1]
		if len(s.unmetGate(next)) == 0 {
			s.prescribe(work{
				Intent: "skill", Candidates: append(chain{}, next.Movement...),
				Standard: next.Standard * 0.5, HoldStandard: next.Standard * 0.5,
				Base: 2, MaxSets: 3, Rest: restOpener,
				Progression: "This is the rung above the one you are training. It moves when the rung below it does.",
				Notes: "Hardest thing first, while you are completely fresh, and deliberately few sets of it. " +
					"Stop the moment the shape is not there — this block is a look at the next position, not a fight with it.",
			})
		}
	}

	// The rung itself, and only the rung. Falling back to its assist drill
	// here would produce a session that looks like skill work and is not: if
	// the movement this rung is measured on cannot be trained, the honest
	// answer is to say so rather than to quietly train something else under
	// the skill's name.
	before := len(s.blocks)
	s.prescribe(work{
		Intent: intent, Candidates: append(chain{}, step.Movement...),
		Standard: step.Standard, Light: light, Base: 4, Rest: restForSkill(s.week, s.goal.StraightArm),
		Notes: "Stop the set the moment the shape breaks, not when the arms give out. " +
			"Several clean efforts beat one taken to collapse — a position held to failure rehearses the failure.",
	})
	if len(s.blocks) == before {
		// Nothing on this rung survived the injury filter. The session still
		// runs; it just stops being about the skill, and says so.
		s.skillGone = true
		s.noteSkillRemoved()
		return
	}

	// The drill that builds the rung, trained beside it rather than instead
	// of it. This is the block people skip, and skipping it is why a rung
	// takes six months instead of two.
	s.prescribe(work{
		Intent: intent, Candidates: append(append(chain{}, step.Assist...), s.goal.Drills...),
		Standard: 8, HoldStandard: 30, Light: light, MaxSets: 4, Rest: restSecondary,
		Notes: "The drill, not the test. It is what makes the position above it possible.",
	})

	// The skills this one is built on, kept alive at maintenance volume. They
	// are not the session's point, and they are not optional either.
	for _, step := range s.maintains() {
		if len(step.Movement) == 0 {
			continue
		}
		s.prescribe(work{
			Intent: "skill", Candidates: append(chain{}, step.Movement...),
			Standard: step.Standard, Light: true, MaxSets: 3, Rest: restSecondary,
			Progression: "Held, not pushed. This is the base the new skill is built on, so it keeps its place " +
				"in the week rather than growing in it.",
			Notes: s.maintenanceNote(step) + "Maintenance, not development: clean efforts at a comfortable " +
				"fraction of your best, enough that the position stays yours while the work above it happens.",
		})
	}

	if light {
		s.noteGreaseTheGroove()
	}
}

// test replaces the skill work in the final week with something that can be
// passed or failed. A plan that ends without a test has no way of telling the
// athlete whether it worked.
func (s *sessionBuilder) test() {
	step := s.currentStep()
	slug := s.pick(step.Movement, s.goal.Drills, s.pullChain())
	if slug == "" {
		return
	}

	prescription := "3 attempts, full rest between them"
	switch s.measureOf(slug) {
	case "static_hold":
		prescription = fmt.Sprintf("3 attempts at a maximum hold — the target is %s", secs(step.Standard))
	case "reps":
		prescription = fmt.Sprintf("3 attempts at a maximum set — the target is %s", plural(int(step.Standard), "rep"))
	case "weighted_reps":
		prescription = fmt.Sprintf("work up to a single — the target is +%s kg", kilos(step.Standard))
	}

	s.literal("skill", chain{slug}, 3, prescription, Block{
		Intensity:   "full effort, and fully rested — four minutes between attempts",
		RestSeconds: 240,
		Progression: "This is the test. What comes next depends on it rather than on the calendar.",
		Notes:       "Film it from the side. A hold you cannot see is a hold you cannot judge, and a plan built on a generous self-assessment is built on nothing.",
	})
}

// strength is the bent-arm work the skill is actually built on, plus the
// pattern that is not being skilled today.
func (s *sessionBuilder) strength() {
	primary := s.goal.Pattern
	if s.day.Role == roleOpposite {
		primary = s.oppositePattern()
	}
	s.strengthFor(primary)

	// One training day a week has to be everything, so it is.
	if s.req.DaysPerWeek == 1 {
		s.strengthFor(s.oppositePattern())
		s.strengthFor(patternLegs)
		return
	}
	if s.day.Role == roleOpposite {
		s.strengthFor(patternLegs)
	}
}

func (s *sessionBuilder) strengthFor(pattern string) {
	var candidates chain
	standard := 8.0
	switch pattern {
	case patternPull:
		candidates = s.pullChain()
	case patternPush:
		candidates = s.pushChain()
	case patternLegs:
		candidates, standard = s.legChain(), 12
	default:
		candidates, standard = s.coreChain(), 10
	}
	s.prescribe(work{
		Intent: "strength", Candidates: candidates, Standard: standard, HoldStandard: 30, MaxSets: 5, Rest: restSecondary,
		Notes: "Full range, no bounce. The set is over when the speed of the rep changes, not when the count runs out.",
	})
}

// accessories are what keep the plan sustainable: the antagonist to whatever
// is being trained hard, and the compression or core work the goal leans on.
// They are also the first thing to drop on a week that runs long, which is
// why they come last and why the deload cuts them first.
func (s *sessionBuilder) accessories() {
	if s.week.Phase == phaseDeload && s.day.Role != roleRecovery {
		return
	}

	// The first accessory balances whatever the session just trained hard.
	// Pushing more than you pull, or the reverse, is the most common way a
	// shoulder in this sport ends up hurt, and it is entirely avoidable.
	if s.day.Hard {
		s.prescribe(work{
			Intent: "accessory", Candidates: s.balanceChain(), Standard: 12, HoldStandard: 30,
			MaxSets: 3, Rest: restAccessory,
			Notes: "The counterweight to the hard work above it. This is the block that keeps the shoulder even.",
		})
	}

	// The second is the goal's own supporting work, rotated by the day so a
	// five-day week does not do the same accessory five times.
	s.prescribe(work{
		Intent:     "accessory",
		Candidates: rotate(append(append(chain{}, s.goal.Accessories...), "band_face_pull", "australian_row", "bulgarian_split"), s.day.Day-1),
		Standard:   12, HoldStandard: 30, MaxSets: 3, Rest: restAccessory,
		Notes: "Volume, not a fight. Drop this before you drop a warm-up, and never before a main block.",
	})

	if s.day.Role != roleRecovery {
		s.prescribe(work{
			Intent: "accessory", Candidates: rotate(s.coreChain(), s.day.Day-1), Standard: 10, HoldStandard: 45,
			MaxSets: 3, Rest: 90,
			Notes: "The moment the lower back lifts off the floor or the hips sag, the set is finished.",
		})
	}
}

// conditioning only ever lands on a day that is not hard, and never on a
// static, a release move or a loaded set. A clock plus a movement that needs
// full attention is the combination that fills the injury tables.
func (s *sessionBuilder) conditioning() {
	if s.day.Hard || s.isTest || s.week.Phase == phaseDeload {
		return
	}
	candidates := chain{"australian_row", "push_up", "bodyweight_squat", "hanging_knee_raise", "jump_squat"}

	if s.day.Role == roleRecovery {
		s.literal("conditioning", candidates, 1, "AMRAP 8 minutes: 8 reps a round, resting whenever you need to", Block{
			Intensity:   "conversational. If you could not hold a conversation, it is too fast for this day.",
			RestSeconds: 0,
			Progression: "Next week: one more round inside the same eight minutes.",
			Notes:       "AMRAP is as many rounds as possible in the window. It builds work capacity; it is not a test, and it is not where a skill belongs.",
		})
		return
	}
	s.literal("conditioning", candidates, 10, "EMOM 10 minutes: 6 reps at the top of each minute", Block{
		Intensity:   "about half of your best set — what is left of the minute is the rest",
		RestSeconds: 0,
		Progression: "Next week: 7 reps a minute, or 12 minutes at 6.",
		Notes: "EMOM means every minute on the minute: start the reps as the minute starts and rest with whatever " +
			"is left. If a minute runs out before the reps do, stop the piece there rather than chasing it.",
	})
}

// ---------- dosage ----------

// setCount scales a baseline by how much training the athlete has actually
// been doing and by where in the block this week sits. The weekly total it
// produces lands inside the 6-to-20 hard sets per pattern the dose-response
// literature supports, without ever taking a step their history does not
// carry.
func (s *sessionBuilder) setCount(base int) int {
	sets := int(math.Round(float64(base) * s.volume))
	sets += min(s.week.SetBonus, s.bonusCap)
	switch s.week.Phase {
	case phaseDeload:
		sets = int(math.Round(float64(sets) * 0.5))
	case phaseTest:
		sets = min(sets, 3)
	}
	return clampInt(sets, 2, 6)
}

// holdWork prescribes a static. Without a logged hold there is nothing to take
// a percentage of, so the rung's own standard stands in and the block says so
// — an honest estimate the athlete can correct by logging one set.
func (s *sessionBuilder) holdWork(slug string, standard float64) (seconds int, best float64, basis string) {
	best = s.rec.hold(slug)
	known := best > 0
	if !known {
		best = math.Max(standard*0.4, 5)
	}
	return clampInt(int(math.Round(best*s.week.Fraction)), 3, 60), best, sourceNote(s.rec.source(slug), known)
}

// repWork prescribes reps against the athlete's best logged set. With no set
// logged it works from the standard instead of from a fraction of a guess,
// which is the difference between a first session of six reps and one of two.
func (s *sessionBuilder) repWork(slug string, standard float64) (reps int, best float64, basis string) {
	fraction := 0.6
	switch s.week.Phase {
	case phaseIntensifation:
		fraction = 0.7
	case phaseDeload, phaseTest:
		fraction = 0.5
	}

	best = s.rec.reps(slug)
	if best <= 0 {
		best = math.Max(standard, 5)
		return clampInt(int(math.Round(best*0.7)), 4, 12), best, sourceNote("", false)
	}
	return clampInt(int(math.Round(best*fraction)), 3, 20), best, sourceNote(s.rec.source(slug), true)
}

// addedWork sizes the belt.
//
// Two things make this less obvious than it looks. A one-rep max that ignores
// bodyweight is wrong by the weight of the athlete, so the estimate runs on
// total load — bodyweight plus belt — and is converted back to something you
// can actually hang on a belt. And the records table keeps best reps and best
// load separately, so the pair a textbook calculation wants does not exist;
// what does exist is a strict-rep maximum, which Epley turns into a total max
// perfectly well. Where there is no bodyweight on file the percentages are
// dropped rather than guessed at, and the load moves from the athlete's own
// best logged set instead.
func (s *sessionBuilder) addedWork(slug string) (reps int, kg float64, basis string) {
	reps = 6
	switch s.week.Phase {
	case phaseIntensifation:
		reps = 4
	case phaseTest:
		reps = 2
	}

	base := "pull_up"
	switch {
	case strings.Contains(slug, "dip"):
		base = "dip"
	case strings.Contains(slug, "push_up"):
		base = "push_up"
	case strings.Contains(slug, "muscle_up"):
		base = "muscle_up"
	}

	logged := s.rec.added(slug)
	strict := s.rec.reps(base)

	if bw := s.snap.Bodyweight; bw != nil {
		oneRM := 0.0
		if strict >= 5 {
			oneRM = *bw * (1 + strict/30) // Epley on total load, bodyweight included
		}
		if logged > 0 {
			// A best logged load is worth at least a hard triple.
			oneRM = math.Max(oneRM, (*bw+logged)*(1+3.0/30))
		}
		if oneRM > 0 {
			kg = roundLoad(oneRM/(1+float64(reps)/30) - *bw)
			basis = fmt.Sprintf("A %d-rep load against an estimated max of %s kg total (bodyweight included), "+
				"from %s strict and %s on the belt.", reps, kilos(oneRM), plural(int(strict), "rep"), kilos(logged)+" kg")
		}
	}

	if kg <= 0 {
		switch {
		case logged > 0:
			factor := map[int]float64{6: 0.6, 4: 0.75, 2: 0.9}[reps]
			kg = roundLoad(logged * factor)
			basis = fmt.Sprintf("Worked back from your best logged %s kg on this movement. Add your bodyweight "+
				"in your profile and this becomes a real percentage instead.", kilos(logged))
		case s.snap.Bodyweight != nil:
			kg = roundLoad(*s.snap.Bodyweight * 0.1)
			basis = "A first load at roughly a tenth of bodyweight, since there is no weighted set logged to work from."
		default:
			kg = 5
			basis = "A deliberately conservative starting load: with no bodyweight on file and no weighted set " +
				"logged, there is nothing honest to calculate from. Add your bodyweight in your profile."
		}
	}

	if s.week.Phase == phaseDeload {
		kg = roundLoad(kg * 0.85)
	}
	return reps, math.Max(kg, 2.5), basis
}

// ---------- notes that belong to the plan rather than the block ----------

func (s *sessionBuilder) noteSkillRemoved() {
	s.builder.restrictions = appendUnique(s.builder.restrictions, fmt.Sprintf(
		"Every movement on the %s ladder loads an injured area, so the skill work itself is out until that "+
			"resolves. What is left keeps the rest of you training in the meantime, which is the point.",
		strings.ToLower(s.goal.Name)))
}

func (s *sessionBuilder) noteGreaseTheGroove() {
	if s.goal.Frequency == "" {
		return
	}
	s.builder.progressionExtras = appendUnique(s.builder.progressionExtras, fmt.Sprintf(
		"On the light day and on rest days: %s Keep those sets at half of what you could manage — frequent and "+
			"easy builds the position, while tired repetitions rehearse a bad one.", s.goal.Frequency))
}

// ---------- session prose ----------

func (s *sessionBuilder) titleAndFocus() (string, string) {
	step := s.currentStep()
	rung := step.Name
	if rung == "" {
		rung = s.goal.Name
	}

	switch {
	case s.skillGone:
		return capitalise(patternWord(s.goal.Pattern)) + " work · " + strings.ToLower(s.goal.Name) + " paused",
			"An open injury takes every movement on this ladder off the table, so the session keeps the rest of " +
				"you training and the skill work waits. That is not a lost month; it is the reason there is " +
				"still a plan at all."
	case s.isTest:
		return "Test: " + s.goal.Name,
			"The session the plan has been building toward. Everything else this week stays easy so that this one is taken fresh."
	case s.day.Role == roleSkill:
		return s.goal.Name + " · " + patternWord(s.goal.Pattern),
			fmt.Sprintf("%s while you are fresh, then the %s strength it is built on. %s",
				rung, patternWord(s.goal.Pattern), s.week.Effort)
	case s.day.Role == roleOpposite:
		return capitalise(patternWord(s.oppositePattern())) + " and legs",
			fmt.Sprintf("The pattern the skill days do not train, so nothing hard repeats inside 48 hours. %s", s.week.Effort)
	case s.day.Role == roleLightSkill:
		return rung + " · technique",
			"Short, easy, and about the position rather than the effort. Frequency is what builds a skill; fatigue is what stalls one."
	default:
		return "Mobility and easy work",
			"The day that makes the hard ones possible: joints through range, easy conditioning, nothing that needs recovering from."
	}
}

func (s *sessionBuilder) cooldown() string {
	switch {
	case s.goal.StraightArm:
		return "Two minutes of easy shoulder extension and light elbow flexion. Straight-arm work is the one thing this app will not let you skip the after-care on."
	case s.goal.Wrists:
		return "Wrist circles, then a minute of gentle extension each way. Nothing forced."
	case s.day.Role == roleRecovery:
		return "Five minutes on whatever feels tight. That is the whole point of the day."
	default:
		return "A couple of minutes of easy movement through the ranges the session used."
	}
}

// estimateMinutes prices the session from its own blocks rather than guessing.
// The warm-up is a flat ten minutes, which is what the protocols take.
func estimateMinutes(blocks []Block) int {
	seconds := 600
	for _, b := range blocks {
		if b.Intent == "conditioning" {
			seconds += 8 * 60
			continue
		}
		work := 30
		if strings.Contains(b.Prescription, "hold") {
			work = 20
		}
		seconds += b.Sets * (work + b.RestSeconds)
	}
	minutes := int(math.Round(float64(seconds)/60/5)) * 5
	return clampInt(minutes, 20, 120)
}

// Rest, as the coaching material actually prescribes it rather than as a
// generic strength template would. Maximal straight-arm work rests 4 to 7
// minutes between sets at every level — the beginner holding a tuck planche
// rests as long as the athlete pressing a maltese, because the rest is for the
// maximality of the position rather than for the athlete's experience. Nothing
// in this app rested longer than three minutes before, which was simply wrong
// for the work it was prescribing.
const (
	restOpener    = 420 // the hardest variant of the day, 2 to 3 sets of it
	restMaximal   = 300 // straight-arm skill work
	restSecondary = 210
	restAccessory = 120
	restPrep      = 45
)

func restForSkill(w weekSpec, straightArm bool) int {
	if !straightArm {
		// A first pull-up is not planche work, and five minutes between sets
		// of band-assisted pull-ups is a way to spend an evening.
		if w.Phase == phaseIntensifation {
			return 180
		}
		return 150
	}
	return restMaximal
}

// The rep and second bands the programme prescribes in. Every prescription in
// it is a range rather than a number, which is what lets an athlete meet it at
// the top on a good day and the bottom on a bad one without the plan being
// wrong either time. Computed targets are snapped onto the nearest band rather
// than printed as a false point value.
var holdBands = [][2]int{{2, 4}, {3, 6}, {3, 8}, {4, 8}, {5, 10}, {8, 15}, {10, 20}, {15, 30}, {20, 45}}
var repBands = [][2]int{{1, 3}, {2, 4}, {3, 5}, {3, 8}, {5, 10}, {8, 15}, {12, 20}}

// band picks the range whose lower bound sits closest to the computed target,
// so a target of 7 seconds is prescribed as "5-10s" rather than as "7s".
func band(target int, bands [][2]int) (int, int) {
	best, distance := bands[0], 1<<30
	for _, candidate := range bands {
		d := target - candidate[0]
		if d < 0 {
			d = -d
		}
		if d < distance {
			best, distance = candidate, d
		}
	}
	return best[0], best[1]
}

func tempoFor(slug string) string {
	switch {
	case strings.Contains(slug, "negative"):
		return "5s down, every rep"
	case strings.Contains(slug, "explosive") || strings.Contains(slug, "jump"):
		return "as fast as the shape allows"
	default:
		return "2s down, no bounce at the bottom"
	}
}

func patternWord(pattern string) string {
	switch pattern {
	case patternPush:
		return "push"
	case patternLegs:
		return "legs"
	case patternCore:
		return "core"
	default:
		return "pull"
	}
}

// rotate offsets a preference list so different days of the week reach for
// different movements out of the same pool, without any of it being random:
// the same request still produces the same plan, every time.
func rotate(list chain, by int) chain {
	if len(list) == 0 {
		return list
	}
	by = ((by % len(list)) + len(list)) % len(list)
	return append(append(chain{}, list[by:]...), list[:by]...)
}

// renumber rewrites the set count in a prescription that has already been
// formatted, for the callers that cap sets after the fact.
func renumber(prescription string, sets int) string {
	if i := strings.Index(prescription, " × "); i > 0 {
		return fmt.Sprintf("%d%s", sets, prescription[i:])
	}
	return prescription
}
