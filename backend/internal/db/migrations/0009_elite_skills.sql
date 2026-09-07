-- The tier above the full planche.
--
-- Every ladder in this app used to end somewhere an athlete can actually
-- reach in a few years. That was a ceiling, not a design: an athlete holding a
-- twelve-second full planche is past the last rung of every ladder here, so the
-- planner had nothing left to place them on and quietly put them back at the
-- bottom. These are the rungs above it.
--
-- Two things in here are not just "harder versions". Rings work appears
-- because it is the one way to load a straight-arm skill with a neutral wrist,
-- which matters more than volume for the wrist complaints this sport produces.
-- And band-assisted holds appear because they are how a maltese is actually
-- entered — the assistance is the progression, not a shortcut around one.
insert into exercises (slug, name, category, measure, difficulty, description) values
    -- Rings: the same skills with the wrist out of extension.
    ('ring_support_hold',    'Ring support hold',           'static',   'static_hold',   3, 'Support on rings, turned out, shoulders depressed. The entry to every ring skill.'),
    ('ring_planche_lean',    'Ring planche lean',           'static',   'static_hold',   5, 'Planche lean on rings. The wrist stays neutral, which is why it is the version an angry wrist can keep training.'),
    ('ring_tuck_planche',    'Ring tuck planche',           'static',   'static_hold',   7, 'Tuck planche on rings. Harder than the floor version because the rings move.'),
    ('ring_straddle_planche','Ring straddle planche',       'static',   'static_hold',   9, 'Straddle planche on rings, neutral wrist.'),
    ('ring_planche',         'Ring planche',                'static',   'static_hold',  10, 'Full planche on rings. Neutral wrist, and the rings punish every asymmetry.'),

    -- The cross family, which is where maltese work actually starts.
    ('band_iron_cross',      'Band-assisted iron cross',    'static',   'static_hold',   7, 'Iron cross with a band under the hips or feet. Reduce band thickness as the hold comes.'),
    ('iron_cross',           'Iron cross',                  'static',   'static_hold',   9, 'Arms straight out to the sides on rings, body vertical.'),
    ('ring_l_cross',         'L-cross',                     'static',   'static_hold',   9, 'Iron cross holding an L. The step between cross and maltese.'),

    -- Maltese.
    ('maltese_lean',         'Maltese lean',                'static',   'static_hold',   8, 'Planche lean taken past the hands with the arms widening. The floor entry to maltese loading.'),
    ('band_maltese',         'Band-assisted maltese',       'static',   'static_hold',   9, 'Maltese on rings with band assistance. This is where a maltese is learned, not skipped.'),
    ('tuck_maltese',         'Tuck maltese',                'static',   'static_hold',   9, 'Maltese line with the knees tucked, band or no band.'),
    ('straddle_maltese',     'Straddle maltese',            'static',   'static_hold',  10, 'Legs straight and wide, arms out to the sides, body horizontal.'),
    ('maltese',              'Maltese',                     'static',   'static_hold',  10, 'Body horizontal with the arms straight out to the sides. The hardest straight-arm hold in the sport.'),

    -- Above the full planche and the full front lever.
    ('planche_press',        'Planche press to handstand',  'push',     'reps',          10, 'Press from a planche to a handstand with straight arms, and back down.'),
    ('straddle_planche_press','Straddle planche press',     'push',     'reps',           9, 'The same press with the legs straddled. The step before the full press.'),
    ('front_lever_pull_up',  'Front lever pull-up',         'pull',     'reps',           9, 'Pull from a front lever to the bar and lower back to the lever, holding the line throughout.'),
    ('weighted_front_lever', 'Weighted front lever',        'static',   'static_hold',   10, 'Full front lever with added load. Record added kg only.'),

    -- One-arm handstand.
    ('handstand_shifts',     'Handstand weight shifts',     'static',   'reps',           6, 'In a handstand, shift the weight fully onto one hand and back without the other leaving the floor.'),
    ('wall_one_arm_handstand','Wall one-arm handstand',     'static',   'static_hold',    7, 'One-arm handstand with the feet or a shoulder on the wall.'),
    ('tuck_one_arm_handstand','Tuck one-arm handstand',     'static',   'static_hold',    8, 'Freestanding one-arm handstand with the legs tucked.'),
    ('straddle_one_arm_handstand','Straddle one-arm handstand','static','static_hold',    9, 'Freestanding one-arm handstand in a straddle. The shape most people get first.'),
    ('one_arm_handstand',    'One-arm handstand',           'static',   'static_hold',   10, 'Freestanding, legs together. Measured in seconds, and years.'),

    -- Wrist work that is specific rather than general.
    ('wrist_flexion_curl',   'Wrist flexion curl',          'mobility', 'reps',           1, 'Light load through wrist flexion. The other half of the extensor work, and usually the missing half.'),
    ('fingertip_hold',       'Fingertip support hold',      'mobility', 'static_hold',    3, 'Weight on the fingertips rather than the palm. Takes the wrist out of end-range extension.'),
    ('knuckle_plank',        'Knuckle plank',               'mobility', 'static_hold',    2, 'Plank on the fists, wrist neutral. What floor work becomes when extension hurts.')
on conflict (slug) do update set
    name        = excluded.name,
    category    = excluded.category,
    measure     = excluded.measure,
    difficulty  = excluded.difficulty,
    description = excluded.description;

-- Which skills the athlete is in the middle of learning.
--
-- Straight-arm tissue has a budget, and a new maximal skill spends from the
-- same account as every other one. An athlete learning three things and adding
-- a fourth is the shape almost every straight-arm injury has, so the planner
-- has to be able to see the other three.
alter table users add column learning text[];
