package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// What to do when the model's JSON will not parse.
//
// Where the provider enforces a schema this never runs. Where it does not —
// and this app speaks to two providers, only one of which has a wire format
// this build can rely on — an unescaped quote inside a string still ends a
// paid call with nothing to show for it. A research turn costs 83k input
// tokens, and losing it to a stray `"` is the most expensive kind of cheap
// mistake.
//
// So the answer gets one chance to be fixed. The repair turn carries the
// broken document and the parser's own complaint, and nothing else: not the
// search results, not the athlete's records, not the library. It is therefore
// priced at the length of the answer rather than the length of what produced
// it, which is what makes it worth trying at all.

const repairSystem = `You repair malformed JSON. You are given a document that was meant to be JSON and the error a parser reported on it.

Rules:
1. Answer with the corrected JSON only. No preamble, no commentary, no code fence.
2. Change nothing except what is needed to make it parse. The most common cause is an unescaped quote inside a string value: escape it rather than rewriting the sentence around it.
3. Never invent content. If the document is cut off, close whatever is open and drop the incomplete trailing item rather than writing a replacement for it.`

// repairPrompt gives the model the two things it needs and nothing it does
// not: what it wrote, and why that would not parse.
func repairPrompt(broken string, schema json.RawMessage, cause error) string {
	var b strings.Builder
	b.WriteString("The parser rejected this document with:\n")
	b.WriteString(cause.Error())
	if len(schema) > 0 {
		b.WriteString("\n\nIt is meant to satisfy this schema:\n")
		b.Write(schema)
	}
	b.WriteString("\n\nDOCUMENT\n")
	b.WriteString(broken)
	return b.String()
}

// repairTokens leaves room for the whole document to come back, since a repair
// rewrites all of it. Roughly three characters to the token, with headroom,
// and never below the ordinary ceiling.
func repairTokens(broken string) int {
	tokens := len(broken)/3 + 2000
	if tokens < defaultMaxTokens {
		tokens = defaultMaxTokens
	}
	if tokens > 64000 {
		tokens = 64000
	}
	return tokens
}

// parseOrRepair unmarshals the answer, spending one short turn on a fix if it
// will not parse. The error it returns on failure is the original one: that is
// the complaint that describes what the model actually did wrong.
func (c *Client) parseOrRepair(ctx context.Context, userID, purpose, text string,
	schema json.RawMessage, dst any) error {

	cause := json.Unmarshal([]byte(extractJSON(text)), dst)
	if cause == nil {
		return nil
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("the model's answer wasn't usable JSON: %w", cause)
	}

	// No cached prefix: a repair is a one-off document that never recurs, so
	// there is nothing here a cache write would ever be read back for.
	fixed, err := c.completeStream(ctx, userID, purpose+"_repair", repairSystem,
		repairPrompt(text, schema, cause), "", repairTokens(text), schema, nil)
	if err != nil {
		return fmt.Errorf("the model's answer wasn't usable JSON: %w", cause)
	}
	if err := json.Unmarshal([]byte(extractJSON(fixed)), dst); err != nil {
		return fmt.Errorf("the model's answer wasn't usable JSON: %w", cause)
	}
	return nil
}
