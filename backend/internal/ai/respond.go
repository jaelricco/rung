package ai

import (
	"net/http"
	"time"

	"calisthenics/api/internal/httpx"
)

// A coaching endpoint answers in one of two ways. A caller that asks for
// server-sent events gets progress while the model works and the result at the
// end; every other caller gets exactly what it always got, one JSON body.
// Handlers are written once, against this.
type responder struct {
	w      http.ResponseWriter
	stream *httpx.Stream
	sink   *progressSink
}

// begin picks the mode from the request. Call close when the handler returns.
func begin(w http.ResponseWriter, r *http.Request, budget time.Duration) *responder {
	// The model call outlives the server's ordinary write timeout in either
	// mode, so push the deadline out before anything slow starts.
	slack := budget + 30*time.Second
	httpx.ExtendWriteDeadline(w, slack)

	if !httpx.WantsStream(r) {
		return &responder{w: w, sink: &progressSink{}}
	}
	stream := httpx.NewStream(w, slack)
	return &responder{w: w, stream: stream, sink: &progressSink{stream: stream}}
}

func (rs *responder) report(u Progress) { rs.sink.report(u) }

// fail reports the error in whichever shape the caller is reading. The status
// code is unused on a stream, where the headers went out long ago.
func (rs *responder) fail(status int, message string) {
	if rs.stream != nil {
		rs.stream.Fail(message)
		return
	}
	httpx.Fail(rs.w, status, message)
}

func (rs *responder) done(v any) {
	if rs.stream != nil {
		rs.stream.Send("done", v)
		return
	}
	httpx.JSON(rs.w, http.StatusOK, v)
}

func (rs *responder) close() {
	if rs.stream != nil {
		rs.stream.Close()
	}
}
