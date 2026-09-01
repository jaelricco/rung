package ai

import (
	"encoding/json"
	"strings"
	"testing"

	"calisthenics/api/internal/plan"
	"calisthenics/api/internal/training"
)

// Plan validation itself now lives in internal/plan, which owns the schema and
// tests it against the real seeded library. What is left here is the AI half:
// what the model is told, and what happens when it cannot answer.

// testLibrary is the small stand-in the research tests prune against.
func testLibrary() plan.Library {
	return plan.Library{
		Exercises: map[string]training.Exercise{
			"pull_up":          {Slug: "pull_up", Name: "Pull-up"},
			"tuck_front_lever": {Slug: "tuck_front_lever", Name: "Tuck front lever"},
		},
		Protocols: map[string]training.Protocol{"wrist_warmup": {Slug: "wrist_warmup"}},
	}
}

func baselinePlan() Plan {
	return Plan{
		Title:            "Front lever — 8 weeks · Advanced tuck",
		Summary:          "This is rung 3 of 5.",
		Weeks:            8,
		Test:             "Pass: advanced tuck front lever held for 15s.",
		Phases:           []PlanPhase{{Weeks: "1-4", Name: "Accumulation", Aim: "Clean sets."}},
		ProgressionRules: []string{"Add one second per set each week."},
		Restrictions:     []string{"Open wrist injury: floor work removed."},
		Method: &plan.Method{
			Source: plan.SourceAlgorithm, Goal: "Front lever", Rung: "Advanced tuck", NextRung: "Straddle",
			Ladder: []plan.Rung{
				{Name: "Tuck", Standard: "20s", Cleared: true},
				{Name: "Advanced tuck", Standard: "15s", Current: true},
			},
		},
		Sessions: []PlanSession{{
			Week: 1, DayOfWeek: 1, Title: "Front lever · pull", Load: "hard", DurationMinutes: 60,
			WarmupProtocols: []string{"general_warmup", "straight_arm_warmup"},
			Blocks: []PlanBlock{{
				ExerciseSlug: "adv_tuck_front_lever", Intent: "skill", Sets: 4,
				Prescription: "4 × 5s hold", Intensity: "about 55% of your best hold",
				RestSeconds: 150, Progression: "One more second per set.",
			}},
		}},
	}
}

func TestThePromptHandsTheModelThePlanItIsImproving(t *testing.T) {
	found := SkillResearch{
		Summary:     "Built with straight-arm work.",
		Progression: []ResearchStage{{Stage: "tuck", ExerciseSlugs: []string{"tuck_front_lever"}, Standard: "20s"}},
	}
	prompt := planPrompt("CONTEXT", found, baselinePlan(),
		skillPlanRequest{Skill: "front lever", Weeks: 8, DaysPerWeek: 3}, 24)

	for _, want := range []string{
		"CONTEXT",              // the athlete's snapshot and the libraries
		"tuck_front_lever",     // the research findings
		"hold to: 20s",         //
		"BASELINE PLAN",        // and the plan it is being asked to improve
		"Advanced tuck",        // including where the athlete was placed
		"adv_tuck_front_lever", // and the week the planner actually wrote
		"4 × 5s hold",
		"Open wrist injury", // the restrictions it must keep
		"Improve on it",     // and what its job is
		"deload", "24 sessions",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt never mentions %q", want)
		}
	}

	short := planPrompt("CONTEXT", SkillResearch{}, Plan{},
		skillPlanRequest{Skill: "muscle-up", Weeks: 4, DaysPerWeek: 3}, 12)
	if strings.Contains(short, "deload\".") {
		t.Error("a four-week plan should not be told to place a deload")
	}
	if strings.Contains(short, "BASELINE PLAN") {
		t.Error("with no baseline to show, the prompt should not claim to have one")
	}
}

func TestFallbackSaysWhyTheModelDidNotWriteIt(t *testing.T) {
	baseline := baselinePlan()
	got := fallback(baseline, "The model could not be reached: connection refused.")

	if got.Method.Source != plan.SourceFallback {
		t.Errorf("source is %q, want the fallback marker", got.Method.Source)
	}
	if !strings.Contains(got.Method.FallbackReason, "connection refused") {
		t.Errorf("the reason was lost: %q", got.Method.FallbackReason)
	}
	if len(got.Notes) == 0 || !strings.Contains(got.Notes[0], "connection refused") {
		t.Errorf("the athlete is not told what happened: %v", got.Notes)
	}
	if !strings.Contains(got.Notes[0], "ready to train") {
		t.Errorf("a fallback note should end on the plan being usable, not on the failure: %q", got.Notes[0])
	}
	// The plan itself is untouched: the sessions are the whole point.
	if len(got.Sessions) != len(baseline.Sessions) || got.Title != baseline.Title {
		t.Error("falling back changed the plan rather than just labelling it")
	}
	// And the original is not mutated, since the caller may still use it.
	if baseline.Method.Source != plan.SourceAlgorithm {
		t.Error("fallback wrote through to the baseline's own method")
	}
}

func TestNotConnectedReadsAsAnInvitationRatherThanAnError(t *testing.T) {
	reason := notConnectedReason(ErrNoCredentials)
	if !strings.Contains(reason, "Settings") {
		t.Errorf("an athlete without an account should be told where to connect one: %q", reason)
	}
	if strings.Contains(strings.ToLower(reason), "error") || strings.Contains(reason, "428") {
		t.Errorf("not having connected an account is not a failure: %q", reason)
	}
	if notConnectedReason(ErrNoKeystore) == reason {
		t.Error("a server-side keystore problem should not be reported as the athlete's doing")
	}
}

func TestMethodFromKeepsThePlacementAndChangesOnlyTheSource(t *testing.T) {
	baseline := baselinePlan()
	method := methodFrom(baseline, plan.SourceAI)
	if method.Source != plan.SourceAI {
		t.Errorf("source is %q", method.Source)
	}
	if method.Rung != "Advanced tuck" || len(method.Ladder) != 2 {
		t.Error("the ladder is computed from the log and is not the model's to revise")
	}
	if empty := methodFrom(Plan{}, plan.SourceAI); empty == nil || empty.Source != plan.SourceAI {
		t.Error("a plan with no method should still come back labelled")
	}
}

func TestTheBriefIsSmallEnoughToSend(t *testing.T) {
	baseline := baselinePlan()
	// Forty sessions is a normal plan; the brief shows week one and the plan
	// level, so it must not grow with the rest of them.
	for week := 2; week <= 8; week++ {
		for day := 1; day <= 5; day++ {
			session := baseline.Sessions[0]
			session.Week, session.DayOfWeek = week, day
			baseline.Sessions = append(baseline.Sessions, session)
		}
	}
	brief := baselineBrief(baseline)
	if strings.Count(brief, "day 1, Front lever") != 1 {
		t.Errorf("the brief should carry week one only, got:\n%s", brief)
	}
	if len(brief) > 4000 {
		t.Errorf("the brief is %d characters, which is too much context to spend on it", len(brief))
	}
}

func TestResearchTravelsWithThePlanAsOpaqueProvenance(t *testing.T) {
	// The plan layer keeps research opaque so neither producer's schema leaks
	// into the other's. It still has to round-trip to the browser intact.
	found := SkillResearch{Summary: "Sources agree on the tuck first.", SearchesUsed: 4}
	encoded, err := json.Marshal(found)
	if err != nil {
		t.Fatal(err)
	}
	p := baselinePlan()
	p.Research = encoded

	body, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var back struct {
		Research SkillResearch `json:"research"`
	}
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if back.Research.Summary != found.Summary || back.Research.SearchesUsed != 4 {
		t.Errorf("research did not survive the round trip: %+v", back.Research)
	}
}
