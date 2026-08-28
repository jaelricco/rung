package training

import (
	"strings"
	"testing"
	"time"
)

var library = map[string]bool{"tuck_planche": true, "pull_up": true}

func day(dayOfWeek int, body SessionBody) routineDayInput {
	return routineDayInput{DayOfWeek: dayOfWeek, Body: body}
}

func pushDay() SessionBody {
	return SessionBody{
		Title:  "Push",
		Focus:  "planche",
		Blocks: []SessionBlock{{ExerciseSlug: "tuck_planche", Intent: "skill", Sets: 5, Prescription: "12s hold"}},
	}
}

func TestMondayOf(t *testing.T) {
	cases := map[string]string{
		"2026-08-31": "2026-08-31", // a Monday is its own week start
		"2026-09-06": "2026-08-31", // Sunday belongs to the week that began
		"2026-09-02": "2026-08-31",
		"2026-01-01": "2025-12-29", // and a week can start in the previous year
	}
	for in, want := range cases {
		parsed, err := parseDate(in)
		if err != nil {
			t.Fatalf("parse %s: %v", in, err)
		}
		if got := MondayOf(parsed).Format("2006-01-02"); got != want {
			t.Errorf("MondayOf(%s) = %s, want %s", in, got, want)
		}
	}
}

func TestWeekOf(t *testing.T) {
	got, err := weekOf("2026-09-03") // a Thursday
	if err != nil {
		t.Fatalf("weekOf: %v", err)
	}
	if got.Format("2006-01-02") != "2026-08-31" {
		t.Errorf("weekOf returned %s, want the Monday 2026-08-31", got.Format("2006-01-02"))
	}

	// No date means this week, which is the case "just this week" hits.
	now, err := weekOf("")
	if err != nil {
		t.Fatalf("weekOf empty: %v", err)
	}
	if now.Weekday() != time.Monday {
		t.Errorf("weekOf(\"\") returned a %s, want a Monday", now.Weekday())
	}

	if _, err := weekOf("next tuesday"); err == nil {
		t.Error("weekOf accepted a date it cannot parse")
	}
}

func TestNormaliseRepeat(t *testing.T) {
	// Empty defaults to weekly: a routine repeats unless told otherwise.
	for _, in := range []string{"weekly", "every_week", "", "WEEKLY"} {
		if got, ok := normaliseRepeat(in); !ok || got != RepeatWeekly {
			t.Errorf("normaliseRepeat(%q) = %q, %v; want weekly", in, got, ok)
		}
	}
	for _, in := range []string{"once", "this_week", " Once "} {
		if got, ok := normaliseRepeat(in); !ok || got != RepeatOnce {
			t.Errorf("normaliseRepeat(%q) = %q, %v; want once", in, got, ok)
		}
	}
	if _, ok := normaliseRepeat("fortnightly"); ok {
		t.Error("normaliseRepeat accepted a repeat it cannot schedule")
	}
}

func TestHorizonForFillsTheWindowBeingLookedAt(t *testing.T) {
	near := horizonFor(time.Now().AddDate(0, 0, 7))
	if near.Before(defaultHorizon().Add(-time.Second)) {
		t.Error("a near window shortened the horizon below the default")
	}
	far := horizonFor(time.Now().AddDate(0, 0, 200))
	if far.Before(time.Now().AddDate(0, 0, 199)) {
		t.Error("paging months ahead did not extend the horizon to what is being read")
	}
	absurd := horizonFor(time.Now().AddDate(50, 0, 0))
	if absurd.After(time.Now().AddDate(0, 0, routineHorizonCap+1)) {
		t.Error("a far future window was allowed past the cap")
	}
}

func TestValidateDays(t *testing.T) {
	if err := validateDays(nil, library); err == nil {
		t.Error("an empty routine was accepted")
	}
	if err := validateDays([]routineDayInput{day(0, pushDay())}, library); err == nil {
		t.Error("a session on day 0 was accepted")
	}
	if err := validateDays([]routineDayInput{day(8, pushDay())}, library); err == nil {
		t.Error("a session on day 8 was accepted")
	}
	if err := validateDays([]routineDayInput{day(1, pushDay()), day(4, pushDay())}, library); err != nil {
		t.Errorf("a plain two-day week was rejected: %v", err)
	}

	// Two sessions on one day is a real week for anyone training twice a day.
	if err := validateDays([]routineDayInput{day(1, pushDay()), day(1, pushDay())}, library); err != nil {
		t.Errorf("two sessions on one day were rejected: %v", err)
	}

	tooMany := make([]routineDayInput, maxRoutineDays+1)
	for i := range tooMany {
		tooMany[i] = day(1, pushDay())
	}
	if err := validateDays(tooMany, library); err == nil {
		t.Error("a routine past the day limit was accepted")
	}

	// The day is named in the message, because "session 3 is wrong" is not
	// something an athlete can act on.
	err := validateDays([]routineDayInput{day(3, SessionBody{Title: ""})}, library)
	if err == nil || !strings.Contains(err.Error(), "Wednesday") {
		t.Errorf("a nameless Wednesday gave %v, want a message naming the day", err)
	}
}

func TestSessionBodyValidate(t *testing.T) {
	unknown := SessionBody{
		Title:  "Push",
		Blocks: []SessionBlock{{ExerciseSlug: "planche_wizardry", Sets: 3, Prescription: "5"}},
	}
	err := unknown.validate(library, "That session")
	if err == nil || !strings.Contains(err.Error(), "planche_wizardry") {
		t.Errorf("an invented exercise gave %v, want a refusal naming the slug", err)
	}

	// A session with nothing in it is still a legitimate calendar entry: a
	// name is enough to plan around.
	bare := SessionBody{Title: "  Track sprints  "}
	if err := bare.validate(library, "That session"); err != nil {
		t.Errorf("a session with no blocks was rejected: %v", err)
	}
	if bare.Title != "Track sprints" {
		t.Errorf("the title was not trimmed: %q", bare.Title)
	}
	if bare.Blocks == nil {
		t.Error("blocks came back nil, which serialises as null rather than an empty list")
	}

	nameless := SessionBody{Title: ""}
	if err := nameless.validate(library, "That session"); err == nil {
		t.Error("a nameless session was accepted")
	}

	bad := []struct {
		name string
		body SessionBody
	}{
		{"no sets", SessionBody{Title: "Push", Blocks: []SessionBlock{{ExerciseSlug: "pull_up", Sets: 0, Prescription: "5"}}}},
		{"too many sets", SessionBody{Title: "Push", Blocks: []SessionBlock{{ExerciseSlug: "pull_up", Sets: 21, Prescription: "5"}}}},
		{"no prescription", SessionBody{Title: "Push", Blocks: []SessionBlock{{ExerciseSlug: "pull_up", Sets: 3}}}},
		{"unknown intent", SessionBody{Title: "Push", Blocks: []SessionBlock{{ExerciseSlug: "pull_up", Intent: "vibes", Sets: 3, Prescription: "5"}}}},
		{"negative rest", SessionBody{Title: "Push", Blocks: []SessionBlock{{ExerciseSlug: "pull_up", Sets: 3, Prescription: "5", RestSeconds: -1}}}},
		{"impossible length", SessionBody{Title: "Push", DurationMinutes: 6000}},
	}
	for _, c := range bad {
		body := c.body
		if err := body.validate(library, "That session"); err == nil {
			t.Errorf("%s was accepted", c.name)
		}
	}

	// Intent is matched case-insensitively, and normalised on the way in.
	ok := SessionBody{Title: "Push", Blocks: []SessionBlock{{ExerciseSlug: "pull_up", Intent: "Strength", Sets: 3, Prescription: "5"}}}
	if err := ok.validate(library, "That session"); err != nil {
		t.Errorf("a capitalised intent was rejected: %v", err)
	}
	if ok.Blocks[0].Intent != "strength" {
		t.Errorf("intent was stored as %q, want the normalised form", ok.Blocks[0].Intent)
	}
}
