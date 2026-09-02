-- Where the server keeps a secret it generates for itself. Today that is one
-- row: the key that seals each athlete's provider API key.
--
-- Putting it here rather than in the environment is a deliberate trade. It is
-- weaker, because a database dump now carries both the ciphertext and the key
-- that opens it. It is also the difference between a server that needs one
-- more thing configured before the coaching features work at all and one that
-- just works — and the alternative to a generated key was never a
-- hand-managed one, it was storing other people's API keys in the clear.
-- AI_CREDENTIALS_KEY still overrides this when it is set, which restores the
-- separation for anyone who wants it.
create table server_secrets (
    name       text primary key,
    value      bytea not null,
    created_at timestamptz not null default now()
);
