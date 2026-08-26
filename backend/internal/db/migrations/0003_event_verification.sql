-- Events gain provenance. Nothing is shown as fact unless a machine check or a
-- human confirmed it against a live page.

alter table events
    add column source_url    text not null default '',
    add column source_title  text not null default '',
    add column evidence      text not null default '',
    add column confidence    text not null default 'unverified'
        check (confidence in ('unverified', 'source_live', 'date_confirmed', 'human_confirmed', 'rejected')),
    add column check_note    text not null default '',
    add column checked_at    timestamptz,
    add column discovered_at timestamptz not null default now();

-- Same event found twice by different searches should collapse into one row.
create unique index events_identity_key
    on events (lower(name), starts_on);

create index events_confidence_idx on events (confidence, starts_on);

-- Discovery is expensive: $10 per 1000 searches plus tokens. Runs are cached
-- by query shape so a hundred users browsing the same region cost one run.
create table discovery_runs (
    id            uuid primary key default gen_random_uuid(),
    discipline    text not null default '',
    country       text not null default '',
    from_date     date not null,
    to_date       date not null,
    searches_used int  not null default 0,
    found         int  not null default 0,
    confirmed     int  not null default 0,
    rejected      int  not null default 0,
    error         text not null default '',
    ran_at        timestamptz not null default now()
);
create index discovery_runs_key_idx
    on discovery_runs (discipline, country, from_date, to_date, ran_at desc);
