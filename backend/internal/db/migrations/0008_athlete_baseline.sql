-- What the athlete can do, before they have logged it here.
--
-- The planner reads records to place someone on a ladder and to size every
-- prescription. Someone who signed up yesterday has none, so they get the
-- bottom of every ladder and a deliberately light week — correct, and useless
-- to the athlete who already has twelve pull-ups. These are what they can tell
-- us instead: the same measurements, self-reported, superseded the moment a
-- real set is logged for that movement.
create table baseline_records (
    user_id      uuid not null references users(id) on delete cascade,
    exercise_id  uuid not null references exercises(id) on delete cascade,
    reps         int           check (reps >= 0),
    weight_kg    numeric(6,2)  check (weight_kg >= 0),
    hold_seconds numeric(6,2)  check (hold_seconds >= 0),
    recorded_at  timestamptz not null default now(),
    primary key (user_id, exercise_id),
    -- A row that says nothing is a row that should have been deleted.
    constraint baseline_states_something check (
        reps is not null or weight_kg is not null or hold_seconds is not null)
);

-- Training context: the things a plan needs to know that are not a set.
--
-- All three are nullable because unanswered and answered-with-nothing are
-- different facts. Equipment especially: an empty array means "I have none of
-- it", which removes most of the library, and null means "not asked yet",
-- which removes nothing.
alter table users add column trains_per_week int          check (trains_per_week between 0 and 14);
alter table users add column sleep_hours     numeric(3,1) check (sleep_hours between 0 and 24);
alter table users add column equipment       text[];
