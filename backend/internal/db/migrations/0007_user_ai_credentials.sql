-- Each athlete brings their own model account. The server holds the key only
-- so it can spend it on that athlete's behalf, so it is stored sealed: the
-- ciphertext here is useless without AI_CREDENTIALS_KEY, which lives in the
-- environment and never in the database.
create table user_ai_credentials (
    user_id      uuid primary key references users(id) on delete cascade,
    provider     text not null check (provider in ('anthropic', 'openai')),
    key_sealed   bytea not null,
    -- The last four characters, which is all the UI ever shows back.
    key_hint     text not null default '',
    model        text not null default '',
    created_at   timestamptz not null default now(),
    updated_at   timestamptz not null default now(),
    last_used_at timestamptz
);

-- Which account paid for a call matters now that the accounts are the users'.
alter table ai_calls add column provider text not null default 'anthropic';
