package training

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

	"github.com/jackc/pgx/v5"
)

// A routine is the week an athlete already trains, written by them: three
// pushes and two pulls on the days they actually go, repeated until they
// change it. Plans are generated and finite; a routine is theirs and open
// ended. Both end up as dated rows in planned_sessions, which is why the
// calendar, the tick box and the .ics export needed no new concepts.
//
// Repeating is done by writing the sessions out ahead of time rather than
// expanding a rule at read time. A materialised session has an id, so it can
// be ticked off, linked to the workout that was actually logged, and deleted
// for the one week the athlete is away — none of which a virtual row can do.

// routineHorizonDays is how far ahead an active routine is written onto the
// calendar. Ten weeks covers the five-week view plus a page forward.
const routineHorizonDays = 70

// routineHorizonCap stops a request for a distant month from writing years of
// sessions in one go.
const routineHorizonCap = 400

const (
	maxRoutineDays = 21 // three sessions a day, every day, is already generous
	maxRoutines    = 20
)

type RoutineDay struct {
	ID        string          `json:"id"`
	DayOfWeek int             `json:"day_of_week"`
	Title     string          `json:"title"`
	Focus     string          `json:"focus"`
	Body      json.RawMessage `json:"body"`
}

type Routine struct {
	ID        string       `json:"id"`
	Title     string       `json:"title"`
	Notes     string       `json:"notes"`
	Active    bool         `json:"active"`
	StartsOn  string       `json:"starts_on"`
	EndsOn    *string      `json:"ends_on"`
	CreatedAt time.Time    `json:"created_at"`
	Days      []RoutineDay `json:"days"`
	// Upcoming is how many sessions this routine currently has on the
	// calendar from today on, so the list can say whether it is doing
	// anything without the caller loading the calendar too.
	Upcoming int `json:"upcoming"`
}

type routineDayInput struct {
	DayOfWeek int         `json:"day_of_week"`
	Body      SessionBody `json:"body"`
}

// Repeat is the choice the athlete is actually making when they save: is this
// the week from now on, or only this one week?
const (
	RepeatWeekly = "weekly"
	RepeatOnce   = "once"
)

type routineInput struct {
	Title  string            `json:"title"`
	Notes  string            `json:"notes"`
	Repeat string            `json:"repeat"`
	WeekOf string            `json:"week_of"`
	Days   []routineDayInput `json:"days"`
}

type routineUpdate struct {
	Title  *string            `json:"title"`
	Notes  *string            `json:"notes"`
	Active *bool              `json:"active"`
	Days   *[]routineDayInput `json:"days"`
}

type applyInput struct {
	WeekOf string `json:"week_of"`
}

// ---------- reading ----------

// ListRoutines returns the athlete's routines with their days, newest first.
func (s *Service) ListRoutines(w http.ResponseWriter, r *http.Request) {
	me := auth.MustUser(r.Context())
	out, err := s.loadRoutines(r.Context(), me.ID, "")
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't load your routines.")
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// loadRoutines reads every routine for a user, or one of them when id is set.
func (s *Service) loadRoutines(ctx context.Context, userID, id string) ([]Routine, error) {
	rows, err := s.pool.Query(ctx, `
		select r.id, r.title, r.notes, r.active, to_char(r.starts_on, 'YYYY-MM-DD'),
		       to_char(r.ends_on, 'YYYY-MM-DD'), r.created_at,
		       (select count(*) from planned_sessions ps
		         where ps.routine_id = r.id and ps.scheduled_on >= current_date)
		from routines r
		where r.user_id = $1 and ($2 = '' or r.id = $2::uuid)
		order by r.active desc, r.created_at desc
		limit 50`, userID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Routine{}
	index := map[string]int{}
	for rows.Next() {
		var rt Routine
		if err := rows.Scan(&rt.ID, &rt.Title, &rt.Notes, &rt.Active, &rt.StartsOn,
			&rt.EndsOn, &rt.CreatedAt, &rt.Upcoming); err != nil {
			return nil, err
		}
		rt.Days = []RoutineDay{}
		index[rt.ID] = len(out)
		out = append(out, rt)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}

	ids := make([]string, 0, len(out))
	for _, rt := range out {
		ids = append(ids, rt.ID)
	}
	dayRows, err := s.pool.Query(ctx, `
		select id, routine_id, day_of_week, title, focus, body
		from routine_days
		where routine_id = any($1)
		order by day_of_week, position`, ids)
	if err != nil {
		return nil, err
	}
	defer dayRows.Close()
	for dayRows.Next() {
		var routineID string
		var d RoutineDay
		if err := dayRows.Scan(&d.ID, &routineID, &d.DayOfWeek, &d.Title, &d.Focus, &d.Body); err != nil {
			return nil, err
		}
		if at, ok := index[routineID]; ok {
			out[at].Days = append(out[at].Days, d)
		}
	}
	return out, dayRows.Err()
}

// ---------- writing ----------

// CreateRoutine saves a week of training. Whether it repeats is the athlete's
// call at save time: weekly keeps filling the calendar ahead, once drops the
// week onto the calendar and then leaves the template sitting there to be
// applied again by hand.
func (s *Service) CreateRoutine(w http.ResponseWriter, r *http.Request) {
	var in routineInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	me := auth.MustUser(r.Context())
	ctx := r.Context()

	in.Title = strings.TrimSpace(in.Title)
	in.Notes = strings.TrimSpace(in.Notes)
	if in.Title == "" {
		httpx.Fail(w, http.StatusBadRequest, "Give the routine a name, such as \"Current week\".")
		return
	}
	if len(in.Title) > maxTitleLength {
		httpx.Fail(w, http.StatusBadRequest, "That name is too long.")
		return
	}
	if len(in.Notes) > maxTextLength {
		httpx.Fail(w, http.StatusBadRequest, "Those notes are too long.")
		return
	}
	repeat, ok := normaliseRepeat(in.Repeat)
	if !ok {
		httpx.Fail(w, http.StatusBadRequest, "Say whether this repeats: \"weekly\" or \"once\".")
		return
	}
	monday, err := weekOf(in.WeekOf)
	if err != nil {
		httpx.Fail(w, http.StatusBadRequest, err.Error())
		return
	}
	known, err := s.knownSlugs(ctx)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't read the exercise library.")
		return
	}
	if err := validateDays(in.Days, known); err != nil {
		httpx.Fail(w, http.StatusBadRequest, err.Error())
		return
	}

	var count int
	if err := s.pool.QueryRow(ctx,
		`select count(*) from routines where user_id = $1`, me.ID).Scan(&count); err == nil && count >= maxRoutines {
		httpx.Fail(w, http.StatusBadRequest,
			fmt.Sprintf("You already have %d routines. Remove one before adding another.", maxRoutines))
		return
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't save that routine.")
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// A one-off ends with the week it was made for, so it stops filling
	// anything while staying in the list to be applied again later.
	var endsOn any
	if repeat == RepeatOnce {
		endsOn = monday.AddDate(0, 0, 6)
	}
	var routineID string
	err = tx.QueryRow(ctx, `
		insert into routines (user_id, title, notes, active, starts_on, ends_on)
		values ($1, $2, $3, $4, $5, $6) returning id`,
		me.ID, in.Title, in.Notes, repeat == RepeatWeekly, monday, endsOn).Scan(&routineID)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't save that routine.")
		return
	}
	if err := insertDays(ctx, tx, routineID, in.Days); err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't save that routine.")
		return
	}
	if err := tx.Commit(ctx); err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't save that routine.")
		return
	}

	// A routine is worth nothing until it is on the calendar, so both kinds
	// are written out immediately: weekly out to the horizon, once for the
	// week it was made for — that week included when it has already started,
	// which is what "just this week" means on a Wednesday.
	if repeat == RepeatWeekly {
		_ = s.fillRoutines(ctx, me.ID, defaultHorizon())
	} else {
		_, _ = s.applyRoutineWeek(ctx, me.ID, routineID, monday)
	}

	out, err := s.loadRoutines(ctx, me.ID, routineID)
	if err != nil || len(out) == 0 {
		httpx.JSON(w, http.StatusCreated, map[string]string{"id": routineID})
		return
	}
	httpx.JSON(w, http.StatusCreated, out[0])
}

// UpdateRoutine changes a routine and re-writes what it has ahead of it.
// Sessions already done, and everything before today, are left alone: they are
// a record of what happened, not of what the routine currently says.
func (s *Service) UpdateRoutine(w http.ResponseWriter, r *http.Request) {
	var in routineUpdate
	if !httpx.Decode(w, r, &in) {
		return
	}
	me := auth.MustUser(r.Context())
	ctx := r.Context()
	id := r.PathValue("id")

	var exists bool
	if err := s.pool.QueryRow(ctx,
		`select exists(select 1 from routines where id = $1 and user_id = $2)`, id, me.ID).Scan(&exists); err != nil {
		httpx.Fail(w, http.StatusBadRequest, "That routine doesn't exist.")
		return
	}
	if !exists {
		httpx.Fail(w, http.StatusNotFound, "That routine doesn't exist.")
		return
	}

	if in.Title != nil {
		*in.Title = strings.TrimSpace(*in.Title)
		if *in.Title == "" || len(*in.Title) > maxTitleLength {
			httpx.Fail(w, http.StatusBadRequest, "A routine needs a name, and a short one.")
			return
		}
	}
	if in.Notes != nil {
		*in.Notes = strings.TrimSpace(*in.Notes)
		if len(*in.Notes) > maxTextLength {
			httpx.Fail(w, http.StatusBadRequest, "Those notes are too long.")
			return
		}
	}
	if in.Days != nil {
		known, err := s.knownSlugs(ctx)
		if err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Couldn't read the exercise library.")
			return
		}
		if err := validateDays(*in.Days, known); err != nil {
			httpx.Fail(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't update that routine.")
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if in.Title != nil || in.Notes != nil {
		_, err = tx.Exec(ctx, `
			update routines
			set title = coalesce($3, title), notes = coalesce($4, notes), updated_at = now()
			where id = $1 and user_id = $2`, id, me.ID, in.Title, in.Notes)
		if err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Couldn't update that routine.")
			return
		}
	}

	// Changing the week, or switching repeating on or off, invalidates every
	// session this routine has put on the calendar from today forward.
	reschedule := in.Days != nil || in.Active != nil
	if reschedule {
		if err := clearFutureRoutineSessions(ctx, tx, me.ID, id); err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Couldn't update that routine.")
			return
		}
		_, err = tx.Exec(ctx,
			`update routines set materialized_through = null, updated_at = now()
			 where id = $1 and user_id = $2`, id, me.ID)
		if err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Couldn't update that routine.")
			return
		}
	}
	if in.Days != nil {
		if _, err := tx.Exec(ctx, `delete from routine_days where routine_id = $1`, id); err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Couldn't update that routine.")
			return
		}
		if err := insertDays(ctx, tx, id, *in.Days); err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Couldn't update that routine.")
			return
		}
	}
	if in.Active != nil {
		// Turning repeating back on reopens the end date a one-off week was
		// given, otherwise the routine would be active and still expired.
		_, err = tx.Exec(ctx, `
			update routines
			set active = $3,
			    ends_on = case when $3 then null else ends_on end,
			    updated_at = now()
			where id = $1 and user_id = $2`, id, me.ID, *in.Active)
		if err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Couldn't update that routine.")
			return
		}
	}
	if err := tx.Commit(ctx); err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't update that routine.")
		return
	}

	if reschedule {
		_ = s.fillRoutines(ctx, me.ID, defaultHorizon())
	}

	out, err := s.loadRoutines(ctx, me.ID, id)
	if err != nil || len(out) == 0 {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't read that routine back.")
		return
	}
	httpx.JSON(w, http.StatusOK, out[0])
}

// ApplyRoutine drops a routine onto one week and nothing more. This is the
// "just this week" path for a routine that is not the standing one, and the
// way to put a week back after it was cleared.
func (s *Service) ApplyRoutine(w http.ResponseWriter, r *http.Request) {
	var in applyInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	me := auth.MustUser(r.Context())
	monday, err := weekOf(in.WeekOf)
	if err != nil {
		httpx.Fail(w, http.StatusBadRequest, err.Error())
		return
	}

	added, err := s.applyRoutineWeek(r.Context(), me.ID, r.PathValue("id"), monday)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Fail(w, http.StatusNotFound, "That routine doesn't exist.")
		return
	}
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't put that routine on the calendar.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"week_of": monday.Format("2006-01-02"),
		"added":   added,
	})
}

// DeleteRoutine removes the template and the sessions it still has ahead of
// it. What is already done stays on the calendar: the routine having been
// deleted does not mean the training never happened.
func (s *Service) DeleteRoutine(w http.ResponseWriter, r *http.Request) {
	me := auth.MustUser(r.Context())
	ctx := r.Context()
	id := r.PathValue("id")

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't remove that routine.")
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := clearFutureRoutineSessions(ctx, tx, me.ID, id); err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't remove that routine.")
		return
	}
	tag, err := tx.Exec(ctx, `delete from routines where id = $1 and user_id = $2`, id, me.ID)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't remove that routine.")
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Fail(w, http.StatusNotFound, "That routine doesn't exist.")
		return
	}
	if err := tx.Commit(ctx); err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't remove that routine.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ---------- putting routines on the calendar ----------

// fillRoutines writes every active routine's sessions out to `through`.
//
// It only ever fills dates after materialized_through, which is what makes a
// deleted week stay deleted: the calendar is not re-derived from the template
// on every read, it is written once and then owned by the athlete.
func (s *Service) fillRoutines(ctx context.Context, userID string, through time.Time) error {
	_, err := s.pool.Exec(ctx, `
		with filled as (
			insert into planned_sessions
				(user_id, scheduled_on, title, focus, body, routine_id, routine_day_id, source)
			select r.user_id, d.day::date, rd.title, rd.focus, rd.body, r.id, rd.id, 'routine'
			from routines r
			join routine_days rd on rd.routine_id = r.id
			cross join lateral generate_series(
				greatest(r.starts_on, current_date, coalesce(r.materialized_through + 1, r.starts_on)),
				least(coalesce(r.ends_on, $2::date), $2::date),
				interval '1 day') as d(day)
			where r.user_id = $1 and r.active
			  and extract(isodow from d.day) = rd.day_of_week
			on conflict (routine_day_id, scheduled_on) do nothing
			returning 1
		)
		update routines
		set materialized_through = $2::date
		where user_id = $1 and active
		  and (materialized_through is null or materialized_through < $2::date)`,
		userID, through)
	return err
}

// applyRoutineWeek writes one routine's days onto one specific week, whether
// or not the routine repeats and whether or not that week is already filled.
// Days that are already there are left as they are.
func (s *Service) applyRoutineWeek(ctx context.Context, userID, routineID string, monday time.Time) (int64, error) {
	var owned bool
	err := s.pool.QueryRow(ctx,
		`select exists(select 1 from routines where id = $1 and user_id = $2)`, routineID, userID).Scan(&owned)
	if err != nil {
		return 0, err
	}
	if !owned {
		return 0, pgx.ErrNoRows
	}

	tag, err := s.pool.Exec(ctx, `
		insert into planned_sessions
			(user_id, scheduled_on, title, focus, body, routine_id, routine_day_id, source)
		select r.user_id, $3::date + (rd.day_of_week - 1), rd.title, rd.focus, rd.body, r.id, rd.id, 'routine'
		from routines r
		join routine_days rd on rd.routine_id = r.id
		where r.id = $1 and r.user_id = $2
		on conflict (routine_day_id, scheduled_on) do nothing`,
		routineID, userID, monday)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// clearFutureRoutineSessions removes what a routine has scheduled from today
// on, except anything already ticked off.
func clearFutureRoutineSessions(ctx context.Context, tx pgx.Tx, userID, routineID string) error {
	_, err := tx.Exec(ctx, `
		delete from planned_sessions
		where user_id = $1 and routine_id = $2
		  and completed_at is null and workout_id is null
		  and scheduled_on >= current_date`, userID, routineID)
	return err
}

func insertDays(ctx context.Context, tx pgx.Tx, routineID string, days []routineDayInput) error {
	for i, day := range days {
		body, err := day.Body.marshal()
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			insert into routine_days (routine_id, day_of_week, position, title, focus, body)
			values ($1, $2, $3, $4, $5, $6)`,
			routineID, day.DayOfWeek, i, day.Body.Title, day.Body.Focus, body)
		if err != nil {
			return err
		}
	}
	return nil
}

// ---------- input helpers ----------

func normaliseRepeat(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case RepeatWeekly, "every_week", "":
		// Empty means weekly: a routine is a repeating thing unless the
		// athlete says otherwise, and that is the common case.
		return RepeatWeekly, true
	case RepeatOnce, "this_week":
		return RepeatOnce, true
	default:
		return "", false
	}
}

// weekOf reads any date in a week and answers with that week's Monday.
func weekOf(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return MondayOf(time.Now()), nil
	}
	parsed, err := parseDate(value)
	if err != nil {
		return time.Time{}, errors.New("The week has to be given as a date like 2026-09-01.")
	}
	return MondayOf(parsed), nil
}

func validateDays(days []routineDayInput, known map[string]bool) error {
	if len(days) == 0 {
		return errors.New("A routine needs at least one training day.")
	}
	if len(days) > maxRoutineDays {
		return fmt.Errorf("A routine can hold at most %d sessions in a week.", maxRoutineDays)
	}
	for i := range days {
		day := &days[i]
		if day.DayOfWeek < 1 || day.DayOfWeek > 7 {
			return fmt.Errorf("Session %d isn't on a day of the week.", i+1)
		}
		where := fmt.Sprintf("%s's session", dayName(day.DayOfWeek))
		if err := day.Body.validate(known, where); err != nil {
			return err
		}
	}
	return nil
}

var dayNames = [...]string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}

func dayName(day int) string {
	if day < 1 || day > 7 {
		return "That"
	}
	return dayNames[day-1]
}

// defaultHorizon is how far ahead an ordinary fill reaches.
func defaultHorizon() time.Time {
	return time.Now().AddDate(0, 0, routineHorizonDays)
}

// horizonFor keeps the calendar filled as far as it is being looked at, so
// paging forward finds the routine already there, without letting a request
// for the year 2400 write a decade of sessions.
func horizonFor(to time.Time) time.Time {
	horizon := defaultHorizon()
	if to.After(horizon) {
		horizon = to
	}
	limit := time.Now().AddDate(0, 0, routineHorizonCap)
	if horizon.After(limit) {
		horizon = limit
	}
	return horizon
}
