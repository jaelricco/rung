-- The last two of the maximal tier, and neither is a harder version of what
-- came before it.
--
-- The one-arm planche is the odd one on the floor: the athletes who have it
-- describe it as a balance skill with a strength requirement rather than the
-- other way round, and the position is not a planche on one arm — the body
-- curves sideways, because a symmetric one would fall over. What it asks for
-- is a straddle planche *and* a one-arm handstand, which is why its ladder is
-- gated on rungs of both.
--
-- The inverted cross is the iron cross upside down: a rings handstand with the
-- arms straight out from the shoulders. It is the one element at this end of
-- the catalogue with a peer-reviewed strength benchmark behind it, and the
-- finding worth having is that overhead pressing correlates with it strongly
-- enough to be the conditioning that builds it.
insert into exercises (slug, name, category, measure, difficulty, description) values
    -- The one-arm planche, on the floor.
    ('one_arm_planche_lean',      'One-arm planche lean',       'static', 'static_hold',  8, 'One hand down, body turned and leaning past the wrist, feet still on the floor. The first honest loading of a position that is not symmetric and is not meant to be.'),
    ('tuck_one_arm_planche',      'Tuck one-arm planche',       'static', 'static_hold',  9, 'One-arm planche with the legs tucked and the body curved away from the supporting arm.'),
    ('straddle_one_arm_planche',  'Straddle one-arm planche',   'static', 'static_hold', 10, 'One-arm planche with the legs straddled. The last rung before the full hold.'),
    ('one_arm_planche',           'One-arm planche',            'static', 'static_hold', 10, 'The planche held on one arm, body curved sideways to keep the mass over the hand. As much balance as strength, and almost nobody has one.'),

    -- The inverted cross, on rings.
    ('ring_handstand',      'Ring handstand',       'static', 'static_hold',  7, 'Handstand on rings. Every skill above it starts by being comfortable here, and comfortable means the rings stop moving.'),
    ('japanese_handstand',  'Japanese handstand',   'static', 'static_hold',  8, 'Ring handstand with the shoulders extended and the rings turned out, body arched. The position an inverted cross collapses into when it loses depth.'),
    ('band_inverted_cross', 'Band-assisted inverted cross', 'static', 'static_hold', 9, 'Inverted cross with a band or a pulley taking part of the load. The assistance comes down between blocks, and this is how the strength is measured as well as built.'),
    ('inverted_cross',      'Inverted cross',       'static', 'static_hold', 10, 'The iron cross upside down: inverted on rings with the arms straight out from the shoulders. Depth is the whole skill — a shallow one is a Japanese handstand with the arms open.')
on conflict (slug) do update set
    name        = excluded.name,
    category    = excluded.category,
    measure     = excluded.measure,
    difficulty  = excluded.difficulty,
    description = excluded.description;
