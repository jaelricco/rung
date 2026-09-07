-- What has been ticked off inside a session.
--
-- The session page lays a session out in the six phases it is performed in,
-- and is read standing in a gym — so it has to remember where you got to.
-- Browser storage would have done until the phone evicted it halfway through
-- the training, which is the one moment it matters, so this lives beside
-- completed_at instead.
--
-- Protocols are named by slug because a session names each of them once.
-- Blocks are named by their position, because a block has no identity of its
-- own and two blocks can share an exercise — a scapular push-up appears in the
-- specific warm-up and again as an accessory. Positions only mean anything
-- against the body they were ticked against, so editing a session's blocks
-- clears them.
alter table planned_sessions
    add column done_protocols text[] not null default '{}',
    add column done_blocks    int[]  not null default '{}';
