package plan

import (
	"fmt"
	"math"
	"strings"
)

// Everything the athlete reads before the first session: where they were
// placed and on what evidence, how the weeks are shaped, how load moves, what
// was taken out for an injury, and what counts as passing. A plan that cannot
// explain itself is a plan you have to take on faith, which is exactly what
// this app is trying not to be.

func (b *builder) finish(p *Plan, weeks []weekSpec) {
	step := b.currentStep()

	p.Title = b.title()
	p.Restrictions = b.restrictions
	p.Phases = b.phases(weeks)
	p.ProgressionRules = b.rules(weeks)
	p.Test = b.test(step)
	p.Notes = b.notes
	p.Summary = b.summary(weeks, step)
	p.Method = &Method{
		Source:      SourceAlgorithm,
		Goal:        b.goal.Name,
		GoalMatched: b.matched,
		Rung:        step.Name,
		NextRung:    b.nextRungName(),
		Ladder:      b.ladderView(),
		Readiness:   b.readines,
	}
}

func (b *builder) title() string {
	step := b.currentStep()
	if step.Name == "" {
		return fmt.Sprintf("%s — %d weeks", b.goal.Name, b.req.Weeks)
	}
	return fmt.Sprintf("%s — %d weeks · %s", b.goal.Name, b.req.Weeks, step.Name)
}

// summary is the paragraph that has to survive being read once. It says where
// the athlete is, why the planner thinks so, what the week does, and what the
// plan will not pretend to deliver.
func (b *builder) summary(weeks []weekSpec, step Step) string {
	var parts []string

	if len(b.ladder) > 0 {
		parts = append(parts, fmt.Sprintf(
			"This is rung %d of %d on the way to %s: %s. %s",
			b.rung+1, len(b.ladder), b.goal.phrase(), step.Name, b.evidence(step)))
	} else {
		parts = append(parts, "This is a balanced strength plan across pull, push, legs and core, "+
			"built from the movements you have logged.")
	}
	if !b.matched && strings.TrimSpace(b.req.Goal) != "" {
		parts = append(parts, fmt.Sprintf(
			"The planner does not have a ladder for %q, so it built balanced strength instead — "+
				"name a skill it knows and you get that skill's progression.", b.req.Goal))
	}

	parts = append(parts, fmt.Sprintf(
		"The week is %s: skill work goes first while you are fresh, the pattern it loads is trained hard no more "+
			"than twice, and the days between carry the opposite pattern so nothing hard repeats inside 48 hours.",
		b.weekSentence()))

	if n := countPhase(weeks, phaseDeload); n > 0 {
		parts = append(parts, fmt.Sprintf(
			"%s at roughly half the sets with the movements and the quality unchanged, because fatigue is "+
				"managed across months, not within a week.", capitalise(plural(n, "lighter week"))))
	}
	if last := weeks[len(weeks)-1]; last.Phase == phaseTest {
		parts = append(parts, fmt.Sprintf("Week %d tests it, and the test either passes or it does not.", last.Week))
	}

	if len(b.restrictions) > 0 {
		parts = append(parts, "Movements that load an open injury have been removed — see the restrictions below.")
	}
	if b.readines != "" {
		parts = append(parts, b.readines)
	}
	if b.goal.Timeline != "" {
		parts = append(parts, "Honest expectation: "+b.goal.Timeline+
			" A plan of this length buys you a rung and the habits to keep climbing, not the whole ladder.")
	}
	return strings.Join(parts, " ")
}

// evidence says what in the log put the athlete on this rung. Being explicit
// about it is the point: a placement the athlete can check is one they can
// correct by logging a set, rather than one they have to argue with.
func (b *builder) evidence(step Step) string {
	if len(step.Movement) == 0 {
		return ""
	}
	slug := step.Movement[0]
	best := b.rec.best(slug, step.Metric)
	name := b.exerciseName(slug)

	if best <= 0 {
		if b.rung == 0 {
			return fmt.Sprintf("You have not logged %s, so the plan starts at the bottom of the ladder — "+
				"which is the right direction to be wrong in.", article(name))
		}
		return fmt.Sprintf("You cleared the rung below it and there is no logged %s yet, so this is where the work is.",
			strings.ToLower(name))
	}
	return fmt.Sprintf("Your best logged %s is %s against the %s this rung is cleared at.",
		strings.ToLower(name), measure(best, step.Metric), measure(step.Standard, step.Metric))
}

func (b *builder) weekSentence() string {
	shape := weekShape(b.req.DaysPerWeek)
	hard := 0
	for _, d := range shape {
		if d.Hard {
			hard++
		}
	}
	if hard == len(shape) {
		return fmt.Sprintf("%s, all of them working sessions", plural(len(shape), "session"))
	}
	return fmt.Sprintf("%s — %d hard and %d light",
		plural(len(shape), "session"), hard, len(shape)-hard)
}

// phases name the blocks of weeks so the shape of the plan is legible without
// reading all forty sessions.
func (b *builder) phases(weeks []weekSpec) []Phase {
	if len(weeks) == 0 {
		return []Phase{}
	}
	out := []Phase{}
	start, current := weeks[0].Week, weeks[0].Phase
	for i := 1; i <= len(weeks); i++ {
		if i < len(weeks) && weeks[i].Phase == current {
			continue
		}
		// A deload inside a block does not end the block; it is part of it.
		if i < len(weeks) && current != phaseDeload && weeks[i].Phase == phaseDeload {
			continue
		}
		if current == phaseDeload && i < len(weeks) {
			current = weeks[i].Phase
			continue
		}
		out = append(out, Phase{
			Weeks: span(start, weeks[i-1].Week),
			Name:  phaseName(current),
			Aim:   phaseAim(current, b.goal),
		})
		if i < len(weeks) {
			start, current = weeks[i].Week, weeks[i].Phase
		}
	}
	return out
}

func phaseName(phase string) string {
	switch phase {
	case phaseIntensifation:
		return "Intensification"
	case phaseTest:
		return "Test"
	case phaseDeload:
		return "Lighter week"
	default:
		return "Accumulation"
	}
}

func phaseAim(phase string, goal Goal) string {
	switch phase {
	case phaseIntensifation:
		return "Fewer, harder sets: closer to failure, longer rests, holds nearer your best. The block where the rung actually opens."
	case phaseTest:
		return "One session that answers the question, with everything around it kept easy so the answer means something."
	case phaseDeload:
		return "Half the sets, the same movements, the same quality. Recovering on purpose rather than by accident."
	default:
		return "Clean sets, well short of failure, adding one step a week. " +
			"This is where the joints get used to the work " + goal.phrase() + " asks of them."
	}
}

// rules are what the athlete applies on the days the plan does not cover. They
// are stated as instructions rather than principles, because "progress when it
// feels easy" is not a progression.
func (b *builder) rules(weeks []weekSpec) []string {
	rules := []string{
		"Statics are prescribed at 55 to 65 percent of your best hold, never to failure. Add one second per set " +
			"each week. Move up a rung only when the rung's standard holds clean for three sets — a sagging hold is " +
			"a different exercise, not a shorter one.",
		"Rep work is prescribed with reps in reserve, not to failure. Add one rep per set each week; when every " +
			"set reaches the top of its range, add a set or add load instead of adding reps.",
		"Weighted work moves in 1.25 to 2.5 kg steps, and only after every set has hit its rep target cleanly. " +
			"A missed target means repeating the load, not pushing through it.",
		"Skill and straight-arm work always comes first in the session, before anything that fatigues it. " +
			"If you are too tired to do it well, that is the session's answer: train the easy blocks and go home.",
		"Hard sessions for one movement pattern stay 48 hours apart. If you move a day, move it away from its " +
			"neighbour, not toward it.",
		"Two missed targets in a row on the same block means repeat the week rather than progressing it. " +
			"The plan is a hypothesis; your log is the evidence.",
	}
	if countPhase(weeks, phaseDeload) > 0 {
		rules = append(rules, "On a lighter week the movements and the quality do not change — only the number of "+
			"sets. If you feel good on the light week, that is the light week working.")
	}
	if len(b.restrictions) > 0 {
		rules = append(rules, "Anything that hurts is off the plan, not on it. Pain that persists past two weeks, "+
			"wakes you at night, or comes with numbness needs an in-person assessment — this plan is not one and "+
			"cannot become one.")
	}
	rules = append(rules, b.progressionExtras...)
	return rules
}

// test states the goal of the plan in terms someone can pass or fail on the
// day, which is the only version of a goal that is worth anything.
func (b *builder) test(step Step) string {
	if b.req.Weeks < 3 || len(step.Movement) == 0 {
		return "Too short a plan to test. Re-log your best set of the main movement at the end and compare it to today's."
	}
	name := b.exerciseName(step.Movement[0])
	switch step.Metric {
	case metricHold:
		return fmt.Sprintf(
			"Pass: %s held for %s with the shape intact, filmed from the side, on the first or second attempt. "+
				"Fail: anything shorter, or a hold that sags to get there. Log it either way — the next plan is "+
				"built from it.", name, secs(step.Standard))
	case metricReps:
		return fmt.Sprintf(
			"Pass: %s for %s, strict, in one set, no kipping and no partial reps. Fail: anything less, or reps "+
				"that need a swing. Log it either way — the next plan is built from it.",
			name, plural(int(step.Standard), "rep"))
	case metricAdded:
		return fmt.Sprintf(
			"Pass: %s with +%s kg for one strict rep from a dead hang. Fail: anything less, or a rep that needs "+
				"a kick. Log it either way — the next plan is built from it.", name, kilos(step.Standard))
	default:
		return fmt.Sprintf("Pass: one clean %s, made and filmed. Fail: anything that needs an assist. "+
			"Log it either way — the next plan is built from it.", strings.ToLower(name))
	}
}

func (b *builder) nextRungName() string {
	if b.rung+1 < len(b.ladder) {
		return b.ladder[b.rung+1].Name
	}
	return ""
}

// ladderView is the whole ladder with the athlete's position marked, returned
// with the plan so the placement can be checked rather than trusted.
func (b *builder) ladderView() []Rung {
	out := make([]Rung, 0, len(b.ladder))
	for i, step := range b.ladder {
		out = append(out, Rung{
			Name:          step.Name,
			ExerciseSlugs: b.lib.Keep(step.Movement),
			Standard:      measure(step.Standard, step.Metric) + typicalSuffix(step.Typical),
			Cleared:       i < b.rung,
			Current:       i == b.rung,
		})
	}
	return out
}

func typicalSuffix(typical string) string {
	if typical == "" {
		return ""
	}
	return " · " + typical
}

func (b *builder) exerciseName(slug string) string {
	if e, ok := b.lib.Exercises[slug]; ok {
		return e.Name
	}
	return strings.ReplaceAll(slug, "_", " ")
}

// ---------- small shared helpers ----------

func measure(value float64, metric string) string {
	switch metric {
	case metricHold:
		return secs(value)
	case metricAdded:
		return "+" + kilos(value) + " kg"
	case metricAttempt:
		return "one clean rep"
	default:
		return plural(int(value), "rep")
	}
}

func secs(value float64) string {
	return fmt.Sprintf("%ds", int(math.Round(value)))
}

func kilos(value float64) string {
	if value == math.Trunc(value) {
		return fmt.Sprintf("%.0f", value)
	}
	return strings.TrimSuffix(fmt.Sprintf("%.2f", value), "0")
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func capitalise(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func span(from, to int) string {
	if from == to {
		return fmt.Sprintf("%d", from)
	}
	return fmt.Sprintf("%d-%d", from, to)
}

func countPhase(weeks []weekSpec, phase string) int {
	n := 0
	for _, w := range weeks {
		if w.Phase == phase {
			n++
		}
	}
	return n
}

func estimatedNote(estimated bool) string {
	if !estimated {
		return ""
	}
	return " (estimated — log a set of this and the number becomes yours)"
}

// article puts the right indefinite article in front of a movement name, so
// the plan does not read as "a inverted hang".
func article(name string) string {
	name = strings.ToLower(name)
	if name == "" {
		return name
	}
	if strings.ContainsRune("aeiou", rune(name[0])) {
		return "an " + name
	}
	return "a " + name
}

func humanList(items []string) string {
	switch len(items) {
	case 0:
		return "general warm-up"
	case 1:
		return items[0]
	default:
		return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
	}
}

func appendUnique(list []string, value string) []string {
	for _, existing := range list {
		if existing == value {
			return list
		}
	}
	return append(list, value)
}

// roundLoad brings a computed load onto something you can actually load: the
// nearest 1.25 kg, which is the smallest plate most people own a pair of.
func roundLoad(kg float64) float64 {
	if kg < 2.5 {
		return 2.5
	}
	return math.Round(kg/1.25) * 1.25
}

func clampInt(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
