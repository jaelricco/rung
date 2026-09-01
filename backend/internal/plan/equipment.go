package plan

import "strings"

// What a movement cannot be performed without.
//
// This is the constraint the app was quietly getting wrong: a plan that
// prescribes weighted pull-ups to someone with no belt, or ring muscle-ups to
// someone with no rings, is not a hard plan — it is an impossible one, and the
// athlete has no way to tell which of the two it is. So equipment is asked for
// and then honoured, with the same mechanism the injury filter uses: the
// movement leaves the candidate chains, and something else is picked.
//
// It is deliberately a softer constraint than an injury in one respect only:
// nothing here is a reason to stop training, so an answer that removes most of
// the library produces a plan built from what is left rather than a warning.
const (
	EquipBar         = "pull_up_bar"
	EquipDipBars     = "dip_bars"
	EquipRings       = "rings"
	EquipParallettes = "parallettes"
	EquipBelt        = "weight_belt"
	EquipBands       = "bands"
	// EquipFloorOnly is how an athlete says "I have answered, and I have none
	// of it" — which is a real answer, and different from not answering.
	EquipFloorOnly = "floor_only"
)

// Equipment is the list offered on the baseline form, in the order it is asked.
var Equipment = []struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Note  string `json:"note"`
}{
	{EquipBar, "A bar to hang from", "Pull-up bar, a beam, a playground frame — anything you can hang full-length on."},
	{EquipDipBars, "Parallel bars", "A dip station, or two bars at hip height."},
	{EquipParallettes, "Parallettes", "Low handles. They take the wrist out of floor work, which matters for planche and L-sits."},
	{EquipRings, "Gymnastic rings", "Rings change what a muscle-up and a dip can be."},
	{EquipBelt, "A way to add load", "A dip belt, a weight vest, or a backpack you trust."},
	{EquipBands, "Resistance bands", "For assistance on pulls and for shoulder work."},
	{EquipFloorOnly, "None of the above", "Floor only. The plan will be built from what that allows."},
}

// requires lists, per movement, the groups of equipment it needs: at least one
// item from every group. Two groups means both are needed — a weighted pull-up
// wants a bar *and* something to hang off it — while one group with two items
// means either will do.
var requires = map[string][][]string{
	// Hanging: everything that starts from a bar.
	"pull_up":                  {{EquipBar}},
	"chin_up":                  {{EquipBar}},
	"wide_pull_up":             {{EquipBar}},
	"archer_pull_up":           {{EquipBar}},
	"typewriter_pull_up":       {{EquipBar}},
	"l_sit_pull_up":            {{EquipBar}},
	"one_arm_pull_up":          {{EquipBar}},
	"one_arm_negative":         {{EquipBar}},
	"one_arm_dead_hang":        {{EquipBar}},
	"assisted_one_arm_pull_up": {{EquipBar}},
	"negative_pull_up":         {{EquipBar}},
	"band_assisted_pull_up":    {{EquipBar}, {EquipBands}},
	"australian_row":           {{EquipBar, EquipRings}},
	"dead_hang":                {{EquipBar}},
	"active_hang":              {{EquipBar}},
	"scapular_pull_up":         {{EquipBar}},
	"false_grip_hang":          {{EquipBar, EquipRings}},
	"false_grip_row":           {{EquipRings, EquipBar}},
	"hanging_knee_raise":       {{EquipBar}},
	"hanging_leg_raise":        {{EquipBar}},
	"toes_to_bar":              {{EquipBar}},
	"inverted_hang":            {{EquipBar, EquipRings}},
	"german_hang":              {{EquipBar, EquipRings}},
	"skin_the_cat":             {{EquipBar, EquipRings}},
	"shoulder_dislocate_hold":  {{EquipBar, EquipDipBars}},

	// Levers, which are hanging work with a straight body.
	"tuck_front_lever":     {{EquipBar, EquipRings}},
	"adv_tuck_front_lever": {{EquipBar, EquipRings}},
	"straddle_front_lever": {{EquipBar, EquipRings}},
	"front_lever":          {{EquipBar, EquipRings}},
	"front_lever_raise":    {{EquipBar, EquipRings}},
	"front_lever_row":      {{EquipBar, EquipRings}},
	"tuck_front_lever_row": {{EquipBar, EquipRings}},
	"ice_cream_maker":      {{EquipBar, EquipRings}},
	"tuck_back_lever":      {{EquipBar, EquipRings}},
	"adv_tuck_back_lever":  {{EquipBar, EquipRings}},
	"straddle_back_lever":  {{EquipBar, EquipRings}},
	"back_lever":           {{EquipBar, EquipRings}},

	// Dynamics.
	"muscle_up":          {{EquipBar}},
	"explosive_pull_up":  {{EquipBar}},
	"jumping_muscle_up":  {{EquipBar}},
	"muscle_up_negative": {{EquipBar, EquipRings}},
	"ring_muscle_up":     {{EquipRings}},
	"360":                {{EquipBar}},
	"shrimp_flip":        {{EquipBar}},

	// Pressing that needs something to press between.
	"dip":              {{EquipDipBars, EquipRings}},
	"straight_bar_dip": {{EquipBar, EquipDipBars}},
	"russian_dip":      {{EquipDipBars}},
	"ring_dip":         {{EquipRings}},

	// Support holds. The floor works, but badly, and parallettes or bars are
	// what the progression is actually written for.
	"l_sit":         {{EquipDipBars, EquipParallettes, EquipBar}},
	"tuck_l_sit":    {{EquipDipBars, EquipParallettes, EquipBar}},
	"one_leg_l_sit": {{EquipDipBars, EquipParallettes, EquipBar}},
	"v_sit":         {{EquipDipBars, EquipParallettes}},

	// The flag needs a vertical pole; a bar upright is the closest thing the
	// library can promise.
	"clutch_flag":         {{EquipBar}},
	"tuck_human_flag":     {{EquipBar}},
	"straddle_human_flag": {{EquipBar}},
	"human_flag":          {{EquipBar}},
	"flag_negative":       {{EquipBar}},

	// Added load.
	"weighted_pull_up":   {{EquipBar}, {EquipBelt}},
	"weighted_dip":       {{EquipDipBars, EquipRings}, {EquipBelt}},
	"weighted_muscle_up": {{EquipBar}, {EquipBelt}},
	"weighted_push_up":   {{EquipBelt}},

	// Bands.
	"band_dislocate": {{EquipBands}},
	"band_face_pull": {{EquipBands}},
}

// ownedEquipment turns the athlete's answer into a lookup, and reports whether
// they answered at all. An unanswered list filters nothing: the app has always
// assumed a bar and a set of bars, and it keeps doing so until told otherwise.
func ownedEquipment(answer []string) (map[string]bool, bool) {
	if answer == nil {
		return nil, false
	}
	owned := map[string]bool{}
	for _, item := range answer {
		item = strings.TrimSpace(item)
		if item == "" || item == EquipFloorOnly {
			continue
		}
		owned[item] = true
	}
	return owned, true
}

// performable reports whether this movement can be done with what the athlete
// has. Anything the table says nothing about needs nothing: the floor, a wall,
// and a body.
func performable(slug string, owned map[string]bool) bool {
	if owned == nil {
		return true
	}
	for _, group := range requires[slug] {
		satisfied := false
		for _, item := range group {
			if owned[item] {
				satisfied = true
				break
			}
		}
		if !satisfied {
			return false
		}
	}
	return true
}

// missingFor names what would have to be bought or found for this movement, so
// a restriction can say why rather than only that.
func missingFor(slug string, owned map[string]bool) []string {
	var missing []string
	for _, group := range requires[slug] {
		satisfied := false
		for _, item := range group {
			if owned[item] {
				satisfied = true
				break
			}
		}
		if !satisfied {
			missing = append(missing, equipmentLabel(group[0]))
		}
	}
	return missing
}

func equipmentLabel(key string) string {
	for _, item := range Equipment {
		if item.Key == key {
			return strings.ToLower(item.Label)
		}
	}
	return strings.ReplaceAll(key, "_", " ")
}
