package plan

import (
	"context"
	"fmt"

	"calisthenics/api/internal/training"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SyncCatalogue writes the reference data into the tables that exist to be
// read from.
//
// The direction matters. Go is where this data is authored — the compiler
// checks it, and the tests that hold the ladders together (that a rung is
// never easier than the one below it, that every slug exists in the library,
// that an athlete lands where their records put them) run in CI with no
// database at all. The tables are a projection of it, rewritten on every boot,
// so anything the app knows can be queried, joined and served without a second
// copy drifting out of step with the first.
//
// It runs after migrations, in one transaction. If it fails the app still
// starts: a stale skills table is a worse read than a fresh one, but it is not
// a reason to refuse to serve training.
func SyncCatalogue(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Wholesale replacement, so a skill deleted in Go disappears here rather
	// than lingering as a row nothing points at any more.
	for _, table := range []string{
		"skill_requirements", "skill_steps", "skills", "injury_regions", "protocols",
		"equipment", "exercise_equipment", "exercise_regions", "category_regions",
		"exercise_substitutes", "level_rubrics",
	} {
		if _, err := tx.Exec(ctx, "delete from "+table); err != nil {
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}

	if err := syncSkills(ctx, tx); err != nil {
		return err
	}
	if err := syncInjuryReference(ctx, tx); err != nil {
		return err
	}
	if err := syncExerciseMaps(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// list makes a slice safe to send to a NOT NULL array column. A nil Go slice
// encodes as SQL NULL, and Postgres does not fall back to a column default for
// a value that was passed explicitly — so a goal with nothing to maintain took
// the whole projection down rather than storing an empty list. It also carries
// the named slice types (chain) over to the plain []string pgx encodes.
func list[S ~[]E, E any](s S) []E {
	if s == nil {
		return []E{}
	}
	return []E(s)
}

func syncSkills(ctx context.Context, tx pgx.Tx) error {
	for i, goal := range Goals {
		_, err := tx.Exec(ctx, `
			insert into skills (key, position, name, phrase, pattern, straight_arm, wrists,
			                    foundation, cost, timeline, frequency, aliases, feeds,
			                    drills, accessories, risks)
			values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
			goal.Key, i, goal.Name, goal.Phrase, goal.Pattern, goal.StraightArm, goal.Wrists,
			goal.Foundation, goal.Units(), goal.Timeline, goal.Frequency,
			list(goal.Aliases), list(goal.Feeds), list(goal.Drills),
			list(goal.Accessories), list(goal.Risks))
		if err != nil {
			return fmt.Errorf("skill %s: %w", goal.Key, err)
		}

		for _, req := range goal.Entry {
			if err := insertRequirement(ctx, tx, goal.Key, nil, req); err != nil {
				return err
			}
		}

		for j, step := range goal.Ladder {
			position := j
			_, err := tx.Exec(ctx, `
				insert into skill_steps (skill_key, position, name, metric, standard,
				                         typical, movements, assists)
				values ($1,$2,$3,$4,$5,$6,$7,$8)`,
				goal.Key, position, step.Name, step.Metric, step.Standard, step.Typical,
				list(step.Movement), list(step.Assist))
			if err != nil {
				return fmt.Errorf("skill %s step %d: %w", goal.Key, position, err)
			}
			for _, req := range step.Gate {
				if err := insertRequirement(ctx, tx, goal.Key, &position, req); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func insertRequirement(ctx context.Context, tx pgx.Tx, skill string, step *int, req Requirement) error {
	_, err := tx.Exec(ctx, `
		insert into skill_requirements (skill_key, step_position, exercise_slug, metric, standard, why)
		values ($1,$2,$3,$4,$5,$6)`,
		skill, step, req.Slug, req.Metric, req.Standard, req.Why)
	if err != nil {
		return fmt.Errorf("requirement %s/%s: %w", skill, req.Slug, err)
	}
	return nil
}

func syncInjuryReference(ctx context.Context, tx pgx.Tx) error {
	for i, region := range training.Regions {
		if _, err := tx.Exec(ctx, `
			insert into injury_regions (key, position, label) values ($1,$2,$3)`,
			region.Key, i, region.Label); err != nil {
			return fmt.Errorf("region %s: %w", region.Key, err)
		}
	}
	for i, p := range training.Protocols {
		if _, err := tx.Exec(ctx, `
			insert into protocols (slug, position, region, title, purpose, steps,
			                       avoid_while, see_clinician)
			values ($1,$2,$3,$4,$5,$6,$7,$8)`,
			p.Slug, i, p.Region, p.Title, p.Purpose, list(p.Steps), list(p.AvoidWhile),
			p.SeeClinician); err != nil {
			return fmt.Errorf("protocol %s: %w", p.Slug, err)
		}
	}
	for i, item := range Equipment {
		if _, err := tx.Exec(ctx, `
			insert into equipment (key, position, label, note) values ($1,$2,$3,$4)`,
			item.Key, i, item.Label, item.Note); err != nil {
			return fmt.Errorf("equipment %s: %w", item.Key, err)
		}
	}
	return nil
}

// syncExerciseMaps writes the three maps that say what a movement loads, what
// it needs, and what it becomes when a joint will not take it. Every one of
// them has a foreign key onto exercises, which is a stronger guarantee than
// the test that used to be the only thing checking these slugs were real.
func syncExerciseMaps(ctx context.Context, tx pgx.Tx) error {
	for _, slug := range sortedKeys(requires) {
		for group, options := range requires[slug] {
			if _, err := tx.Exec(ctx, `
				insert into exercise_equipment (exercise_slug, group_no, options)
				values ($1,$2,$3)`, slug, group, list(options)); err != nil {
				return fmt.Errorf("equipment for %s: %w", slug, err)
			}
		}
	}

	// The explicit per-movement regions, plus the wrist set folded in, so the
	// table says everything the planner knows rather than half of it.
	regions := map[string][]string{}
	for slug, list := range extraRegions {
		regions[slug] = append([]string(nil), list...)
	}
	for slug := range wristLoaded {
		regions[slug] = appendUnique(regions[slug], regionWrist)
	}
	for _, slug := range sortedKeys(regions) {
		if _, err := tx.Exec(ctx, `
			insert into exercise_regions (exercise_slug, regions) values ($1,$2)`,
			slug, list(regions[slug])); err != nil {
			return fmt.Errorf("regions for %s: %w", slug, err)
		}
	}

	for _, category := range sortedKeys(byCategory) {
		if _, err := tx.Exec(ctx, `
			insert into category_regions (category, regions) values ($1,$2)`,
			category, list(byCategory[category])); err != nil {
			return fmt.Errorf("category %s: %w", category, err)
		}
	}

	for _, slug := range sortedKeys(neutralWrist) {
		if _, err := tx.Exec(ctx, `
			insert into exercise_substitutes (exercise_slug, substitute_slug, reason)
			values ($1,$2,$3)`, slug, neutralWrist[slug],
			"keeps the training effect with the wrist out of extension"); err != nil {
			return fmt.Errorf("substitute for %s: %w", slug, err)
		}
	}

	for _, r := range training.Rubrics() {
		if _, err := tx.Exec(ctx, `
			insert into level_rubrics (exercise_slug, metric, cuts) values ($1,$2,$3)`,
			r.Slug, r.Metric, list(r.Cuts)); err != nil {
			return fmt.Errorf("rubric for %s: %w", r.Slug, err)
		}
	}
	return nil
}
