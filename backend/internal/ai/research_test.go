package ai

import (
	"strings"
	"testing"
)

func TestResearchKeyCollapsesTheWaysASkillGetsTyped(t *testing.T) {
	same := []string{"Front lever", "front-lever", "the full front lever", "  FRONT   LEVER!  "}
	want := researchKey(same[0])
	if want == "" {
		t.Fatal("a real skill produced an empty cache key")
	}
	for _, variant := range same[1:] {
		if got := researchKey(variant); got != want {
			t.Fatalf("%q keyed as %q, want %q", variant, got, want)
		}
	}
	if researchKey("   ") != "" {
		t.Fatal("an empty skill should not produce a cache key")
	}
	// Stripping filler must never empty out a skill that is only filler words.
	if researchKey("the full") == "" {
		t.Fatal("a skill made only of filler words lost its key entirely")
	}
}

func TestResearchIsPrunedToTheLibrary(t *testing.T) {
	found := SkillResearch{
		Summary: "something",
		Progression: []ResearchStage{{
			Stage:         "tuck",
			ExerciseSlugs: []string{"tuck_front_lever", "invented_lever"},
		}},
		KeyDrills: []ResearchDrill{
			{ExerciseSlug: "pull_up"}, {ExerciseSlug: "nonsense_row"},
		},
		Accessories: []ResearchDrill{{ExerciseSlug: "also_nonsense"}},
	}
	found.pruneToLibrary(testLibrary())

	if got := found.Progression[0].ExerciseSlugs; len(got) != 1 || got[0] != "tuck_front_lever" {
		t.Fatalf("progression kept %v, want only the real slug", got)
	}
	if len(found.KeyDrills) != 1 || found.KeyDrills[0].ExerciseSlug != "pull_up" {
		t.Fatalf("drills kept %+v, want only pull_up", found.KeyDrills)
	}
	if len(found.Accessories) != 0 {
		t.Fatalf("accessories kept %+v, want none", found.Accessories)
	}
}

func TestTopSourcesPrefersThePassageItRead(t *testing.T) {
	sources := topSources([]Source{
		{URL: "https://example.com/a/"},
		{URL: "http://www.example.com/a", CitedText: "the passage"},
		{URL: "https://example.com/b"},
		{URL: ""},
	}, 8)

	if len(sources) != 2 {
		t.Fatalf("kept %d sources, want 2 after collapsing the duplicate", len(sources))
	}
	if sources[0].CitedText != "the passage" {
		t.Fatal("the duplicate carrying the cited passage should have won")
	}

	if got := topSources([]Source{{URL: "https://a.test"}, {URL: "https://b.test"}}, 1); len(got) != 1 {
		t.Fatalf("the limit was ignored: %d sources", len(got))
	}
}

func TestBriefRendersOnlyWhatItHas(t *testing.T) {
	if (SkillResearch{}).brief() != "" {
		t.Fatal("empty research should contribute nothing to the prompt")
	}
	brief := SkillResearch{
		Summary:   "how it is built",
		KeyDrills: []ResearchDrill{{ExerciseSlug: "pull_up", Role: "pulling strength", Dosage: "4x5"}},
	}.brief()
	if brief == "" {
		t.Fatal("research with drills rendered nothing")
	}
	if !strings.Contains(brief, "pull_up") || !strings.Contains(brief, "4x5") {
		t.Fatalf("the brief dropped the drill it was given:\n%s", brief)
	}
}
