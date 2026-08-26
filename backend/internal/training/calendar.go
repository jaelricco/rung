package training

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"calisthenics/api/internal/auth"
	"calisthenics/api/internal/httpx"
)

type CalendarEntry struct {
	ID           string          `json:"id"`
	PlanID       *string         `json:"plan_id"`
	ScheduledOn  string          `json:"scheduled_on"`
	Title        string          `json:"title"`
	Focus        string          `json:"focus"`
	Body         json.RawMessage `json:"body"`
	CompletedAt  *time.Time      `json:"completed_at"`
	WorkoutID    *string         `json:"workout_id"`
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

	out := CalendarResponse{
		From:     from.Format("2006-01-02"),
		To:       to.Format("2006-01-02"),
		Sessions: []CalendarEntry{},
		Events:   []CalendarEvent{},
	}

	rows, err := s.pool.Query(r.Context(), `
		select id, plan_id, to_char(scheduled_on, 'YYYY-MM-DD'), title, focus, body, completed_at, workout_id
		from planned_sessions
		where user_id = $1 and scheduled_on between $2 and $3
		order by scheduled_on`, me.ID, from, to)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't load your calendar.")
		return
	}
	for rows.Next() {
		var e CalendarEntry
		if err := rows.Scan(&e.ID, &e.PlanID, &e.ScheduledOn, &e.Title, &e.Focus,
			&e.Body, &e.CompletedAt, &e.WorkoutID); err != nil {
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
