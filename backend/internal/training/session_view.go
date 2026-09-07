package training

import (
	"encoding/json"
	"net/http"

	"calisthenics/api/internal/auth"
	"calisthenics/api/internal/httpx"
)

// SessionView is one session with everything needed to perform it, in one
// request, because the page that reads this is opened in its own tab and
// standing in a gym: a second round trip is a second chance to be looking at a
// spinner instead of the set you are about to do.
type SessionView struct {
	Session CalendarEntry `json:"session"`
	// Phases is the running order, served rather than hard-coded in the page,
	// so the order a session is performed in has one definition.
	Phases    any                 `json:"phases"`
	Protocols []Protocol          `json:"protocols"`
	Exercises map[string]Exercise `json:"exercises"`
}

// Session answers with one planned session by id. It is what the calendar
// links to: the warm-up resolved into its actual steps rather than a line of
// slugs, and every movement named rather than left as the slug it is stored
// under.
func (s *Service) Session(w http.ResponseWriter, r *http.Request) {
	me := auth.MustUser(r.Context())
	id := r.PathValue("id")

	var entry CalendarEntry
	err := s.pool.QueryRow(r.Context(), `
		select id, plan_id, routine_id, source, to_char(scheduled_on, 'YYYY-MM-DD'),
		       title, focus, body, completed_at, workout_id
		from planned_sessions
		where id = $1 and user_id = $2`, id, me.ID,
	).Scan(&entry.ID, &entry.PlanID, &entry.RoutineID, &entry.Source, &entry.ScheduledOn,
		&entry.Title, &entry.Focus, &entry.Body, &entry.CompletedAt, &entry.WorkoutID)
	if err != nil {
		// Not found and not yours are the same answer on purpose: whether a
		// session id exists is not something to confirm to someone else.
		httpx.Fail(w, http.StatusNotFound, "That session isn't there.")
		return
	}

	var body SessionBody
	if err := json.Unmarshal(entry.Body, &body); err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "That session couldn't be read.")
		return
	}

	out := SessionView{
		Session:   entry,
		Phases:    SessionPhases,
		Protocols: []Protocol{},
		Exercises: map[string]Exercise{},
	}

	// The protocols this session names, in the catalogue's own order so the
	// warm-up reads the same way twice.
	wanted := map[string]bool{}
	for _, slug := range body.WarmupProtocols {
		wanted[slug] = true
	}
	for _, p := range Protocols {
		if wanted[p.Slug] {
			out.Protocols = append(out.Protocols, p)
		}
	}

	slugs := make([]string, 0, len(body.Blocks))
	for _, b := range body.Blocks {
		if b.ExerciseSlug != "" {
			slugs = append(slugs, b.ExerciseSlug)
		}
	}
	if len(slugs) > 0 {
		rows, err := s.pool.Query(r.Context(), `
			select slug, name, category, measure, difficulty, description
			from exercises where slug = any($1)`, slugs)
		if err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Couldn't read the exercise library.")
			return
		}
		defer rows.Close()
		for rows.Next() {
			var e Exercise
			if err := rows.Scan(&e.Slug, &e.Name, &e.Category, &e.Measure, &e.Difficulty, &e.Description); err != nil {
				httpx.Fail(w, http.StatusInternalServerError, "Couldn't read the exercise library.")
				return
			}
			out.Exercises[e.Slug] = e
		}
	}

	httpx.JSON(w, http.StatusOK, out)
}
