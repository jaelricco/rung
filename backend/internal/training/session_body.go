package training

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// A session written by the athlete has the same body as a session written by
// the model: same field names, same order of blocks, so one component renders
// both and a routine day can be dropped onto the calendar unchanged.
type SessionBlock struct {
	ExerciseSlug string `json:"exercise_slug"`
	// Intent is what the block is there for — prep, skill, strength,
	// accessory, conditioning — and is what fixes its place in the session.
	Intent       string `json:"intent,omitempty"`
	Sets         int    `json:"sets"`
	Prescription string `json:"prescription"`
	Intensity    string `json:"intensity,omitempty"`
	Tempo        string `json:"tempo,omitempty"`
	RestSeconds  int    `json:"rest_seconds,omitempty"`
	Progression  string `json:"progression,omitempty"`
	Notes        string `json:"notes,omitempty"`
}

type SessionBody struct {
	Title           string         `json:"title"`
	Focus           string         `json:"focus"`
	Load            string         `json:"load,omitempty"`
	DurationMinutes int            `json:"duration_minutes,omitempty"`
	WarmupProtocols []string       `json:"warmup_protocols,omitempty"`
	Blocks          []SessionBlock `json:"blocks"`
	Cooldown        string         `json:"cooldown,omitempty"`
}

const (
	maxBlocksPerSession = 30
	maxTitleLength      = 120
	maxTextLength       = 500
)

var blockIntents = map[string]bool{
	"":             true,
	"prep":         true,
	"skill":        true,
	"strength":     true,
	"accessory":    true,
	"conditioning": true,
}

// validate normalises a session written in the browser and rejects what the
// rest of the app could not render or the athlete could not perform. Unknown
// exercise slugs are refused rather than dropped: a session quietly missing
// the movement it was built around is worse than an error message.
//
// known is the set of slugs in the library, as loaded by knownSlugs.
func (b *SessionBody) validate(known map[string]bool, where string) error {
	b.Title = strings.TrimSpace(b.Title)
	b.Focus = strings.TrimSpace(b.Focus)
	b.Load = strings.TrimSpace(b.Load)
	b.Cooldown = strings.TrimSpace(b.Cooldown)

	if b.Title == "" {
		return fmt.Errorf("%s needs a name", where)
	}
	if len(b.Title) > maxTitleLength {
		return fmt.Errorf("%s has a name longer than %d characters", where, maxTitleLength)
	}
	if len(b.Focus) > maxTextLength || len(b.Cooldown) > maxTextLength {
		return fmt.Errorf("%s has a focus or cool-down longer than %d characters", where, maxTextLength)
	}
	if b.DurationMinutes < 0 || b.DurationMinutes > 600 {
		return fmt.Errorf("%s has a length that isn't a plausible session", where)
	}
	if len(b.Blocks) > maxBlocksPerSession {
		return fmt.Errorf("%s has more than %d blocks", where, maxBlocksPerSession)
	}
	if b.Blocks == nil {
		b.Blocks = []SessionBlock{}
	}

	for i := range b.Blocks {
		block := &b.Blocks[i]
		block.ExerciseSlug = strings.TrimSpace(block.ExerciseSlug)
		block.Prescription = strings.TrimSpace(block.Prescription)
		block.Intensity = strings.TrimSpace(block.Intensity)
		block.Tempo = strings.TrimSpace(block.Tempo)
		block.Progression = strings.TrimSpace(block.Progression)
		block.Notes = strings.TrimSpace(block.Notes)
		block.Intent = strings.ToLower(strings.TrimSpace(block.Intent))

		pos := fmt.Sprintf("%s, block %d", where, i+1)
		if block.ExerciseSlug == "" {
			return fmt.Errorf("%s needs an exercise", pos)
		}
		if !known[block.ExerciseSlug] {
			return fmt.Errorf("%s refers to an exercise that isn't in the library: %s", pos, block.ExerciseSlug)
		}
		if !blockIntents[block.Intent] {
			return fmt.Errorf("%s has an unknown intent %q", pos, block.Intent)
		}
		if block.Sets < 1 || block.Sets > 20 {
			return fmt.Errorf("%s needs between 1 and 20 sets", pos)
		}
		if block.Prescription == "" {
			return fmt.Errorf("%s needs a prescription, such as \"5 reps\" or \"20s hold\"", pos)
		}
		if len(block.Prescription) > maxTextLength || len(block.Notes) > maxTextLength {
			return fmt.Errorf("%s has a prescription or note longer than %d characters", pos, maxTextLength)
		}
		if block.RestSeconds < 0 || block.RestSeconds > 3600 {
			return fmt.Errorf("%s has a rest time outside 0 to 3600 seconds", pos)
		}
	}
	return nil
}

// knownSlugs is the library as a lookup. One query serves a whole routine, so
// a twelve-block week is not twelve round trips.
func (s *Service) knownSlugs(ctx context.Context) (map[string]bool, error) {
	rows, err := s.pool.Query(ctx, `select slug from exercises`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	known := map[string]bool{}
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		known[slug] = true
	}
	return known, rows.Err()
}

func (b SessionBody) marshal() ([]byte, error) { return json.Marshal(b) }

// parseDate reads a YYYY-MM-DD date as a plain calendar day. Everything on the
// calendar is a day, never an instant, so no zone is applied.
func parseDate(value string) (time.Time, error) {
	return time.Parse("2006-01-02", value)
}

// MondayOf is the start of the ISO week a date falls in. A routine is a week,
// and a week starts on Monday everywhere in this app.
func MondayOf(t time.Time) time.Time {
	offset := (int(t.Weekday()) + 6) % 7 // Sunday==0 becomes 6
	return time.Date(t.Year(), t.Month(), t.Day()-offset, 0, 0, 0, 0, time.UTC)
}
