package ai

import (
	"context"
	"net/http"
	"time"

	"calisthenics/api/internal/auth"
	"calisthenics/api/internal/httpx"
)

// What the coaching has actually cost this athlete. The question every athlete
// asks before pasting a key is "how much will this run me", and the app has
// been recording the answer in ai_calls since the first plan: tokens in,
// tokens out, per call, per model. This turns that into a sentence.
//
// The figure is an estimate from a published price list, not a bill. The bill
// is at the provider, and the page says so.

type usageWindow struct {
	Calls        int64 `json:"calls"`
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	// Estimated is the rendered figure, empty when no model in the window has
	// a known price. Priced says whether every call in it was priced, so the
	// page can say "at least" rather than overstate its own precision.
	Estimated string `json:"estimated,omitempty"`
	Priced    bool   `json:"priced"`
}

type usageReport struct {
	Month usageWindow `json:"month"`
	Total usageWindow `json:"total"`
	// Since is when the first call was recorded, so "total" has a beginning.
	Since *time.Time `json:"since,omitempty"`
}

// Usage reports this athlete's own model spend. It reads only their rows.
func (h *Handler) Usage(w http.ResponseWriter, r *http.Request) {
	me := auth.MustUser(r.Context())

	month, err := h.usageSince(r.Context(), me.ID, startOfMonth())
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't read your usage.")
		return
	}
	total, err := h.usageSince(r.Context(), me.ID, time.Time{})
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't read your usage.")
		return
	}

	out := usageReport{Month: month, Total: total}
	var since time.Time
	if err := h.pool.QueryRow(r.Context(),
		`select min(created_at) from ai_calls where user_id = $1`, me.ID).Scan(&since); err == nil &&
		!since.IsZero() {
		out.Since = &since
	}
	httpx.JSON(w, http.StatusOK, out)
}

func startOfMonth() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// usageSince totals one athlete's calls per model, so each model's own rate
// applies. A model with no published price still counts towards the calls and
// the tokens; it just contributes nothing to the estimate, and says so.
func (h *Handler) usageSince(ctx context.Context, userID string, since time.Time) (usageWindow, error) {
	var window usageWindow

	rows, err := h.pool.Query(ctx, `
		select model, count(*), coalesce(sum(input_tokens), 0), coalesce(sum(output_tokens), 0)
		from ai_calls
		where user_id = $1 and created_at >= $2
		group by model`, userID, since)
	if err != nil {
		return window, err
	}
	defer rows.Close()

	var (
		usd    float64
		known  bool
		missed bool
	)
	for rows.Next() {
		var (
			model   string
			calls   int64
			in, out int64
		)
		if err := rows.Scan(&model, &calls, &in, &out); err != nil {
			return window, err
		}
		window.Calls += calls
		window.InputTokens += in
		window.OutputTokens += out

		if cost, ok := estimate(model, in, out); ok {
			usd += cost
			known = true
		} else if calls > 0 {
			missed = true
		}
	}
	if err := rows.Err(); err != nil {
		return window, err
	}

	if known {
		window.Estimated = money(usd)
		window.Priced = !missed
	}
	return window, nil
}
