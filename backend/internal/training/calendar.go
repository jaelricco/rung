package training

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"calisthenics/api/internal/auth"
	"calisthenics/api/internal/httpx"
)

type CalendarEntry struct {
	ID          string          `json:"id"`
	PlanID      *string         `json:"plan_id"`
	RoutineID   *string         `json:"routine_id"`
	// Source is plan, routine or manual: where this session came from, which
	// is what decides whether editing it is editing one day or a template.
	Source      string          `json:"source"`
	ScheduledOn string          `json:"scheduled_on"`
	Title       string          `json:"title"`
	Focus       string          `json:"focus"`
	Body        json.RawMessage `json:"body"`
	CompletedAt *time.Time      `json:"completed_at"`
	WorkoutID   *string         `json:"workout_id"`
}

type CalendarEvent struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Discipline string  `json:"discipline"`
	StartsOn   string  `json:"starts_on"`
	EndsOn     *string `json:"ends_on"`
	City       string  `json:"city"`
	Country    string  `json:"country"`
	URL        string  `json:"url"`
	Goal       string  `json:"goal"`
}

type CalendarResponse struct {
	From     string          `json:"from"`
	To       string          `json:"to"`
	Sessions []CalendarEntry `json:"sessions"`
	Events   []CalendarEvent `json:"events"`
}

// Calendar returns planned sessions and the athlete's registered events in a
// window. Defaults to the next eight weeks.
func (s *Service) Calendar(w http.ResponseWriter, r *http.Request) {
	me := auth.MustUser(r.Context())

	from := time.Now().AddDate(0, 0, -7)
	to := time.Now().AddDate(0, 0, 56)
	if v := r.URL.Query().Get("from"); v != "" {
		parsed, err := time.Parse("2006-01-02", v)
		if err != nil {
			httpx.Fail(w, http.StatusBadRequest, "The from date must look like 2026-09-01.")
			return
		}
		from = parsed
	}
	if v := r.URL.Query().Get("to"); v != "" {
		parsed, err := time.Parse("2006-01-02", v)
		if err != nil {
			httpx.Fail(w, http.StatusBadRequest, "The to date must look like 2026-09-30.")
			return
		}
		to = parsed
	}
	if to.Before(from) {
		httpx.Fail(w, http.StatusBadRequest, "The to date must come after the from date.")
		return
	}

	// Repeating routines are written onto the calendar rather than expanded
	// on the fly, and this is where the writing happens: the window being
	// looked at is filled first, so paging into a future month finds the
	// routine already there. A failure here is not fatal — the calendar
	// still shows everything that is already scheduled.
	if err := s.fillRoutines(r.Context(), me.ID, horizonFor(to)); err != nil {
		log.Printf("fill routines for %s: %v", me.ID, err)
	}

	out := CalendarResponse{
		From:     from.Format("2006-01-02"),
		To:       to.Format("2006-01-02"),
		Sessions: []CalendarEntry{},
		Events:   []CalendarEvent{},
	}

	rows, err := s.pool.Query(r.Context(), `
		select id, plan_id, routine_id, source, to_char(scheduled_on, 'YYYY-MM-DD'),
		       title, focus, body, completed_at, workout_id
		from planned_sessions
		where user_id = $1 and scheduled_on between $2 and $3
		order by scheduled_on`, me.ID, from, to)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't load your calendar.")
		return
	}
	for rows.Next() {
		var e CalendarEntry
		if err := rows.Scan(&e.ID, &e.PlanID, &e.RoutineID, &e.Source, &e.ScheduledOn,
			&e.Title, &e.Focus, &e.Body, &e.CompletedAt, &e.WorkoutID); err != nil {
			rows.Close()
			httpx.Fail(w, http.StatusInternalServerError, "Couldn't read your calendar.")
			return
		}
		out.Sessions = append(out.Sessions, e)
	}
	rows.Close()

	eventRows, err := s.pool.Query(r.Context(), `
		select e.id, e.name, e.discipline, to_char(e.starts_on, 'YYYY-MM-DD'),
		       to_char(e.ends_on, 'YYYY-MM-DD'), e.city, e.country, e.url, ue.goal
		from user_events ue
		join events e on e.id = ue.event_id
		where ue.user_id = $1 and e.starts_on between $2 and $3
		order by e.starts_on`, me.ID, from, to)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't load your events.")
		return
	}
	defer eventRows.Close()
	for eventRows.Next() {
		var e CalendarEvent
		if err := eventRows.Scan(&e.ID, &e.Name, &e.Discipline, &e.StartsOn, &e.EndsOn,
			&e.City, &e.Country, &e.URL, &e.Goal); err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Couldn't read your events.")
			return
		}
		out.Events = append(out.Events, e)
	}

	httpx.JSON(w, http.StatusOK, out)
}

// CompleteSession marks a planned session done, optionally linking the workout
// that was actually logged against it.
func (s *Service) CompleteSession(w http.ResponseWriter, r *http.Request) {
	me := auth.MustUser(r.Context())
	var in struct {
		WorkoutID *string `json:"workout_id"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	tag, err := s.pool.Exec(r.Context(), `
		update planned_sessions
		set completed_at = now(), workout_id = coalesce($3, workout_id)
		where id = $1 and user_id = $2`, r.PathValue("id"), me.ID, in.WorkoutID)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't update that session.")
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Fail(w, http.StatusNotFound, "That planned session doesn't exist.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

// UncompleteSession undoes a tick. A calendar you cannot correct is one people
// stop trusting after the first misclick.
func (s *Service) UncompleteSession(w http.ResponseWriter, r *http.Request) {
	me := auth.MustUser(r.Context())
	tag, err := s.pool.Exec(r.Context(), `
		update planned_sessions set completed_at = null, workout_id = null
		where id = $1 and user_id = $2`, r.PathValue("id"), me.ID)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't update that session.")
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Fail(w, http.StatusNotFound, "That planned session doesn't exist.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "scheduled"})
}

// PlanSummary is a saved plan as the calendar lists it: enough to recognise it
// and to see how far through it the athlete is.
type PlanSummary struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Goal      string    `json:"goal"`
	StartsOn  string    `json:"starts_on"`
	EndsOn    *string   `json:"ends_on"`
	Weeks     int       `json:"weeks"`
	Sessions  int       `json:"sessions"`
	Completed int       `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
}

// ListPlans returns the athlete's saved plans, newest first.
func (s *Service) ListPlans(w http.ResponseWriter, r *http.Request) {
	me := auth.MustUser(r.Context())
	rows, err := s.pool.Query(r.Context(), `
		select p.id, p.title, p.goal, to_char(p.starts_on, 'YYYY-MM-DD'), p.weeks, p.created_at,
		       count(ps.id), count(ps.completed_at), to_char(max(ps.scheduled_on), 'YYYY-MM-DD')
		from plans p
		left join planned_sessions ps on ps.plan_id = p.id
		where p.user_id = $1
		group by p.id
		order by p.created_at desc
		limit 50`, me.ID)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't load your plans.")
		return
	}
	defer rows.Close()

	out := []PlanSummary{}
	for rows.Next() {
		var p PlanSummary
		if err := rows.Scan(&p.ID, &p.Title, &p.Goal, &p.StartsOn, &p.Weeks, &p.CreatedAt,
			&p.Sessions, &p.Completed, &p.EndsOn); err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Couldn't read your plans.")
			return
		}
		out = append(out, p)
	}
	httpx.JSON(w, http.StatusOK, out)
}

// DeletePlan removes a plan and, with it, every session it put on the
// calendar. Sessions already logged as workouts are untouched: the workout is
// what happened, the planned session was only the intention.
func (s *Service) DeletePlan(w http.ResponseWriter, r *http.Request) {
	me := auth.MustUser(r.Context())
	tag, err := s.pool.Exec(r.Context(),
		`delete from plans where id = $1 and user_id = $2`, r.PathValue("id"), me.ID)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't remove that plan.")
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Fail(w, http.StatusNotFound, "That plan doesn't exist.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// CalendarICS exports the calendar so it can be subscribed to from a phone.
func (s *Service) CalendarICS(w http.ResponseWriter, r *http.Request) {
	me := auth.MustUser(r.Context())
	rows, err := s.pool.Query(r.Context(), `
		select id, to_char(scheduled_on, 'YYYYMMDD'), title, focus
		from planned_sessions
		where user_id = $1 and scheduled_on > current_date - interval '90 days'
		order by scheduled_on`, me.ID)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't export your calendar.")
		return
	}
	defer rows.Close()

	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//calisthenics//training//EN\r\n")
	for rows.Next() {
		var id, date, title, focus string
		if err := rows.Scan(&id, &date, &title, &focus); err != nil {
			break
		}
		fmt.Fprintf(&b, "BEGIN:VEVENT\r\nUID:%s\r\nDTSTART;VALUE=DATE:%s\r\nSUMMARY:%s\r\nDESCRIPTION:%s\r\nEND:VEVENT\r\n",
			id, date, icsEscape(title), icsEscape(focus))
	}
	b.WriteString("END:VCALENDAR\r\n")

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="training.ics"`)
	_, _ = w.Write([]byte(b.String()))
}

func icsEscape(s string) string {
	r := strings.NewReplacer("\\", "\\\\", ";", "\\;", ",", "\\,", "\n", "\\n")
	return r.Replace(s)
}

// ---------- sessions written by hand ----------

// A session can also be created straight onto a day, with no plan and no
// routine behind it: the extra session someone decides to do on Saturday, or
// the whole calendar for an athlete who would rather write their own training
// than have it generated.

type sessionInput struct {
	ScheduledOn string      `json:"scheduled_on"`
	Body        SessionBody `json:"body"`
}

type sessionUpdate struct {
	ScheduledOn *string      `json:"scheduled_on"`
	Body        *SessionBody `json:"body"`
}

// CreateSession puts one session on one day.
func (s *Service) CreateSession(w http.ResponseWriter, r *http.Request) {
	var in sessionInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	me := auth.MustUser(r.Context())
	ctx := r.Context()

	date, err := parseDate(in.ScheduledOn)
	if err != nil {
		httpx.Fail(w, http.StatusBadRequest, "The date must look like 2026-09-01.")
		return
	}
	known, err := s.knownSlugs(ctx)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't read the exercise library.")
		return
	}
	if err := in.Body.validate(known, "That session"); err != nil {
		httpx.Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	body, err := in.Body.marshal()
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't save that session.")
		return
	}

	var id string
	err = s.pool.QueryRow(ctx, `
		insert into planned_sessions (user_id, scheduled_on, title, focus, body, source)
		values ($1, $2, $3, $4, $5, 'manual') returning id`,
		me.ID, date, in.Body.Title, in.Body.Focus, body).Scan(&id)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't save that session.")
		return
	}

	entry, err := s.loadSession(ctx, me.ID, id)
	if err != nil {
		httpx.JSON(w, http.StatusCreated, map[string]string{"id": id})
		return
	}
	httpx.JSON(w, http.StatusCreated, entry)
}

// UpdateSession edits one day on the calendar: its contents, or the day it
// falls on. A session that came from a routine is edited here for that week
// only — the routine itself is unchanged, and editing the routine later
// rewrites the weeks ahead.
func (s *Service) UpdateSession(w http.ResponseWriter, r *http.Request) {
	var in sessionUpdate
	if !httpx.Decode(w, r, &in) {
		return
	}
	me := auth.MustUser(r.Context())
	ctx := r.Context()

	var date any
	if in.ScheduledOn != nil {
		parsed, err := parseDate(*in.ScheduledOn)
		if err != nil {
			httpx.Fail(w, http.StatusBadRequest, "The date must look like 2026-09-01.")
			return
		}
		date = parsed
	}

	var body []byte
	var title, focus any
	if in.Body != nil {
		known, err := s.knownSlugs(ctx)
		if err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Couldn't read the exercise library.")
			return
		}
		if err := in.Body.validate(known, "That session"); err != nil {
			httpx.Fail(w, http.StatusBadRequest, err.Error())
			return
		}
		body, err = in.Body.marshal()
		if err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Couldn't save that session.")
			return
		}
		title, focus = in.Body.Title, in.Body.Focus
	}

	tag, err := s.pool.Exec(ctx, `
		update planned_sessions
		set scheduled_on = coalesce($3, scheduled_on),
		    title        = coalesce($4, title),
		    focus        = coalesce($5, focus),
		    body         = coalesce($6, body)
		where id = $1 and user_id = $2`,
		r.PathValue("id"), me.ID, date, title, focus, body)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't update that session.")
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Fail(w, http.StatusNotFound, "That planned session doesn't exist.")
		return
	}

	entry, err := s.loadSession(ctx, me.ID, r.PathValue("id"))
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't read that session back.")
		return
	}
	httpx.JSON(w, http.StatusOK, entry)
}

// DeleteSession takes one session off the calendar. A session from a repeating
// routine stays deleted: the routine fills the weeks it has not written yet,
// not the days that were cleared on purpose.
func (s *Service) DeleteSession(w http.ResponseWriter, r *http.Request) {
	me := auth.MustUser(r.Context())
	tag, err := s.pool.Exec(r.Context(),
		`delete from planned_sessions where id = $1 and user_id = $2`, r.PathValue("id"), me.ID)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't remove that session.")
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Fail(w, http.StatusNotFound, "That planned session doesn't exist.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Service) loadSession(ctx context.Context, userID, id string) (CalendarEntry, error) {
	var e CalendarEntry
	err := s.pool.QueryRow(ctx, `
		select id, plan_id, routine_id, source, to_char(scheduled_on, 'YYYY-MM-DD'),
		       title, focus, body, completed_at, workout_id
		from planned_sessions where id = $1 and user_id = $2`, id, userID).
		Scan(&e.ID, &e.PlanID, &e.RoutineID, &e.Source, &e.ScheduledOn,
			&e.Title, &e.Focus, &e.Body, &e.CompletedAt, &e.WorkoutID)
	return e, err
}
