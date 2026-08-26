package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"calisthenics/api/internal/auth"
	"calisthenics/api/internal/httpx"
	"calisthenics/api/internal/training"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	client   *Client
	pool     *pgxpool.Pool
	training *training.Service
}

func NewHandler(client *Client, pool *pgxpool.Pool, tr *training.Service) *Handler {
	return &Handler{client: client, pool: pool, training: tr}
}

// coachSystem is the standing brief for every coaching call. The rules exist
// because the failure modes are predictable: inventing exercises, ignoring an
// injury, and drifting into medical advice.
const coachSystem = `You are a calisthenics coach writing training for one athlete.

Rules you must follow:
1. Only prescribe exercises whose slug appears in the EXERCISE LIBRARY given to you. Never invent a slug.
2. Only reference warm-up and rehab protocols whose slug appears in the PROTOCOL LIBRARY given to you.
3. If the athlete has an open injury, treat it as a hard restriction. Remove every movement that loads the injured area and say in the summary what you removed and why.
4. Progress from where the athlete actually is, using their records. Do not prescribe a movement more than one clear step above their demonstrated level.
5. Straight-arm work (levers, planche) needs elbow and wrist preparation in every session that includes it.
6. You are not a clinician. Never diagnose. Where pain is involved, say plainly that persistent or worsening pain needs an in-person assessment.
7. Answer with the requested JSON only. No preamble, no code fence, no commentary.`

type PlanBlock struct {
	ExerciseSlug string `json:"exercise_slug"`
	Sets         int    `json:"sets"`
	Prescription string `json:"prescription"`
	RestSeconds  int    `json:"rest_seconds"`
	Notes        string `json:"notes"`
}

type PlanSession struct {
	Week            int         `json:"week"`
	DayOfWeek       int         `json:"day_of_week"`
	Title           string      `json:"title"`
	Focus           string      `json:"focus"`
	WarmupProtocols []string    `json:"warmup_protocols"`
	Blocks          []PlanBlock `json:"blocks"`
}

type Plan struct {
	Title        string        `json:"title"`
	Summary      string        `json:"summary"`
	Weeks        int           `json:"weeks"`
	Restrictions []string      `json:"restrictions"`
	Sessions     []PlanSession `json:"sessions"`
}

type skillPlanRequest struct {
	Skill       string `json:"skill"`
	Weeks       int    `json:"weeks"`
	DaysPerWeek int    `json:"days_per_week"`
	StartsOn    string `json:"starts_on"`
	Notes       string `json:"notes"`
	Save        bool   `json:"save"`
}

type planResponse struct {
	PlanID string `json:"plan_id,omitempty"`
	Plan   Plan   `json:"plan"`
	Saved  bool   `json:"saved"`
}

// How long a coaching call may run. Both are far past the usual case; they
// exist so a stuck request eventually ends rather than to pace anything.
const (
	planBudget  = 8 * time.Minute
	proseBudget = 4 * time.Minute
)

// planTokens sizes the ceiling to the plan being asked for. It has to cover
// the model's reasoning as well as the plan itself: they come out of the same
// budget, and a ceiling that only fits the answer gets spent on the thinking
// and returns nothing at all.
func planTokens(sessions int) int {
	tokens := 6000 + sessions*400
	if tokens > 48000 {
		tokens = 48000
	}
	return tokens
}

func (h *Handler) SkillPlan(w http.ResponseWriter, r *http.Request) {
	var in skillPlanRequest
	if !httpx.Decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Skill) == "" {
		httpx.Fail(w, http.StatusBadRequest, "Name the skill you want to work toward.")
		return
	}
	if in.Weeks < 1 || in.Weeks > 24 {
		in.Weeks = 8
	}
	if in.DaysPerWeek < 1 || in.DaysPerWeek > 7 {
		in.DaysPerWeek = 3
	}
	startsOn := time.Now()
	if in.StartsOn != "" {
		parsed, err := time.Parse("2006-01-02", in.StartsOn)
		if err != nil {
			httpx.Fail(w, http.StatusBadRequest, "Start date must look like 2026-09-01.")
			return
		}
		startsOn = parsed
	}

	me := auth.MustUser(r.Context())
	if !h.client.Configured() {
		httpx.Fail(w, http.StatusServiceUnavailable, "Plan generation is switched off: no API key is set on the server.")
		return
	}

	// Everything past here can take minutes, so the response either streams
	// progress or waits; both need longer than the server's write timeout.
	out := begin(w, r, planBudget)
	defer out.close()
	ctx, cancel := context.WithTimeout(r.Context(), planBudget)
	defer cancel()

	out.report(Progress{Stage: "reading", Label: "Reading your training history", Percent: 2})
	promptContext, err := h.buildContext(ctx, me)
	if err != nil {
		out.fail(http.StatusInternalServerError, "Couldn't read your training history.")
		return
	}

	prompt := fmt.Sprintf(`%s

TASK
Write a %d-week plan to reach: %s
Training days per week: %d
Athlete's extra notes: %s

Return JSON in exactly this shape:
{
  "title": "string",
  "summary": "2-4 sentences on the approach and anything removed for injuries",
  "weeks": %d,
  "restrictions": ["what you avoided and why"],
  "sessions": [
    {
      "week": 1,
      "day_of_week": 1,
      "title": "string",
      "focus": "string",
      "warmup_protocols": ["protocol_slug"],
      "blocks": [
        {"exercise_slug": "slug_from_library", "sets": 4, "prescription": "e.g. 6-8 reps or 3x15s hold",
         "rest_seconds": 120, "notes": "cue or regression"}
      ]
    }
  ]
}

day_of_week is 1 for Monday through 7 for Sunday. Include every session for all %d weeks.`,
		promptContext, in.Weeks, in.Skill, in.DaysPerWeek, orNone(in.Notes), in.Weeks, in.Weeks)

	expected := in.Weeks * in.DaysPerWeek
	tracker := newPlanTracker(expected)
	out.report(Progress{Stage: "sending", Label: "Briefing the coach", Percent: 5, Total: expected})

	var plan Plan
	err = h.client.CompleteJSONStream(ctx, me.ID, "skill_plan", coachSystem, prompt,
		planTokens(expected), func(d Delta) { out.report(tracker.update(d)) }, &plan)
	if err != nil {
		out.fail(http.StatusBadGateway, "Couldn't build that plan: "+err.Error())
		return
	}
	if len(plan.Sessions) == 0 {
		out.fail(http.StatusBadGateway, "The plan came back empty. Try again.")
		return
	}
	if plan.Title == "" {
		plan.Title = in.Skill
	}
	plan.Weeks = in.Weeks

	result := planResponse{Plan: plan}
	if in.Save {
		out.report(Progress{Stage: "saving", Label: "Adding the sessions to your calendar",
			Percent: 95, Done: len(plan.Sessions), Total: expected})
		id, err := h.savePlan(ctx, me.ID, plan, in.Skill, startsOn)
		if err != nil {
			out.fail(http.StatusInternalServerError, "The plan was built but couldn't be saved.")
			return
		}
		result.PlanID = id
		result.Saved = true
	}
	out.report(Progress{Stage: "done", Label: "Plan ready", Percent: 100,
		Done: len(plan.Sessions), Total: expected})
	out.done(result)
}

// savePlan writes the plan and expands it into dated calendar entries.
func (h *Handler) savePlan(ctx context.Context, userID string, plan Plan, goal string, startsOn time.Time) (string, error) {
	body, err := json.Marshal(plan)
	if err != nil {
		return "", err
	}

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var planID string
	err = tx.QueryRow(ctx, `
		insert into plans (user_id, title, goal, starts_on, weeks, body)
		values ($1, $2, $3, $4, $5, $6) returning id`,
		userID, plan.Title, goal, startsOn, plan.Weeks, body).Scan(&planID)
	if err != nil {
		return "", err
	}

	monday := mondayOf(startsOn)
	for _, session := range plan.Sessions {
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

func mondayOf(t time.Time) time.Time {
	offset := (int(t.Weekday()) + 6) % 7 // Sunday==0 becomes 6
	return time.Date(t.Year(), t.Month(), t.Day()-offset, 0, 0, 0, 0, time.UTC)
}

// ---------- review and recovery ----------

type textResponse struct {
	Text string `json:"text"`
}

// The prose answers are short, but the ceiling also has to cover the model's
// reasoning, which is spent from the same budget.
const proseTokens = 8000

func (h *Handler) Review(w http.ResponseWriter, r *http.Request) {
	me := auth.MustUser(r.Context())
	if !h.client.Configured() {
		httpx.Fail(w, http.StatusServiceUnavailable, "Coaching is switched off: no API key is set on the server.")
		return
	}

	out := begin(w, r, proseBudget)
	defer out.close()
	ctx, cancel := context.WithTimeout(r.Context(), proseBudget)
	defer cancel()

	out.report(Progress{Stage: "reading", Label: "Reading your last four weeks", Percent: 3})
	ctxText, err := h.buildContext(ctx, me)
	if err != nil {
		out.fail(http.StatusInternalServerError, "Couldn't read your training history.")
		return
	}

	prompt := ctxText + `

TASK
Review the last four weeks of training. In plain prose, under 300 words:
- name the single biggest gap or imbalance you can see in the data
- say what is going well, specifically, citing a number from the records
- give three concrete changes for the next four weeks
If there is too little data to judge, say so and name what to log first.
Answer as prose, not JSON.`

	tracker := newProseTracker(1800, "Reading your records")
	text, err := h.client.CompleteStream(ctx, me.ID, "review", coachSystem, prompt, proseTokens,
		func(d Delta) { out.report(tracker.update(d)) })
	if err != nil {
		out.fail(http.StatusBadGateway, "Couldn't produce a review: "+err.Error())
		return
	}
	out.report(Progress{Stage: "done", Label: "Review ready", Percent: 100})
	out.done(textResponse{Text: text})
}

func (h *Handler) Recovery(w http.ResponseWriter, r *http.Request) {
	me := auth.MustUser(r.Context())
	if !h.client.Configured() {
		httpx.Fail(w, http.StatusServiceUnavailable, "Recovery guidance is switched off: no API key is set on the server.")
		return
	}

	out := begin(w, r, proseBudget)
	defer out.close()
	ctx, cancel := context.WithTimeout(r.Context(), proseBudget)
	defer cancel()

	out.report(Progress{Stage: "reading", Label: "Reading your recent load", Percent: 3})
	ctxText, err := h.buildContext(ctx, me)
	if err != nil {
		out.fail(http.StatusInternalServerError, "Couldn't read your training history.")
		return
	}

	prompt := ctxText + `

TASK
Give recovery guidance for the coming week, under 350 words, covering:
- which protocol slugs from the library to run, and on which days
- how to adjust current training around any open injury
- sleep and rest-day structure given the recent session count
- general nutrition guidance for this training load: protein target as g per kg of bodyweight,
  overall energy direction, and hydration. Give ranges, not a meal plan, and do not prescribe
  supplements or restrict food groups.
End with one line on when to stop self-managing and see a clinician.
Answer as prose, not JSON.`

	tracker := newProseTracker(2100, "Weighing your recent load")
	text, err := h.client.CompleteStream(ctx, me.ID, "recovery", coachSystem, prompt, proseTokens,
		func(d Delta) { out.report(tracker.update(d)) })
	if err != nil {
		out.fail(http.StatusBadGateway, "Couldn't produce recovery guidance: "+err.Error())
		return
	}
	out.report(Progress{Stage: "done", Label: "Guidance ready", Percent: 100})
	out.done(textResponse{Text: text})
}

// ---------- prompt context ----------

// buildContext assembles everything the model is allowed to reason from: the
// athlete's computed snapshot plus the two libraries it must choose within.
func (h *Handler) buildContext(ctx context.Context, user auth.User) (string, error) {
	snapshot, err := h.training.BuildSnapshot(ctx, user)
	if err != nil {
		return "", err
	}
	snapshotJSON, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return "", err
	}

	rows, err := h.pool.Query(ctx, `
		select slug, name, category, measure, difficulty from exercises order by category, difficulty`)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var library strings.Builder
	for rows.Next() {
		var slug, name, category, measure string
		var difficulty int
		if err := rows.Scan(&slug, &name, &category, &measure, &difficulty); err != nil {
			return "", err
		}
		fmt.Fprintf(&library, "- %s | %s | %s | measured in %s | difficulty %d\n",
			slug, name, category, measure, difficulty)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	var protocols strings.Builder
	for _, p := range training.Protocols {
		fmt.Fprintf(&protocols, "- %s | %s | %s | for %s\n", p.Slug, p.Title, p.Purpose, p.Region)
	}

	return fmt.Sprintf(`ATHLETE SNAPSHOT (computed from their log, treat as fact)
%s

EXERCISE LIBRARY (the only exercises you may prescribe)
%s
PROTOCOL LIBRARY (the only warm-up and rehab protocols you may reference)
%s`, snapshotJSON, library.String(), protocols.String()), nil
}

func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "none"
	}
	return s
}
