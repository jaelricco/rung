-- Core schema. Every migration runs inside a transaction.

create extension if not exists postgis;

create table users (
    id            uuid primary key default gen_random_uuid(),
    email         text not null,
    password_hash text not null,
    display_name  text not null default '',
    bodyweight_kg numeric(5,2),
    created_at    timestamptz not null default now()
);
create unique index users_email_key on users (lower(email));

create table sessions (
    token_hash bytea primary key,
    user_id    uuid not null references users(id) on delete cascade,
    expires_at timestamptz not null,
    created_at timestamptz not null default now()
);
create index sessions_user_idx on sessions (user_id);
create index sessions_expiry_idx on sessions (expires_at);

-- The exercise library. `measure` decides which columns a set must fill in.
create table exercises (
    id          uuid primary key default gen_random_uuid(),
    slug        text not null unique,
    name        text not null,
    category    text not null check (category in
                  ('pull','push','static','dynamic','weighted','core','legs','mobility')),
    measure     text not null check (measure in
                  ('reps','weighted_reps','static_hold','skill_attempt')),
    difficulty  int  not null default 1 check (difficulty between 1 and 10),
    description text not null default ''
);

create table workouts (
    id           uuid primary key default gen_random_uuid(),
    user_id      uuid not null references users(id) on delete cascade,
    performed_at timestamptz not null default now(),
    notes        text not null default '',
    rpe          int check (rpe between 1 and 10),
    created_at   timestamptz not null default now()
);
create index workouts_user_time_idx on workouts (user_id, performed_at desc);

create table workout_sets (
    id           uuid primary key default gen_random_uuid(),
    workout_id   uuid not null references workouts(id) on delete cascade,
    exercise_id  uuid not null references exercises(id),
    position     int  not null,
    kind         text not null check (kind in
                   ('reps','weighted_reps','static_hold','skill_attempt')),
    reps         int           check (reps >= 0),
    weight_kg    numeric(6,2),
    hold_seconds numeric(6,2)  check (hold_seconds >= 0),
    success      boolean,
    constraint set_shape check (
        (kind = 'reps'          and reps is not null)
     or (kind = 'weighted_reps' and reps is not null and weight_kg is not null)
     or (kind = 'static_hold'   and hold_seconds is not null)
     or (kind = 'skill_attempt' and success is not null)
    )
);
create index workout_sets_workout_idx on workout_sets (workout_id);
create index workout_sets_exercise_idx on workout_sets (exercise_id);

-- Injuries constrain plan generation: an open injury is passed to the model
-- as a hard restriction, and filters the exercise pool it may draw from.
create table injuries (
    id          uuid primary key default gen_random_uuid(),
    user_id     uuid not null references users(id) on delete cascade,
    region      text not null check (region in
                  ('wrist','elbow','shoulder','chest','back','core','hip','knee','ankle','other')),
    severity    int  not null check (severity between 1 and 5),
    description text not null default '',
    started_on  date not null default current_date,
    resolved_on date,
    created_at  timestamptz not null default now()
);
create index injuries_open_idx on injuries (user_id) where resolved_on is null;

-- Competitions. `verified` stays false until a human confirms the entry;
-- unverified rows are never shown to users as fact.
create table events (
    id          uuid primary key default gen_random_uuid(),
    name        text not null,
    discipline  text not null check (discipline in
                  ('weighted','statics','dynamics','streetlifting','freestyle','endurance','mixed')),
    starts_on   date not null,
    ends_on     date,
    city        text not null default '',
    country     text not null default '',
    location    geography(Point, 4326),
    url         text not null default '',
    source      text not null default '',
    verified    boolean not null default false,
    created_at  timestamptz not null default now()
);
create index events_date_idx on events (starts_on) where verified;

create table user_events (
    user_id  uuid not null references users(id) on delete cascade,
    event_id uuid not null references events(id) on delete cascade,
    goal     text not null default '',
    primary key (user_id, event_id)
);

-- A generated plan, plus the individual sessions it schedules on the calendar.
create table plans (
    id         uuid primary key default gen_random_uuid(),
    user_id    uuid not null references users(id) on delete cascade,
    title      text not null,
    goal       text not null default '',
    event_id   uuid references events(id) on delete set null,
    starts_on  date not null,
    weeks      int  not null check (weeks between 1 and 52),
    body       jsonb not null,
    created_at timestamptz not null default now()
);
create index plans_user_idx on plans (user_id, created_at desc);

create table planned_sessions (
    id           uuid primary key default gen_random_uuid(),
    plan_id      uuid references plans(id) on delete cascade,
    user_id      uuid not null references users(id) on delete cascade,
    scheduled_on date not null,
    title        text not null,
    focus        text not null default '',
    body         jsonb not null,
    workout_id   uuid references workouts(id) on delete set null,
    completed_at timestamptz
);
create index planned_sessions_cal_idx on planned_sessions (user_id, scheduled_on);

-- Parks are cached from OpenStreetMap and corrected by users on top.
create table parks (
    id         uuid primary key default gen_random_uuid(),
    osm_type   text,
    osm_id     bigint,
    name       text not null default '',
    location   geography(Point, 4326) not null,
    equipment  text[] not null default '{}',
    surface    text,
    roofed     boolean,
    source     text not null default 'osm',
    updated_at timestamptz not null default now()
);
create unique index parks_osm_key on parks (osm_type, osm_id) where osm_id is not null;
create index parks_location_idx on parks using gist (location);

-- Every model call is recorded: cost visibility and a paper trail for bad output.
create table ai_calls (
    id            uuid primary key default gen_random_uuid(),
    user_id       uuid references users(id) on delete set null,
    purpose       text not null,
    model         text not null,
    input_tokens  int not null default 0,
    output_tokens int not null default 0,
    duration_ms   int not null default 0,
    ok            boolean not null default true,
    prompt        text not null default '',
    completion    text not null default '',
    created_at    timestamptz not null default now()
);
create index ai_calls_user_idx on ai_calls (user_id, created_at desc);
