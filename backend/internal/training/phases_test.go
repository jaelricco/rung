package training

import "testing"

// The phases are served to the session page rather than written there, so this
// is the one place the running order is defined. A protocol with a phase that
// is not one of them would be dropped from the page without a word.
func TestEveryProtocolLandsInAPhaseThePageRenders(t *testing.T) {
	rendered := map[string]bool{PhaseRehab: true}
	for _, p := range SessionPhases {
		rendered[p.Key] = true
	}

	for _, p := range Protocols {
		if p.Phase == "" {
			t.Errorf("protocol %q has no phase", p.Slug)
			continue
		}
		if !rendered[p.Phase] {
			t.Errorf("protocol %q is in phase %q, which no section renders", p.Slug, p.Phase)
		}
		if p.Purpose == "rehab" && p.Phase != PhaseRehab {
			t.Errorf("rehab protocol %q claims warm-up phase %q", p.Slug, p.Phase)
		}
		if p.Purpose == "warmup" && p.Phase == PhaseRehab {
			t.Errorf("warm-up protocol %q is filed as rehab", p.Slug)
		}
	}
}

func TestTheRunningOrderIsTheSixPhases(t *testing.T) {
	want := []string{PhaseJoint, PhaseMuscular, PhaseMobility, PhaseSpecific, PhaseTraining, PhaseCooldown}
	if len(SessionPhases) != len(want) {
		t.Fatalf("the running order has %d phases, want %d", len(SessionPhases), len(want))
	}
	for i, phase := range SessionPhases {
		if phase.Key != want[i] {
			t.Errorf("phase %d is %q, want %q", i, phase.Key, want[i])
		}
		if phase.Label == "" || phase.Note == "" {
			t.Errorf("phase %q needs a label and a note — the page shows both", phase.Key)
		}
	}
}
