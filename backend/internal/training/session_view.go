package training

import (
	"encoding/json"
	"net/http"
	"sort"

	"calisthenics/api/internal/auth"
	"calisthenics/api/internal/httpx"
)

// SessionView is one session with everything needed to perform it, in one
// request, because the page that reads this is opened in its own tab and
// standing in a gym: a second round trip is a second chance to be looking at a
// spinner instead of the set you are about to do.
type SessionView struct {
	Session CalendarEntry `json:"session"`
	// Phases is the running order, served rather than hard-coded in the page,
	// so the order a session is performed in has one definition.
	Phases    any                 `json:"phases"`
	Protocols []Protocol          `json:"protocols"`
	Exercises map[string]Exercise `json:"exercises"`
}

// Session answers with one planned session by id. It is what the calendar
// links to: the warm-up resolved into its actual steps rather than a line of
// slugs, and every movement named rather than left as the slug it is stored
// under.
func (s *Service) Session(w http.ResponseWriter, r *http.Request) {
	me := auth.MustUser(r.Context())
	id := r.PathValue("id")

	entry, err := s.loadSession(r.Context(), me.ID, id)
	if err != nil {
		// Not found and not yours are the same answer on purpose: whether a
		// session id exists is not something to confirm to someone else.
		httpx.Fail(w, http.StatusNotFound, "That session isn't there.")
		return
	}

	var body SessionBody
	if err := json.Unmarshal(entry.Body, &body); err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "That session couldn't be read.")
		return
	}

	out := SessionView{
		Session:   entry,
		Phases:    SessionPhases,
		Protocols: []Protocol{},
		Exercises: map[string]Exercise{},
	}

	// The protocols this session names, in the catalogue's own order so the
	// warm-up reads the same way twice.
	wanted := map[string]bool{}
	for _, slug := range body.WarmupProtocols {
		wanted[slug] = true
	}
	for _, p := range Protocols {
		if wanted[p.Slug] {
			out.Protocols = append(out.Protocols, p)
		}
	}

	slugs := make([]string, 0, len(body.Blocks))
	for _, b := range body.Blocks {
		if b.ExerciseSlug != "" {
			slugs = append(slugs, b.ExerciseSlug)
		}
	}
	if len(slugs) > 0 {
		rows, err := s.pool.Query(r.Context(), `
			select slug, name, category, measure, difficulty, description
			from exercises where slug = any($1)`, slugs)
		if err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Couldn't read the exercise library.")
			return
		}
		defer rows.Close()
		for rows.Next() {
			var e Exercise
			if err := rows.Scan(&e.Slug, &e.Name, &e.Category, &e.Measure, &e.Difficulty, &e.Description); err != nil {
				httpx.Fail(w, http.StatusInternalServerError, "Couldn't read the exercise library.")
				return
			}
			out.Exercises[e.Slug] = e
		}
	}

	httpx.JSON(w, http.StatusOK, out)
}

// progressUpdate replaces what has been ticked off in a session. Wholesale
// rather than one tick at a time: the page sends the state it is showing, so a
// request that arrives late cannot leave a box ticked that has since been
// cleared, and a lost request costs one tap rather than a wrong session.
type progressUpdate struct {
	DoneProtocols []string `json:"done_protocols"`
	DoneBlocks    []int    `json:"done_blocks"`
}

// Progress records how far through a session the athlete has got.
func (s *Service) Progress(w http.ResponseWriter, r *http.Request) {
	var in progressUpdate
	if !httpx.Decode(w, r, &in) {
		return
	}
	me := auth.MustUser(r.Context())
	ctx := r.Context()

	entry, err := s.loadSession(ctx, me.ID, r.PathValue("id"))
	if err != nil {
		httpx.Fail(w, http.StatusNotFound, "That session isn't there.")
		return
	}
	var body SessionBody
	if err := json.Unmarshal(entry.Body, &body); err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "That session couldn't be read.")
		return
	}

	// Only what this session actually contains. A tick on a block that is not
	// there would survive an edit that removed it and reappear as a box
	// against whatever took its place.
	named := map[string]bool{}
	for _, slug := range body.WarmupProtocols {
		named[slug] = true
	}
	protocols := []string{}
	seen := map[string]bool{}
	for _, slug := range in.DoneProtocols {
		if named[slug] && !seen[slug] {
			seen[slug] = true
			protocols = append(protocols, slug)
		}
	}
	blocks := []int{}
	ticked := map[int]bool{}
	for _, i := range in.DoneBlocks {
		if i >= 0 && i < len(body.Blocks) && !ticked[i] {
			ticked[i] = true
			blocks = append(blocks, i)
		}
	}
	sort.Ints(blocks)

	if _, err := s.pool.Exec(ctx, `
		update planned_sessions set done_protocols = $3, done_blocks = $4
		where id = $1 and user_id = $2`, entry.ID, me.ID, protocols, blocks); err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't save your progress.")
		return
	}

	entry.DoneProtocols, entry.DoneBlocks = protocols, blocks
	httpx.JSON(w, http.StatusOK, entry)
}
