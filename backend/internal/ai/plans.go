package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"calisthenics/api/internal/auth"
	"calisthenics/api/internal/httpx"
	"calisthenics/api/internal/plan"
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

// The plan schema lives in internal/plan, which owns it: the algorithm and the
// model are two producers of the same object, checked by the same validator
// and saved by the same writer. These aliases keep the names this package has
// always used for them.
type (
	Plan        = plan.Plan
	PlanSession = plan.Session
	PlanBlock   = plan.Block
	PlanPhase   = plan.Phase
)

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

type planResponse = plan.Response

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

// SkillPlan answers with the best plan it can get, and always with a plan.
//
// The order is the whole design. The algorithm in internal/plan runs first and
// produces a complete, checked, athlete-specific plan in milliseconds. Only
// then is the model asked to improve on it — and every way that can fail (no
// account connected, a provider that is down, a budget spent, a timeout, JSON
// that does not survive the library check) ends in the same place: the plan
// the algorithm already wrote, delivered with a line saying why the model did
// not get to touch it.
//
// So this endpoint has no 428 and no 502 any more. Not being able to reach a
// model is a reason for a less specific plan, never a reason for no plan.
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

	// Everything past here can take minutes, so the response either streams
	// progress or waits; both need longer than the server's write timeout.
	out := begin(w, r, planBudget)
	defer out.close()
	ctx, cancel := context.WithTimeout(r.Context(), planBudget)
	defer cancel()

	out.report(Progress{Stage: "reading", Label: "Reading your training history", Percent: 2})
	snapshot, lib, promptContext, err := h.buildContext(ctx, me)
	if err != nil {
		// The one thing there is no working around: without the athlete's log
		// there is nothing for either producer to write from.
		out.fail(http.StatusInternalServerError, "Couldn't read your training history.")
		return
	}

	// The algorithm always goes first, whatever happens next.
	out.report(Progress{Stage: "planning", Label: "Building your plan from your records", Percent: 4})
	request := plan.Request{Goal: in.Skill, Weeks: in.Weeks, DaysPerWeek: in.DaysPerWeek, Notes: in.Notes}
	baseline, baseWarnings := plan.Generate(request, snapshot, lib)

	deliver := func(final Plan, source string, warnings []string) {
		if source != plan.SourceAI && final.Method != nil {
			out.report(Progress{Stage: "fallback", Label: "Using the plan built from your records",
				Detail: final.Method.FallbackReason, Percent: 90})
		}
		h.finishPlan(ctx, out, me.ID, final, in, source, startsOn, warnings)
	}

	// From here on, anything that goes wrong hands back the baseline.
	client, err := h.store.ClientFor(ctx, me.ID)
	if err != nil {
		deliver(fallback(baseline, notConnectedReason(err)), plan.SourceFallback, baseWarnings)
		return
	}

	// Research: what the ladder to this skill looks like, what each rung is
	// held to, and what tends to go wrong. It is web search, so it cannot
	// report a fraction of itself; the bar sweeps and the elapsed time ticks.
	var found SkillResearch
	if !in.NoResearch {
		found = h.researchWhileTicking(ctx, client, me.ID, in.Skill, lib, out)
	}

	expected := in.Weeks * in.DaysPerWeek
	prompt := planPrompt(promptContext, found, baseline, in, expected)

	tracker := newPlanTracker(expected)
	out.report(Progress{Stage: "sending", Label: "Asking the coach to sharpen it",
		Percent: researchCeiling, Total: expected})

	var refined Plan
	err = client.CompleteJSONStream(ctx, me.ID, "skill_plan", coachSystem, prompt,
		planTokens(expected), func(d Delta) { out.report(tracker.update(d)) }, &refined)
	if err != nil {
		deliver(fallback(baseline, "The model could not be reached: "+err.Error()), plan.SourceFallback, baseWarnings)
		return
	}

	warnings := plan.Validate(&refined, lib, in.Weeks)
	if len(refined.Sessions) == 0 {
		deliver(fallback(baseline, "The model's answer had no session in it that could be trained."),
			plan.SourceFallback, baseWarnings)
		return
	}

	// The model wrote the sessions; the placement, the ladder and the
	// restrictions stay the algorithm's, because those are computed from the
	// log and are not the model's to revise.
	if refined.Title == "" {
		refined.Title = baseline.Title
	}
	if refined.Test == "" {
		refined.Test = baseline.Test
	}
	refined.Weeks = in.Weeks
	refined.Notes = baseline.Notes
	refined.Method = methodFrom(baseline, plan.SourceAI)
	refined.Research = nil
	if !found.Empty() {
		if encoded, err := json.Marshal(found); err == nil {
			refined.Research = encoded
		}
	}
	deliver(refined, plan.SourceAI, warnings)
}

// fallback stamps the algorithm's plan with why the model did not improve it.
// The athlete is told, in the plan itself, rather than being left to wonder
// why it reads differently from last time.
func fallback(baseline Plan, reason string) Plan {
	baseline.Method = methodFrom(baseline, plan.SourceFallback)
	baseline.Method.FallbackReason = reason
	baseline.Notes = append([]string{
		"This plan was written by the app's own planner rather than by a model. " + reason +
			" Everything in it comes from your logged records, and it is ready to train.",
	}, baseline.Notes...)
	return baseline
}

func methodFrom(baseline Plan, source string) *plan.Method {
	if baseline.Method == nil {
		return &plan.Method{Source: source}
	}
	method := *baseline.Method
	method.Source = source
	return &method
}

// notConnectedReason turns a missing model account into a sentence worth
// reading. It is not an error here: most athletes never connect one.
func notConnectedReason(err error) string {
	switch {
	case errors.Is(err, ErrNoCredentials):
		return "No AI account is connected, so nothing was sent to a model. Connect one under Settings if you want a model to refine this."
	case errors.Is(err, ErrNoKeystore):
		return "The server cannot open stored API keys at the moment, so nothing was sent to a model."
	default:
		return "Your stored API key could not be read: " + err.Error()
	}
}

// finishPlan saves the plan if asked and answers. Both producers end here, so
// a plan lands on the calendar the same way whoever wrote it.
func (h *Handler) finishPlan(ctx context.Context, out *responder, userID string, final Plan,
	in skillPlanRequest, source string, startsOn time.Time, warnings []string) {

	result := planResponse{Plan: final, Source: source, Warnings: warnings}
	if in.Save && len(final.Sessions) > 0 {
		out.report(Progress{Stage: "saving", Label: "Adding the sessions to your calendar", Percent: 95})
		// A model call that ran out of budget takes the request context down
		// with it, and the plan we are falling back to still has to reach the
		// calendar. The write gets its own short deadline rather than
		// inheriting a dead one.
		saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()

		id, err := plan.Save(saveCtx, h.pool, userID, final, in.Skill, startsOn)
		if err != nil {
			out.fail(http.StatusInternalServerError, "The plan was built but couldn't be saved.")
			return
		}
		result.PlanID, result.Saved = id, true
	}
	out.report(Progress{Stage: "done", Label: "Plan ready", Percent: 100,
		Done: len(final.Sessions), Total: len(final.Sessions)})
	out.done(result)
}

func (h *Handler) researchWhileTicking(ctx context.Context, client *Client, userID, skill string,
	lib plan.Library, out *responder) SkillResearch {

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
// every field it names is a field the model would otherwise leave out.// planPrompt assembles the task. The model is not asked to write a plan from
// nothing: it is given the one the algorithm already produced and asked to
// improve it. That is worth doing for two reasons. It anchors the answer to
// the athlete's real placement and real numbers, so a model having an
// imaginative day cannot invent a level; and it turns the model's job from
// "produce forty sessions" into "make these forty better", which is the job it
// is actually good at.
func planPrompt(promptContext string, found SkillResearch, baseline Plan, in skillPlanRequest, expected int) string {
	deload := "No deload is needed at this length."
	if in.Weeks >= 6 {
		deload = fmt.Sprintf("Place one lighter week inside the %d, and mark its sessions with load \"deload\".", in.Weeks)
	}

	return fmt.Sprintf(`%s

%s
%s
TASK
Write a %d-week plan to reach: %s
Training days per week: %d (%d sessions in total)
Athlete's extra notes: %s
%s

The baseline above was computed from this athlete's log by the app's own planner. It is correct
but generic. Improve on it rather than replacing it:
- Keep the rung it placed the athlete on and the standards it named. They come from the records
  and are not yours to revise.
- Keep every restriction it lists. Those are open injuries.
- Sharpen what it cannot do: movement selection for this athlete's history, the cue that fixes
  the position they will actually lose, how the weeks differ from one another, and what to do
  when a session goes badly.
- If the baseline is already the right call for a block, keep it. Changing it to look different
  is not an improvement.

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
		promptContext, found.brief(), baselineBrief(baseline), in.Weeks, in.Skill, in.DaysPerWeek, expected,
		orNone(in.Notes), deload, in.Weeks, in.Weeks, expected)
}

// baselineBrief renders the algorithm's plan compactly: everything at plan
// level, plus the first week in full so the model can see the shape and the
// numbers it is being asked to improve. Sending all forty sessions would cost
// more context than it is worth — the first week is the pattern the rest of
// them repeat.
func baselineBrief(baseline Plan) string {
	if len(baseline.Sessions) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("BASELINE PLAN (computed from the athlete's log; improve on it)\n")
	fmt.Fprintf(&b, "Title: %s\nSummary: %s\nTest: %s\n", baseline.Title, baseline.Summary, baseline.Test)

	if baseline.Method != nil {
		fmt.Fprintf(&b, "Placed on: %s", baseline.Method.Rung)
		if baseline.Method.NextRung != "" {
			fmt.Fprintf(&b, " (next rung: %s)", baseline.Method.NextRung)
		}
		b.WriteString("\nLadder: ")
		for i, rung := range baseline.Method.Ladder {
			mark := " "
			if rung.Cleared {
				mark = "x"
			}
			if rung.Current {
				mark = ">"
			}
			if i > 0 {
				b.WriteString(" | ")
			}
			fmt.Fprintf(&b, "[%s] %s to %s", mark, rung.Name, rung.Standard)
		}
		b.WriteString("\n")
	}
	for _, phase := range baseline.Phases {
		fmt.Fprintf(&b, "Phase %s: %s — %s\n", phase.Weeks, phase.Name, phase.Aim)
	}
	for _, rule := range baseline.ProgressionRules {
		fmt.Fprintf(&b, "Rule: %s\n", rule)
	}
	for _, restriction := range baseline.Restrictions {
		fmt.Fprintf(&b, "Restriction: %s\n", restriction)
	}

	b.WriteString("\nWeek 1 as the planner wrote it (the other weeks follow the same shape):\n")
	for _, session := range baseline.Sessions {
		if session.Week != 1 {
			continue
		}
		fmt.Fprintf(&b, "- day %d, %s, load %s, %d min, warm-up %s\n",
			session.DayOfWeek, session.Title, session.Load, session.DurationMinutes,
			strings.Join(session.WarmupProtocols, "+"))
		for _, block := range session.Blocks {
			fmt.Fprintf(&b, "    %s | %s | %s | %s | rest %ds | next: %s\n",
				block.Intent, block.ExerciseSlug, block.Prescription, block.Intensity,
				block.RestSeconds, block.Progression)
		}
	}
	b.WriteString("\n")
	return b.String()
}

// ---------- prompt context ----------

// buildContext assembles everything a coaching call may reason from: the
// athlete's computed snapshot and the two libraries it must choose within. The
// snapshot comes back as well as its rendering, because the algorithm reasons
// from the struct and the model from the text.
func (h *Handler) buildContext(ctx context.Context, user auth.User) (training.Snapshot, plan.Library, string, error) {
	snapshot, err := h.training.BuildSnapshot(ctx, user)
	if err != nil {
		return snapshot, plan.Library{}, "", err
	}
	snapshotJSON, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return snapshot, plan.Library{}, "", err
	}

	lib, err := plan.LoadLibrary(ctx, h.pool)
	if err != nil {
		return snapshot, lib, "", err
	}

	return snapshot, lib, fmt.Sprintf(`ATHLETE SNAPSHOT (computed from their log, treat as fact)
%s

%s`, snapshotJSON, lib.Text()), nil
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
	_, _, ctxText, err := h.buildContext(ctx, me)
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
	_, _, ctxText, err := h.buildContext(ctx, me)
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
func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "none"
	}
	return s
}
