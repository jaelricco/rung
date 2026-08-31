package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"calisthenics/api/internal/auth"
	"calisthenics/api/internal/httpx"
	"calisthenics/api/internal/training"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	store    *Store
	pool     *pgxpool.Pool
	training *training.Service
}

func NewHandler(store *Store, pool *pgxpool.Pool, tr *training.Service) *Handler {
	return &Handler{store: store, pool: pool, training: tr}
}

// coachSystem is the standing brief for every coaching call.
//
// The first block exists because the failure modes are predictable: inventing
// exercises, ignoring an injury, drifting into medical advice. The second
// block is about the plan being worth following. A model asked for "a plan"
// writes something plan-shaped — three sets of a plausible movement, repeated
// for eight weeks, with no ordering, no intensity and no way to tell whether
// week 6 is harder than week 1. Each rule below turns off one of those.
const coachSystem = `You are a strength coach writing calisthenics training for one athlete.

Rules you must follow:
1. Only prescribe exercises whose slug appears in the EXERCISE LIBRARY given to you. Never invent a slug.
2. Only reference warm-up and rehab protocols whose slug appears in the PROTOCOL LIBRARY given to you.
3. If the athlete has an open injury, treat it as a hard restriction. Remove every movement that loads the injured area and say in the summary what you removed and why.
4. Progress from where the athlete actually is, using their records. Do not prescribe a movement more than one clear step above their demonstrated level.
5. Straight-arm work (levers, planche) needs elbow and wrist preparation in every session that includes it.
6. You are not a clinician. Never diagnose. Where pain is involved, say plainly that persistent or worsening pain needs an in-person assessment.
7. Answer with the requested JSON only. No preamble, no code fence, no commentary.

How the training itself has to be built:
8. A session has an order: joint preparation, then skill and straight-arm work while the athlete is fresh, then heavy strength, then accessories, then anything with a conditioning intent. Never put the hardest skill work after the work that fatigues it.
9. Prescribe intensity, not only volume. For reps, say how many should be left in reserve. For holds, prescribe a fraction of the athlete's best hold — several clean holds at around 60 to 70 percent beat one at failure, because a static held to collapse trains the collapse.
10. Load has to move across the weeks, and how it moves has to be written down. Every block states what changes the following week: a second added to the hold, a rep added, a kilo added, a lever opened. "Progress when it feels easy" is not a progression.
11. Straight-arm statics stay at low reps and high quality. Stop the set when the line breaks, not when the arms give out.
12. Any plan of six weeks or more contains a lighter week: roughly half the working volume, intensity kept, placed before the block steps up.
13. Leave 48 hours between hard sessions for the same movement pattern. If the athlete trains many days a week, alternate the pattern rather than repeating it.
14. The last week tests the goal, and the test is stated in terms someone can pass or fail.
15. No filler. If a session only needs four blocks, write four. Every block earns its place by naming what it builds toward.`

type PlanBlock struct {
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

type PlanSession struct {
	Week            int         `json:"week"`
	DayOfWeek       int         `json:"day_of_week"`
	Title           string      `json:"title"`
	Focus           string      `json:"focus"`
	Load            string      `json:"load"`
	DurationMinutes int         `json:"duration_minutes"`
	WarmupProtocols []string    `json:"warmup_protocols"`
	Blocks          []PlanBlock `json:"blocks"`
	Cooldown        string      `json:"cooldown,omitempty"`
}

// PlanPhase is a block of weeks with one aim, so the shape of the plan is
// legible without reading all forty sessions.
type PlanPhase struct {
	Weeks string `json:"weeks"`
	Name  string `json:"name"`
	Aim   string `json:"aim"`
}

type Plan struct {
	Title            string         `json:"title"`
	Summary          string         `json:"summary"`
	Weeks            int            `json:"weeks"`
	Restrictions     []string       `json:"restrictions"`
	Phases           []PlanPhase    `json:"phases"`
	ProgressionRules []string       `json:"progression_rules"`
	Test             string         `json:"test"`
	Sessions         []PlanSession  `json:"sessions"`
	Research         *SkillResearch `json:"research,omitempty"`
}

type skillPlanRequest struct {
	Skill       string `json:"skill"`
	Weeks       int    `json:"weeks"`
	DaysPerWeek int    `json:"days_per_week"`
	StartsOn    string `json:"starts_on"`
	Notes       string `json:"notes"`
	Save        bool   `json:"save"`
	// NoResearch skips the web-search pass. The plan is written from the
	// snapshot and the library alone, which is faster and cheaper.
	NoResearch bool `json:"no_research"`
}

type planResponse struct {
	PlanID   string   `json:"plan_id,omitempty"`
	Plan     Plan     `json:"plan"`
	Saved    bool     `json:"saved"`
	Warnings []string `json:"warnings,omitempty"`
}

// How long a coaching call may run. Both are far past the usual case; they
// exist so a stuck request eventually ends rather than to pace anything.
const (
	planBudget  = 12 * time.Minute
	proseBudget = 4 * time.Minute
)

// planTokens sizes the ceiling to the plan being asked for. It has to cover
// the model's reasoning as well as the plan itself: they come out of the same
// budget, and a ceiling that only fits the answer gets spent on the thinking
// and returns nothing at all. A session now carries intensity, tempo and its
// own progression rule, so it costs more to write than it used to.
func planTokens(sessions int) int {
	tokens := 8000 + sessions*650
	if tokens > 64000 {
		tokens = 64000
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
	// The plan is written on this athlete's own model account, so the account
	// has to exist before anything expensive starts.
	client, err := h.store.ClientFor(r.Context(), me.ID)
	if err != nil {
		FailNotConnected(w, err)
		return
	}

	// Everything past here can take minutes, so the response either streams
	// progress or waits; both need longer than the server's write timeout.
	out := begin(w, r, planBudget)
	defer out.close()
	ctx, cancel := context.WithTimeout(r.Context(), planBudget)
	defer cancel()

	out.report(Progress{Stage: "reading", Label: "Reading your training history", Percent: 2})
	promptContext, lib, err := h.buildContext(ctx, me)
	if err != nil {
		out.fail(http.StatusInternalServerError, "Couldn't read your training history.")
		return
	}

	// Research first: what the ladder to this skill looks like, what each rung
	// is held to, and what tends to go wrong. It is web search, so it cannot
	// report a fraction of itself; the bar sweeps and the elapsed time ticks.
	var found SkillResearch
	if !in.NoResearch {
		found = h.researchWhileTicking(ctx, client, me.ID, in.Skill, lib, out)
	}

	expected := in.Weeks * in.DaysPerWeek
	prompt := planPrompt(promptContext, found, in, expected)

	tracker := newPlanTracker(expected)
	out.report(Progress{Stage: "sending", Label: "Briefing the coach", Percent: researchCeiling, Total: expected})

	var plan Plan
	err = client.CompleteJSONStream(ctx, me.ID, "skill_plan", coachSystem, prompt,
		planTokens(expected), func(d Delta) { out.report(tracker.update(d)) }, &plan)
	if err != nil {
		out.fail(http.StatusBadGateway, "Couldn't build that plan: "+err.Error())
		return
	}

	warnings := validatePlan(&plan, lib, in.Weeks)
	if len(plan.Sessions) == 0 {
		out.fail(http.StatusBadGateway, "The plan came back without a single usable session. Try again.")
		return
	}
	if plan.Title == "" {
		plan.Title = in.Skill
	}
	plan.Weeks = in.Weeks
	if !found.Empty() {
		plan.Research = &found
	}

	result := planResponse{Plan: plan, Warnings: warnings}
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

// researchWhileTicking runs the research turn, keeping the progress line alive
// while it does. The search call is one long request that reports nothing
// until it returns, so the only honest thing to show is how long it has been
// running — the percentage stays put and the bar sweeps.
func (h *Handler) researchWhileTicking(ctx context.Context, client *Client, userID, skill string,
	lib library, out *responder) SkillResearch {

	out.report(Progress{Stage: "researching", Percent: researchFloor, Indeterminate: true,
		Label: "Researching how " + skill + " is trained", Detail: "searching coaching sources"})

	done := make(chan SkillResearch, 1)
	go func() { done <- h.research(ctx, client, userID, skill, lib) }()

	started := time.Now()
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case found := <-done:
			label := "Wrote the plan from the library alone"
			detail := "the research turn came back empty"
			if !found.Empty() {
				label = "Read " + plural(len(found.Sources), "coaching source")
				detail = researchDetail(found)
			}
			out.report(Progress{Stage: "researched", Percent: researchCeiling,
				Label: label, Detail: detail})
			return found
		case <-ticker.C:
			out.report(Progress{Stage: "researching", Percent: researchFloor, Indeterminate: true,
				Label:  "Researching how " + skill + " is trained",
				Detail: "reading coaching sources · " + elapsed(started)})
		case <-ctx.Done():
			return SkillResearch{}
		}
	}
}

func researchDetail(found SkillResearch) string {
	parts := []string{plural(len(found.Progression), "rung") + " on the ladder"}
	if n := len(found.KeyDrills) + len(found.Accessories); n > 0 {
		parts = append(parts, plural(n, "drill")+" mapped to the library")
	}
	if found.Cached {
		parts = append(parts, "from cache")
	}
	return strings.Join(parts, ", ")
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func elapsed(since time.Time) string {
	d := time.Since(since).Round(time.Second)
	return fmt.Sprintf("%d:%02d elapsed", int(d.Minutes()), int(d.Seconds())%60)
}

// planPrompt assembles the task. The JSON shape is spelled out in full because
// every field it names is a field the model would otherwise leave out.
func planPrompt(promptContext string, found SkillResearch, in skillPlanRequest, expected int) string {
	deload := "No deload is needed at this length."
	if in.Weeks >= 6 {
		deload = fmt.Sprintf("Place one lighter week inside the %d, and mark its sessions with load \"deload\".", in.Weeks)
	}

	return fmt.Sprintf(`%s

%s
TASK
Write a %d-week plan to reach: %s
Training days per week: %d (%d sessions in total)
Athlete's extra notes: %s
%s

Return JSON in exactly this shape:
{
  "title": "string",
  "summary": "3-5 sentences: the approach, why it suits this athlete's records, and anything removed for injuries",
  "weeks": %d,
  "restrictions": ["what you avoided and why"],
  "phases": [
    {"weeks": "1-3", "name": "string", "aim": "what this block is for"}
  ],
  "progression_rules": ["how load moves week to week, stated so the athlete can apply it without asking"],
  "test": "how the athlete tests the goal in the final week, stated as pass or fail",
  "sessions": [
    {
      "week": 1,
      "day_of_week": 1,
      "title": "string",
      "focus": "one line on what this session is for",
      "load": "one of: hard, moderate, easy, deload",
      "duration_minutes": 60,
      "warmup_protocols": ["protocol_slug"],
      "blocks": [
        {"exercise_slug": "slug_from_library",
         "intent": "one of: prep, skill, strength, accessory, conditioning",
         "sets": 4,
         "prescription": "e.g. 6-8 reps, or 4x12s hold",
         "intensity": "e.g. 2 reps in reserve, or 60%% of best hold",
         "tempo": "e.g. 3s down, 1s pause",
         "rest_seconds": 120,
         "progression": "what changes next week",
         "notes": "cue, regression if it fails, or what to do if it is easy"}
      ],
      "cooldown": "one line, optional"
    }
  ]
}

day_of_week is 1 for Monday through 7 for Sunday. Order the blocks inside each session the way
they are to be performed. Write every session for all %d weeks: %d sessions, none summarised,
none written as "repeat week 2".`,
		promptContext, found.brief(), in.Weeks, in.Skill, in.DaysPerWeek, expected,
		orNone(in.Notes), deload, in.Weeks, in.Weeks, expected)
}

// ---------- validation ----------

// validatePlan holds the model to the library it was given. A prescription
// naming an exercise that does not exist is not a small error: the athlete
// cannot log it, the level calculation never sees it, and the block is silent
// about what it wanted. Those blocks are dropped and the drop is reported,
// which is a smaller lie than showing a slug that resolves to nothing.
func validatePlan(plan *Plan, lib library, weeks int) []string {
	var warnings []string
	seen := map[string]bool{}
	warn := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		if !seen[msg] {
			seen[msg] = true
			warnings = append(warnings, msg)
		}
	}

	kept := plan.Sessions[:0]
	for _, session := range plan.Sessions {
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

		session.WarmupProtocols = lib.keepProtocols(session.WarmupProtocols, warn)

		blocks := session.Blocks[:0]
		for _, block := range session.Blocks {
			slug := strings.TrimSpace(block.ExerciseSlug)
			if !lib.has(slug) {
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
	plan.Sessions = kept

	// Calendar order, so week 3 Tuesday never renders before week 2 Friday.
	sort.SliceStable(plan.Sessions, func(i, j int) bool {
		if plan.Sessions[i].Week != plan.Sessions[j].Week {
			return plan.Sessions[i].Week < plan.Sessions[j].Week
		}
		return plan.Sessions[i].DayOfWeek < plan.Sessions[j].DayOfWeek
	})
	return warnings
}

// ---------- saving ----------

type savePlanRequest struct {
	Plan     Plan   `json:"plan"`
	Goal     string `json:"goal"`
	StartsOn string `json:"starts_on"`
}

// SavePlan puts an already-generated plan on the athlete's calendar. Generation
// and saving are separate so a plan can be read before it is committed to,
// which is the difference between a suggestion and a schedule.
func (h *Handler) SavePlan(w http.ResponseWriter, r *http.Request) {
	var in savePlanRequest
	if !httpx.Decode(w, r, &in) {
		return
	}
	if len(in.Plan.Sessions) == 0 {
		httpx.Fail(w, http.StatusBadRequest, "That plan has no sessions to schedule.")
		return
	}
	if len(in.Plan.Sessions) > 400 {
		httpx.Fail(w, http.StatusBadRequest, "That plan has more sessions than a calendar can take.")
		return
	}
	weeks := in.Plan.Weeks
	if weeks < 1 || weeks > 52 {
		httpx.Fail(w, http.StatusBadRequest, "A plan has to run between 1 and 52 weeks.")
		return
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
	lib, err := h.loadLibrary(r.Context())
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't read the exercise library.")
		return
	}

	// The plan arrives from the browser, so it is checked against the library
	// again rather than trusted for having been ours a minute ago.
	warnings := validatePlan(&in.Plan, lib, weeks)
	if len(in.Plan.Sessions) == 0 {
		httpx.Fail(w, http.StatusBadRequest, "None of those sessions could be scheduled.")
		return
	}
	if in.Plan.Title == "" {
		in.Plan.Title = orNone(in.Goal)
	}

	id, err := h.savePlan(r.Context(), me.ID, in.Plan, in.Goal, startsOn)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't add that plan to your calendar.")
		return
	}
	httpx.JSON(w, http.StatusOK, planResponse{PlanID: id, Plan: in.Plan, Saved: true, Warnings: warnings})
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
	client, err := h.store.ClientFor(r.Context(), me.ID)
	if err != nil {
		FailNotConnected(w, err)
		return
	}

	out := begin(w, r, proseBudget)
	defer out.close()
	ctx, cancel := context.WithTimeout(r.Context(), proseBudget)
	defer cancel()

	out.report(Progress{Stage: "reading", Label: "Reading your last four weeks", Percent: 3})
	ctxText, _, err := h.buildContext(ctx, me)
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
	text, err := client.CompleteStream(ctx, me.ID, "review", coachSystem, prompt, proseTokens,
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
	client, err := h.store.ClientFor(r.Context(), me.ID)
	if err != nil {
		FailNotConnected(w, err)
		return
	}

	out := begin(w, r, proseBudget)
	defer out.close()
	ctx, cancel := context.WithTimeout(r.Context(), proseBudget)
	defer cancel()

	out.report(Progress{Stage: "reading", Label: "Reading your recent load", Percent: 3})
	ctxText, _, err := h.buildContext(ctx, me)
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
	text, err := client.CompleteStream(ctx, me.ID, "recovery", coachSystem, prompt, proseTokens,
		func(d Delta) { out.report(tracker.update(d)) })
	if err != nil {
		out.fail(http.StatusBadGateway, "Couldn't produce recovery guidance: "+err.Error())
		return
	}
	out.report(Progress{Stage: "done", Label: "Guidance ready", Percent: 100})
	out.done(textResponse{Text: text})
}

// ---------- prompt context ----------

// library is what the model is allowed to prescribe from, and the thing every
// answer is checked back against.
type library struct {
	text      string
	exercises map[string]training.Exercise
	protocols map[string]bool
}

func (l library) has(slug string) bool {
	_, ok := l.exercises[strings.TrimSpace(slug)]
	return ok
}

// keep filters a slug list down to what exists, quietly. Used for research
// findings, where a dropped slug costs a line of prose rather than a set.
func (l library) keep(slugs []string) []string {
	kept := []string{}
	for _, slug := range slugs {
		if l.has(slug) {
			kept = append(kept, strings.TrimSpace(slug))
		}
	}
	return kept
}

func (l library) keepProtocols(slugs []string, warn func(string, ...any)) []string {
	kept := []string{}
	for _, slug := range slugs {
		slug = strings.TrimSpace(slug)
		if l.protocols[slug] {
			kept = append(kept, slug)
			continue
		}
		warn("The warm-up protocol %q does not exist, so it was removed.", slug)
	}
	return kept
}

// loadLibrary reads the exercise table and the protocol list into both the
// text the model is given and the sets the answer is checked against.
func (h *Handler) loadLibrary(ctx context.Context) (library, error) {
	lib := library{
		exercises: map[string]training.Exercise{},
		protocols: map[string]bool{},
	}

	rows, err := h.pool.Query(ctx, `
		select slug, name, category, measure, difficulty, description
		from exercises order by category, difficulty, slug`)
	if err != nil {
		return lib, err
	}
	defer rows.Close()

	var b strings.Builder
	b.WriteString("EXERCISE LIBRARY (the only exercises you may prescribe)\n")
	for rows.Next() {
		var e training.Exercise
		if err := rows.Scan(&e.Slug, &e.Name, &e.Category, &e.Measure, &e.Difficulty, &e.Description); err != nil {
			return lib, err
		}
		lib.exercises[e.Slug] = e
		// The description is what tells the model that a planche lean scales
		// by lean angle and a negative pull-up is a five-second lower. Without
		// it, the slug is a name and the prescription is a guess.
		fmt.Fprintf(&b, "- %s | %s | %s | measured in %s | difficulty %d/10 | %s\n",
			e.Slug, e.Name, e.Category, e.Measure, e.Difficulty, e.Description)
	}
	if err := rows.Err(); err != nil {
		return lib, err
	}

	b.WriteString("\nPROTOCOL LIBRARY (the only warm-up and rehab protocols you may reference)\n")
	for _, p := range training.Protocols {
		lib.protocols[p.Slug] = true
		fmt.Fprintf(&b, "- %s | %s | %s | for %s\n", p.Slug, p.Title, p.Purpose, p.Region)
	}

	lib.text = b.String()
	return lib, nil
}

// buildContext assembles everything the model is allowed to reason from: the
// athlete's computed snapshot plus the two libraries it must choose within.
func (h *Handler) buildContext(ctx context.Context, user auth.User) (string, library, error) {
	snapshot, err := h.training.BuildSnapshot(ctx, user)
	if err != nil {
		return "", library{}, err
	}
	snapshotJSON, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return "", library{}, err
	}

	lib, err := h.loadLibrary(ctx)
	if err != nil {
		return "", library{}, err
	}

	return fmt.Sprintf(`ATHLETE SNAPSHOT (computed from their log, treat as fact)
%s

%s`, snapshotJSON, lib.text), lib, nil
}

func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "none"
	}
	return s
}
