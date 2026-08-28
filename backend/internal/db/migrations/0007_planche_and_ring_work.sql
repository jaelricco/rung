-- More of the library, weighted toward the planche, because that is where the
-- gaps were: everything before this was holds and one push-up variant, so a
-- coach — or an athlete writing their own week — had no way to prescribe the
-- presses, the assisted holds or the straight-arm work the skill is actually
-- built from. The rings block is here for the same reason: turned-out support
-- and the curls are what the elbows and biceps need before straight-arm
-- pressing is safe to load.
--
-- Safe to re-run: conflicts on slug update the row.
insert into exercises (slug, name, category, measure, difficulty, description) values
    -- planche: the steps between the holds
    ('straight_arm_frog_stand',  'Straight-arm frog stand',      'static',   'static_hold',   3, 'Frog stand with the elbows locked and the knees off the arms. The first hold that is planche rather than balance.'),
    ('weighted_planche_lean',    'Weighted planche lean',        'static',   'static_hold',   5, 'Planche lean wearing a vest or belt. Loads the lean once the angle itself has stopped being the limit.'),
    ('band_assisted_planche',    'Band-assisted planche',        'static',   'static_hold',   6, 'Full planche line with a band under the hips. Thin the band as the hold becomes honest.'),
    ('straddle_planche_push_up', 'Straddle planche push-up',     'push',     'reps',          8, 'Bend and straighten the arms holding a straddle planche.'),
    ('ninety_degree_push_up',    '90-degree push-up',            'push',     'reps',         10, 'Lower to a bent-arm planche with the elbows at 90 degrees against the ribs, then press back out.'),

    -- planche: pressing, which is what turns a hold into strength
    ('l_sit_to_tuck_planche',    'L-sit press to tuck planche',  'push',     'reps',          6, 'From an L-sit on parallettes, press the hips up and the shoulders forward into a tuck planche, then lower. The drill that teaches the lean.'),
    ('planche_negative',         'Planche negative',             'push',     'reps',          8, 'From a handstand, lower with straight arms toward the planche line as slowly as control allows.'),
    ('planche_press',            'Planche press to handstand',   'push',     'reps',          9, 'From a planche, press to handstand with the arms straight, then lower back to the planche.'),
    ('maltese_lean',             'Maltese lean',                 'static',   'static_hold',   8, 'Planche lean taken past the hands until the shoulders are behind them, on parallettes or rings.'),
    ('maltese',                  'Maltese',                      'static',   'static_hold',  10, 'Body horizontal with the arms out to the sides, shoulders behind the hands. The end of the straight-arm push line.'),

    -- rings: turned-out support and the straight-arm work the elbows need first
    ('ring_support_hold',        'Ring support hold',            'static',   'static_hold',   2, 'Support on the rings, arms locked, shoulders down. The base every ring skill is held from.'),
    ('rto_support_hold',         'Rings-turned-out support',     'static',   'static_hold',   4, 'Support hold with the rings turned out to the sides and held there. Builds the straight-arm position under rotation.'),
    ('ring_push_up',             'Ring push-up',                 'push',     'reps',          3, 'Push-up on rings, which have to be stabilised as well as pressed.'),
    ('rto_push_up',              'Rings-turned-out push-up',     'push',     'reps',          6, 'Push-up keeping the rings turned out through the whole rep.'),
    ('bulgarian_dip',            'Bulgarian dip',                'push',     'reps',          7, 'Wide ring dip from a turned-out support, letting the rings turn in at the bottom. The step toward the cross.'),
    ('ring_curl',                'Ring curl',                    'pull',     'reps',          4, 'Body angled back, curling up on the rings with the elbows fixed. Elbow preparation for straight-arm work.'),
    ('pelican_curl',             'Pelican curl',                 'pull',     'reps',          7, 'From a support, roll out until the arms are straight and overhead, then pull back. Loads the biceps and elbows at full extension.'),
    ('band_assisted_iron_cross', 'Band-assisted iron cross',     'static',   'static_hold',   8, 'Cross position on rings with a band taking part of the load.'),
    ('iron_cross',               'Iron cross',                   'static',   'static_hold',  10, 'Arms straight out to the sides on the rings, body vertical.'),

    -- handstand: shrugs and the press ladder
    ('handstand_shrug',          'Handstand shrug',              'push',     'reps',          3, 'Chest to wall, pushing the floor away and letting the shoulders sink. Scapular strength in the position pressing needs.'),
    ('handstand_walk',           'Handstand walk',               'static',   'reps',          5, 'Walking on the hands. Count steps as reps.'),
    ('wall_assisted_press',      'Wall-assisted press',          'push',     'reps',          5, 'Back to the wall, pressing from a pike to a handstand with the wall taking the balance.'),
    ('press_negative',           'Press negative',               'push',     'reps',          6, 'From a handstand, lower with straight arms and straddled legs to the floor.'),
    ('stalder_press',            'Stalder press',                'static',   'skill_attempt', 9, 'Straight-arm press to handstand from a straddle with the legs outside the arms.'),
    ('l_sit_press',              'L-sit press to handstand',     'static',   'skill_attempt', 9, 'Straight-arm press to handstand from an L-sit, legs together throughout.'),

    -- levers and compression, where the ladder had a rung missing
    ('one_leg_front_lever',      'One-leg front lever',          'static',   'static_hold',   6, 'Front lever with one leg extended and one tucked. The step between advanced tuck and straddle.'),
    ('straddle_l_sit',           'Straddle L-sit',               'static',   'static_hold',   3, 'L-sit with the legs straight and wide. Easier to hold, harder to compress.'),
    ('manna',                    'Manna',                        'static',   'static_hold',  10, 'Seated support with the legs above the head and the hips behind the hands. The end of the compression line.'),
    ('back_extension',           'Back extension',               'core',     'reps',          2, 'Face down, lifting the chest and legs against the floor or a box. Posterior chain work the straight-arm holds lean on.')
on conflict (slug) do update set
    name        = excluded.name,
    category    = excluded.category,
    measure     = excluded.measure,
    difficulty  = excluded.difficulty,
    description = excluded.description;
