package ai

import (
	"strings"
	"testing"

	"calisthenics/api/internal/training"
)

func testLibrary() library {
	return library{
		exercises: map[string]training.Exercise{
			"pull_up":          {Slug: "pull_up", Name: "Pull-up"},
			"tuck_front_lever": {Slug: "tuck_front_lever", Name: "Tuck front lever"},
		},
		protocols: map[string]bool{"wrist_warmup": true},
	}
}

func TestValidatePlanDropsWhatTheLibraryDoesNotHave(t *testing.T) {
	plan := Plan{Sessions: []PlanSession{
		{
			Week: 1, DayOfWeek: 2, WarmupProtocols: []string{"wrist_warmup", "invented_protocol"},
			Blocks: []PlanBlock{
				{ExerciseSlug: "pull_up", Sets: 3},
				{ExerciseSlug: "planche_wizardry", Sets: 3},
			},
		},
		// Nothing in this one survives, so the session goes with it.
		{Week: 1, DayOfWeek: 4, Blocks: []PlanBlock{{ExerciseSlug: "made_up", Sets: 3}}},
		// Past the end of the plan.
		{Week: 9, DayOfWeek: 1, Blocks: []PlanBlock{{ExerciseSlug: "pull_up", Sets: 3}}},
	}}

	warnings := validatePlan(&plan, testLibrary(), 4)

	if len(plan.Sessions) != 1 {
		t.Fatalf("kept %d sessions, want 1", len(plan.Sessions))
	}
	kept := plan.Sessions[0]
	if len(kept.Blocks) != 1 || kept.Blocks[0].ExerciseSlug != "pull_up" {
		t.Fatalf("kept blocks %+v, want only pull_up", kept.Blocks)
	}
	if len(kept.WarmupProtocols) != 1 || kept.WarmupProtocols[0] != "wrist_warmup" {
		t.Fatalf("kept protocols %v, want only wrist_warmup", kept.WarmupProtocols)
	}
	if len(warnings) != 5 {
		t.Fatalf("reported %d warnings, want 5: %v", len(warnings), warnings)
	}
	for _, w := range warnings {
		if !strings.Contains(w, "drop") && !strings.Contains(w, "removed") {
			t.Fatalf("warning %q should say what happened to the prescription", w)
		}
	}
}

func TestValidatePlanRepairsAndOrders(t *testing.T) {
	plan := Plan{Sessions: []PlanSession{
		{Week: 2, DayOfWeek: 1, Blocks: []PlanBlock{{ExerciseSlug: "pull_up", Sets: 3}}},
		{Week: 1, DayOfWeek: 5, Blocks: []PlanBlock{{ExerciseSlug: "pull_up", Sets: 3}}},
		{Week: 1, DayOfWeek: 9, Blocks: []PlanBlock{
			{ExerciseSlug: " tuck_front_lever ", Sets: 0, RestSeconds: 4000},
		}},
	}}

	validatePlan(&plan, testLibrary(), 8)

	if len(plan.Sessions) != 3 {
		t.Fatalf("kept %d sessions, want 3", len(plan.Sessions))
	}
	got := []int{}
	for _, s := range plan.Sessions {
		got = append(got, s.Week*10+s.DayOfWeek)
	}
	// Week 1 day 5, week 1 day 9 clamped to day 1, week 2 day 1.
	if got[0] != 11 || got[1] != 15 || got[2] != 21 {
		t.Fatalf("sessions came out in the order %v, want calendar order", got)
	}
	repaired := plan.Sessions[0].Blocks[0]
	if repaired.ExerciseSlug != "tuck_front_lever" {
		t.Fatalf("slug %q was not trimmed", repaired.ExerciseSlug)
	}
	if repaired.Sets != 1 {
		t.Fatalf("a block with no sets became %d, want 1", repaired.Sets)
	}
	if repaired.RestSeconds != 0 {
		t.Fatalf("an hour of rest survived as %d seconds", repaired.RestSeconds)
	}
}

func TestPlanPromptCarriesTheResearchAndTheDeload(t *testing.T) {
	found := SkillResearch{
		Summary:     "Built with straight-arm work.",
		Progression: []ResearchStage{{Stage: "tuck", ExerciseSlugs: []string{"tuck_front_lever"}, Standard: "20s"}},
	}
	prompt := planPrompt("CONTEXT", found, skillPlanRequest{Skill: "front lever", Weeks: 8, DaysPerWeek: 3}, 24)

	for _, want := range []string{"CONTEXT", "tuck_front_lever", "hold to: 20s", "deload", "24 sessions"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("the prompt never mentions %q", want)
		}
	}

	short := planPrompt("CONTEXT", SkillResearch{}, skillPlanRequest{Skill: "muscle-up", Weeks: 4, DaysPerWeek: 3}, 12)
	if strings.Contains(short, "deload\".") {
		t.Fatal("a four-week plan should not be told to place a deload")
	}
}
