-- Two switches an athlete may want on their own connection, without having to
-- delete the key and paste it again from the provider's console.
--
-- paused: the key stays sealed here, but nothing is spent on it. Everything
-- the app does without a model carries on, which is most of it.
--
-- forget_on_logout: signing out drops the key. For a shared or borrowed
-- machine, where "stored until I say otherwise" is the wrong default.
alter table user_ai_credentials
    add column paused           boolean not null default false,
    add column forget_on_logout boolean not null default false;
