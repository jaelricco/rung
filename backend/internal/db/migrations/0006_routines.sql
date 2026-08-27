-- A routine is the training an athlete already does: the same week, repeated,
-- written by them rather than generated. Plans answer "how do I get to the
-- planche"; a routine answers "this is what my week looks like right now", and
-- until now there was nowhere to put that.
--
-- The template lives here; the calendar keeps holding dated planned_sessions,
-- so completion, the workout link and the .ics export keep working unchanged.
create table routines (
    id         uuid primary key default gen_random_uuid(),
    user_id    uuid not null references users(id) on delete cascade,
    title      text not null,
    notes      text not null default '',
    -- Active is the "every week the same" switch. An inactive routine is kept
    -- as a template: it still exists, it just stops filling the weeks ahead.
    active     boolean not null default true,
    starts_on  date not null default current_date,
    ends_on    date,
    -- How far ahead this routine has already been written onto the calendar.
    -- Filling only past this date is what makes a deleted session stay
    -- deleted instead of reappearing on the next calendar load.
    materialized_through date,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);
create index routines_user_idx on routines (user_id, created_at desc);

-- One training day of the week. The body is the same shape a plan session
-- has, so the calendar and the session detail render both without knowing
-- which one they were handed.
create table routine_days (
    id          uuid primary key default gen_random_uuid(),
    routine_id  uuid not null references routines(id) on delete cascade,
    day_of_week int  not null check (day_of_week between 1 and 7),
    position    int  not null default 0,
    title       text not null,
    focus       text not null default '',
    body        jsonb not null default '{}'::jsonb
);
create index routine_days_routine_idx on routine_days (routine_id, day_of_week, position);

alter table planned_sessions
    add column routine_id     uuid references routines(id) on delete set null,
    add column routine_day_id uuid references routine_days(id) on delete set null,
    -- Where the session came from: a generated plan, a repeating routine, or
    -- typed straight onto the day. It survives the routine being deleted,
    -- which is why it is stored rather than derived from the ids.
    add column source text not null default 'plan'
        check (source in ('plan', 'routine', 'manual'));

update planned_sessions set source = 'manual' where plan_id is null;

-- One routine day can only land once on a given date. Repeated fills are
-- therefore insert-or-ignore, and nulls stay distinct so hand-made sessions
-- are unaffected.
create unique index planned_sessions_routine_slot
    on planned_sessions (routine_day_id, scheduled_on);
