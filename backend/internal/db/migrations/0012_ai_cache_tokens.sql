-- Cached prompt tokens are counted and billed apart from ordinary input:
-- a write costs 1.25x the input rate, a read a tenth of it, and neither
-- appears in the input_tokens the API reports. Recording only input_tokens
-- once caching is on would quietly undercount every call.
alter table ai_calls
    add column cache_read_tokens  integer not null default 0,
    add column cache_write_tokens integer not null default 0;
