package events

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"calisthenics/api/internal/ai"

	"github.com/jackc/pgx/v5/pgxpool"
)

// discoverySystem forces the model into a reporting role rather than a
// recalling one. The two rules that matter: never answer from memory, and
// never cite a page you did not open.
const discoverySystem = `You find calisthenics and street workout competitions using web search.

Hard rules:
1. Search before answering. Never list an event from memory. If your searches find nothing, return an empty list.
2. Every event must carry a source_url that is one of the pages your search actually returned. Never construct, guess, or shorten a URL.
3. Copy the date from the source page. If a page gives a month but no day, or the date is described as "TBA", set starts_on to null and say so in date_note.
4. Prefer the organiser's own page or the official federation page over aggregators, blogs and news write-ups.
5. If a page describes a past edition of a recurring event, do not project a future date from it. Report only dates the page actually states.
6. Answer with the requested JSON only. No preamble, no code fence.`

type Candidate struct {
	Name       string `json:"name"`
	Discipline string `json:"discipline"`
	StartsOn   string `json:"starts_on"`
	EndsOn     string `json:"ends_on"`
	City       string `json:"city"`
	Country    string `json:"country"`
	URL        string `json:"url"`
	SourceURL  string `json:"source_url"`
	DateNote   string `json:"date_note"`
}

type Outcome struct {
	Candidate  Candidate `json:"candidate"`
	EventID    string    `json:"event_id,omitempty"`
	Confidence string    `json:"confidence"`
	Note       string    `json:"note"`
	Evidence   string    `json:"evidence,omitempty"`
	Stored     bool      `json:"stored"`
}

type DiscoverRequest struct {
	Discipline string
	Country    string
	From       time.Time
	To         time.Time
}

type DiscoverReport struct {
	Outcomes    []Outcome `json:"outcomes"`
	SearchCount int       `json:"searches_used"`
	Confirmed   int       `json:"confirmed"`
	NeedsReview int       `json:"needs_review"`
	Rejected    int       `json:"rejected"`
	FromCache   bool      `json:"from_cache"`
}

var validDisciplines = map[string]bool{
	"weighted": true, "statics": true, "dynamics": true, "streetlifting": true,
	"freestyle": true, "endurance": true, "mixed": true,
}

type Service struct {
	pool *pgxpool.Pool
	// ai hands out one client per athlete: a discovery run is searched and
	// paid for on the account of whoever asked for it.
	ai       *ai.Store
	verifier *Verifier
}

func New(pool *pgxpool.Pool, store *ai.Store) *Service {
	return &Service{pool: pool, ai: store, verifier: NewVerifier()}
}

// RunDiscovery runs the full pipeline: search, discard anything whose source the
// API did not actually retrieve, verify the survivors against their live page,
// then store with the confidence that check earned.
func (s *Service) RunDiscovery(ctx context.Context, client *ai.Client, userID string, in DiscoverRequest) (DiscoverReport, error) {
	report := DiscoverReport{Outcomes: []Outcome{}}

	prompt := fmt.Sprintf(`Find calisthenics or street workout competitions taking place between %s and %s.
Discipline wanted: %s
Region wanted: %s

Search for organiser pages, national federation calendars and competition registration pages.
Run several different searches before answering.

Return JSON:
{
  "events": [
    {
      "name": "official event name as written on the page",
      "discipline": "one of: weighted, statics, dynamics, streetlifting, freestyle, endurance, mixed",
      "starts_on": "YYYY-MM-DD or null if the page does not state a day",
      "ends_on": "YYYY-MM-DD or null",
      "city": "",
      "country": "two-letter country code",
      "url": "the public page a competitor would visit",
      "source_url": "the exact URL of the page you read the date from",
      "date_note": "quote or paraphrase how the page states the date"
    }
  ]
}`,
		in.From.Format("2006-01-02"), in.To.Format("2006-01-02"),
		orAny(in.Discipline), orAny(in.Country))

	var payload struct {
		Events []Candidate `json:"events"`
	}

	// Roomy on purpose: the ceiling covers the model's reasoning and the
	// pages it reads as well as the answer it writes.
	searchResult, err := client.SearchJSON(ctx, userID, "event_discovery", discoverySystem, prompt,
		16000, ai.SearchOptions{MaxSearches: 8}, &payload)

	report.SearchCount = searchResult.SearchCount
	if err != nil {
		s.recordRun(ctx, in, report, err.Error())
		return report, err
	}

	// The allowlist: only pages the search API genuinely fetched.
	retrieved := searchResult.SourceURLs()

	for _, candidate := range payload.Events {
		outcome := s.assess(ctx, candidate, retrieved)
		switch outcome.Confidence {
		case ConfidenceDate:
			report.Confirmed++
		case ConfidenceRejected:
			report.Rejected++
		default:
			report.NeedsReview++
		}
		report.Outcomes = append(report.Outcomes, outcome)
	}

	s.recordRun(ctx, in, report, "")
	return report, nil
}

func (s *Service) assess(ctx context.Context, candidate Candidate, retrieved map[string]ai.Source) Outcome {
	outcome := Outcome{Candidate: candidate}

	if strings.TrimSpace(candidate.Name) == "" {
		outcome.Confidence = ConfidenceRejected
		outcome.Note = "The entry had no event name."
		return outcome
	}

	// A source the search never returned is an invented one. This is the single
	// most important check in the pipeline.
	source, ok := retrieved[normaliseForLookup(candidate.SourceURL)]
	if !ok {
		outcome.Confidence = ConfidenceRejected
		outcome.Note = "The cited source was not among the pages the search actually retrieved, so it cannot be trusted."
		return outcome
	}

	if strings.TrimSpace(candidate.StartsOn) == "" || strings.EqualFold(candidate.StartsOn, "null") {
		outcome.Confidence = ConfidenceRejected
		outcome.Note = "No specific date was stated. " + candidate.DateNote
		return outcome
	}

	startsOn, err := time.Parse("2006-01-02", candidate.StartsOn)
	if err != nil {
		outcome.Confidence = ConfidenceRejected
		outcome.Note = "The date was not a usable calendar date."
		return outcome
	}

	verdict := s.verifier.Check(ctx, candidate.SourceURL, candidate.Name, startsOn)
	outcome.Confidence = verdict.Confidence
	outcome.Note = verdict.Note
	outcome.Evidence = verdict.Evidence
	if verdict.Evidence == "" {
		outcome.Evidence = source.CitedText
	}

	if verdict.Confidence == ConfidenceRejected {
		return outcome
	}

	id, err := s.store(ctx, candidate, startsOn, source, verdict)
	if err != nil {
		log.Printf("store event %q: %v", candidate.Name, err)
		outcome.Note += " (It could not be saved.)"
		return outcome
	}
	outcome.EventID = id
	outcome.Stored = true
	return outcome
}

func (s *Service) store(ctx context.Context, candidate Candidate, startsOn time.Time,
	source ai.Source, verdict Verdict) (string, error) {

	discipline := strings.ToLower(strings.TrimSpace(candidate.Discipline))
	if !validDisciplines[discipline] {
		discipline = "mixed"
	}

	var endsOn *time.Time
	if candidate.EndsOn != "" && !strings.EqualFold(candidate.EndsOn, "null") {
		if parsed, err := time.Parse("2006-01-02", candidate.EndsOn); err == nil {
			endsOn = &parsed
		}
	}

	// verified drives what users see as fact: a machine-checked date on a live
	// page, or a human sign-off. Everything else stays in the review queue.
	verified := verdict.Confidence == ConfidenceDate || verdict.Confidence == ConfidenceHuman

	var id string
	err := s.pool.QueryRow(ctx, `
		insert into events
			(name, discipline, starts_on, ends_on, city, country, url,
			 source, source_url, source_title, evidence, confidence, check_note, checked_at, verified)
		values ($1, $2, $3, $4, $5, $6, $7, 'ai_search', $8, $9, $10, $11, $12, now(), $13)
		on conflict (lower(name), starts_on) do update set
			ends_on      = coalesce(excluded.ends_on, events.ends_on),
			city         = case when events.city = '' then excluded.city else events.city end,
			country      = case when events.country = '' then excluded.country else events.country end,
			url          = case when events.url = '' then excluded.url else events.url end,
			source_url   = excluded.source_url,
			source_title = excluded.source_title,
			evidence     = excluded.evidence,
			check_note   = excluded.check_note,
			checked_at   = now(),
			-- never downgrade a human decision
			confidence   = case when events.confidence in ('human_confirmed', 'rejected')
			                    then events.confidence else excluded.confidence end,
			verified     = case when events.confidence in ('human_confirmed', 'rejected')
			                    then events.verified else excluded.verified end
		returning id`,
		strings.TrimSpace(candidate.Name), discipline, startsOn, endsOn,
		candidate.City, strings.ToUpper(candidate.Country), candidate.URL,
		candidate.SourceURL, source.Title, truncate(verdict.Evidence+" "+candidate.DateNote, 600),
		verdict.Confidence, verdict.Note, verified,
	).Scan(&id)
	return id, err
}

// Recheck re-verifies a stored event against its source. Run it on a schedule:
// event pages change dates and disappear.
func (s *Service) Recheck(ctx context.Context, eventID string) (Verdict, error) {
	var name, sourceURL, confidence string
	var startsOn time.Time
	err := s.pool.QueryRow(ctx,
		`select name, source_url, starts_on, confidence from events where id = $1`, eventID).
		Scan(&name, &sourceURL, &startsOn, &confidence)
	if err != nil {
		return Verdict{}, err
	}

	verdict := s.verifier.Check(ctx, sourceURL, name, startsOn)

	// A human decision outranks a machine recheck, but a dead link is still
	// worth recording in the note.
	if confidence == ConfidenceHuman {
		_, err = s.pool.Exec(ctx,
			`update events set check_note = $2, checked_at = now() where id = $1`,
			eventID, verdict.Note)
		return verdict, err
	}

	_, err = s.pool.Exec(ctx, `
		update events
		set confidence = $2, check_note = $3, checked_at = now(),
		    evidence = case when $4 = '' then evidence else $4 end,
		    verified = ($2 = 'date_confirmed')
		where id = $1`,
		eventID, verdict.Confidence, verdict.Note, verdict.Evidence)
	return verdict, err
}

// RecheckStale re-verifies whatever has not been checked in a week.
func (s *Service) RecheckStale(ctx context.Context, limit int) {
	rows, err := s.pool.Query(ctx, `
		select id from events
		where starts_on >= current_date
		  and (checked_at is null or checked_at < now() - interval '7 days')
		order by checked_at nulls first
		limit $1`, limit)
	if err != nil {
		log.Printf("recheck query: %v", err)
		return
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()

	for _, id := range ids {
		if _, err := s.Recheck(ctx, id); err != nil {
			log.Printf("recheck %s: %v", id, err)
		}
		// Be a polite visitor to small event sites.
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
	if len(ids) > 0 {
		log.Printf("rechecked %d events", len(ids))
	}
}

func (s *Service) recordRun(ctx context.Context, in DiscoverRequest, report DiscoverReport, errText string) {
	_, err := s.pool.Exec(ctx, `
		insert into discovery_runs
			(discipline, country, from_date, to_date, searches_used, found, confirmed, rejected, error)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		in.Discipline, in.Country, in.From, in.To,
		report.SearchCount, len(report.Outcomes), report.Confirmed, report.Rejected, errText)
	if err != nil {
		log.Printf("record discovery run: %v", err)
	}
}

func normaliseForLookup(u string) string {
	u = strings.TrimSpace(strings.ToLower(u))
	u = strings.TrimSuffix(u, "/")
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	return strings.TrimPrefix(u, "www.")
}

func orAny(s string) string {
	if strings.TrimSpace(s) == "" {
		return "any"
	}
	return s
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n]
}
