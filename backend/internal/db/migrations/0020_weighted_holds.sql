-- A hold with weight on it is a thing, and the library could not say so.
--
-- Every exercise declares one of four measures, and the measure decides what a
-- set of it looks like: reps, reps with added load, a hold in seconds, or an
-- attempt that was made or missed. There is no fifth, and there needed to be —
-- a weighted front lever is held for seconds *and* carries kilos, and neither
-- of the two candidates says both.
--
-- The workaround shipped in 0013 and its own seed comment gave it away:
-- "Record added kg only". So the ladder rung asked for kilos, the exercise
-- claimed to be measured in seconds, and the planner — which prescribes from
-- the measure rather than from the rung — wrote out a hold and dropped the
-- load on the floor. An athlete past a twenty-second front lever was handed
-- "3 sets of a 4-8s hold" with no mention of the belt that is the entire point
-- of the rung.
--
-- Nothing about the data was missing: a set row has always had reps, weight
-- and seconds columns, all nullable and independent. It was only the two
-- enumerations that could not spell it.
alter table exercises drop constraint if exists exercises_measure_check;
alter table exercises add constraint exercises_measure_check
    check (measure in ('reps','weighted_reps','static_hold','weighted_hold','skill_attempt'));

alter table workout_sets drop constraint if exists workout_sets_kind_check;
alter table workout_sets add constraint workout_sets_kind_check
    check (kind in ('reps','weighted_reps','static_hold','weighted_hold','skill_attempt'));

alter table workout_sets drop constraint if exists set_shape;
alter table workout_sets add constraint set_shape check (
    (kind = 'reps'          and reps is not null)
 or (kind = 'weighted_reps' and reps is not null and weight_kg is not null)
 or (kind = 'static_hold'   and hold_seconds is not null)
 or (kind = 'weighted_hold' and hold_seconds is not null and weight_kg is not null)
 or (kind = 'skill_attempt' and success is not null)
);

-- And the one exercise that has been lying about itself since 0013. It is
-- rewritten as an insert rather than an update because the tests build their
-- library by parsing these seed rows: a bare update would fix the database and
-- leave every test still looking at the old measure, which is the shape of bug
-- this migration exists to end.
insert into exercises (slug, name, category, measure, difficulty, description) values
    ('weighted_front_lever', 'Weighted front lever', 'static', 'weighted_hold', 10, 'Full front lever with added load, held for time. Both numbers matter: the seconds and the kilos.')
on conflict (slug) do update set
    name        = excluded.name,
    category    = excluded.category,
    measure     = excluded.measure,
    difficulty  = excluded.difficulty,
    description = excluded.description;
