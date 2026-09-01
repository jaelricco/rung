// Package plan owns the training plan: what one is, how it is checked, how it
// is written to the calendar, and how one is built from an athlete's records
// without asking anything of a model.
//
// The split with internal/ai is deliberate. A plan is a domain object, not an
// AI artifact, and it has two producers: this package's Generate, which always
// answers, and the model, which answers better and sometimes not at all. Both
// hand back the same struct, both are checked by the same Validate, and both
// are saved by the same Save — so the athlete's calendar cannot tell which one
// wrote the week, and nothing downstream has to care.
package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"calisthenics/api/internal/training"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Block is one movement in a session, with everything needed to perform it.
type Block struct {
	ExerciseSlug string `json:"exercise_slug"`
	// Intent is what the block is there for, which is also what fixes its
	// place in the session: prep, skill, strength, accessory, conditioning.
	Intent       string `json:"intent"`
	Sets         int    `json:"sets"`
	Prescription string `json:"prescription"`
	Intensity    string `json:"intensity"`
	Tempo        string `json:"tempo,omitempty"`
	RestSeconds  int    `json:"rest_seconds"`
	Progression  string `json:"progression"`
	Notes        string `json:"notes"`
}

type Session struct {
	Week            int      `json:"week"`
	DayOfWeek       int      `json:"day_of_week"`
	Title           string   `json:"title"`
	Focus           string   `json:"focus"`
	Load            string   `json:"load"`
	DurationMinutes int      `json:"duration_minutes"`
	WarmupProtocols []string `json:"warmup_protocols"`
	Blocks          []Block  `json:"blocks"`
	Cooldown        string   `json:"cooldown,omitempty"`
}

// Phase is a block of weeks with one aim, so the shape of a plan is legible
// without reading all forty sessions.
type Phase struct {
	Weeks string `json:"weeks"`
	Name  string `json:"name"`
	Aim   string `json:"aim"`
}

// Rung is one step of the ladder to a skill, as it appeared to the planner:
// what it is, what clears it, and whether this athlete has cleared it. It is
// returned with the plan so the athlete can see why they were put where they
// were put, rather than being told to trust it.
type Rung struct {
	Name          string   `json:"name"`
	ExerciseSlugs []string `json:"exercise_slugs"`
	Standard      string   `json:"standard"`
	Cleared       bool     `json:"cleared"`
	Current       bool     `json:"current"`
}

// Method is the plan's provenance: which producer wrote it, what it decided
// about the athlete, and — when the model was asked and could not answer —
// why the athlete is reading the algorithm's plan instead.
type Method struct {
	Source         string `json:"source"` // one of the Source constants below
	Goal           string `json:"goal"`
	GoalMatched    bool   `json:"goal_matched"`
	Rung           string `json:"rung,omitempty"`
	NextRung       string `json:"next_rung,omitempty"`
	Ladder         []Rung `json:"ladder,omitempty"`
	Readiness      string `json:"readiness,omitempty"`
	FallbackReason string `json:"fallback_reason,omitempty"`
}

// Where a plan came from.
const (
	SourceAlgorithm = "algorithm"
	SourceAI        = "ai"
	// SourceFallback is an algorithmic plan the athlete asked the model for
	// and did not get. The plan is still a plan; Method.FallbackReason says
	// what happened.
	SourceFallback = "algorithm_fallback"
)

type Plan struct {
	Title            string   `json:"title"`
	Summary          string   `json:"summary"`
	Weeks            int      `json:"weeks"`
	Restrictions     []string `json:"restrictions"`
	Phases           []Phase  `json:"phases"`
	ProgressionRules []string `json:"progression_rules"`
	Test             string   `json:"test"`
	// Notes are things worth knowing that are not warnings: an unrecognised
	// goal, a frequency the log does not support yet. Warnings are for what
	// was taken *out* of the plan, and mixing the two makes both easier to
	// ignore.
	Notes    []string  `json:"notes,omitempty"`
	Sessions []Session `json:"sessions"`
	Method   *Method   `json:"method,omitempty"`
	// Research is whatever the producer read before writing. The AI path puts
	// its search findings and citations here; the algorithm leaves it empty.
	// It stays opaque at this layer so neither producer's schema leaks into
	// the other's.
	Research json.RawMessage `json:"research,omitempty"`
}

// ---------- the library ----------

// Library is what may be prescribed, and the thing every plan is checked back
// against. A slug the athlete cannot log is not a small error: it never
// reaches the level calculation and the block is silent about what it wanted.
type Library struct {
	Exercises map[string]training.Exercise
	Protocols map[string]training.Protocol
	// order keeps the catalogue's rendering stable between calls.
	order []string
}

func (l Library) Has(slug string) bool {
	_, ok := l.Exercises[strings.TrimSpace(slug)]
	return ok
}

// Keep filters a slug list down to what exists, quietly. Used where a dropped
// slug costs a line of prose rather than a set.
func (l Library) Keep(slugs []string) []string {
	kept := []string{}
	for _, slug := range slugs {
		if l.Has(slug) {
			kept = append(kept, strings.TrimSpace(slug))
		}
	}
	return kept
}

func (l Library) KeepProtocols(slugs []string, warn func(string, ...any)) []string {
	kept := []string{}
	for _, slug := range slugs {
		slug = strings.TrimSpace(slug)
		if _, ok := l.Protocols[slug]; ok {
			kept = append(kept, slug)
			continue
		}
		if warn != nil {
			warn("The warm-up protocol %q does not exist, so it was removed.", slug)
		}
	}
	return kept
}

// Text renders the catalogue for a model prompt. The description is what tells
// a reader that a planche lean scales by lean angle and a negative pull-up is
// a five-second lower; without it the slug is a name and the prescription is a
// guess.
func (l Library) Text() string {
	var b strings.Builder
	b.WriteString("EXERCISE LIBRARY (the only exercises you may prescribe)\n")
	for _, slug := range l.order {
		e := l.Exercises[slug]
		fmt.Fprintf(&b, "- %s | %s | %s | measured in %s | difficulty %d/10 | %s\n",
			e.Slug, e.Name, e.Category, e.Measure, e.Difficulty, e.Description)
	}
	b.WriteString("\nPROTOCOL LIBRARY (the only warm-up and rehab protocols you may reference)\n")
	for _, p := range training.Protocols {
		fmt.Fprintf(&b, "- %s | %s | %s | for %s\n", p.Slug, p.Title, p.Purpose, p.Region)
	}
	return b.String()
}

// LoadLibrary reads the exercise table and the curated protocol list.
func LoadLibrary(ctx context.Context, pool *pgxpool.Pool) (Library, error) {
	lib := Library{
		Exercises: map[string]training.Exercise{},
		Protocols: map[string]training.Protocol{},
	}
	for _, p := range training.Protocols {
		lib.Protocols[p.Slug] = p
	}

	rows, err := pool.Query(ctx, `
		select slug, name, category, measure, difficulty, description
		from exercises order by category, difficulty, slug`)
	if err != nil {
		return lib, err
	}
	defer rows.Close()

	for rows.Next() {
		var e training.Exercise
		if err := rows.Scan(&e.Slug, &e.Name, &e.Category, &e.Measure, &e.Difficulty, &e.Description); err != nil {
			return lib, err
		}
		lib.Exercises[e.Slug] = e
		lib.order = append(lib.order, e.Slug)
	}
	return lib, rows.Err()
}

// ---------- validation ----------

// Validate holds a plan to the library it was written against. A prescription
// naming an exercise that does not exist is dropped and the drop is reported,
// which is a smaller lie than showing a slug that resolves to nothing.
//
// It runs over both producers' output: the model's, because models invent
// slugs, and the algorithm's, because the algorithm is written against a
// library that a migration could change underneath it.
func Validate(p *Plan, lib Library, weeks int) []string {
	var warnings []string
	seen := map[string]bool{}
	warn := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		if !seen[msg] {
			seen[msg] = true
			warnings = append(warnings, msg)
		}
	}

	kept := p.Sessions[:0]
	for _, session := range p.Sessions {
		if session.Week < 1 {
			session.Week = 1
		}
		if session.Week > weeks {
			warn("A session was scheduled in week %d, past the end of the plan, and was dropped.", session.Week)
			continue
		}
		if session.DayOfWeek < 1 || session.DayOfWeek > 7 {
			session.DayOfWeek = 1
		}

		session.WarmupProtocols = lib.KeepProtocols(session.WarmupProtocols, warn)

		blocks := session.Blocks[:0]
		for _, block := range session.Blocks {
			slug := strings.TrimSpace(block.ExerciseSlug)
			if !lib.Has(slug) {
				warn("The exercise %q is not in the library, so that block was dropped.", slug)
				continue
			}
			block.ExerciseSlug = slug
			if block.Sets < 1 {
				block.Sets = 1
			}
			if block.RestSeconds < 0 || block.RestSeconds > 900 {
				block.RestSeconds = 0
			}
			blocks = append(blocks, block)
		}
		session.Blocks = blocks

		if len(session.Blocks) == 0 {
			warn("A session lost every block it had and was dropped.")
			continue
		}
		kept = append(kept, session)
	}
	p.Sessions = kept

	// Calendar order, so week 3 Tuesday never renders before week 2 Friday.
	sort.SliceStable(p.Sessions, func(i, j int) bool {
		if p.Sessions[i].Week != p.Sessions[j].Week {
			return p.Sessions[i].Week < p.Sessions[j].Week
		}
		return p.Sessions[i].DayOfWeek < p.Sessions[j].DayOfWeek
	})
	return warnings
}

// ---------- persistence ----------

// Save writes the plan and expands it into dated calendar entries. Generating
// a plan and committing to it are separate everywhere in this app: a plan is
// worth reading before it becomes eight weeks of appointments.
func Save(ctx context.Context, pool *pgxpool.Pool, userID string, p Plan, goal string, startsOn time.Time) (string, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return "", err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var planID string
	err = tx.QueryRow(ctx, `
		insert into plans (user_id, title, goal, starts_on, weeks, body)
		values ($1, $2, $3, $4, $5, $6) returning id`,
		userID, p.Title, goal, startsOn, p.Weeks, body).Scan(&planID)
	if err != nil {
		return "", err
	}

	monday := training.MondayOf(startsOn)
	for _, session := range p.Sessions {
		day := session.DayOfWeek
		if day < 1 || day > 7 {
			day = 1
		}
		week := session.Week
		if week < 1 {
			week = 1
		}
		date := monday.AddDate(0, 0, (week-1)*7+(day-1))

		sessionBody, err := json.Marshal(session)
		if err != nil {
			return "", err
		}
		_, err = tx.Exec(ctx, `
			insert into planned_sessions (plan_id, user_id, scheduled_on, title, focus, body)
			values ($1, $2, $3, $4, $5, $6)`,
			planID, userID, date, session.Title, session.Focus, sessionBody)
		if err != nil {
			return "", err
		}
	}
	return planID, tx.Commit(ctx)
}
