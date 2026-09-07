package plan

import (
	"regexp"
	"strings"
	"testing"

	"calisthenics/api/internal/training"
)

// Every session is performed in phases, and the page that lays them out can
// only show what the planner named. A session whose warm-up skips straight to
// the specific work is the warm-up an athlete writes for themselves, not one
// this app should be handing them.
func TestEverySessionOpensWithTheGeneralPhases(t *testing.T) {
	lib := seededLibrary(t)
	phaseOf := map[string]string{}
	for _, p := range training.Protocols {
		phaseOf[p.Slug] = p.Phase
	}

	for _, goal := range Goals {
		p, _ := Generate(Request{Goal: goal.Key, Weeks: 6}, healthyElite(), lib)
		for _, s := range p.Sessions {
			seen := map[string]int{}
			for _, slug := range s.WarmupProtocols {
				seen[phaseOf[slug]]++
			}
			for _, want := range []string{training.PhaseJoint, training.PhaseMuscular, training.PhaseMobility} {
				if seen[want] == 0 {
					t.Errorf("%s week %d day %d has no %s protocol: %v",
						goal.Key, s.Week, s.DayOfWeek, want, s.WarmupProtocols)
				}
			}
			// The cap is what keeps a warm-up from becoming the session.
			for phase, n := range seen {
				if phase != training.PhaseRehab && n > 2 {
					t.Errorf("%s week %d day %d stacks %d protocols into %s",
						goal.Key, s.Week, s.DayOfWeek, n, phase)
				}
			}
		}
	}
}

// sets and prescription are separate fields and every renderer writes
// "<sets> × <prescription>". A prescription that opens with its own count is
// printed twice, which is what "3 × 3 × 3-6s hold" was.
var leadingCount = regexp.MustCompile(`^\s*\d+\s*[×x]\s`)

func TestAPrescriptionDoesNotRepeatTheSetCount(t *testing.T) {
	lib := seededLibrary(t)
	for _, goal := range Goals {
		p, _ := Generate(Request{Goal: goal.Key, Weeks: 6}, healthyElite(), lib)
		for _, s := range p.Sessions {
			for _, b := range s.Blocks {
				if leadingCount.MatchString(b.Prescription) {
					t.Errorf("%s: %q already carries a set count, and the block says %d sets",
						goal.Key, b.Prescription, b.Sets)
				}
				if strings.TrimSpace(b.Prescription) == "" {
					t.Errorf("%s: %s has an empty prescription", goal.Key, b.ExerciseSlug)
				}
			}
		}
	}
}
