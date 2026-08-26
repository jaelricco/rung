package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

// Before writing a plan, find out how the skill is actually trained.
//
// The failure mode this fixes is a plan that reads like a plan: three sets of
// something plausible, repeated for eight weeks, with no idea of the ladder
// the skill is climbed by or the standards each rung is held to. The model
// knows a lot about calisthenics, but "knows" and "can name the entry standard
// for a straddle front lever" are different things, and the second one is what
// makes a plan specific.
//
// So the skill gets researched first, with web search, against real coaching
// sources — and the findings are mapped onto library slugs before they are
// handed to the plan-writing turn. The research is cached by skill: what a
// front lever needs does not change between two athletes, only what *this*
// athlete needs from it does, and that comes from the snapshot.

// researchSystem keeps the research turn in a reporting role. The rules mirror
// the event-discovery ones for the same reason: a model asked to research will
// otherwise happily answer from memory and cite a page it never opened.
const researchSystem = `You research how a calisthenics skill is trained, using web search, for a coach who will write the plan.

Hard rules:
1. Search before answering. Run several different searches: the progression ladder, entry standards, weekly structure, accessory work, and the injuries the skill is known for.
2. Prefer coaches, gymnastics sources and federation material that give concrete numbers over blog listicles. Where good sources disagree, report the disagreement rather than averaging it away.
3. Express everything you can as slugs from the EXERCISE LIBRARY you are given. Never invent a slug. If something the sources recommend has no slug, describe it in prose instead and do not invent one.
4. Standards must be concrete and measurable: seconds held, reps, added kilos, degrees of lean. "Until strong enough" is not a standard.
5. Report what the sources say, not what you would program. The coach does the programming.
6. Answer with the requested JSON only. No preamble, no code fence.`

// ResearchStage is one rung of the ladder to the skill.
type ResearchStage struct {
	Stage         string   `json:"stage"`
	ExerciseSlugs []string `json:"exercise_slugs"`
	Standard      string   `json:"standard"`
	TypicalWeeks  string   `json:"typical_weeks"`
}

// ResearchDrill is one movement worth training for this skill, and why.
type ResearchDrill struct {
	ExerciseSlug string `json:"exercise_slug"`
	Role         string `json:"role"`
	Dosage       string `json:"dosage"`
}

type SkillResearch struct {
	Skill           string          `json:"skill"`
	Summary         string          `json:"summary"`
	Prerequisites   []string        `json:"prerequisites"`
	Progression     []ResearchStage `json:"progression"`
	KeyDrills       []ResearchDrill `json:"key_drills"`
	Accessories     []ResearchDrill `json:"accessories"`
	WeeklyStructure string          `json:"weekly_structure"`
	VolumeGuidance  string          `json:"volume_guidance"`
	CommonMistakes  []string        `json:"common_mistakes"`
	InjuryRisks     []string        `json:"injury_risks"`
	Sources         []Source        `json:"sources"`
	SearchesUsed    int             `json:"searches_used"`
	Cached          bool            `json:"cached"`
	ResearchedAt    time.Time       `json:"researched_at"`
}

// Empty reports whether the research found nothing worth handing on.
func (r SkillResearch) Empty() bool {
	return strings.TrimSpace(r.Summary) == "" && len(r.Progression) == 0 && len(r.KeyDrills) == 0
}

// How long findings stay usable. Training methodology moves slowly; this is
// short enough that a genuinely new consensus arrives within a season.
const researchTTL = 60 * 24 * time.Hour

// The research turn is one non-streamed call that reads pages, so its ceiling
// has to cover the pages as well as the reasoning and the answer.
const (
	researchTokens   = 14000
	researchSearches = 7
)

// research returns findings for the skill, from cache when they are fresh.
// Research is a convenience, not a dependency: every failure path returns an
// empty result and lets the plan be written without it.
func (h *Handler) research(ctx context.Context, userID, skill string, lib library) SkillResearch {
	key := researchKey(skill)
	if key == "" {
		return SkillResearch{}
	}

	if cached, ok := h.cachedResearch(ctx, key); ok {
		return cached
	}

	prompt := fmt.Sprintf(`Research how athletes actually train toward: %s

Find the progression ladder, what each rung is held to before moving on, how the work is
usually spread across a week, the accessory and joint-preparation work the skill needs, and
what tends to go wrong.

EXERCISE LIBRARY (the only slugs you may use)
%s
Return JSON:
{
  "summary": "3-5 sentences on how this skill is built, and what actually limits people",
  "prerequisites": ["concrete entry standards before starting, e.g. 10 strict pull-ups"],
  "progression": [
    {"stage": "name of the rung", "exercise_slugs": ["slug"],
     "standard": "what to hit before moving on, with numbers",
     "typical_weeks": "how long this rung usually takes"}
  ],
  "key_drills": [
    {"exercise_slug": "slug", "role": "what this builds for the skill", "dosage": "sets, reps or seconds, and frequency"}
  ],
  "accessories": [
    {"exercise_slug": "slug", "role": "why it supports the skill", "dosage": "sets and reps"}
  ],
  "weekly_structure": "how the sessions are usually laid out across a week, and why",
  "volume_guidance": "sets per week, how close to failure, how it progresses",
  "common_mistakes": ["what people get wrong, stated as what to do instead"],
  "injury_risks": ["the tissue at risk and the preparation that protects it"]
}`, skill, lib.text)

	var found SkillResearch
	searchResult, err := h.client.SearchJSON(ctx, userID, "skill_research", researchSystem, prompt,
		researchTokens, SearchOptions{MaxSearches: researchSearches}, &found)
	if err != nil {
		// A plan without research is the old behaviour, which is still a plan.
		log.Printf("skill research for %q failed, writing the plan without it: %v", skill, err)
		return SkillResearch{}
	}

	found.Skill = skill
	found.Sources = topSources(searchResult.Sources, 8)
	found.SearchesUsed = searchResult.SearchCount
	found.ResearchedAt = time.Now()
	found.pruneToLibrary(lib)

	if found.Empty() {
		return SkillResearch{}
	}
	h.storeResearch(ctx, key, found)
	return found
}

// pruneToLibrary drops every slug the model reached for that does not exist.
// A stage or drill that loses its slug keeps its prose: "hold the advanced
// tuck for 15 seconds" is still useful guidance even when the rung it names
// has no row in the table.
func (r *SkillResearch) pruneToLibrary(lib library) {
	for i := range r.Progression {
		r.Progression[i].ExerciseSlugs = lib.keep(r.Progression[i].ExerciseSlugs)
	}
	r.KeyDrills = pruneDrills(r.KeyDrills, lib)
	r.Accessories = pruneDrills(r.Accessories, lib)
}

func pruneDrills(drills []ResearchDrill, lib library) []ResearchDrill {
	kept := drills[:0]
	for _, d := range drills {
		if lib.has(d.ExerciseSlug) {
			kept = append(kept, d)
		}
	}
	return kept
}

// brief renders the findings for the plan-writing prompt. Only the parts that
// change what gets written are included; the sources are for the athlete.
func (r SkillResearch) brief() string {
	if r.Empty() {
		return ""
	}
	var b strings.Builder
	b.WriteString("RESEARCH ON THIS SKILL (gathered from coaching sources, use it)\n")
	fmt.Fprintf(&b, "%s\n", strings.TrimSpace(r.Summary))

	if len(r.Prerequisites) > 0 {
		fmt.Fprintf(&b, "\nEntry standards: %s\n", strings.Join(r.Prerequisites, "; "))
	}
	if len(r.Progression) > 0 {
		b.WriteString("\nProgression ladder:\n")
		for i, stage := range r.Progression {
			fmt.Fprintf(&b, "%d. %s [%s] — hold to: %s%s\n", i+1, stage.Stage,
				strings.Join(stage.ExerciseSlugs, ", "), stage.Standard, parenthetical(stage.TypicalWeeks))
		}
	}
	if len(r.KeyDrills) > 0 {
		b.WriteString("\nDrills that build it:\n")
		for _, d := range r.KeyDrills {
			fmt.Fprintf(&b, "- %s — %s. Usual dose: %s\n", d.ExerciseSlug, d.Role, d.Dosage)
		}
	}
	if len(r.Accessories) > 0 {
		b.WriteString("\nAccessory work:\n")
		for _, d := range r.Accessories {
			fmt.Fprintf(&b, "- %s — %s. Usual dose: %s\n", d.ExerciseSlug, d.Role, d.Dosage)
		}
	}
	if r.WeeklyStructure != "" {
		fmt.Fprintf(&b, "\nHow the week is usually laid out: %s\n", r.WeeklyStructure)
	}
	if r.VolumeGuidance != "" {
		fmt.Fprintf(&b, "Volume and intensity: %s\n", r.VolumeGuidance)
	}
	if len(r.CommonMistakes) > 0 {
		fmt.Fprintf(&b, "Avoid: %s\n", strings.Join(r.CommonMistakes, "; "))
	}
	if len(r.InjuryRisks) > 0 {
		fmt.Fprintf(&b, "Known injury risks: %s\n", strings.Join(r.InjuryRisks, "; "))
	}
	return b.String()
}

func parenthetical(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return " (" + s + ")"
}

// ---------- cache ----------

func (h *Handler) cachedResearch(ctx context.Context, key string) (SkillResearch, bool) {
	var body, sources []byte
	var searches int
	var at time.Time
	err := h.pool.QueryRow(ctx, `
		select findings, sources, searches_used, created_at
		from skill_research
		where skill_key = $1 and created_at > now() - $2::interval`,
		key, researchTTL.String()).Scan(&body, &sources, &searches, &at)
	if err != nil {
		return SkillResearch{}, false
	}

	var found SkillResearch
	if json.Unmarshal(body, &found) != nil {
		return SkillResearch{}, false
	}
	_ = json.Unmarshal(sources, &found.Sources)
	found.SearchesUsed = searches
	found.ResearchedAt = at
	found.Cached = true
	return found, !found.Empty()
}

func (h *Handler) storeResearch(ctx context.Context, key string, found SkillResearch) {
	body, err := json.Marshal(found)
	if err != nil {
		return
	}
	sources, err := json.Marshal(found.Sources)
	if err != nil {
		return
	}
	// The cache is an optimisation; a failed write costs a search next time.
	_, err = h.pool.Exec(ctx, `
		insert into skill_research (skill_key, skill, findings, sources, searches_used)
		values ($1, $2, $3, $4, $5)
		on conflict (skill_key) do update set
			skill = excluded.skill, findings = excluded.findings,
			sources = excluded.sources, searches_used = excluded.searches_used,
			created_at = now()`,
		key, found.Skill, body, sources, found.SearchesUsed)
	if err != nil {
		log.Printf("caching research for %q: %v", key, err)
	}
}

var researchKeyNoise = regexp.MustCompile(`[^a-z0-9]+`)

// researchKey collapses the ways one skill gets typed — "Front Lever",
// "front-lever", "a full front lever!" — onto one cache entry.
func researchKey(skill string) string {
	key := researchKeyNoise.ReplaceAllString(strings.ToLower(strings.TrimSpace(skill)), " ")
	fields := strings.Fields(key)
	kept := fields[:0]
	for _, f := range fields {
		switch f {
		case "a", "an", "the", "my", "full", "get", "getting", "to", "reach", "learn", "learning":
			continue
		}
		kept = append(kept, f)
	}
	if len(kept) == 0 {
		kept = fields
	}
	if len(kept) > 8 {
		kept = kept[:8]
	}
	return strings.Join(kept, " ")
}

// topSources keeps one entry per page, preferring the one that carries a cited
// passage, since that is the part the model actually read.
func topSources(sources []Source, limit int) []Source {
	seen := map[string]int{}
	out := []Source{}
	for _, s := range sources {
		if s.URL == "" {
			continue
		}
		key := normaliseURL(s.URL)
		if at, ok := seen[key]; ok {
			if out[at].CitedText == "" && s.CitedText != "" {
				out[at] = s
			}
			continue
		}
		if len(out) >= limit {
			continue
		}
		seen[key] = len(out)
		out = append(out, s)
	}
	return out
}

func humanAge(at time.Time) string {
	if at.IsZero() {
		return "recently"
	}
	days := int(time.Since(at).Hours() / 24)
	switch {
	case days <= 0:
		return "today"
	case days == 1:
		return "yesterday"
	default:
		return fmt.Sprintf("%d days ago", days)
	}
}
