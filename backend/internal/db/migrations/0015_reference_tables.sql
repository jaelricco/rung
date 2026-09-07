-- The reference data, as tables.
--
-- Exercises have always been a table. Everything else the app reasons from —
-- the skills and their ladders, the injury regions, the rehab protocols, the
-- equipment list, and the maps that say which movement loads which joint and
-- needs which kit — lived only in Go. That split had no reason behind it, and
-- it meant nothing could query or serve any of it.
--
-- These tables are that data. They are *projected* from the Go catalogue at
-- startup rather than being its source, and that is deliberate: CI runs
-- `go test ./...` with no database, and the ladder tests — that a rung is never
-- easier than the one below it, that every slug exists, that an athlete lands
-- where their records put them — are the app's main quality mechanism. Moving
-- the catalogue into SQL would mean either standing up Postgres in CI or
-- parsing SQL in the tests. So Go stays the place it is authored, where the
-- compiler and the tests can see it, and this is where it is readable from.
--
-- Every table is rewritten wholesale on each boot, so a skill deleted in Go
-- disappears here too.

create table skills (
    key          text primary key,
    position     int  not null,
    name         text not null,
    phrase       text not null default '',
    pattern      text not null,
    straight_arm boolean not null default false,
    wrists       boolean not null default false,
    foundation   boolean not null default false,
    -- What one week of this costs the athlete's tolerance for maximal
    -- straight-arm work. Summed across everything being learned at once.
    cost         int  not null default 1,
    timeline     text not null default '',
    frequency    text not null default '',
    aliases      text[] not null default '{}',
    feeds        text[] not null default '{}',
    drills       text[] not null default '{}',
    accessories  text[] not null default '{}',
    risks        text[] not null default '{}'
);

create table skill_steps (
    skill_key text not null references skills(key) on delete cascade,
    position  int  not null,
    name      text not null,
    metric    text not null,
    standard  numeric(7,2) not null,
    typical   text not null default '',
    -- Candidates in preference order: the first that exists and is not ruled
    -- out by an injury or missing equipment is the one prescribed.
    movements text[] not null default '{}',
    assists   text[] not null default '{}',
    primary key (skill_key, position)
);

-- What has to be demonstrated before a ladder, or a single rung of one, opens.
-- A null step_position is a requirement on the whole goal.
create table skill_requirements (
    id            uuid primary key default gen_random_uuid(),
    skill_key     text not null references skills(key) on delete cascade,
    step_position int,
    exercise_slug text not null,
    metric        text not null,
    standard      numeric(7,2) not null,
    why           text not null default ''
);
create index skill_requirements_skill_idx on skill_requirements (skill_key, step_position);

create table injury_regions (
    key      text primary key,
    position int  not null,
    label    text not null
);

create table protocols (
    slug           text primary key,
    position       int  not null,
    region         text not null,
    title          text not null,
    purpose        text not null,
    steps          text[] not null default '{}',
    avoid_while    text[] not null default '{}',
    see_clinician  text not null default ''
);

create table equipment (
    key      text primary key,
    position int  not null,
    label    text not null,
    note     text not null default ''
);

-- A movement needs at least one option from every group it has: two groups
-- means both are needed, one group with two options means either will do.
create table exercise_equipment (
    exercise_slug text not null references exercises(slug) on delete cascade,
    group_no      int  not null,
    options       text[] not null,
    primary key (exercise_slug, group_no)
);

-- Which joints a movement puts under load, beyond what its category implies.
create table exercise_regions (
    exercise_slug text primary key references exercises(slug) on delete cascade,
    regions       text[] not null
);

create table category_regions (
    category text primary key,
    regions  text[] not null
);

-- The version of a movement that trains the same thing without the joint that
-- hurts: the ring planche for the floor planche, fists for palms.
create table exercise_substitutes (
    exercise_slug   text primary key references exercises(slug) on delete cascade,
    substitute_slug text not null references exercises(slug),
    reason          text not null default ''
);

-- The cut points that turn a record into a tier.
create table level_rubrics (
    exercise_slug text primary key references exercises(slug) on delete cascade,
    metric        text not null,
    cuts          numeric(7,2)[] not null
);
