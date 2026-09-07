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

// Region is a body area an injury can be recorded against. Ordered, because
// the form shows them in this order and the reference table records it.
type Region struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

var Regions = []Region{
	{"wrist", "Wrist"},
	{"elbow", "Elbow"},
	{"shoulder", "Shoulder"},
	{"chest", "Chest"},
	{"back", "Back"},
	{"core", "Core"},
	{"hip", "Hip"},
	{"knee", "Knee"},
	{"ankle", "Ankle"},
	{"other", "Somewhere else"},
}

var validRegions = func() map[string]bool {
	out := make(map[string]bool, len(Regions))
	for _, r := range Regions {
		out[r.Key] = true
	}
	return out
}()

// Protocol is a curated prehab/rehab block. The model picks from these and
// sequences them; it does not invent rehab of its own. Adding a protocol here
// is how the app learns a new one.
type Protocol struct {
	Slug    string `json:"slug"`
	Region  string `json:"region"`
	Title   string `json:"title"`
	Purpose string `json:"purpose"` // "warmup" or "rehab"
	// Phase is where in a session this belongs, and it is what the session
	// page groups by. The four warm-up phases are RAMP as the research states
	// it — raise, mobilise, potentiate, with joint preparation named
	// separately because in this sport it is the part that gets skipped.
	// A rehab protocol is not a warm-up phase and carries PhaseRehab.
	Phase        string   `json:"phase"`
	Steps        []string `json:"steps"`
	AvoidWhile   []string `json:"avoid_while"`
	SeeClinician string   `json:"see_clinician"`
}

// The phases a session is performed in, in order. Blocks fill PhaseSpecific
// and PhaseTraining by their intent; everything else is protocol work.
const (
	PhaseJoint    = "joint"
	PhaseMuscular = "muscular"
	PhaseMobility = "mobility"
	PhaseSpecific = "specific"
	PhaseTraining = "training"
	PhaseCooldown = "cooldown"
	PhaseRehab    = "rehab"
)

// SessionPhases is the running order, with the name each phase is shown under.
// It is served with the protocols so the page and the planner cannot disagree
// about either the set of phases or the order they come in.
var SessionPhases = []struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Note  string `json:"note"`
}{
	{PhaseJoint, "Joint Warm Up",
		"Every joint the session will load, taken through its range unloaded. Slow, and never into pain."},
	{PhaseMuscular, "Muscular Warm Up",
		"Raise temperature and blood flow. You should be warm and slightly out of breath before anything hard."},
	{PhaseMobility, "Mobility / Dynamic Stretching",
		"Moving through range, not holding an end position. Long static holds before training cost strength for the next hour."},
	{PhaseSpecific, "Specific Warm-Up",
		"The movements of this session, at a fraction of the effort. This is rehearsal and potentiation, not training."},
	{PhaseTraining, "Training",
		"The session itself, in the order it is written: skill and straight-arm work while fresh, then strength, then the rest."},
	{PhaseCooldown, "Cooldown",
		"Bring the breathing down and move gently through what was worked."},
}

// Protocols is intentionally small and hand-checked. Grow it deliberately.
var Protocols = []Protocol{
	// The three that open every session. They were one protocol until the
	// session page started showing the warm-up as the phases it is actually
	// performed in — at which point "general_warmup" turned out to be four
	// lines belonging to four different phases, which is exactly the warm-up
	// an athlete skips three quarters of.
	{
		Slug: "joint_warmup", Region: "other", Title: "Joints, unloaded", Purpose: "warmup", Phase: PhaseJoint,
		Steps: []string{
			"Ankles, knees and hips: 10 slow circles each, from the ground up.",
			"Spine: 10 cat-cows, then 10 seated rotations each way.",
			"Shoulders: 10 slow circles back, 10 forward, arms long.",
			"Elbows: 10 full flexions and extensions, then 10 rotations of the forearm.",
			"Wrists: 10 circles each way, then 10 slow flexions and extensions per hand.",
		},
		AvoidWhile:   []string{},
		SeeClinician: "A joint that is painful before it has been loaded at all is not a warm-up problem.",
	},
	{
		Slug: "muscular_warmup", Region: "other", Title: "Raise", Purpose: "warmup", Phase: PhaseMuscular,
		Steps: []string{
			"Five minutes of easy cardio — skipping, rowing, a brisk walk — until you are warm and slightly out of breath.",
			"Push-ups at half effort: 2 sets of 10.",
			"Rows or an active hang at half effort: 2 sets of 10.",
			"Bodyweight squats: 1 set of 15, unhurried.",
		},
		AvoidWhile:   []string{},
		SeeClinician: "",
	},
	{
		Slug: "mobility_warmup", Region: "other", Title: "Mobilise", Purpose: "warmup", Phase: PhaseMobility,
		Steps: []string{
			"Band or stick dislocates: 2 sets of 10, straight arms, as wide as you need.",
			"Wall slides: 2 sets of 10, ribs down.",
			"Deep squat, 5 slow rocks side to side, then stand and repeat once.",
			"Leg swings, front to back and across: 10 each way per leg.",
			"Thoracic rotations on all fours: 8 per side.",
		},
		AvoidWhile:   []string{},
		SeeClinician: "",
	},
	{
		Slug: "wrist_warmup", Region: "wrist", Title: "Wrist preparation", Purpose: "warmup", Phase: PhaseJoint,
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
		Slug: "wrist_rehab_light", Region: "wrist", Title: "Wrist irritation: reduced-load work", Purpose: "rehab", Phase: PhaseRehab,
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
		Slug: "straight_arm_warmup", Region: "elbow", Title: "Straight-arm and elbow preparation", Purpose: "warmup", Phase: PhaseSpecific,
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
		Slug: "shoulder_warmup", Region: "shoulder", Title: "Shoulder preparation", Purpose: "warmup", Phase: PhaseMobility,
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
		Slug: "shoulder_rehab_light", Region: "shoulder", Title: "Shoulder irritation: reduced-load work", Purpose: "rehab", Phase: PhaseRehab,
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
		Slug: "chest_shoulder_girdle_warmup", Region: "chest", Title: "Chest and girdle preparation", Purpose: "warmup", Phase: PhaseMuscular,
		Steps: []string{
			"Push-ups at half effort: 2 sets of 10.",
			"Band chest flyes: 2 sets of 15.",
			"Ring or bar support hold with a slight turn-out: 3 holds of 15 seconds.",
			"Slow eccentric dips: 2 sets of 5, 4 seconds down.",
		},
		AvoidWhile:   []string{"sharp pain at the sternum"},
		SeeClinician: "A sudden tearing sensation during a dip or press needs urgent assessment.",
	},
	// Retained because sessions saved before the split still name it. The
	// planner no longer selects it.
	{
		Slug: "general_warmup", Region: "other", Title: "General session warm-up", Purpose: "warmup", Phase: PhaseMuscular,
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
