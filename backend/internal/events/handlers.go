package events

import (
	"net/http"
	"strings"
	"time"

	"calisthenics/api/internal/auth"
	"calisthenics/api/internal/httpx"
)

type Event struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Discipline  string  `json:"discipline"`
	StartsOn    string  `json:"starts_on"`
	EndsOn      *string `json:"ends_on"`
	City        string  `json:"city"`
	Country     string  `json:"country"`
	URL         string  `json:"url"`
	SourceURL   string  `json:"source_url"`
	SourceTitle string  `json:"source_title"`
	Evidence    string  `json:"evidence"`
	Confidence  string  `json:"confidence"`
	CheckNote   string  `json:"check_note"`
	CheckedAt   *string `json:"checked_at"`
	Verified    bool    `json:"verified"`
	Registered  bool    `json:"registered"`
}

// List returns events. Unconfirmed ones are excluded unless asked for, so the
// default view only shows dates that were checked against a live page.
func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	me, _ := auth.UserFrom(r.Context())

	from := time.Now().AddDate(0, 0, -1)
	to := time.Now().AddDate(1, 0, 0)
	if v := r.URL.Query().Get("from"); v != "" {
		if parsed, err := time.Parse("2006-01-02", v); err == nil {
			from = parsed
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if parsed, err := time.Parse("2006-01-02", v); err == nil {
			to = parsed
		}
	}
	includeUnconfirmed := r.URL.Query().Get("include_unconfirmed") == "true"

	rows, err := s.pool.Query(r.Context(), `
		select e.id, e.name, e.discipline,
		       to_char(e.starts_on, 'YYYY-MM-DD'), to_char(e.ends_on, 'YYYY-MM-DD'),
		       e.city, e.country, e.url, e.source_url, e.source_title, e.evidence,
		       e.confidence, e.check_note, to_char(e.checked_at, 'YYYY-MM-DD'), e.verified,
		       ($6 <> '' and exists (
		           select 1 from user_events ue
		           where ue.event_id = e.id and ue.user_id::text = $6
		       )) as registered
		from events e
		where e.starts_on between $1 and $2
		  and e.confidence <> 'rejected'
		  and ($3 or e.verified)
		  and ($4 = '' or e.discipline = $4)
		  and ($5 = '' or e.country = $5)
		order by e.starts_on
		limit 200`,
		from, to, includeUnconfirmed,
		strings.ToLower(r.URL.Query().Get("discipline")),
		strings.ToUpper(r.URL.Query().Get("country")),
		me.ID)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't load events.")
		return
	}
	defer rows.Close()

	out := []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Name, &e.Discipline, &e.StartsOn, &e.EndsOn,
			&e.City, &e.Country, &e.URL, &e.SourceURL, &e.SourceTitle, &e.Evidence,
			&e.Confidence, &e.CheckNote, &e.CheckedAt, &e.Verified, &e.Registered); err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Couldn't read events.")
			return
		}
		out = append(out, e)
	}
	httpx.JSON(w, http.StatusOK, out)
}

type discoverInput struct {
	Discipline string `json:"discipline"`
	Country    string `json:"country"`
	From       string `json:"from"`
	To         string `json:"to"`
	Force      bool   `json:"force"`
}

// Discover runs a search. Results are cached for a day per query shape, because
// each run costs real money and a hundred users browsing the same region should
// not pay for a hundred runs.
func (s *Service) Discover(w http.ResponseWriter, r *http.Request) {
	var in discoverInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	if !s.ai.Configured() {
		httpx.Fail(w, http.StatusServiceUnavailable, "Event search is switched off: no API key is set on the server.")
		return
	}

	from := time.Now()
	to := time.Now().AddDate(0, 6, 0)
	if in.From != "" {
		parsed, err := time.Parse("2006-01-02", in.From)
		if err != nil {
			httpx.Fail(w, http.StatusBadRequest, "The from date must look like 2026-09-01.")
			return
		}
		from = parsed
	}
	if in.To != "" {
		parsed, err := time.Parse("2006-01-02", in.To)
		if err != nil {
			httpx.Fail(w, http.StatusBadRequest, "The to date must look like 2027-03-01.")
			return
		}
		to = parsed
	}
	if to.Before(from) {
		httpx.Fail(w, http.StatusBadRequest, "The to date must come after the from date.")
		return
	}
	if in.Discipline != "" && !validDisciplines[strings.ToLower(in.Discipline)] {
		httpx.Fail(w, http.StatusBadRequest, "Pick a discipline from the list.")
		return
	}

	request := DiscoverRequest{
		Discipline: strings.ToLower(in.Discipline),
		Country:    strings.ToUpper(in.Country),
		From:       from,
		To:         to,
	}

	if !in.Force {
		var recent bool
		err := s.pool.QueryRow(r.Context(), `
			select exists (
				select 1 from discovery_runs
				where discipline = $1 and country = $2
				  and from_date = $3 and to_date = $4
				  and error = '' and ran_at > now() - interval '24 hours'
			)`, request.Discipline, request.Country, request.From, request.To).Scan(&recent)
		if err == nil && recent {
			httpx.JSON(w, http.StatusOK, DiscoverReport{
				Outcomes:  []Outcome{},
				FromCache: true,
			})
			return
		}
	}

	me := auth.MustUser(r.Context())
	report, err := s.RunDiscovery(r.Context(), me.ID, request)
	if err != nil {
		httpx.Fail(w, http.StatusBadGateway, "The event search failed: "+err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, report)
}

type confirmInput struct {
	Confirmed bool   `json:"confirmed"`
	Note      string `json:"note"`
}

// Confirm records a human decision, which outranks every machine check.
func (s *Service) Confirm(w http.ResponseWriter, r *http.Request) {
	var in confirmInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	confidence := ConfidenceRejected
	if in.Confirmed {
		confidence = ConfidenceHuman
	}

	tag, err := s.pool.Exec(r.Context(), `
		update events
		set confidence = $2, verified = $3, check_note = $4, checked_at = now()
		where id = $1`,
		r.PathValue("id"), confidence, in.Confirmed,
		strings.TrimSpace("Confirmed by hand. "+in.Note))
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't update that event.")
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Fail(w, http.StatusNotFound, "That event doesn't exist.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"confidence": confidence})
}

// RecheckOne re-verifies a single event's source on demand.
func (s *Service) RecheckOne(w http.ResponseWriter, r *http.Request) {
	verdict, err := s.Recheck(r.Context(), r.PathValue("id"))
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't recheck that event.")
		return
	}
	httpx.JSON(w, http.StatusOK, verdict)
}

type registerInput struct {
	Goal string `json:"goal"`
}

func (s *Service) Register(w http.ResponseWriter, r *http.Request) {
	var in registerInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	me := auth.MustUser(r.Context())
	_, err := s.pool.Exec(r.Context(), `
		insert into user_events (user_id, event_id, goal) values ($1, $2, $3)
		on conflict (user_id, event_id) do update set goal = excluded.goal`,
		me.ID, r.PathValue("id"), in.Goal)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't add that event to your calendar.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "registered"})
}

func (s *Service) Unregister(w http.ResponseWriter, r *http.Request) {
	me := auth.MustUser(r.Context())
	_, err := s.pool.Exec(r.Context(),
		`delete from user_events where user_id = $1 and event_id = $2`, me.ID, r.PathValue("id"))
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't remove that event.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "removed"})
}
