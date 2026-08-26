package events

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// Verification is done here, in Go, not by asking a model whether it was right.
// A candidate is only believed if:
//
//  1. its source URL is one the search API actually retrieved,
//  2. that URL still returns 200 to us, and
//  3. the event's date and name are literally present in the page text.
//
// Anything short of all three lands in a review queue instead of the calendar.

const (
	ConfidenceUnverified = "unverified"
	ConfidenceSourceLive = "source_live"
	ConfidenceDate       = "date_confirmed"
	ConfidenceHuman      = "human_confirmed"
	ConfidenceRejected   = "rejected"
)

type Verdict struct {
	Confidence string
	Note       string
	Evidence   string
}

var (
	scriptOrStyle = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	htmlTag       = regexp.MustCompile(`(?s)<[^>]*>`)
	whitespace    = regexp.MustCompile(`\s+`)
)

// monthNames covers the languages the events we care about are actually
// published in. Add more as you widen the search region.
var monthNames = map[time.Month][]string{
	time.January:   {"january", "jan", "januar", "janvier", "gennaio", "enero"},
	time.February:  {"february", "feb", "februar", "février", "fevrier", "febbraio", "febrero"},
	time.March:     {"march", "mar", "märz", "marz", "mars", "marzo"},
	time.April:     {"april", "apr", "avril", "aprile", "abril"},
	time.May:       {"may", "mai", "maggio", "mayo"},
	time.June:      {"june", "jun", "juni", "juin", "giugno", "junio"},
	time.July:      {"july", "jul", "juli", "juillet", "luglio", "julio"},
	time.August:    {"august", "aug", "août", "aout", "agosto"},
	time.September: {"september", "sep", "sept", "septembre", "settembre", "septiembre"},
	time.October:   {"october", "oct", "oktober", "octobre", "ottobre", "octubre"},
	time.November:  {"november", "nov", "novembre", "noviembre"},
	time.December:  {"december", "dec", "dezember", "décembre", "decembre", "dicembre", "diciembre"},
}

type Verifier struct {
	http *http.Client
}

func NewVerifier() *Verifier {
	return &Verifier{
		http: &http.Client{
			Timeout: 20 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}
}

// Check fetches the source page and looks for the claimed date and name.
func (v *Verifier) Check(ctx context.Context, sourceURL, eventName string, startsOn time.Time) Verdict {
	if sourceURL == "" {
		return Verdict{Confidence: ConfidenceRejected, Note: "No source URL was given."}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return Verdict{Confidence: ConfidenceRejected, Note: "The source URL is malformed."}
	}
	// Some event sites serve a bare 403 to unknown agents.
	req.Header.Set("User-Agent",
		"Mozilla/5.0 (compatible; calisthenics-training-app/0.1; +https://github.com/)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en,de;q=0.8,fr;q=0.6")

	resp, err := v.http.Do(req)
	if err != nil {
		return Verdict{
			Confidence: ConfidenceRejected,
			Note:       "The source page could not be reached: " + err.Error(),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Verdict{
			Confidence: ConfidenceRejected,
			Note:       fmt.Sprintf("The source page returned %s.", resp.Status),
		}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return Verdict{Confidence: ConfidenceSourceLive, Note: "The source page could not be read in full."}
	}
	text := plainText(string(body))

	nameHit := nameAppears(text, eventName)
	dateHit, evidence := dateAppears(text, startsOn)

	switch {
	case dateHit && nameHit:
		return Verdict{
			Confidence: ConfidenceDate,
			Note:       "The date and the event name both appear on the source page.",
			Evidence:   evidence,
		}
	case dateHit:
		return Verdict{
			Confidence: ConfidenceSourceLive,
			Note:       "The date appears on the page but the event name does not. Confirm this is the right event.",
			Evidence:   evidence,
		}
	case nameHit:
		return Verdict{
			Confidence: ConfidenceSourceLive,
			Note:       "The page mentions the event but not this date. The date may be wrong, or the page may load its schedule with JavaScript.",
		}
	default:
		return Verdict{
			Confidence: ConfidenceSourceLive,
			Note:       "The page is reachable but mentions neither the name nor the date. Treat this as unconfirmed.",
		}
	}
}

func plainText(html string) string {
	html = scriptOrStyle.ReplaceAllString(html, " ")
	html = htmlTag.ReplaceAllString(html, " ")
	html = strings.NewReplacer(
		"&nbsp;", " ", "&amp;", "&", "&#39;", "'", "&quot;", `"`,
		"&auml;", "ä", "&ouml;", "ö", "&uuml;", "ü", "&eacute;", "é",
	).Replace(html)
	return strings.ToLower(whitespace.ReplaceAllString(html, " "))
}

// nameAppears requires a distinctive word from the event name, ignoring the
// filler words that appear on every competition page.
func nameAppears(text, name string) bool {
	filler := map[string]bool{
		"the": true, "and": true, "cup": true, "open": true, "championship": true,
		"championships": true, "competition": true, "event": true, "battle": true,
		"street": true, "workout": true, "calisthenics": true, "international": true,
		"world": true, "national": true, "national's": true, "series": true, "tour": true,
	}

	fields := strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	distinctive := 0
	for _, word := range fields {
		if len(word) < 4 || filler[word] {
			continue
		}
		distinctive++
		if strings.Contains(text, word) {
			return true
		}
	}
	// A name made entirely of filler can't be checked this way; don't claim a
	// match we didn't find.
	return distinctive == 0 && strings.Contains(text, strings.ToLower(strings.TrimSpace(name)))
}

// dateAppears looks for the date written the many ways event pages write it.
func dateAppears(text string, date time.Time) (bool, string) {
	day := date.Day()
	month := int(date.Month())
	year := date.Year()

	numeric := []string{
		fmt.Sprintf("%04d-%02d-%02d", year, month, day),
		fmt.Sprintf("%02d.%02d.%04d", day, month, year),
		fmt.Sprintf("%d.%d.%04d", day, month, year),
		fmt.Sprintf("%02d/%02d/%04d", day, month, year),
		fmt.Sprintf("%02d/%02d/%04d", month, day, year),
		fmt.Sprintf("%d/%d/%04d", day, month, year),
		fmt.Sprintf("%02d-%02d-%04d", day, month, year),
	}
	for _, form := range numeric {
		if strings.Contains(text, form) {
			return true, snippetAround(text, form)
		}
	}

	yearStr := fmt.Sprintf("%d", year)
	for _, name := range monthNames[date.Month()] {
		// "12 september" / "september 12" / "12. september", each near the year.
		forms := []string{
			fmt.Sprintf("%d %s", day, name),
			fmt.Sprintf("%d. %s", day, name),
			fmt.Sprintf("%s %d", name, day),
			fmt.Sprintf("%02d %s", day, name),
		}
		for _, form := range forms {
			index := strings.Index(text, form)
			if index < 0 {
				continue
			}
			// The year must be nearby, or "12 september" matches any year.
			window := text[max(0, index-60):min(len(text), index+len(form)+60)]
			if strings.Contains(window, yearStr) {
				return true, snippetAround(text, form)
			}
		}
	}
	return false, ""
}

func snippetAround(text, needle string) string {
	index := strings.Index(text, needle)
	if index < 0 {
		return ""
	}
	start := max(0, index-70)
	end := min(len(text), index+len(needle)+70)
	return strings.TrimSpace(text[start:end])
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
