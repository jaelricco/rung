package ai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// answer is a stand-in for a coaching handler: some progress, then a result.
func answer(w http.ResponseWriter, r *http.Request) {
	out := begin(w, r, time.Minute)
	defer out.close()
	out.report(Progress{Stage: "reading", Label: "Reading your training history", Percent: 2})
	out.report(Progress{Stage: "writing", Label: "Writing session 1 of 3", Percent: 40, Done: 1, Total: 3})
	out.done(map[string]string{"text": "ready"})
}

func TestResponderStreamsProgressThenResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(answer))
	defer server.Close()

	req, _ := http.NewRequest(http.MethodPost, server.URL, nil)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content type = %q", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	got := string(body)

	for _, want := range []string{
		"event: progress\ndata: {\"stage\":\"reading\"",
		"\"percent\":40",
		"\"done\":1",
		"event: done\ndata: {\"text\":\"ready\"}",
		"event: end\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stream missing %q:\n%s", want, got)
		}
	}
}

func TestResponderStillAnswersPlainJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(answer))
	defer server.Close()

	resp, err := http.Post(server.URL, "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "event:") {
		t.Fatalf("a caller that did not ask for a stream got one:\n%s", body)
	}
	if strings.TrimSpace(string(body)) != `{"text":"ready"}` {
		t.Fatalf("body = %s", body)
	}
}

func TestStreamedFailureCarriesTheMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out := begin(w, r, time.Minute)
		defer out.close()
		out.fail(http.StatusBadGateway, "Couldn't build that plan: the model hit its ceiling")
	}))
	defer server.Close()

	req, _ := http.NewRequest(http.MethodPost, server.URL, nil)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `event: error`) ||
		!strings.Contains(string(body), "hit its ceiling") {
		t.Fatalf("error frame missing:\n%s", body)
	}
}
