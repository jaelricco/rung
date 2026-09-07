package training

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"calisthenics/api/internal/auth"
	"calisthenics/api/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// A tick names a block by its position, which only means anything against the
// body it was ticked against. Everything that could make a position lie —
// something out of range, a protocol the session does not name, the same tick
// twice, a body that has since been rewritten — has to be refused here, or a
// box comes back ticked against whatever took that position.
//
// Runs against TEST_DATABASE_URL and skips without it.
func TestProgressOnlyKeepsTicksThatStillMeanSomething(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run the session progress test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := New(pool)

	var userID string
	if err := pool.QueryRow(ctx, `
		insert into users (email, password_hash, display_name)
		values ($1, 'x', 'Progress') returning id`,
		"progress+"+t.Name()+"@example.test").Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "delete from users where id = $1", userID) })

	body := `{"title":"Ticking","focus":"push","warmup_protocols":["joint_warmup"],
		"blocks":[{"exercise_slug":"push_up","sets":3,"prescription":"10 reps"},
		          {"exercise_slug":"dip","sets":3,"prescription":"8 reps"}]}`
	var sessionID string
	if err := pool.QueryRow(ctx, `
		insert into planned_sessions (user_id, source, scheduled_on, title, focus, body)
		values ($1, 'manual', current_date, 'Ticking', 'push', $2) returning id`,
		userID, body).Scan(&sessionID); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	put := func(payload string) CalendarEntry {
		t.Helper()
		r := httptest.NewRequest(http.MethodPut,
			"/api/v1/sessions/"+sessionID+"/progress", strings.NewReader(payload))
		r.SetPathValue("id", sessionID)
		r = r.WithContext(auth.WithUser(r.Context(), auth.User{ID: userID}))
		w := httptest.NewRecorder()
		svc.Progress(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("progress: %d %s", w.Code, w.Body.String())
		}
		var out CalendarEntry
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return out
	}

	got := put(`{"done_protocols":["joint_warmup","shoulder_warmup"],"done_blocks":[1,1,5,-1,0]}`)

	// shoulder_warmup is a real protocol, but not one this session names.
	if len(got.DoneProtocols) != 1 || got.DoneProtocols[0] != "joint_warmup" {
		t.Errorf("kept protocols %v, want just the one the session names", got.DoneProtocols)
	}
	// 5 is past the end, -1 is not a position, and 1 was sent twice.
	if len(got.DoneBlocks) != 2 || got.DoneBlocks[0] != 0 || got.DoneBlocks[1] != 1 {
		t.Errorf("kept blocks %v, want [0 1]", got.DoneBlocks)
	}

	// The write is a replacement, not an addition.
	if got := put(`{"done_protocols":[],"done_blocks":[1]}`); len(got.DoneProtocols) != 0 ||
		len(got.DoneBlocks) != 1 || got.DoneBlocks[0] != 1 {
		t.Errorf("a second write left %v / %v, want none and [1]", got.DoneProtocols, got.DoneBlocks)
	}

	// Rewriting the body voids the positions the ticks were made against.
	rewrite := httptest.NewRequest(http.MethodPatch, "/api/v1/sessions/"+sessionID,
		strings.NewReader(`{"body":{"title":"Rewritten","focus":"pull",
			"blocks":[{"exercise_slug":"pull_up","sets":3,"prescription":"5 reps"}]}}`))
	rewrite.SetPathValue("id", sessionID)
	rewrite = rewrite.WithContext(auth.WithUser(rewrite.Context(), auth.User{ID: userID}))
	w := httptest.NewRecorder()
	svc.UpdateSession(w, rewrite)
	if w.Code != http.StatusOK {
		t.Fatalf("rewrite: %d %s", w.Code, w.Body.String())
	}
	var after CalendarEntry
	if err := json.Unmarshal(w.Body.Bytes(), &after); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(after.DoneBlocks) != 0 || len(after.DoneProtocols) != 0 {
		t.Errorf("a rewritten body kept %v / %v ticked", after.DoneProtocols, after.DoneBlocks)
	}

	// A change that is not to the body leaves them alone.
	if _, err := pool.Exec(ctx,
		`update planned_sessions set done_blocks = '{0}' where id = $1`, sessionID); err != nil {
		t.Fatalf("re-tick: %v", err)
	}
	move := httptest.NewRequest(http.MethodPatch, "/api/v1/sessions/"+sessionID,
		strings.NewReader(`{"scheduled_on":"2027-01-04"}`))
	move.SetPathValue("id", sessionID)
	move = move.WithContext(auth.WithUser(move.Context(), auth.User{ID: userID}))
	w = httptest.NewRecorder()
	svc.UpdateSession(w, move)
	if w.Code != http.StatusOK {
		t.Fatalf("move: %d %s", w.Code, w.Body.String())
	}
	var moved CalendarEntry
	if err := json.Unmarshal(w.Body.Bytes(), &moved); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(moved.DoneBlocks) != 1 {
		t.Errorf("moving the session to another day cleared its ticks: %v", moved.DoneBlocks)
	}
}
