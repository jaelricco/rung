-- The maltese as calisthenics trains it, rather than as gymnastics does.
--
-- Migration 0009 built the maltese out of rings: support hold, band-assisted
-- cross, tuck and straddle maltese. That is the rings-gymnastics lineage, and
-- it is a real one — but it is not the tradition this app's athletes are in.
-- In the floor-and-parallel-bars lineage a maltese is a planche whose hands
-- keep travelling outward, and the ladder to it runs through a *wide planche*
-- — hands wider than a planche, narrower than a maltese — which 0009 had no
-- rung for at all.
--
-- These are the movements that ladder is actually made of. The rings rows from
-- 0009 stay: they are still what the iron cross is built from, and still the
-- neutral-wrist substitutions an angry wrist needs.
insert into exercises (slug, name, category, measure, difficulty, description) values
    -- The band between planche and maltese.
    ('wide_planche_hold',    'Wide planche hold',            'static',   'static_hold',   9, 'Planche with the hands set wider than the shoulders. The rung between a full planche and a maltese, and the one most ladders leave out.'),
    ('wide_planche_press',   'Wide planche press',           'push',     'reps',          9, 'Press to and from a wide planche.'),
    ('dead_planche_hold',    'Dead planche hold',            'static',   'static_hold',  10, 'Full planche entered from a dead hang position rather than pressed into. No momentum to hide behind.'),

    -- Maltese, on the floor and the bars.
    ('lean_maltese',         'Lean maltese',                 'static',   'static_hold',   7, 'Maltese lean on the floor or bars, usually with a band. The first honest maltese loading, and it starts early.'),
    ('lean_maltese_elevator','Lean maltese elevator',        'push',     'reps',          8, 'Rise and lower through the lean maltese position under control.'),
    ('zanetti',              'Zanetti',                      'push',     'reps',         10, 'From a vertical position, lower out to the maltese line and return. Named for the gymnast.'),
    ('maltese_elevator',     'Maltese elevator',             'push',     'reps',         10, 'Rise and lower through the maltese position, banded at first.'),
    ('maltese_press',        'Maltese press',                'push',     'reps',         10, 'Press to and from a maltese. The top of this ladder.'),

    -- The planche vocabulary the programme actually prescribes, which the
    -- library was too coarse to express: a hold and a press are different
    -- exercises, and so are a kick-up and a negative.
    ('planche_kicks',        'Planche kicks',                'static',   'static_hold',   6, 'Kick into the planche line and hold what you catch. How time under tension is accumulated above your best static.'),
    ('negative_to_planche',  'Negative to planche hold',     'static',   'reps',          8, 'Lower under control into the planche and hold. The eccentric entry.'),
    ('planche_hold_to_press','Planche hold to press',        'push',     'reps',          9, 'Hold the planche, then press out of it. Hold and press trained as one movement.'),
    ('l_sit_to_planche',     'L-sit to planche',             'static',   'reps',          6, 'Press from an L-sit into the planche line. The entry drill at every level.'),
    ('half_rom_planche_push_up','Half-ROM planche push-up',  'push',     'reps',          9, 'Planche push-up through the top or bottom half only. Splitting the range is how the full one is built.')
on conflict (slug) do update set
    name        = excluded.name,
    category    = excluded.category,
    measure     = excluded.measure,
    difficulty  = excluded.difficulty,
    description = excluded.description;
