package training

import (
	"context"
	"net/http"
	"time"

	"calisthenics/api/internal/auth"
	"calisthenics/api/internal/httpx"
)

type Injury struct {
	ID          string     `json:"id"`
	Region      string     `json:"region"`
	Severity    int        `json:"severity"`
	Description string     `json:"description"`
	StartedOn   time.Time  `json:"started_on"`
	ResolvedOn  *time.Time `json:"resolved_on"`
}

var validRegions = map[string]bool{
	"wrist": true, "elbow": true, "shoulder": true, "chest": true, "back": true,
	"core": true, "hip": true, "knee": true, "ankle": true, "other": true,
}

// Protocol is a curated prehab/rehab block. The model picks from these and
// sequences them; it does not invent rehab of its own. Adding a protocol here
// is how the app learns a new one.
type Protocol struct {
	Slug        string   `json:"slug"`
	Region      string   `json:"region"`
	Title       string   `json:"title"`
	Purpose     string   `json:"purpose"` // "warmup" or "rehab"
	Steps       []string `json:"steps"`
	AvoidWhile  []string `json:"avoid_while"`
	SeeClinician string  `json:"see_clinician"`
}

// Protocols is intentionally small and hand-checked. Grow it deliberately.
var Protocols = []Protocol{
	{
		Slug: "wrist_warmup", Region: "wrist", Title: "Wrist preparation", Purpose: "warmup",
		Steps: []string{
			"Palms down on the floor, fingers forward: rock forward and back, 10 slow reps.",
			"Palms down, fingers pointing back toward the knees: rock back gently, 10 reps.",
			"Backs of the hands on the floor, fingers forward: press down lightly, 10 reps.",
			"Fists on the floor, knuckle push-up position: shift weight side to side, 10 reps.",
			"Finger-tip pulses on the floor, 20 short pulses.",
		},
		AvoidWhile:   []string{"sharp pain on load", "recent fracture"},
		SeeClinician: "Wrist pain that persists beyond two weeks, or any numbness or tingling, needs a clinician rather than a warm-up.",
	},
	{
		Slug: "wrist_rehab_light", Region: "wrist", Title: "Wrist irritation: reduced-load work", Purpose: "rehab",
		Steps: []string{
			"Move floor pressing to parallettes or push-up handles so the wrist stays neutral.",
			"Replace straight-arm floor holds with hanging work for two weeks.",
			"Isometric wrist extension against the other hand: 5 holds of 20 seconds, pain-free effort only.",
			"Eccentric wrist curls with a very light weight: 3 sets of 12, 3 seconds down.",
			"Reintroduce floor loading only once the isometrics are entirely pain-free.",
		},
		AvoidWhile:   []string{"planche work", "floor handstand", "false grip work"},
		SeeClinician: "Swelling, night pain, or pain that has not improved in two weeks should be assessed in person.",
	},
	{
		Slug: "straight_arm_warmup", Region: "elbow", Title: "Straight-arm and elbow preparation", Purpose: "warmup",
		Steps: []string{
			"Scapular pull-ups: 2 sets of 8, slow and controlled.",
			"German hang, easing in: 2 holds of 20 seconds.",
			"Straight-arm band pulldowns: 2 sets of 15, elbows locked.",
			"Tuck front lever holds at low intensity: 3 holds of 8 seconds.",
			"Light biceps and forearm curls: 2 sets of 15 to load the elbow before heavy straight-arm work.",
		},
		AvoidWhile:   []string{"acute elbow pain"},
		SeeClinician: "Inner elbow pain that sharpens on straight-arm loading is common and slow to heal; get it looked at early.",
	},
	{
		Slug: "shoulder_warmup", Region: "shoulder", Title: "Shoulder preparation", Purpose: "warmup",
		Steps: []string{
			"Band shoulder dislocates: 2 sets of 10, straight arms, wide grip.",
			"Scapular push-ups: 2 sets of 10.",
			"Wall slides: 2 sets of 10.",
			"External rotation with a light band: 2 sets of 15 per side.",
			"Support hold on parallel bars, shoulders depressed: 3 holds of 15 seconds.",
		},
		AvoidWhile:   []string{"recent dislocation", "sharp pain overhead"},
		SeeClinician: "Pain with overhead reaching that does not settle within two weeks, or any sense of instability, needs assessment.",
	},
	{
		Slug: "shoulder_rehab_light", Region: "shoulder", Title: "Shoulder irritation: reduced-load work", Purpose: "rehab",
		Steps: []string{
			"Pause all overhead pressing and dips until pain-free at rest.",
			"Keep pulling volume but reduce range: stop the pull-up short of full extension for two weeks.",
			"Band external rotation: 3 sets of 15 per side, daily, light.",
			"Prone Y and T raises with no weight: 3 sets of 10.",
			"Reintroduce dips at half the previous volume once ten pain-free push-ups are possible.",
		},
		AvoidWhile:   []string{"dips", "handstand push-ups", "muscle-ups"},
		SeeClinician: "Weakness rather than pain, or pain waking you at night, should be assessed promptly.",
	},
	{
		Slug: "chest_shoulder_girdle_warmup", Region: "chest", Title: "Chest and girdle preparation", Purpose: "warmup",
		Steps: []string{
			"Push-ups at half effort: 2 sets of 10.",
			"Band chest flyes: 2 sets of 15.",
			"Ring or bar support hold with a slight turn-out: 3 holds of 15 seconds.",
			"Slow eccentric dips: 2 sets of 5, 4 seconds down.",
		},
		AvoidWhile:   []string{"sharp pain at the sternum"},
		SeeClinician: "A sudden tearing sensation during a dip or press needs urgent assessment.",
	},
	{
		Slug: "general_warmup", Region: "other", Title: "General session warm-up", Purpose: "warmup",
		Steps: []string{
			"Five minutes of easy cardio to raise temperature.",
			"Wrist preparation circuit.",
			"Shoulder preparation circuit.",
			"Two ramp-up sets of the first main exercise, at roughly half and three-quarters effort.",
		},
		AvoidWhile:   []string{},
		SeeClinician: "",
	},
}

func (s *Service) ListProtocols(w http.ResponseWriter, r *http.Request) {
	region := r.URL.Query().Get("region")
	out := []Protocol{}
	for _, p := range Protocols {
		if region == "" || p.Region == region {
			out = append(out, p)
		}
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (s *Service) openInjuries(ctx context.Context, userID string) ([]Injury, error) {
	rows, err := s.pool.Query(ctx, `
		select id, region, severity, description, started_on, resolved_on
		from injuries where user_id = $1 and resolved_on is null
		order by started_on desc`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Injury{}
	for rows.Next() {
		var in Injury
		if err := rows.Scan(&in.ID, &in.Region, &in.Severity, &in.Description, &in.StartedOn, &in.ResolvedOn); err != nil {
			return nil, err
		}
		out = append(out, in)
	}
	return out, rows.Err()
}

func (s *Service) ListInjuries(w http.ResponseWriter, r *http.Request) {
	me := auth.MustUser(r.Context())
	rows, err := s.pool.Query(r.Context(), `
		select id, region, severity, description, started_on, resolved_on
		from injuries where user_id = $1 order by started_on desc`, me.ID)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't load your injury history.")
		return
	}
	defer rows.Close()

	out := []Injury{}
	for rows.Next() {
		var in Injury
		if err := rows.Scan(&in.ID, &in.Region, &in.Severity, &in.Description, &in.StartedOn, &in.ResolvedOn); err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Couldn't read your injury history.")
			return
		}
		out = append(out, in)
	}
	httpx.JSON(w, http.StatusOK, out)
}

type injuryInput struct {
	Region      string `json:"region"`
	Severity    int    `json:"severity"`
	Description string `json:"description"`
}

func (s *Service) CreateInjury(w http.ResponseWriter, r *http.Request) {
	var in injuryInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	if !validRegions[in.Region] {
		httpx.Fail(w, http.StatusBadRequest, "Pick a body region from the list.")
		return
	}
	if in.Severity < 1 || in.Severity > 5 {
		httpx.Fail(w, http.StatusBadRequest, "Rate the severity from 1 to 5.")
		return
	}

	me := auth.MustUser(r.Context())
	var out Injury
	err := s.pool.QueryRow(r.Context(), `
		insert into injuries (user_id, region, severity, description)
		values ($1, $2, $3, $4)
		returning id, region, severity, description, started_on, resolved_on`,
		me.ID, in.Region, in.Severity, in.Description,
	).Scan(&out.ID, &out.Region, &out.Severity, &out.Description, &out.StartedOn, &out.ResolvedOn)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't record that injury.")
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (s *Service) ResolveInjury(w http.ResponseWriter, r *http.Request) {
	me := auth.MustUser(r.Context())
	tag, err := s.pool.Exec(r.Context(), `
		update injuries set resolved_on = current_date
		where id = $1 and user_id = $2 and resolved_on is null`,
		r.PathValue("id"), me.ID)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't update that injury.")
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Fail(w, http.StatusNotFound, "That injury is already resolved or doesn't exist.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}
