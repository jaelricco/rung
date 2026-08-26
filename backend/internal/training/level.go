package training

import (
	"context"
	"net/http"

	"calisthenics/api/internal/auth"
	"calisthenics/api/internal/httpx"
)

// Tiers are derived in Go rather than asked of the model, so the same log
// always produces the same level. Thresholds are deliberately easy to tune:
// edit the table below and the whole app follows.
var tierNames = []string{"untested", "beginner", "novice", "intermediate", "advanced", "elite"}

type rubric struct {
	Slug   string
	Metric string // "reps", "hold", or "added_kg"
	// Cut points for beginner / novice / intermediate / advanced / elite.
	Cuts [5]float64
}

var rubrics = []rubric{
	// Reps
	{"pull_up", "reps", [5]float64{1, 5, 10, 15, 22}},
	{"dip", "reps", [5]float64{1, 8, 15, 25, 35}},
	{"push_up", "reps", [5]float64{5, 20, 35, 50, 70}},
	{"muscle_up", "reps", [5]float64{1, 3, 6, 10, 15}},
	{"ring_muscle_up", "reps", [5]float64{1, 2, 5, 8, 12}},
	{"handstand_push_up", "reps", [5]float64{1, 3, 6, 10, 15}},
	{"pistol_squat", "reps", [5]float64{1, 5, 10, 15, 20}},
	{"hanging_leg_raise", "reps", [5]float64{1, 6, 12, 18, 25}},

	// Static holds, in seconds
	{"l_sit", "hold", [5]float64{5, 15, 30, 45, 60}},
	{"tuck_front_lever", "hold", [5]float64{5, 15, 30, 45, 60}},
	{"adv_tuck_front_lever", "hold", [5]float64{3, 10, 20, 30, 45}},
	{"straddle_front_lever", "hold", [5]float64{2, 6, 12, 20, 30}},
	{"front_lever", "hold", [5]float64{1, 4, 10, 18, 30}},
	{"back_lever", "hold", [5]float64{2, 8, 15, 25, 40}},
	{"tuck_planche", "hold", [5]float64{3, 10, 20, 30, 45}},
	{"straddle_planche", "hold", [5]float64{1, 4, 8, 15, 25}},
	{"full_planche", "hold", [5]float64{1, 3, 6, 10, 20}},
	{"handstand", "hold", [5]float64{5, 20, 45, 90, 180}},
	{"human_flag", "hold", [5]float64{1, 4, 8, 15, 25}},

	// Added load in kg. Absolute, since not everyone records bodyweight.
	{"weighted_pull_up", "added_kg", [5]float64{2.5, 12, 24, 40, 60}},
	{"weighted_dip", "added_kg", [5]float64{2.5, 16, 32, 55, 80}},
	{"weighted_muscle_up", "added_kg", [5]float64{2.5, 8, 16, 28, 40}},
}

func tierFor(value float64, cuts [5]float64) int {
	tier := 0
	for i, cut := range cuts {
		if value >= cut {
			tier = i + 1
		}
	}
	return tier
}

type Record struct {
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	BestReps    *int     `json:"best_reps"`
	BestWeight  *float64 `json:"best_added_kg"`
	BestHold    *float64 `json:"best_hold_seconds"`
	TotalSets   int      `json:"total_sets"`
	Tier        string   `json:"tier,omitempty"`
	LastTrained *string  `json:"last_trained"`
}

type CategoryLevel struct {
	Category string `json:"category"`
	Tier     string `json:"tier"`
	TierRank int    `json:"tier_rank"`
	BasedOn  string `json:"based_on"`
}

type Snapshot struct {
	Bodyweight     *float64        `json:"bodyweight_kg"`
	Categories     []CategoryLevel `json:"categories"`
	Records        []Record        `json:"records"`
	SessionsLast7  int             `json:"sessions_last_7_days"`
	SessionsLast28 int             `json:"sessions_last_28_days"`
	SetsLast28     int             `json:"sets_last_28_days"`
	OpenInjuries   []Injury        `json:"open_injuries"`
}

// BuildSnapshot is the single source of truth for "what level is this athlete".
// The AI package consumes it verbatim; it never recomputes anything itself.
func (s *Service) BuildSnapshot(ctx context.Context, user auth.User) (Snapshot, error) {
	snap := Snapshot{
		Bodyweight:   user.BodyweightKg,
		Categories:   []CategoryLevel{},
		Records:      []Record{},
		OpenInjuries: []Injury{},
	}

	rows, err := s.pool.Query(ctx, `
		select e.slug, e.name, e.category,
		       max(ws.reps), max(ws.weight_kg), max(ws.hold_seconds),
		       count(*), to_char(max(w.performed_at), 'YYYY-MM-DD')
		from workout_sets ws
		join exercises e on e.id = ws.exercise_id
		join workouts  w on w.id = ws.workout_id
		where w.user_id = $1
		group by e.slug, e.name, e.category
		order by e.category, e.name`, user.ID)
	if err != nil {
		return snap, err
	}
	defer rows.Close()

	byCategory := map[string]CategoryLevel{}
	rubricBySlug := map[string]rubric{}
	for _, r := range rubrics {
		rubricBySlug[r.Slug] = r
	}

	for rows.Next() {
		var rec Record
		if err := rows.Scan(&rec.Slug, &rec.Name, &rec.Category,
			&rec.BestReps, &rec.BestWeight, &rec.BestHold, &rec.TotalSets, &rec.LastTrained); err != nil {
			return snap, err
		}

		if rb, ok := rubricBySlug[rec.Slug]; ok {
			var value float64
			switch rb.Metric {
			case "reps":
				if rec.BestReps != nil {
					value = float64(*rec.BestReps)
				}
			case "hold":
				if rec.BestHold != nil {
					value = *rec.BestHold
				}
			case "added_kg":
				if rec.BestWeight != nil {
					value = *rec.BestWeight
				}
			}
			rank := tierFor(value, rb.Cuts)
			rec.Tier = tierNames[rank]

			if current, seen := byCategory[rec.Category]; !seen || rank > current.TierRank {
				byCategory[rec.Category] = CategoryLevel{
					Category: rec.Category,
					Tier:     tierNames[rank],
					TierRank: rank,
					BasedOn:  rec.Name,
				}
			}
		}
		snap.Records = append(snap.Records, rec)
	}
	if err := rows.Err(); err != nil {
		return snap, err
	}

	for _, category := range []string{"pull", "push", "static", "dynamic", "weighted", "core", "legs"} {
		if level, ok := byCategory[category]; ok {
			snap.Categories = append(snap.Categories, level)
		} else {
			snap.Categories = append(snap.Categories, CategoryLevel{
				Category: category, Tier: tierNames[0], TierRank: 0, BasedOn: "",
			})
		}
	}

	err = s.pool.QueryRow(ctx, `
		select
			count(distinct w.id) filter (where w.performed_at > now() - interval '7 days'),
			count(distinct w.id) filter (where w.performed_at > now() - interval '28 days'),
			count(ws.id)         filter (where w.performed_at > now() - interval '28 days')
		from workouts w
		left join workout_sets ws on ws.workout_id = w.id
		where w.user_id = $1`, user.ID,
	).Scan(&snap.SessionsLast7, &snap.SessionsLast28, &snap.SetsLast28)
	if err != nil {
		return snap, err
	}

	injuries, err := s.openInjuries(ctx, user.ID)
	if err != nil {
		return snap, err
	}
	snap.OpenInjuries = injuries

	return snap, nil
}

func (s *Service) Level(w http.ResponseWriter, r *http.Request) {
	snap, err := s.BuildSnapshot(r.Context(), auth.MustUser(r.Context()))
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't work out your current level.")
		return
	}
	httpx.JSON(w, http.StatusOK, snap)
}
