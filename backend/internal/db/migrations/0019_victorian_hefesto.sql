-- The rings twin of the SAT, and the one elite skill that is a pull rather
-- than a hold.
--
-- 0018 added the SAT and the one-arm front lever and named the rings victorian
-- as the SAT's neutral-wrist substitute, but only as two exercises: a tuck and
-- a full. It is a skill in its own right and it has its own ladder, which the
-- sources give in a shape the SAT's does not have — tuck, then *one leg*, then
-- straddle — so it gets the rungs to say that.
--
-- The hefesto is different from everything else at this end of the catalogue.
-- Every other elite skill here is a hold; this one is a pull, and it is pulled
-- from the worst position the shoulder has: a german hang, arms behind the
-- body, finishing in a korean dip. That is why its ladder is mostly other
-- people's skills — a german hang, a back lever, a korean dip — and only the
-- last three rungs are the movement itself.
insert into exercises (slug, name, category, measure, difficulty, description) values
    -- The victorian, on rings, where the ring turns with the forearm.
    ('band_victorian',     'Band-assisted victorian', 'static', 'static_hold',  9, 'Victorian with a band under the hips. The assistance comes down between blocks, never inside a session.'),
    ('one_leg_victorian',  'One-leg victorian',       'static', 'static_hold',  9, 'Victorian with one leg extended and one tucked. The rung the rings tradition runs through and the bar version does not have.'),
    ('straddle_victorian', 'Straddle victorian',      'static', 'static_hold', 10, 'Victorian with the legs straddled. The last rung before the full hold.'),

    -- The hefesto, and the two skills it is assembled from.
    ('korean_dip',       'Korean dip',            'push', 'reps', 6, 'Dip on a bar behind the body, shoulders taken into extension. The top half of a hefesto, and a shoulder position with a hard anatomical limit — this is not a range to push into.'),
    ('hefesto_negative', 'Hefesto negative',      'pull', 'reps', 7, 'From the korean dip position, lower as slowly as the shoulders allow until the arms are straight. The eccentric half, and where most of the strength for this is built.'),
    ('band_hefesto',     'Band-assisted hefesto', 'pull', 'reps', 7, 'Hefesto with a band under the feet. Same movement pattern, less of the load, and the band comes down one step per block.'),
    ('tuck_hefesto',     'Tuck hefesto',          'pull', 'reps', 8, 'Hefesto pulled through a tucked back lever rather than a straight one. The shortened lever, same path.'),
    ('hefesto',          'Hefesto',               'pull', 'reps', 9, 'From a german hang, pull behind the back over the bar to a korean dip. Pull to the armpits, not to the lower back.')
on conflict (slug) do update set
    name        = excluded.name,
    category    = excluded.category,
    measure     = excluded.measure,
    difficulty  = excluded.difficulty,
    description = excluded.description;
