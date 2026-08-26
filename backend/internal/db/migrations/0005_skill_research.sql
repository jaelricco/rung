-- What the coach found out about a skill before writing a plan for it.
--
-- Research is a web-search turn: it costs money per search and half a minute
-- of the athlete's wait. The findings for "front lever" do not change between
-- two athletes or between two Tuesdays, so they are cached by skill and reused
-- until they go stale. `sources` keeps the pages the search actually retrieved,
-- which is the only reason to believe any of it.
create table skill_research (
    id            uuid primary key default gen_random_uuid(),
    skill_key     text not null unique,
    skill         text not null,
    findings      jsonb not null,
    sources       jsonb not null default '[]'::jsonb,
    searches_used int  not null default 0,
    created_at    timestamptz not null default now()
);
create index skill_research_age_idx on skill_research (created_at desc);
