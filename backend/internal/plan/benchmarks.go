package plan

import "fmt"

// The questions worth asking a new athlete.
//
// The planner branches on a small number of figures, and the useful baseline
// form is exactly those figures and nothing else. Asking for body fat, or how
// many years someone has trained, would collect data that changes no
// prescription — and every extra field costs answers on the ones that do.
//
// So the list is derived rather than written twice: eight universal tests that
// resolve every chain the planner picks from, plus the rungs of whichever
// ladder the athlete is actually climbing. A front lever plan asks about tuck
// holds; a pistol plan does not.

// Benchmark is one question, in the units the app measures that movement in.
type Benchmark struct {
	ExerciseSlug string `json:"exercise_slug"`
	Name         string `json:"name"`
	// Measure is the library's own, so the form asks for seconds where the
	// movement is held and reps where it is repeated.
	Measure string `json:"measure"`
	Prompt  string `json:"prompt"`
	Why     string `json:"why"`
	// Scope is "core" for the universal tests, or the goal key for a rung of
	// that goal's ladder.
	Scope string `json:"scope"`
}

// universal is the smallest set that resolves every branch the planner makes:
// which pull it prescribes, which push, whether a belt is on the table at all,
// which core progression, which leg progression, and where the handstand work
// starts. Each one is a test somebody can do today and write a number against.
var universal = []Benchmark{
	{ExerciseSlug: "pull_up", Prompt: "Most strict pull-ups in one set",
		Why: "Decides the whole pulling ladder, and whether added load is on the table yet — a belt only appears past ten."},
	{ExerciseSlug: "dip", Prompt: "Most strict dips in one set",
		Why: "The same decision for pushing. Past twelve, the plan starts loading them."},
	{ExerciseSlug: "push_up", Prompt: "Most push-ups in one set",
		Why: "What the pushing work is built from when dips are not there yet."},
	{ExerciseSlug: "dead_hang", Prompt: "Longest dead hang, in seconds",
		Why: "Grip is the quiet limit on every hanging skill, and this is the first rung of the pull-up ladder."},
	{ExerciseSlug: "hollow_body_hold", Prompt: "Longest hollow body hold, in seconds",
		Why: "The body line every lever is held with. A lever plan written over a weak hollow stalls."},
	{ExerciseSlug: "hanging_leg_raise", Prompt: "Most strict hanging leg raises",
		Why: "Decides which core progression the plan uses."},
	{ExerciseSlug: "bodyweight_squat", Prompt: "Most bodyweight squats in one set",
		Why: "Decides where the leg work starts."},
	{ExerciseSlug: "wall_handstand", Prompt: "Longest chest-to-wall handstand, in seconds",
		Why: "Where the handstand and overhead pressing work begins."},
}

// Benchmarks returns the questions to ask an athlete working toward this goal.
// Everything is filtered to what exists in the library, so a form never asks
// about a movement the app could not accept an answer for.
func Benchmarks(goalText string, lib Library) []Benchmark {
	out := make([]Benchmark, 0, len(universal)+8)
	seen := map[string]bool{}

	add := func(b Benchmark, scope string) {
		exercise, ok := lib.Exercises[b.ExerciseSlug]
		if !ok || seen[b.ExerciseSlug] {
			return
		}
		seen[b.ExerciseSlug] = true
		b.Name, b.Measure, b.Scope = exercise.Name, exercise.Measure, scope
		out = append(out, b)
	}

	for _, b := range universal {
		add(b, "core")
	}

	// The goal's own ladder. These are what actually place the athlete, so
	// they are worth asking even though most will be left blank: the first one
	// they cannot fill in is roughly where they are.
	goal, matched := MatchGoal(goalText)
	if !matched {
		return out
	}
	for _, step := range goal.Ladder {
		for _, slug := range step.Movement {
			exercise, ok := lib.Exercises[slug]
			if !ok {
				continue
			}
			add(Benchmark{
				ExerciseSlug: slug,
				Prompt:       promptFor(exercise.Name, exercise.Measure),
				Why: fmt.Sprintf("The %q rung. It is cleared at %s.",
					step.Name, measure(step.Standard, step.Metric)),
			}, goal.Key)
			break
		}
	}
	return out
}

func promptFor(name, measureKind string) string {
	switch measureKind {
	case "static_hold":
		return "Longest " + lowerFirst(name) + ", in seconds"
	case "weighted_reps":
		return "Heaviest " + lowerFirst(name) + ", in added kg"
	case "skill_attempt":
		return "Can you do a " + lowerFirst(name) + "?"
	default:
		return "Most " + lowerFirst(name) + "s in one set"
	}
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	// Only the first letter, so "L-sit" and "V-sit" keep their shape.
	if len(s) > 1 && s[1] >= 'A' && s[1] <= 'Z' {
		return s
	}
	return string(s[0]|0x20) + s[1:]
}
