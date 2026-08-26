package httpx

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

// Server-sent events. AI work takes tens of seconds and the caller deserves to
// see it move, so the same handlers answer either with one JSON body or with a
// stream of progress events. The client asks for the stream with an
// Accept: text/event-stream header; everything else keeps the old behaviour.

// WantsStream reports whether the caller asked for server-sent events.
func WantsStream(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/event-stream")
}

// Stream writes server-sent events. Every send flushes, because a progress bar
// that arrives in one lump at the end is worse than no progress bar.
type Stream struct {
	w    http.ResponseWriter
	ctrl *http.ResponseController
	// budget is how long the whole stream may run; the write deadline is
	// pushed to it on every flush so the server's WriteTimeout, sized for
	// ordinary requests, does not cut a long generation off.
	budget time.Duration
	failed bool
}

// NewStream writes the SSE headers and returns the writer. Call Close when done.
func NewStream(w http.ResponseWriter, budget time.Duration) *Stream {
	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("Connection", "keep-alive")
	// Tells nginx-style proxies not to buffer. Caddy is configured not to
	// compress this content type for the same reason.
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	s := &Stream{w: w, ctrl: http.NewResponseController(w), budget: budget}
	s.flush()
	return s
}

// Send writes one named event carrying v as JSON.
func (s *Stream) Send(event string, v any) {
	if s.failed {
		return
	}
	payload, err := json.Marshal(v)
	if err != nil {
		log.Printf("stream encode %s: %v", event, err)
		return
	}
	if _, err := s.w.Write([]byte("event: " + event + "\ndata: " + string(payload) + "\n\n")); err != nil {
		// The client went away. Stop writing; the handler's context will be
		// cancelled by the server shortly after.
		s.failed = true
		return
	}
	s.flush()
}

// Fail sends a terminal error event in the same shape as a JSON error body.
func (s *Stream) Fail(message string) {
	s.Send("error", map[string]string{"error": message})
}

// Close sends the end-of-stream marker.
func (s *Stream) Close() {
	if s.failed {
		return
	}
	_, _ = s.w.Write([]byte("event: end\ndata: {}\n\n"))
	s.flush()
}

func (s *Stream) flush() {
	// Both are best-effort: a ResponseWriter that supports neither still
	// works, it just buffers.
	_ = s.ctrl.SetWriteDeadline(time.Now().Add(s.budget))
	_ = s.ctrl.Flush()
}

// ExtendWriteDeadline gives one handler longer than the server's WriteTimeout.
// Used by the AI endpoints, whose answers depend on a model call.
func ExtendWriteDeadline(w http.ResponseWriter, d time.Duration) {
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(d))
}
