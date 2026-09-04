package ai

import "fmt"

// What a model costs to run, so the settings page can turn an athlete's logged
// token counts into a number they recognise.
//
// This is for display only. It is a copy of a published price list, and a copy
// goes stale: it is never used to bill anyone, an unlisted model simply shows
// no estimate rather than a wrong one, and the page says the real figure lives
// at the provider. Update it here when the published prices move.
//
// Rates are US dollars per million tokens, as published 2026-06-24.
type price struct {
	input  float64
	output float64
}

var prices = map[string]price{
	"claude-opus-5":             {input: 5, output: 25},
	"claude-sonnet-5":           {input: 2, output: 10},
	"claude-haiku-4-5":          {input: 1, output: 5},
	"claude-haiku-4-5-20251001": {input: 1, output: 5},
	"claude-opus-4-8":           {input: 5, output: 25},
	"claude-opus-4-7":           {input: 5, output: 25},
	"claude-sonnet-4-6":         {input: 3, output: 15},
}

// Cached prompt tokens are billed at their own multiples of the input rate: a
// five-minute write costs a quarter more than sending the text plainly, and a
// read a tenth of it. Both are counted apart from input_tokens by the API, so
// leaving them out would undercount every cached call rather than round it.
const (
	cacheWriteMultiple = 1.25
	cacheReadMultiple  = 0.1
)

// estimate returns the dollar cost of one model's tokens, and whether the
// model's price is known at all.
func estimate(model string, in, out, cacheRead, cacheWrite int64) (float64, bool) {
	p, ok := prices[model]
	if !ok {
		return 0, false
	}
	return float64(in)/1e6*p.input +
		float64(cacheWrite)/1e6*p.input*cacheWriteMultiple +
		float64(cacheRead)/1e6*p.input*cacheReadMultiple +
		float64(out)/1e6*p.output, true
}

// money renders a figure small enough to be reassuring without rounding it to
// nothing: a third of a cent should not print as $0.00.
func money(usd float64) string {
	if usd > 0 && usd < 0.01 {
		return "under $0.01"
	}
	return fmt.Sprintf("$%.2f", usd)
}
