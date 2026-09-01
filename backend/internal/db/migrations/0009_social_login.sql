-- Signing in with Google is identity only: it says who the athlete is and
-- nothing more. It buys no model access — both Anthropic and OpenAI bill
-- inference to an API key and never to a signed-in consumer account — so an
-- athlete who signs in this way still connects their own key under Settings.

-- An account created by signing in with a provider has no password to store.
alter table users alter column password_hash drop not null;

create table user_identities (
    provider   text not null,
    -- The provider's own stable id for this person. Emails change; this does not.
    subject    text not null,
    user_id    uuid not null references users(id) on delete cascade,
    email      text not null default '',
    created_at timestamptz not null default now(),
    primary key (provider, subject)
);
create index user_identities_user_idx on user_identities (user_id);
