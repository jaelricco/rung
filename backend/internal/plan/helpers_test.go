package plan

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"calisthenics/api/internal/training"
)

// The planner names exercise slugs in Go and the database seeds them in SQL,
// which is exactly the kind of pair that drifts apart silently: a renamed slug
// would leave the knowledge base pointing at nothing and the only symptom
// would be quieter plans. So the tests build their library by reading the
// migrations themselves. If the seed changes, these tests are what notices.
var seedRow = regexp.MustCompile(`\('([a-z0-9_]+)',\s*'([^']*)',\s*'([a-z_]+)',\s*'([a-z_]+)',\s*(\d+),\s*'`)

func seededLibrary(t *testing.T) Library {
	t.Helper()

	lib := Library{Exercises: map[string]training.Exercise{}, Protocols: map[string]training.Protocol{}}
	for _, p := range training.Protocols {
		lib.Protocols[p.Slug] = p
	}

	files, err := filepath.Glob(filepath.Join("..", "db", "migrations", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("could not find the migrations: %v", err)
	}
	for _, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if !strings.Contains(string(body), "insert into exercises") {
			continue
		}
		for _, m := range seedRow.FindAllStringSubmatch(string(body), -1) {
			difficulty, _ := strconv.Atoi(m[5])
			e := training.Exercise{Slug: m[1], Name: m[2], Category: m[3], Measure: m[4], Difficulty: difficulty}
			if _, seen := lib.Exercises[e.Slug]; !seen {
				lib.order = append(lib.order, e.Slug)
			}
			lib.Exercises[e.Slug] = e
		}
	}
	if len(lib.Exercises) < 50 {
		t.Fatalf("only parsed %d exercises out of the migrations; the seed format must have changed", len(lib.Exercises))
	}
	return lib
}

func rec(slug string, reps int, added, hold float64) training.Record {
	r := training.Record{Slug: slug, Name: slug, TotalSets: 3}
	if reps > 0 {
		r.BestReps = &reps
	}
	if added > 0 {
		r.BestWeight = &added
	}
	if hold > 0 {
		r.BestHold = &hold
	}
	return r
}

func snapshotOf(sessions28 int, bodyweight float64, records ...training.Record) training.Snapshot {
	snap := training.Snapshot{SessionsLast28: sessions28, Records: records}
	if bodyweight > 0 {
		snap.Bodyweight = &bodyweight
	}
	return snap
}
