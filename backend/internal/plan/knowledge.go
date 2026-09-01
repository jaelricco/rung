package plan

import (
	"regexp"
	"strconv"
	"strings"

	"calisthenics/api/internal/training"
)

// The knowledge the planner reasons from. Every number here is sourced in
// docs/training-research.md; the two are meant to be edited together. What is
// deliberately *not* here is anything about one athlete — that all comes from
// the snapshot, so the same edit changes every plan the app will ever write.

// How a rung is measured. These mirror the set kinds an athlete can log, which
// is what makes a standard checkable rather than aspirational.
const (
	metricHold    = "hold"    // seconds
	metricReps    = "reps"    // clean repetitions
	metricAdded   = "added"   // kilograms on the belt
	metricAttempt = "attempt" // made or missed
)

// A chain is a list of candidate slugs in preference order. The first one that
// exists in the library and survives the injury filter is used. One mechanism
// covers three problems: a library that a migration changed underneath us, an
// athlete who cannot load a joint this month, and a rung that has more than one
// honest way to train it.
type chain []string

// Step is one rung of a ladder: what to train, and what clears it.
type Step struct {
	Name string
	// Movement is the rung itself — what the athlete works and is measured on.
	Movement chain
	Metric   string
	Standard float64
	// Assist is the drill that builds this rung, trained beside it.
	Assist chain
	// Typical is how long this rung usually takes, in the sources" words.
	Typical string
}

// Goal is one thing an athlete can train toward, with the ladder to it.
type Goal struct {
	Key  string
	Name string
	// Phrase is the goal in a sentence — "the front lever", "balanced
	// strength" — because the name alone does not read as either.
	Phrase string
	// Aliases are matched against whatever the athlete typed, in English and
	// German. Longest match wins, so "ring muscle up" beats "muscle up".
	Aliases []string
	// Pattern is the movement pattern the goal belongs to, which decides which
	// days carry the skill work and which days are its opposite.
	Pattern string // pull, push, legs, core
	// StraightArm marks a goal whose work loads the elbow at full extension.
	// Those sessions get elbow preparation, always, no exceptions.
	StraightArm bool
	// Wrists marks a goal loaded through the hands on the floor.
	Wrists      bool
	Ladder      []Step
	Drills      chain
	Accessories chain
	// Risks is what the injury tables say goes wrong here, in one line each.
	Risks []string
	// Timeline is an honest expectation, because a plan that implies a full
	// planche in eight weeks has already failed.
	Timeline string
	// Frequency is the note on how often the skill wants to be touched.
	Frequency string
	// Foundation marks a goal that has no separate skill to practise: its
	// ladder *is* the strength work, so its sessions read as strength rather
	// than as skill practice with strength behind it.
	Foundation bool
}

// Patterns a session can work.
const (
	patternPull = "pull"
	patternPush = "push"
	patternLegs = "legs"
	patternCore = "core"
)

// Goals is the catalogue, ordered so that the more specific goals are offered
// to the matcher first.
var Goals = []Goal{
	{
		Key: "ring_muscle_up", Phrase: "the ring muscle-up", Name: "Ring muscle-up", Pattern: patternPull,
		Aliases:  []string{"ring muscle up", "ringe muscle up", "rmu"},
		Timeline: "12 to 20 weeks from ten strict pull-ups, longer if the false grip is new.",
		Ladder: []Step{
			{Name: "Pulling base", Movement: chain{"pull_up"}, Metric: metricReps, Standard: 10,
				Assist: chain{"australian_row"}, Typical: "however long it takes; everything else waits on it"},
			{Name: "False grip", Movement: chain{"false_grip_hang"}, Metric: metricHold, Standard: 20,
				Assist: chain{"false_grip_row"}, Typical: "3 to 6 weeks, limited by wrist tolerance"},
			{Name: "Ring pulling and dipping", Movement: chain{"false_grip_row", "ring_dip"}, Metric: metricReps, Standard: 8,
				Assist: chain{"ring_dip", "straight_bar_dip"}, Typical: "4 to 8 weeks"},
			{Name: "The transition", Movement: chain{"muscle_up_negative"}, Metric: metricReps, Standard: 4,
				Assist: chain{"jumping_muscle_up"}, Typical: "4 to 8 weeks"},
			{Name: "Ring muscle-up", Movement: chain{"ring_muscle_up"}, Metric: metricReps, Standard: 2,
				Assist: chain{"muscle_up_negative"}, Typical: "the goal"},
		},
		Drills:      chain{"explosive_pull_up", "false_grip_hang", "straight_bar_dip"},
		Accessories: chain{"band_face_pull", "hanging_leg_raise", "scapular_pull_up"},
		Risks: []string{
			"The false grip is the usual reason a ring muscle-up plan stops: it loads the wrist in a position nothing else does.",
			"Dynamic work is the single largest slice of the calisthenics injury tables. Attempts go early, fresh, and in low volume.",
		},
		Frequency: "Two dedicated sessions a week; false-grip hangs can be greased in on off days.",
	},
	{
		Key: "muscle_up", Phrase: "the bar muscle-up", Name: "Bar muscle-up", Pattern: patternPull,
		Aliases:  []string{"muscle up", "muscleup", "muscle-up", "mu", "bar muscle up", "klimmzug umschwung"},
		Timeline: "8 to 12 weeks for someone who already has eight strict pull-ups.",
		Ladder: []Step{
			{Name: "Pulling base", Movement: chain{"pull_up"}, Metric: metricReps, Standard: 8,
				Assist: chain{"australian_row", "negative_pull_up"}, Typical: "everything else waits on this"},
			{Name: "Top position", Movement: chain{"straight_bar_dip"}, Metric: metricReps, Standard: 8,
				Assist: chain{"dip"}, Typical: "3 to 5 weeks; the half of the movement people forget to train"},
			{Name: "Explosive pull", Movement: chain{"explosive_pull_up"}, Metric: metricReps, Standard: 6,
				Assist: chain{"pull_up"}, Typical: "3 to 5 weeks"},
			{Name: "The transition", Movement: chain{"muscle_up_negative"}, Metric: metricReps, Standard: 4,
				Assist: chain{"jumping_muscle_up"}, Typical: "4 to 8 weeks — this is where attempts stall, and it is technique, not strength"},
			{Name: "Bar muscle-up", Movement: chain{"muscle_up"}, Metric: metricReps, Standard: 3,
				Assist: chain{"explosive_pull_up"}, Typical: "the goal"},
		},
		Drills:      chain{"explosive_pull_up", "straight_bar_dip", "false_grip_hang"},
		Accessories: chain{"band_face_pull", "hanging_leg_raise", "australian_row"},
		Risks: []string{
			"Around 90 percent of failed attempts stall at the transition. Adding pulling volume does not fix a path problem.",
			"Muscle-ups account for roughly one in eight recorded calisthenics injuries. Never train them to failure or under a clock.",
		},
		Frequency: "Two sessions a week with attempts, a third day of pulling volume.",
	},
	{
		Key: "one_arm_pull_up", Phrase: "the one-arm pull-up", Name: "One-arm pull-up", Pattern: patternPull,
		Aliases:  []string{"one arm pull up", "one-arm pull-up", "oap", "einarmiger klimmzug", "one arm chin up"},
		Timeline: "One to three years from a solid two-arm pull-up. It is the longest ladder in the app.",
		Ladder: []Step{
			{Name: "Two-arm base", Movement: chain{"pull_up"}, Metric: metricReps, Standard: 15,
				Assist: chain{"australian_row"}, Typical: "the entry price"},
			{Name: "Loaded pulling", Movement: chain{"weighted_pull_up"}, Metric: metricAdded, Standard: 20,
				Assist: chain{"pull_up"}, Typical: "3 to 6 months; roughly a third of bodyweight"},
			{Name: "Archer", Movement: chain{"archer_pull_up"}, Metric: metricReps, Standard: 5,
				Assist: chain{"wide_pull_up"}, Typical: "2 to 4 months"},
			{Name: "Typewriter", Movement: chain{"typewriter_pull_up"}, Metric: metricReps, Standard: 5,
				Assist: chain{"archer_pull_up"}, Typical: "2 to 4 months"},
			{Name: "Assisted one-arm", Movement: chain{"assisted_one_arm_pull_up"}, Metric: metricReps, Standard: 5,
				Assist: chain{"one_arm_dead_hang"}, Typical: "3 to 6 months, assist reduced a little at a time"},
			{Name: "One-arm negative", Movement: chain{"one_arm_negative"}, Metric: metricReps, Standard: 3,
				Assist: chain{"assisted_one_arm_pull_up"}, Typical: "2 to 4 months"},
			{Name: "One-arm pull-up", Movement: chain{"one_arm_pull_up"}, Metric: metricAttempt, Standard: 1,
				Assist: chain{"one_arm_negative"}, Typical: "the goal"},
		},
		Drills:      chain{"one_arm_dead_hang", "archer_pull_up", "weighted_pull_up"},
		Accessories: chain{"band_face_pull", "elbow_prep_circuit", "hanging_leg_raise"},
		Risks: []string{
			"Elbow tendinopathy is the classic one-arm injury, and it is slow. Elbow preparation is not optional on this ladder.",
			"The assist has to come down in small steps. A towel that suddenly does nothing is how the elbow finds out.",
		},
		Frequency: "Two heavy sessions a week, no more. This ladder is limited by tendon, not by will.",
	},
	{
		Key: "weighted_pull_up", Phrase: "a heavy weighted pull-up", Name: "Weighted pull-up", Pattern: patternPull,
		Aliases:  []string{"weighted pull up", "weighted pullup", "weighted chin up", "gewichtete klimmzuge", "gewichtete klimmzüge", "zusatzgewicht klimmzug", "klimmzug mit gewicht"},
		Timeline: "Bodyweight-plus-half in one to three years of consistent loading.",
		Ladder: []Step{
			{Name: "Strict base", Movement: chain{"pull_up"}, Metric: metricReps, Standard: 10,
				Assist: chain{"australian_row"}, Typical: "before any belt goes on"},
			{Name: "First load", Movement: chain{"weighted_pull_up"}, Metric: metricAdded, Standard: 12,
				Assist: chain{"pull_up"}, Typical: "8 to 12 weeks"},
			{Name: "Quarter bodyweight", Movement: chain{"weighted_pull_up"}, Metric: metricAdded, Standard: 24,
				Assist: chain{"pull_up"}, Typical: "3 to 6 months"},
			{Name: "Heavy", Movement: chain{"weighted_pull_up"}, Metric: metricAdded, Standard: 40,
				Assist: chain{"archer_pull_up"}, Typical: "6 to 12 months"},
			{Name: "Elite", Movement: chain{"weighted_pull_up"}, Metric: metricAdded, Standard: 60,
				Assist: chain{"archer_pull_up"}, Typical: "the goal"},
		},
		Drills:      chain{"pull_up", "australian_row"},
		Accessories: chain{"band_face_pull", "scapular_pull_up", "hanging_leg_raise"},
		Risks: []string{
			"Load exposes the elbow before it exposes the back. Ramp the belt, not the ego.",
			"A belt on a movement you own for fewer than eight strict reps is load on a position you cannot yet hold.",
		},
		Frequency: "Two heavy sessions a week, spaced by 72 hours where the week allows.",
	},
	{
		Key: "weighted_dip", Phrase: "a heavy weighted dip", Name: "Weighted dip", Pattern: patternPush,
		Aliases:  []string{"weighted dip", "weighted dips", "gewichtete dips", "dips mit gewicht"},
		Timeline: "Bodyweight-plus-half in one to three years of consistent loading.",
		Ladder: []Step{
			{Name: "Strict base", Movement: chain{"dip"}, Metric: metricReps, Standard: 12,
				Assist: chain{"straight_bar_dip", "push_up"}, Typical: "before any belt goes on"},
			{Name: "First load", Movement: chain{"weighted_dip"}, Metric: metricAdded, Standard: 16,
				Assist: chain{"dip"}, Typical: "8 to 12 weeks"},
			{Name: "Quarter bodyweight", Movement: chain{"weighted_dip"}, Metric: metricAdded, Standard: 32,
				Assist: chain{"dip"}, Typical: "3 to 6 months"},
			{Name: "Heavy", Movement: chain{"weighted_dip"}, Metric: metricAdded, Standard: 55,
				Assist: chain{"ring_dip"}, Typical: "6 to 12 months"},
			{Name: "Elite", Movement: chain{"weighted_dip"}, Metric: metricAdded, Standard: 80,
				Assist: chain{"ring_dip"}, Typical: "the goal"},
		},
		Drills:      chain{"dip", "straight_bar_dip"},
		Accessories: chain{"band_face_pull", "australian_row", "scapular_push_up"},
		Risks: []string{
			"The bottom of a loaded dip is the most reported shoulder position in the sport. Depth stops where the shoulder stays packed.",
			"Sternum pain that arrives suddenly during a dip is not soreness. Stop and get it looked at.",
		},
		Frequency: "Two heavy sessions a week, with pulling volume to match, or the shoulder pays for it.",
	},
	{
		Key: "front_lever", Phrase: "the front lever", Name: "Front lever", Pattern: patternPull, StraightArm: true,
		Aliases:  []string{"front lever", "frontlever", "front-lever", "fl"},
		Timeline: "A straddle in six to twelve months for someone with a solid tuck; a full lever is often a two-year project.",
		Ladder: []Step{
			{Name: "Inversion and body line", Movement: chain{"inverted_hang"}, Metric: metricHold, Standard: 20,
				Assist: chain{"hollow_body_hold"}, Typical: "2 to 4 weeks"},
			{Name: "Tuck", Movement: chain{"tuck_front_lever"}, Metric: metricHold, Standard: 20,
				Assist: chain{"tuck_front_lever_row"}, Typical: "4 to 10 weeks"},
			{Name: "Advanced tuck", Movement: chain{"adv_tuck_front_lever"}, Metric: metricHold, Standard: 15,
				Assist: chain{"tuck_front_lever_row"}, Typical: "8 to 16 weeks — the longest rung for most people"},
			{Name: "Straddle", Movement: chain{"straddle_front_lever"}, Metric: metricHold, Standard: 12,
				Assist: chain{"front_lever_raise"}, Typical: "8 to 20 weeks"},
			{Name: "Full front lever", Movement: chain{"front_lever"}, Metric: metricHold, Standard: 10,
				Assist: chain{"front_lever_raise", "ice_cream_maker"}, Typical: "the goal"},
		},
		Drills:      chain{"tuck_front_lever_row", "front_lever_raise", "ice_cream_maker", "scapular_pull_up"},
		Accessories: chain{"pull_up", "hanging_leg_raise", "hollow_body_hold", "band_face_pull"},
		Risks: []string{
			"Front levers are about one in ten recorded calisthenics injuries, and the elbow takes most of them.",
			"A hold that sags is not a shorter hold, it is a different exercise. Stop the set when the line breaks.",
		},
		Frequency: "Two to three sessions a week, holds well short of failure.",
	},
	{
		Key: "back_lever", Phrase: "the back lever", Name: "Back lever", Pattern: patternPull, StraightArm: true,
		Aliases:  []string{"back lever", "backlever", "back-lever", "bl"},
		Timeline: "Three to nine months, faster than the front lever for most people.",
		Ladder: []Step{
			{Name: "Shoulder extension", Movement: chain{"german_hang"}, Metric: metricHold, Standard: 30,
				Assist: chain{"skin_the_cat"}, Typical: "2 to 6 weeks, and rushed at your shoulder's expense"},
			{Name: "Tuck", Movement: chain{"tuck_back_lever"}, Metric: metricHold, Standard: 20,
				Assist: chain{"skin_the_cat"}, Typical: "3 to 6 weeks"},
			{Name: "Advanced tuck", Movement: chain{"adv_tuck_back_lever"}, Metric: metricHold, Standard: 15,
				Assist: chain{"inverted_hang"}, Typical: "4 to 8 weeks"},
			{Name: "Straddle", Movement: chain{"straddle_back_lever"}, Metric: metricHold, Standard: 12,
				Assist: chain{"german_hang"}, Typical: "6 to 12 weeks"},
			{Name: "Full back lever", Movement: chain{"back_lever"}, Metric: metricHold, Standard: 10,
				Assist: chain{"straddle_back_lever"}, Typical: "the goal"},
		},
		Drills:      chain{"skin_the_cat", "german_hang", "inverted_hang"},
		Accessories: chain{"pull_up", "band_face_pull", "hollow_body_hold"},
		Risks: []string{
			"The back lever is the most common way to find out your biceps tendon was not ready. Open the lever slowly.",
			"Shoulder extension has to be earned in the German hang before it is loaded horizontally.",
		},
		Frequency: "Two sessions a week is plenty; this position is hard on connective tissue.",
	},
	{
		Key: "planche", Phrase: "the planche", Name: "Planche", Pattern: patternPush, StraightArm: true, Wrists: true,
		Aliases:  []string{"planche", "planch", "full planche", "straddle planche", "tuck planche"},
		Timeline: "Tuck around a year, straddle at two to three, full at three to five and up. An eight-week plan buys you a rung, not the skill.",
		Ladder: []Step{
			{Name: "Frog stand", Movement: chain{"frog_stand"}, Metric: metricHold, Standard: 30,
				Assist: chain{"crow_pose"}, Typical: "2 to 6 weeks; the first straight-arm balance"},
			{Name: "Lean and wrists", Movement: chain{"planche_lean"}, Metric: metricHold, Standard: 30,
				Assist: chain{"pseudo_planche_push_up"}, Typical: "4 to 8 weeks, and the wrists set the pace"},
			{Name: "Tuck planche", Movement: chain{"tuck_planche"}, Metric: metricHold, Standard: 15,
				Assist: chain{"planche_lean"}, Typical: "8 to 20 weeks"},
			{Name: "Advanced tuck", Movement: chain{"adv_tuck_planche"}, Metric: metricHold, Standard: 12,
				Assist: chain{"tuck_planche_push_up"}, Typical: "3 to 9 months"},
			{Name: "Straddle planche", Movement: chain{"straddle_planche"}, Metric: metricHold, Standard: 10,
				Assist: chain{"pseudo_planche_push_up"}, Typical: "6 to 18 months"},
			{Name: "Full planche", Movement: chain{"full_planche"}, Metric: metricHold, Standard: 5,
				Assist: chain{"straddle_planche"}, Typical: "the goal, and it is measured in years"},
		},
		Drills:      chain{"pseudo_planche_push_up", "planche_lean", "scapular_push_up"},
		Accessories: chain{"dip", "band_face_pull", "hollow_body_hold", "wrist_extensor_curl"},
		Risks: []string{
			"Planche work is about one in eight recorded injuries, split between wrist and shoulder.",
			"Wrist preparation before every session, and parallettes whenever the floor position hurts.",
		},
		Frequency: "Two to three sessions a week, and the lean can be greased in daily at low intensity.",
	},
	{
		Key: "handstand_push_up", Phrase: "the handstand push-up", Name: "Handstand push-up", Pattern: patternPush, Wrists: true,
		Aliases:  []string{"handstand push up", "handstand pushup", "hspu", "handstand liegestutz", "handstand liegestütz"},
		Timeline: "Wall reps in three to six months from a solid pike push-up; freestanding is a different sport again.",
		Ladder: []Step{
			{Name: "Pike strength", Movement: chain{"pike_push_up"}, Metric: metricReps, Standard: 12,
				Assist: chain{"push_up"}, Typical: "4 to 8 weeks"},
			{Name: "Elevated pike", Movement: chain{"elevated_pike_push_up"}, Metric: metricReps, Standard: 10,
				Assist: chain{"pike_push_up"}, Typical: "4 to 8 weeks"},
			{Name: "Negatives", Movement: chain{"negative_hspu"}, Metric: metricReps, Standard: 5,
				Assist: chain{"wall_handstand"}, Typical: "4 to 8 weeks"},
			{Name: "Wall handstand push-up", Movement: chain{"wall_hspu"}, Metric: metricReps, Standard: 5,
				Assist: chain{"negative_hspu"}, Typical: "8 to 16 weeks"},
			{Name: "Freestanding or deficit", Movement: chain{"handstand_push_up", "deficit_hspu"}, Metric: metricReps, Standard: 3,
				Assist: chain{"wall_hspu"}, Typical: "the goal"},
		},
		Drills:      chain{"wall_handstand", "elevated_pike_push_up", "scapular_push_up"},
		Accessories: chain{"dip", "band_face_pull", "hollow_body_hold", "wrist_prep"},
		Risks: []string{
			"Overhead pressing volume without matching pulling volume is the shortest path to a shoulder that clicks.",
			"Wrists take the whole session. Prepare them, and use a wedge or parallettes if extension is the limit.",
		},
		Frequency: "Two pressing sessions a week, plus handstand line work on the days between.",
	},
	{
		Key: "handstand", Phrase: "a freestanding handstand", Name: "Freestanding handstand", Pattern: patternPush, Wrists: true,
		Aliases:  []string{"handstand", "hand stand", "handstand hold", "freestanding handstand", "hs"},
		Timeline: "Three to twelve months to a held freestanding handstand, decided mostly by how often you practise.",
		Ladder: []Step{
			{Name: "Wall line", Movement: chain{"wall_handstand"}, Metric: metricHold, Standard: 60,
				Assist: chain{"wall_walk"}, Typical: "4 to 10 weeks"},
			{Name: "Weight shifts", Movement: chain{"handstand_shoulder_taps"}, Metric: metricReps, Standard: 10,
				Assist: chain{"wall_handstand"}, Typical: "4 to 8 weeks"},
			{Name: "First freestanding seconds", Movement: chain{"handstand"}, Metric: metricHold, Standard: 10,
				Assist: chain{"crow_pose"}, Typical: "2 to 6 months"},
			{Name: "Held handstand", Movement: chain{"handstand"}, Metric: metricHold, Standard: 30,
				Assist: chain{"handstand_shoulder_taps"}, Typical: "the goal"},
		},
		Drills:      chain{"wall_walk", "crow_pose", "wall_handstand"},
		Accessories: chain{"wrist_prep", "hollow_body_hold", "band_face_pull", "elevated_pike_push_up"},
		Risks: []string{
			"Wrist extension under load is the limiter for most people, and it responds to preparation, not to grinding.",
			"Balance is a nervous-system skill: frequency beats volume, and a fatigued handstand rehearses a bad one.",
		},
		Frequency: "Practise most days, short and fresh. Five sessions of ten minutes beat one of fifty.",
	},
	{
		Key: "human_flag", Phrase: "the human flag", Name: "Human flag", Pattern: patternCore,
		Aliases:  []string{"human flag", "humanflag", "flag", "menschliche flagge", "fahne", "flagge"},
		Timeline: "Six to eighteen months, and it is as much a side-body project as a pressing one.",
		Ladder: []Step{
			{Name: "Side-body strength", Movement: chain{"side_plank"}, Metric: metricHold, Standard: 45,
				Assist: chain{"copenhagen_plank"}, Typical: "3 to 6 weeks"},
			{Name: "Clutch flag", Movement: chain{"clutch_flag"}, Metric: metricHold, Standard: 10,
				Assist: chain{"side_plank"}, Typical: "4 to 10 weeks"},
			{Name: "Negatives", Movement: chain{"flag_negative"}, Metric: metricReps, Standard: 5,
				Assist: chain{"clutch_flag"}, Typical: "4 to 10 weeks"},
			{Name: "Tuck flag", Movement: chain{"tuck_human_flag"}, Metric: metricHold, Standard: 10,
				Assist: chain{"flag_negative"}, Typical: "8 to 16 weeks"},
			{Name: "Straddle flag", Movement: chain{"straddle_human_flag"}, Metric: metricHold, Standard: 8,
				Assist: chain{"tuck_human_flag"}, Typical: "3 to 9 months"},
			{Name: "Full human flag", Movement: chain{"human_flag"}, Metric: metricHold, Standard: 5,
				Assist: chain{"straddle_human_flag"}, Typical: "the goal"},
		},
		Drills:      chain{"copenhagen_plank", "side_plank", "flag_negative"},
		Accessories: chain{"pull_up", "dip", "hanging_leg_raise", "band_face_pull"},
		Risks: []string{
			"The bottom shoulder presses and the top shoulder pulls; whichever is weaker is where the pain shows up.",
			"Obliques and adductors are the quiet limiters. Copenhagen planks are in the plan for a reason.",
		},
		Frequency: "Two flag sessions a week, with side-body work on a third day.",
	},
	{
		Key: "l_sit", Phrase: "the L-sit and V-sit", Name: "L-sit and V-sit", Pattern: patternCore,
		Aliases:  []string{"l sit", "lsit", "l-sit", "l sitz", "v sit", "vsit", "v-sit"},
		Timeline: "A clean L-sit in two to four months; the V-sit is a year-plus project of compression and hamstrings.",
		Ladder: []Step{
			{Name: "Compression", Movement: chain{"compression_leg_lift"}, Metric: metricReps, Standard: 10,
				Assist: chain{"seated_pike_stretch"}, Typical: "3 to 6 weeks"},
			{Name: "Tuck L-sit", Movement: chain{"tuck_l_sit"}, Metric: metricHold, Standard: 30,
				Assist: chain{"hollow_body_hold"}, Typical: "3 to 6 weeks"},
			{Name: "One leg", Movement: chain{"one_leg_l_sit"}, Metric: metricHold, Standard: 20,
				Assist: chain{"compression_leg_lift"}, Typical: "4 to 8 weeks"},
			{Name: "L-sit", Movement: chain{"l_sit"}, Metric: metricHold, Standard: 30,
				Assist: chain{"hanging_leg_raise"}, Typical: "8 to 16 weeks"},
			{Name: "V-sit", Movement: chain{"v_sit"}, Metric: metricHold, Standard: 10,
				Assist: chain{"compression_leg_lift"}, Typical: "the goal"},
		},
		Drills:      chain{"compression_leg_lift", "hanging_leg_raise", "seated_pike_stretch"},
		Accessories: chain{"dip", "pull_up", "hollow_body_hold", "pancake_stretch"},
		Risks: []string{
			"Hip flexor cramp is normal and passes; hamstring pain at the sit bone does not, and needs backing off.",
			"Compression is trained actively. Stretching alone will not lift the legs.",
		},
		Frequency: "Three short sessions a week; this is a position that tolerates frequency well.",
	},
	{
		Key: "pistol_squat", Phrase: "the pistol squat", Name: "Pistol squat", Pattern: patternLegs,
		Aliases:  []string{"pistol squat", "pistol", "einbeinige kniebeuge", "einbeinkniebeuge", "single leg squat"},
		Timeline: "Two to six months for most people, sooner with decent ankle mobility.",
		Ladder: []Step{
			{Name: "Squat depth", Movement: chain{"bodyweight_squat"}, Metric: metricReps, Standard: 30,
				Assist: chain{"single_leg_calf_raise"}, Typical: "2 to 4 weeks"},
			{Name: "Split squat", Movement: chain{"bulgarian_split"}, Metric: metricReps, Standard: 12,
				Assist: chain{"bodyweight_squat"}, Typical: "3 to 6 weeks"},
			{Name: "Assisted pistol", Movement: chain{"assisted_pistol_squat"}, Metric: metricReps, Standard: 8,
				Assist: chain{"bulgarian_split"}, Typical: "4 to 8 weeks"},
			{Name: "Pistol squat", Movement: chain{"pistol_squat"}, Metric: metricReps, Standard: 5,
				Assist: chain{"assisted_pistol_squat"}, Typical: "the goal; the shrimp squat is the harder cousin to move on to"},
		},
		Drills:      chain{"assisted_pistol_squat", "single_leg_calf_raise", "reverse_nordic"},
		Accessories: chain{"nordic_curl", "jump_squat", "hollow_body_hold"},
		Risks: []string{
			"Knee pain at the bottom is usually ankle range, not knee weakness. Raise the heel and keep training.",
			"The descent is where control is built. Do not drop into the hole to save the rep.",
		},
		Frequency: "Two leg sessions a week, one heavy and one about control.",
	},
	{
		Key: "pull_up", Phrase: "a first strict pull-up", Name: "First strict pull-up", Pattern: patternPull,
		Aliases:  []string{"first pull up", "erster klimmzug", "pull up", "pullup", "pull-up", "klimmzug", "klimmzuge", "klimmzüge", "chin up", "chinup", "klimmzuege"},
		Timeline: "Six to sixteen weeks from a dead hang, for most people who train it three times a week.",
		Ladder: []Step{
			{Name: "Hang and scapular control", Movement: chain{"dead_hang"}, Metric: metricHold, Standard: 30,
				Assist: chain{"scapular_pull_up"}, Typical: "2 to 4 weeks"},
			{Name: "Horizontal pulling", Movement: chain{"australian_row"}, Metric: metricReps, Standard: 12,
				Assist: chain{"scapular_pull_up"}, Typical: "3 to 6 weeks"},
			{Name: "Negatives", Movement: chain{"negative_pull_up"}, Metric: metricReps, Standard: 5,
				Assist: chain{"australian_row"}, Typical: "3 to 6 weeks; five seconds down, every rep"},
			{Name: "Band-assisted", Movement: chain{"band_assisted_pull_up"}, Metric: metricReps, Standard: 8,
				Assist: chain{"negative_pull_up"}, Typical: "3 to 8 weeks, thinner band as reps come"},
			{Name: "Strict pull-up", Movement: chain{"pull_up"}, Metric: metricReps, Standard: 5,
				Assist: chain{"band_assisted_pull_up"}, Typical: "the goal"},
		},
		Drills:      chain{"scapular_pull_up", "australian_row", "dead_hang"},
		Accessories: chain{"push_up", "band_face_pull", "hollow_body_hold"},
		Risks: []string{
			"Kipping to make the rep count teaches the kip. If the rep needs a swing, it is a band day.",
			"Grip and elbow soreness early on is normal; sharp inner-elbow pain is not.",
		},
		Frequency: "Three sessions a week, and low-effort sets greased in on the days between.",
	},
	{
		Key: "general", Phrase: "balanced strength", Name: "Balanced strength", Pattern: patternPull, Foundation: true,
		Aliases:  []string{"general", "allgemein", "strength", "kraft", "ganzkorper", "ganzkörper", "full body", "conditioning", "basics", "grundlagen"},
		Timeline: "Continuous. This is the plan that makes the skill plans possible.",
		Ladder: []Step{
			{Name: "The basics, owned", Movement: chain{"pull_up", "australian_row"}, Metric: metricReps, Standard: 10,
				Assist: chain{"dip", "push_up"}, Typical: "the base every skill is built on"},
			{Name: "Strength across the patterns", Movement: chain{"dip", "push_up"}, Metric: metricReps, Standard: 15,
				Assist: chain{"pull_up"}, Typical: "3 to 6 months"},
			{Name: "Loaded basics", Movement: chain{"weighted_pull_up", "weighted_dip"}, Metric: metricAdded, Standard: 20,
				Assist: chain{"pull_up", "dip"}, Typical: "6 to 18 months"},
		},
		Drills:      chain{"australian_row", "push_up", "hollow_body_hold"},
		Accessories: chain{"band_face_pull", "hanging_leg_raise", "bulgarian_split", "scapular_pull_up"},
		Risks: []string{
			"Pushing more than you pull is how shoulders in this sport get hurt. The plan keeps them level on purpose.",
		},
		Frequency: "Three full-body sessions a week covers it.",
	},
}

// phrase is the goal as it reads inside a sentence.
func (g Goal) phrase() string {
	if g.Phrase != "" {
		return g.Phrase
	}
	return strings.ToLower(g.Name)
}

// goalByKey is the catalogue as a lookup.
var goalByKey = func() map[string]Goal {
	m := make(map[string]Goal, len(Goals))
	for _, g := range Goals {
		m[g.Key] = g
	}
	return m
}()

var nonWord = regexp.MustCompile(`[^a-z0-9]+`)

// normalise flattens whatever the athlete typed into something matchable:
// lower case, punctuation gone, umlauts folded, one space between words.
func normalise(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	replacer := strings.NewReplacer("ä", "a", "ö", "o", "ü", "u", "ß", "ss", "é", "e")
	s = replacer.Replace(s)
	s = nonWord.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// MatchGoal resolves free text to a track. Longest alias wins, so "ring muscle
// up" does not land on the bar muscle-up and "weighted pull-up" does not land
// on the first pull-up. Nothing matched is not a failure: an athlete who typed
// something the app has never heard of still gets balanced strength work, and
// the plan says plainly that it did not recognise the goal.
func MatchGoal(text string) (Goal, bool) {
	want := " " + normalise(text) + " "
	best, bestLen := Goal{}, 0
	for _, g := range Goals {
		if g.Key == "general" {
			continue
		}
		for _, alias := range g.Aliases {
			a := normalise(alias)
			if len(a) > bestLen && strings.Contains(want, " "+a+" ") {
				best, bestLen = g, len(a)
			}
		}
	}
	if bestLen == 0 {
		return goalByKey["general"], false
	}
	return best, true
}

var targetNumber = regexp.MustCompile(`(\d+(?:[.,]\d+)?)\s*(kg|kilo|kilos|s|sec|secs|second|seconds|sek|sekunden)\b`)

// namedTarget reads a number out of the goal text — "20 kg weighted pull-up",
// "10s front lever" — so the plan aims at what was asked for rather than at
// the ladder's own last rung. Anything absurd is ignored rather than trusted.
func namedTarget(text string) (value float64, metric string, ok bool) {
	m := targetNumber.FindStringSubmatch(normalise(text) + " ")
	if m == nil {
		return 0, "", false
	}
	v, err := strconv.ParseFloat(strings.Replace(m[1], ",", ".", 1), 64)
	if err != nil || v <= 0 {
		return 0, "", false
	}
	if strings.HasPrefix(m[2], "k") {
		if v > 200 {
			return 0, "", false
		}
		return v, metricAdded, true
	}
	if v > 600 {
		return 0, "", false
	}
	return v, metricHold, true
}

// ---------- what a movement loads ----------

// Injury regions, as the injury form records them.
const (
	regionWrist    = "wrist"
	regionElbow    = "elbow"
	regionShoulder = "shoulder"
	regionChest    = "chest"
	regionBack     = "back"
	regionCore     = "core"
	regionHip      = "hip"
	regionKnee     = "knee"
	regionAnkle    = "ankle"
)

// byCategory is what a movement loads by virtue of what kind of movement it is.
var byCategory = map[string][]string{
	"pull":     {regionShoulder, regionElbow, regionBack},
	"push":     {regionShoulder, regionElbow, regionChest},
	"static":   {regionShoulder, regionElbow, regionCore},
	"dynamic":  {regionShoulder, regionElbow, regionBack},
	"weighted": {regionShoulder, regionElbow, regionBack},
	"core":     {regionCore},
	"legs":     {regionKnee, regionHip},
	"mobility": {},
}

// wristLoaded is every movement that puts bodyweight through the hand — on the
// floor, on a false grip, or over the bar. The category cannot know this: a
// pull-up and a muscle-up are both "pull", and only one of them wrecks a wrist.
var wristLoaded = map[string]bool{
	"planche_lean": true, "frog_stand": true, "tuck_planche": true, "adv_tuck_planche": true,
	"straddle_planche": true, "full_planche": true, "tuck_planche_push_up": true,
	"planche_push_up": true, "pseudo_planche_push_up": true, "push_up": true,
	"weighted_push_up": true, "pike_push_up": true, "elevated_pike_push_up": true,
	"handstand": true, "wall_handstand": true, "wall_walk": true, "handstand_shoulder_taps": true,
	"crow_pose": true, "wall_hspu": true, "negative_hspu": true, "deficit_hspu": true,
	"handstand_push_up": true, "press_to_handstand": true, "ab_wheel_rollout": true,
	"false_grip_hang": true, "false_grip_row": true, "muscle_up": true, "ring_muscle_up": true,
	"jumping_muscle_up": true, "muscle_up_negative": true, "wrist_prep": true,
	"wrist_extensor_curl": true, "scapular_push_up": true, "plank": true, "side_plank": true,
	"clutch_flag": true, "tuck_human_flag": true, "straddle_human_flag": true,
	"human_flag": true, "flag_negative": true, "l_sit": true, "tuck_l_sit": true,
	"one_leg_l_sit": true, "v_sit": true, "russian_dip": true,
}

// extraRegions is everything else the category misses.
var extraRegions = map[string][]string{
	"german_hang":             {regionShoulder},
	"scapular_pull_up":        {regionShoulder, regionBack},
	"scapular_push_up":        {regionShoulder, regionChest},
	"active_hang":             {regionShoulder},
	"dead_hang":               {regionShoulder},
	"one_arm_dead_hang":       {regionShoulder, regionElbow},
	"wall_walk":               {regionShoulder},
	"skin_the_cat":            {regionShoulder, regionElbow},
	"shoulder_dislocate_hold": {regionShoulder},
	"band_dislocate":          {regionShoulder},
	"band_face_pull":          {regionShoulder},
	"inverted_hang":           {regionShoulder},
	"arch_body_hold":          {regionBack},
	"dragon_flag":             {regionCore, regionBack},
	"dragon_flag_negative":    {regionCore, regionBack},
	"hollow_rock":             {regionCore, regionBack},
	"ab_wheel_rollout":        {regionCore, regionBack, regionShoulder},
	"hanging_leg_raise":       {regionCore, regionShoulder, regionHip},
	"hanging_knee_raise":      {regionCore, regionShoulder, regionHip},
	"toes_to_bar":             {regionCore, regionShoulder, regionHip},
	"compression_leg_lift":    {regionCore, regionHip},
	"seated_pike_stretch":     {regionHip},
	"pancake_stretch":         {regionHip},
	"copenhagen_plank":        {regionCore, regionHip},
	"jump_squat":              {regionKnee, regionHip, regionAnkle},
	"single_leg_calf_raise":   {regionAnkle},
	"nordic_curl":             {regionKnee, regionHip},
	"reverse_nordic":          {regionKnee},
	"pistol_squat":            {regionKnee, regionHip, regionAnkle},
	"shrimp_squat":            {regionKnee, regionHip},
	"clutch_flag":             {regionShoulder, regionCore, regionChest},
	"tuck_human_flag":         {regionShoulder, regionCore, regionHip},
	"straddle_human_flag":     {regionShoulder, regionCore, regionHip},
	"human_flag":              {regionShoulder, regionCore, regionHip},
	"flag_negative":           {regionShoulder, regionCore, regionHip},
	"l_sit":                   {regionCore, regionShoulder, regionHip},
	"v_sit":                   {regionCore, regionShoulder, regionHip},
	"tuck_l_sit":              {regionCore, regionShoulder},
	"one_leg_l_sit":           {regionCore, regionShoulder, regionHip},
	"dip":                     {regionChest},
	"ring_dip":                {regionChest},
	"straight_bar_dip":        {regionChest},
	"weighted_dip":            {regionChest},
	"bulgarian_split":         {regionKnee, regionHip},
	"assisted_pistol_squat":   {regionKnee, regionHip},
	"bodyweight_squat":        {regionKnee, regionHip},
}

// loadedRegions is what this movement puts under load. It decides what an open
// injury takes off the table, so it errs toward listing a region rather than
// omitting it: a plan that trains around an injury too carefully costs a
// fortnight, and one that does not costs a season.
func loadedRegions(e training.Exercise) map[string]bool {
	out := map[string]bool{}
	for _, region := range byCategory[e.Category] {
		out[region] = true
	}
	for _, region := range extraRegions[e.Slug] {
		out[region] = true
	}
	if wristLoaded[e.Slug] {
		out[regionWrist] = true
	}
	return out
}

// warmupFor and rehabFor map an injured or working region onto the curated
// protocol list. Anything without its own protocol falls back to the general
// warm-up, which is better than naming a protocol that does not exist.
var warmupFor = map[string]string{
	regionWrist:    "wrist_warmup",
	regionElbow:    "straight_arm_warmup",
	regionShoulder: "shoulder_warmup",
	regionChest:    "chest_shoulder_girdle_warmup",
}

var rehabFor = map[string]string{
	regionWrist:    "wrist_rehab_light",
	regionShoulder: "shoulder_rehab_light",
}
