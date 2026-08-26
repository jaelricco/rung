package training

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"calisthenics/api/internal/auth"
	"calisthenics/api/internal/httpx"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Set kinds. The kind decides which fields must be present, which is enforced
// here and again by a check constraint in the database.
const (
	KindReps         = "reps"
	KindWeightedReps = "weighted_reps"
	KindStaticHold   = "static_hold"
	KindSkillAttempt = "skill_attempt"
)

type Exercise struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Measure     string `json:"measure"`
	Difficulty  int    `json:"difficulty"`
	Description string `json:"description"`
}

type SetInput struct {
	ExerciseSlug string   `json:"exercise_slug"`
	Kind         string   `json:"kind"`
	Reps         *int     `json:"reps"`
	WeightKg     *float64 `json:"weight_kg"`
	HoldSeconds  *float64 `json:"hold_seconds"`
	Success      *bool    `json:"success"`
}

func (s SetInput) validate(index int) error {
	pos := fmt.Sprintf("Set %d", index+1)
	if s.ExerciseSlug == "" {
		return fmt.Errorf("%s needs an exercise", pos)
	}
	switch s.Kind {
	case KindReps:
		if s.Reps == nil {
			return fmt.Errorf("%s needs a rep count", pos)
		}
	case KindWeightedReps:
		if s.Reps == nil || s.WeightKg == nil {
			return fmt.Errorf("%s needs both reps and added weight", pos)
		}
	case KindStaticHold:
		if s.HoldSeconds == nil {
			return fmt.Errorf("%s needs a hold time in seconds", pos)
		}
	case KindSkillAttempt:
		if s.Success == nil {
			return fmt.Errorf("%s needs to be marked made or missed", pos)
		}
	default:
		return fmt.Errorf("%s has an unknown set type %q", pos, s.Kind)
	}
	return nil
}

type WorkoutInput struct {
	PerformedAt *time.Time `json:"performed_at"`
	Notes       string     `json:"notes"`
	Rpe         *int       `json:"rpe"`
	Sets        []SetInput `json:"sets"`
}

type Set struct {
	ID           string   `json:"id"`
	ExerciseSlug string   `json:"exercise_slug"`
	ExerciseName string   `json:"exercise_name"`
	Category     string   `json:"category"`
	Kind         string   `json:"kind"`
	Position     int      `json:"position"`
	Reps         *int     `json:"reps"`
	WeightKg     *float64 `json:"weight_kg"`
	HoldSeconds  *float64 `json:"hold_seconds"`
	Success      *bool    `json:"success"`
}

type Workout struct {
	ID          string    `json:"id"`
	PerformedAt time.Time `json:"performed_at"`
	Notes       string    `json:"notes"`
	Rpe         *int      `json:"rpe"`
	Sets        []Set     `json:"sets"`
}

type Service struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// ---------- exercises ----------

func (s *Service) ListExercises(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `
		select slug, name, category, measure, difficulty, description
		from exercises order by category, difficulty, name`)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't load the exercise library.")
		return
	}
	defer rows.Close()

	out := []Exercise{}
	for rows.Next() {
		var e Exercise
		if err := rows.Scan(&e.Slug, &e.Name, &e.Category, &e.Measure, &e.Difficulty, &e.Description); err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Couldn't read the exercise library.")
			return
		}
		out = append(out, e)
	}
	httpx.JSON(w, http.StatusOK, out)
}

// ---------- workouts ----------

func (s *Service) CreateWorkout(w http.ResponseWriter, r *http.Request) {
	var in WorkoutInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	if len(in.Sets) == 0 {
		httpx.Fail(w, http.StatusBadRequest, "Add at least one set before saving.")
		return
	}
	if len(in.Sets) > 200 {
		httpx.Fail(w, http.StatusBadRequest, "That's more than 200 sets. Split it into separate sessions.")
		return
	}
	for i, set := range in.Sets {
		if err := set.validate(i); err != nil {
			httpx.Fail(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	performed := time.Now()
	if in.PerformedAt != nil {
		performed = *in.PerformedAt
	}

	me := auth.MustUser(r.Context())
	ctx := r.Context()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't save the session. Try again.")
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var workoutID string
	err = tx.QueryRow(ctx, `
		insert into workouts (user_id, performed_at, notes, rpe)
		values ($1, $2, $3, $4) returning id`,
		me.ID, performed, in.Notes, in.Rpe).Scan(&workoutID)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't save the session. Try again.")
		return
	}

	for i, set := range in.Sets {
		var exerciseID string
		err := tx.QueryRow(ctx, `select id from exercises where slug = $1`, set.ExerciseSlug).Scan(&exerciseID)
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Fail(w, http.StatusBadRequest,
				fmt.Sprintf("Set %d refers to an exercise that isn't in the library: %s", i+1, set.ExerciseSlug))
			return
		}
		if err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Couldn't save the session. Try again.")
			return
		}
		_, err = tx.Exec(ctx, `
			insert into workout_sets
				(workout_id, exercise_id, position, kind, reps, weight_kg, hold_seconds, success)
			values ($1, $2, $3, $4, $5, $6, $7, $8)`,
			workoutID, exerciseID, i, set.Kind, set.Reps, set.WeightKg, set.HoldSeconds, set.Success)
		if err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Couldn't save the session. Try again.")
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't save the session. Try again.")
		return
	}

	out, err := s.loadWorkout(ctx, me.ID, workoutID)
	if err != nil {
		httpx.JSON(w, http.StatusCreated, map[string]string{"id": workoutID})
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (s *Service) loadWorkout(ctx context.Context, userID, workoutID string) (Workout, error) {
	var out Workout
	err := s.pool.QueryRow(ctx, `
		select id, performed_at, notes, rpe from workouts
		where id = $1 and user_id = $2`, workoutID, userID,
	).Scan(&out.ID, &out.PerformedAt, &out.Notes, &out.Rpe)
	if err != nil {
		return out, err
	}
	sets, err := s.setsFor(ctx, []string{workoutID})
	if err != nil {
		return out, err
	}
	out.Sets = sets[workoutID]
	if out.Sets == nil {
		out.Sets = []Set{}
	}
	return out, nil
}

func (s *Service) setsFor(ctx context.Context, workoutIDs []string) (map[string][]Set, error) {
	result := map[string][]Set{}
	if len(workoutIDs) == 0 {
		return result, nil
	}
	rows, err := s.pool.Query(ctx, `
		select ws.workout_id, ws.id, e.slug, e.name, e.category, ws.kind, ws.position,
		       ws.reps, ws.weight_kg, ws.hold_seconds, ws.success
		from workout_sets ws
		join exercises e on e.id = ws.exercise_id
		where ws.workout_id = any($1)
		order by ws.workout_id, ws.position`, workoutIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var wid string
		var st Set
		if err := rows.Scan(&wid, &st.ID, &st.ExerciseSlug, &st.ExerciseName, &st.Category,
			&st.Kind, &st.Position, &st.Reps, &st.WeightKg, &st.HoldSeconds, &st.Success); err != nil {
			return nil, err
		}
		result[wid] = append(result[wid], st)
	}
	return result, rows.Err()
}

func (s *Service) ListWorkouts(w http.ResponseWriter, r *http.Request) {
	me := auth.MustUser(r.Context())
	limit := 30
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	rows, err := s.pool.Query(r.Context(), `
		select id, performed_at, notes, rpe from workouts
		where user_id = $1 order by performed_at desc limit $2`, me.ID, limit)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't load your sessions.")
		return
	}
	defer rows.Close()

	workouts := []Workout{}
	ids := []string{}
	for rows.Next() {
		var wk Workout
		if err := rows.Scan(&wk.ID, &wk.PerformedAt, &wk.Notes, &wk.Rpe); err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Couldn't read your sessions.")
			return
		}
		wk.Sets = []Set{}
		workouts = append(workouts, wk)
		ids = append(ids, wk.ID)
	}

	sets, err := s.setsFor(r.Context(), ids)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't read your sets.")
		return
	}
	for i := range workouts {
		if got := sets[workouts[i].ID]; got != nil {
			workouts[i].Sets = got
		}
	}
	httpx.JSON(w, http.StatusOK, workouts)
}

func (s *Service) DeleteWorkout(w http.ResponseWriter, r *http.Request) {
	me := auth.MustUser(r.Context())
	id := r.PathValue("id")
	tag, err := s.pool.Exec(r.Context(),
		`delete from workouts where id = $1 and user_id = $2`, id, me.ID)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't delete that session.")
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Fail(w, http.StatusNotFound, "That session no longer exists.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
