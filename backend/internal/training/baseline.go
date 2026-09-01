package training

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"calisthenics/api/internal/auth"
	"calisthenics/api/internal/httpx"
)

// What the athlete can do, as they describe it, before they have logged it.
//
// The planner sizes every prescription from records. An athlete who joined
// yesterday has none, so it puts them at the bottom of every ladder on a
// deliberately light week — the right answer given what it knows, and the
// wrong one for someone who already has twelve pull-ups. This is how they say
// so, in the same units the app measures everything else in, so nothing
// downstream needs a second way of reading it.
//
// Two rules keep it honest. A declared figure never overwrites a logged one;
// the record is whichever is higher, and it says which kind it is. And a
// declaration is dated, so a year-old claim can be told apart from a set
// performed last week.

// BaselineRecord is one movement the athlete has put a number against.
type BaselineRecord struct {
	ExerciseSlug string   `json:"exercise_slug"`
	Name         string   `json:"name,omitempty"`
	Reps         *int     `json:"reps,omitempty"`
	AddedKg      *float64 `json:"added_kg,omitempty"`
	HoldSeconds  *float64 `json:"hold_seconds,omitempty"`
	RecordedOn   string   `json:"recorded_on,omitempty"`
}

func (b BaselineRecord) empty() bool {
	return b.Reps == nil && b.AddedKg == nil && b.HoldSeconds == nil
}

// Baseline is the whole self-assessment: the numbers, plus the training
// context that is not a set.
type Baseline struct {
	BodyweightKg  *float64         `json:"bodyweight_kg"`
	TrainsPerWeek *int             `json:"trains_per_week"`
	SleepHours    *float64         `json:"sleep_hours"`
	Equipment     []string         `json:"equipment"`
	Records       []BaselineRecord `json:"records"`
}

// Equipment an athlete can own. The planner will not prescribe a movement the
// athlete has nothing to perform it on, which is a filter with teeth: answer
// "no bar" and most of this sport goes away.
var validEquipment = map[string]bool{
	"pull_up_bar": true, "dip_bars": true, "rings": true, "parallettes": true,
	"weight_belt": true, "bands": true, "floor_only": true,
}

// mergeBaseline folds declared figures into the snapshot's records. A slug
// with no logged set gets a record of its own; a slug with one keeps the
// higher of the two figures per measurement, so logging a light set never
// makes an athlete look weaker than they said they were.
func (s *Service) mergeBaseline(ctx context.Context, userID string, snap *Snapshot) error {
	rows, err := s.pool.Query(ctx, `
		select e.slug, e.name, e.category, b.reps, b.weight_kg, b.hold_seconds
		from baseline_records b
		join exercises e on e.id = b.exercise_id
		where b.user_id = $1`, userID)
	if err != nil {
		return err
	}
	defer rows.Close()

	byslug := map[string]int{}
	for i, rec := range snap.Records {
		byslug[rec.Slug] = i
	}

	for rows.Next() {
		var declared Record
		if err := rows.Scan(&declared.Slug, &declared.Name, &declared.Category,
			&declared.BestReps, &declared.BestWeight, &declared.BestHold); err != nil {
			return err
		}

		index, seen := byslug[declared.Slug]
		if !seen {
			declared.Source = SourceDeclared
			snap.Records = append(snap.Records, declared)
			byslug[declared.Slug] = len(snap.Records) - 1
			continue
		}

		existing := &snap.Records[index]
		used := false
		if higherInt(&existing.BestReps, declared.BestReps) {
			used = true
		}
		if higherFloat(&existing.BestWeight, declared.BestWeight) {
			used = true
		}
		if higherFloat(&existing.BestHold, declared.BestHold) {
			used = true
		}
		if used {
			existing.Source = SourceBoth
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	sort.SliceStable(snap.Records, func(i, j int) bool {
		if snap.Records[i].Category != snap.Records[j].Category {
			return snap.Records[i].Category < snap.Records[j].Category
		}
		return snap.Records[i].Name < snap.Records[j].Name
	})

	return s.pool.QueryRow(ctx, `
		select trains_per_week, sleep_hours, equipment from users where id = $1`, userID,
	).Scan(&snap.TrainsPerWeek, &snap.SleepHours, &snap.Equipment)
}

// higherInt raises the target to the candidate and reports whether it did.
func higherInt(target **int, candidate *int) bool {
	if candidate == nil {
		return false
	}
	if *target == nil || *candidate > **target {
		*target = candidate
		return true
	}
	return false
}

func higherFloat(target **float64, candidate *float64) bool {
	if candidate == nil {
		return false
	}
	if *target == nil || *candidate > **target {
		*target = candidate
		return true
	}
	return false
}

// ---------- endpoints ----------

func (s *Service) GetBaseline(w http.ResponseWriter, r *http.Request) {
	me := auth.MustUser(r.Context())
	out, err := s.loadBaseline(r.Context(), me.ID)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't read your baseline.")
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (s *Service) loadBaseline(ctx context.Context, userID string) (Baseline, error) {
	out := Baseline{Records: []BaselineRecord{}}

	err := s.pool.QueryRow(ctx, `
		select bodyweight_kg, trains_per_week, sleep_hours, equipment
		from users where id = $1`, userID,
	).Scan(&out.BodyweightKg, &out.TrainsPerWeek, &out.SleepHours, &out.Equipment)
	if err != nil {
		return out, err
	}

	rows, err := s.pool.Query(ctx, `
		select e.slug, e.name, b.reps, b.weight_kg, b.hold_seconds,
		       to_char(b.recorded_at, 'YYYY-MM-DD')
		from baseline_records b
		join exercises e on e.id = b.exercise_id
		where b.user_id = $1
		order by e.category, e.difficulty, e.slug`, userID)
	if err != nil {
		return out, err
	}
	defer rows.Close()

	for rows.Next() {
		var rec BaselineRecord
		if err := rows.Scan(&rec.ExerciseSlug, &rec.Name, &rec.Reps, &rec.AddedKg,
			&rec.HoldSeconds, &rec.RecordedOn); err != nil {
			return out, err
		}
		out.Records = append(out.Records, rec)
	}
	return out, rows.Err()
}

// PutBaseline stores the self-assessment. Fields that are absent are left
// alone, so the plan page can send the two numbers it asked for without
// wiping the ten the athlete entered last month. A record sent with every
// measurement empty is a deletion, which is how a mistake gets taken back.
func (s *Service) PutBaseline(w http.ResponseWriter, r *http.Request) {
	var in Baseline
	if !httpx.Decode(w, r, &in) {
		return
	}
	if in.BodyweightKg != nil && (*in.BodyweightKg < 20 || *in.BodyweightKg > 300) {
		httpx.Fail(w, http.StatusBadRequest, "Bodyweight should be between 20 and 300 kg.")
		return
	}
	if in.TrainsPerWeek != nil && (*in.TrainsPerWeek < 0 || *in.TrainsPerWeek > 14) {
		httpx.Fail(w, http.StatusBadRequest, "Sessions per week should be between 0 and 14.")
		return
	}
	if in.SleepHours != nil && (*in.SleepHours < 3 || *in.SleepHours > 14) {
		httpx.Fail(w, http.StatusBadRequest, "Sleep should be between 3 and 14 hours a night.")
		return
	}
	for _, item := range in.Equipment {
		if !validEquipment[item] {
			httpx.Fail(w, http.StatusBadRequest, fmt.Sprintf("%q isn't equipment this app knows about.", item))
			return
		}
	}
	if len(in.Records) > 100 {
		httpx.Fail(w, http.StatusBadRequest, "That's more baseline entries than the library has exercises.")
		return
	}
	for i := range in.Records {
		rec := &in.Records[i]
		rec.ExerciseSlug = strings.TrimSpace(rec.ExerciseSlug)
		if rec.ExerciseSlug == "" {
			httpx.Fail(w, http.StatusBadRequest, "A baseline entry is missing its exercise.")
			return
		}
		if rec.Reps != nil && (*rec.Reps < 0 || *rec.Reps > 1000) {
			httpx.Fail(w, http.StatusBadRequest, "A rep count has to be between 0 and 1000.")
			return
		}
		if rec.HoldSeconds != nil && (*rec.HoldSeconds < 0 || *rec.HoldSeconds > 3600) {
			httpx.Fail(w, http.StatusBadRequest, "A hold has to be between 0 and 3600 seconds.")
			return
		}
		if rec.AddedKg != nil && (*rec.AddedKg < 0 || *rec.AddedKg > 300) {
			httpx.Fail(w, http.StatusBadRequest, "Added load has to be between 0 and 300 kg.")
			return
		}
	}

	me := auth.MustUser(r.Context())
	ctx := r.Context()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't save your baseline. Try again.")
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Absent and empty are different answers, and the difference has to survive
	// the driver: a nil slice is sent as SQL NULL so coalesce keeps whatever
	// was there, while an empty one is sent as an empty array and means "none
	// of it". Passing the slice directly would leave that to the encoder.
	var equipment any
	if in.Equipment != nil {
		equipment = in.Equipment
	}

	_, err = tx.Exec(ctx, `
		update users set
			bodyweight_kg   = coalesce($2, bodyweight_kg),
			trains_per_week = coalesce($3, trains_per_week),
			sleep_hours     = coalesce($4, sleep_hours),
			equipment       = coalesce($5, equipment)
		where id = $1`,
		me.ID, in.BodyweightKg, in.TrainsPerWeek, in.SleepHours, equipment)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't save your baseline. Try again.")
		return
	}

	for _, rec := range in.Records {
		if rec.empty() {
			_, err = tx.Exec(ctx, `
				delete from baseline_records
				where user_id = $1 and exercise_id = (select id from exercises where slug = $2)`,
				me.ID, rec.ExerciseSlug)
			if err != nil {
				httpx.Fail(w, http.StatusInternalServerError, "Couldn't save your baseline. Try again.")
				return
			}
			continue
		}

		tag, err := tx.Exec(ctx, `
			insert into baseline_records (user_id, exercise_id, reps, weight_kg, hold_seconds)
			select $1, e.id, $3, $4, $5 from exercises e where e.slug = $2
			on conflict (user_id, exercise_id) do update set
				reps         = excluded.reps,
				weight_kg    = excluded.weight_kg,
				hold_seconds = excluded.hold_seconds,
				recorded_at  = now()`,
			me.ID, rec.ExerciseSlug, rec.Reps, rec.AddedKg, rec.HoldSeconds)
		if err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Couldn't save your baseline. Try again.")
			return
		}
		// The insert selects from exercises, so an unknown slug writes nothing
		// rather than failing a constraint. Say so instead of silently
		// dropping a number the athlete typed.
		if tag.RowsAffected() == 0 {
			httpx.Fail(w, http.StatusBadRequest,
				fmt.Sprintf("%q isn't an exercise in the library.", rec.ExerciseSlug))
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't save your baseline. Try again.")
		return
	}

	out, err := s.loadBaseline(ctx, me.ID)
	if err != nil {
		httpx.JSON(w, http.StatusOK, in)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}
