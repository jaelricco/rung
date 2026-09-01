package plan

import (
	"net/http"
	"time"

	"calisthenics/api/internal/auth"
	"calisthenics/api/internal/httpx"
	"calisthenics/api/internal/training"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The deterministic endpoint. It needs no model account, no network beyond the
// database, and no budget, and it answers in milliseconds — which is why it is
// what the app reaches for first and what everything else falls back to.
type Handler struct {
	pool     *pgxpool.Pool
	training *training.Service
}

func NewHandler(pool *pgxpool.Pool, tr *training.Service) *Handler {
	return &Handler{pool: pool, training: tr}
}

type generateRequest struct {
	// Goal and Skill are the same field under two names: the AI endpoint has
	// always called it "skill", and there is no reason to make the browser
	// send a different body to the two.
	Goal        string `json:"goal"`
	Skill       string `json:"skill"`
	Weeks       int    `json:"weeks"`
	DaysPerWeek int    `json:"days_per_week"`
	StartsOn    string `json:"starts_on"`
	Notes       string `json:"notes"`
	Save        bool   `json:"save"`
}

func (in generateRequest) goal() string {
	if in.Goal != "" {
		return in.Goal
	}
	return in.Skill
}

// Response is what both plan endpoints answer with. Source says which producer
// wrote it, so the browser can be honest about what the athlete is reading.
type Response struct {
	PlanID   string   `json:"plan_id,omitempty"`
	Plan     Plan     `json:"plan"`
	Source   string   `json:"source"`
	Saved    bool     `json:"saved"`
	Warnings []string `json:"warnings,omitempty"`
}

// Generate writes a plan from the athlete's records alone.
//
// There is no failure path here that ends without a plan. A goal nobody
// recognises, an empty log, an injury that removes half the library — each of
// those changes what comes back, and none of them turns into an error. The
// only 5xx this can answer with is a database that would not tell us who the
// athlete is.
func (h *Handler) Generate(w http.ResponseWriter, r *http.Request) {
	var in generateRequest
	if !httpx.Decode(w, r, &in) {
		return
	}
	startsOn, ok := parseStart(w, in.StartsOn)
	if !ok {
		return
	}

	me := auth.MustUser(r.Context())
	snapshot, err := h.training.BuildSnapshot(r.Context(), me)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't read your training history.")
		return
	}
	lib, err := LoadLibrary(r.Context(), h.pool)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't read the exercise library.")
		return
	}

	built, warnings := Generate(Request{
		Goal: in.goal(), Weeks: in.Weeks, DaysPerWeek: in.DaysPerWeek, Notes: in.Notes,
	}, snapshot, lib)

	out := Response{Plan: built, Source: SourceAlgorithm, Warnings: warnings}
	if in.Save && len(built.Sessions) > 0 {
		id, err := Save(r.Context(), h.pool, me.ID, built, in.goal(), startsOn)
		if err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "The plan was built but couldn't be saved.")
			return
		}
		out.PlanID, out.Saved = id, true
	}
	httpx.JSON(w, http.StatusOK, out)
}

type saveRequest struct {
	Plan     Plan   `json:"plan"`
	Goal     string `json:"goal"`
	StartsOn string `json:"starts_on"`
}

// SavePlan puts an already-generated plan on the calendar. Generating and
// committing stay separate whoever wrote the plan: eight weeks of appointments
// is worth reading first.
func (h *Handler) SavePlan(w http.ResponseWriter, r *http.Request) {
	var in saveRequest
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
	if in.Plan.Weeks < 1 || in.Plan.Weeks > 52 {
		httpx.Fail(w, http.StatusBadRequest, "A plan has to run between 1 and 52 weeks.")
		return
	}
	startsOn, ok := parseStart(w, in.StartsOn)
	if !ok {
		return
	}

	me := auth.MustUser(r.Context())
	lib, err := LoadLibrary(r.Context(), h.pool)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't read the exercise library.")
		return
	}

	// The plan arrives from the browser, so it is checked against the library
	// again rather than trusted for having been ours a minute ago.
	warnings := Validate(&in.Plan, lib, in.Plan.Weeks)
	if len(in.Plan.Sessions) == 0 {
		httpx.Fail(w, http.StatusBadRequest, "None of those sessions could be scheduled.")
		return
	}
	if in.Plan.Title == "" {
		in.Plan.Title = in.Goal
	}

	id, err := Save(r.Context(), h.pool, me.ID, in.Plan, in.Goal, startsOn)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't add that plan to your calendar.")
		return
	}

	source := SourceAlgorithm
	if in.Plan.Method != nil && in.Plan.Method.Source != "" {
		source = in.Plan.Method.Source
	}
	httpx.JSON(w, http.StatusOK, Response{
		PlanID: id, Plan: in.Plan, Source: source, Saved: true, Warnings: warnings,
	})
}

func parseStart(w http.ResponseWriter, value string) (time.Time, bool) {
	if value == "" {
		return time.Now(), true
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		httpx.Fail(w, http.StatusBadRequest, "Start date must look like 2026-09-01.")
		return time.Time{}, false
	}
	return parsed, true
}
