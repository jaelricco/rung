package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Every completion is streamed, for two reasons.
//
// The first is correctness. Current models think before they answer, and
// thinking tokens are spent out of max_tokens. A plan asked for in one
// non-streaming call with a small ceiling can hit that ceiling while still
// thinking: the API answers 200 with a thinking block and no text block at
// all, which reads downstream as "the model returned an empty response". A
// stream tells us which phase the tokens went to and why the turn ended, so
// the failure names itself.
//
// The second is the progress bar. The deltas are the only honest source of
// "how far along is it".

// Delta is one increment of the model's output. Kind is "thinking" while the
// model is reasoning and "text" once it is writing the answer.
type Delta struct {
	Kind string
	Text string
}

type streamEvent struct {
	Type    string `json:"type"`
	Message struct {
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
	ContentBlock struct {
		Type string `json:"type"`
	} `json:"content_block"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		Thinking    string `json:"thinking"`
		StopReason  string `json:"stop_reason"`
		StopDetails *struct {
			Type        string `json:"type"`
			Category    string `json:"category"`
			Explanation string `json:"explanation"`
		} `json:"stop_details"`
	} `json:"delta"`
	Usage struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// CompleteStream sends one prompt and returns the answer text, calling onDelta
// (which may be nil) for every increment as it arrives.
func (c *Client) CompleteStream(ctx context.Context, userID, purpose, system, prompt string,
	maxTokens int, onDelta func(Delta)) (string, error) {

	if !c.Configured() {
		return "", ErrNotConfigured
	}
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}

	body, err := json.Marshal(apiRequest{
		Model:     c.model,
		MaxTokens: maxTokens,
		System:    system,
		Messages:  []message{{Role: "user", Content: prompt}},
		Stream:    true,
		Thinking:  c.thinkingConfig(),
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.key)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Accept", "text/event-stream")

	started := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		c.record(ctx, userID, purpose, prompt, "", 0, 0, time.Since(started), false)
		return "", fmt.Errorf("reaching the model failed: %w", err)
	}
	defer resp.Body.Close()

	// An error answers the streaming request with an ordinary JSON body.
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		msg := resp.Status
		var parsed apiResponse
		if json.Unmarshal(raw, &parsed) == nil && parsed.Error != nil {
			msg = parsed.Error.Message
		}
		c.record(ctx, userID, purpose, prompt, msg, 0, 0, time.Since(started), false)
		return "", fmt.Errorf("model returned an error: %s", msg)
	}

	var (
		text       strings.Builder
		thinking   int
		stopReason string
		stopNote   string
		inTokens   int
		outTokens  int
	)

	reader := bufio.NewReaderSize(resp.Body, 64<<10)
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			payload, ok := sseData(line)
			if ok {
				var ev streamEvent
				if json.Unmarshal([]byte(payload), &ev) != nil {
					continue // a frame we do not model; keep reading
				}
				switch ev.Type {
				case "message_start":
					inTokens = ev.Message.Usage.InputTokens
				case "content_block_delta":
					switch ev.Delta.Type {
					case "text_delta":
						text.WriteString(ev.Delta.Text)
						if onDelta != nil {
							onDelta(Delta{Kind: "text", Text: ev.Delta.Text})
						}
					case "thinking_delta":
						thinking += len(ev.Delta.Thinking)
						if onDelta != nil {
							onDelta(Delta{Kind: "thinking", Text: ev.Delta.Thinking})
						}
					}
				case "message_delta":
					if ev.Delta.StopReason != "" {
						stopReason = ev.Delta.StopReason
					}
					if d := ev.Delta.StopDetails; d != nil {
						stopNote = strings.TrimSpace(d.Category + " " + d.Explanation)
					}
					if ev.Usage.OutputTokens > 0 {
						outTokens = ev.Usage.OutputTokens
					}
				case "error":
					if ev.Error != nil {
						c.record(ctx, userID, purpose, prompt, ev.Error.Message, inTokens, outTokens, time.Since(started), false)
						return text.String(), fmt.Errorf("model returned an error: %s", ev.Error.Message)
					}
				}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			c.record(ctx, userID, purpose, prompt, text.String(), inTokens, outTokens, time.Since(started), false)
			return text.String(), fmt.Errorf("the model's answer was cut off: %w", err)
		}
	}

	out := text.String()
	ok := strings.TrimSpace(out) != ""
	c.record(ctx, userID, purpose, prompt, out, inTokens, outTokens, time.Since(started), ok)

	// A turn that ended at the ceiling is not an answer even when some of it
	// arrived: half a plan is not half usable.
	if stopReason == "max_tokens" {
		return out, ceilingError(ok, maxTokens)
	}
	if !ok {
		return "", emptyAnswerError(stopReason, stopNote, thinking)
	}
	return out, nil
}

// ceilingError names the ceiling and the fix. Both halves matter: the ceiling
// covers reasoning as well as the answer, so the number in the message is the
// one to raise.
func ceilingError(hadText bool, maxTokens int) error {
	if hadText {
		return fmt.Errorf("the model ran out of room partway through the answer, at its %d-token ceiling. "+
			"Ask for less at a time, or raise the ceiling for this call", maxTokens)
	}
	return fmt.Errorf("the model spent its whole %d-token ceiling reasoning and never wrote an answer. "+
		"Ask for less at a time, or raise the ceiling for this call", maxTokens)
}

// emptyAnswerError explains a turn that produced no text for some other reason.
func emptyAnswerError(stopReason, stopNote string, thinkingChars int) error {
	if stopReason == "refusal" {
		if stopNote != "" {
			return fmt.Errorf("the model declined to answer (%s)", stopNote)
		}
		return errors.New("the model declined to answer this request")
	}
	if thinkingChars > 0 {
		return fmt.Errorf("the model reasoned but wrote no answer (stop reason: %s)", orUnknown(stopReason))
	}
	return fmt.Errorf("the model returned an empty response (stop reason: %s)", orUnknown(stopReason))
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

// sseData pulls the payload out of one "data: ..." line, ignoring the event
// name lines, comments and blank separators around it.
func sseData(line string) (string, bool) {
	line = strings.TrimRight(line, "\r\n")
	if !strings.HasPrefix(line, "data:") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(line, "data:")), true
}
