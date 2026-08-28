-- The planche block filled one gap and made the others obvious: several
-- headline skills had an end position in the library and no way to reach it,
-- and a few were missing entirely. This gives every skill a path — the
-- assisted version, the negative, the tuck or one-leg rung in the middle — so
-- a plan or a routine can prescribe the step the athlete is actually on.
--
-- Difficulties are set against the holds already in the table rather than on
-- their own scale: a one-leg flag sits between the tuck and the straddle
-- because that is where it sits in training.
--
-- Safe to re-run: conflicts on slug update the row.
insert into exercises (slug, name, category, measure, difficulty, description) values
    -- front lever: the rungs between a tuck and the full lay
    ('band_assisted_front_lever', 'Band-assisted front lever',    'static',   'static_hold',   5, 'Front lever with a band under the hips or feet. Thin the band rather than breaking the line.'),
    ('front_lever_negative',      'Front lever negative',         'static',   'reps',          5, 'From an inverted hang, lower through the lever line as slowly as control allows.'),
    ('front_lever_pull_up',       'Front lever pull-up',          'pull',     'reps',         10, 'A pull-up performed while holding the full front lever.'),

    -- back lever
    ('back_lever_negative',       'Back lever negative',          'static',   'reps',          4, 'From an inverted hang, lower face-down to the lever line under control.'),
    ('one_leg_back_lever',        'One-leg back lever',           'static',   'static_hold',   5, 'Back lever with one leg extended and one tucked.'),

    -- pulling: what the one-arm and the muscle-up are built on
    ('chest_to_bar_pull_up',      'Chest-to-bar pull-up',         'pull',     'reps',          3, 'Pull until the chest touches the bar. The height a muscle-up transition needs.'),
    ('one_arm_scapular_pull_up',  'One-arm scapular pull-up',     'pull',     'reps',          5, 'Hanging from one arm, pull the shoulder down and back without bending the elbow.'),

    -- hefesto: german hang strength, which nothing else in the library trained
    ('band_assisted_hefesto',     'Band-assisted hefesto',        'pull',     'reps',          8, 'Hefesto with a band taking part of the load. The way the elbows are prepared for it.'),
    ('hefesto',                   'Hefesto',                      'pull',     'skill_attempt',10, 'From a german hang, pull behind the back to the bar. Elbow and shoulder strength at full stretch.'),

    -- pressing: the push-up and dip ladders past their first rung
    ('diamond_push_up',           'Diamond push-up',              'push',     'reps',          2, 'Hands together under the chest. Triceps-dominant push-up.'),
    ('archer_push_up',            'Archer push-up',               'push',     'reps',          4, 'One arm straightens along the floor while the other takes the rep.'),
    ('one_arm_push_up',           'One-arm push-up',              'push',     'reps',          7, 'Push-up on one arm, feet apart, hips square.'),
    ('clap_push_up',              'Clap push-up',                 'dynamic',  'reps',          4, 'Push hard enough to leave the floor and clap. Power for the muscle-up transition.'),
    ('impossible_dip',            'Impossible dip',               'push',     'reps',          9, 'On a straight bar, press with the hips travelling past the hands and the arms rotating behind.'),

    -- elbow lever: a whole skill the library had no entry for
    ('tuck_elbow_lever',          'Tuck elbow lever',             'static',   'static_hold',   2, 'Elbows into the ribs, knees resting on the arms, feet off the floor.'),
    ('elbow_lever',               'Elbow lever',                  'static',   'static_hold',   4, 'Body straight and horizontal, resting on the elbows planted in the ribs.'),
    ('one_arm_elbow_lever',       'One-arm elbow lever',          'static',   'static_hold',   7, 'Elbow lever balanced on a single arm.'),

    -- human flag: the middle of the ladder
    ('vertical_flag',             'Vertical flag',                'static',   'static_hold',   6, 'From the chamber position, legs stacked vertically above the hands. The first honest flag hold.'),
    ('band_assisted_human_flag',  'Band-assisted human flag',     'static',   'static_hold',   6, 'Full flag line with a band supporting the hips.'),
    ('one_leg_human_flag',        'One-leg human flag',           'static',   'static_hold',   7, 'Flag with one leg extended and one tucked to the chest.'),

    -- handstand: the one-arm line
    ('one_arm_handstand_lean',    'One-arm handstand lean',       'static',   'static_hold',   8, 'In a handstand, shift the weight over one hand until the other barely touches.'),
    ('one_arm_handstand',         'One-arm handstand',            'static',   'skill_attempt',10, 'Handstand held on a single arm.'),

    -- victorian: the reverse of the planche line, and its first rung
    ('tuck_victorian',            'Tuck victorian',               'static',   'static_hold',   9, 'Hanging between bars, arms straight at the sides, body tucked and horizontal.'),
    ('victorian',                 'Victorian',                    'static',   'static_hold',  10, 'Body horizontal and straight with the arms locked at the sides. The reverse of the maltese.'),

    -- dynamic bar work
    ('pull_over',                 'Pull-over',                    'dynamic',  'reps',          3, 'From a hang, pull and rotate over the bar into support. The first bar dynamic.'),
    ('three_sixty_pull',          '360 pull',                     'dynamic',  'skill_attempt', 8, 'Release the bar and rotate a full turn around it before catching it again.'),

    -- core: compression and rotation the levers actually ask for
    ('tuck_dragon_flag',          'Tuck dragon flag',             'core',     'reps',          4, 'Dragon flag with the knees tucked. The step before the negatives.'),
    ('hanging_windshield_wiper',  'Hanging windshield wiper',     'core',     'reps',          7, 'Hanging with the legs raised, sweep them side to side under control.'),

    -- legs, which a bar athlete still has to own
    ('wall_sit',                  'Wall sit',                     'legs',     'static_hold',   1, 'Back to the wall, thighs parallel to the floor.'),
    ('single_leg_deadlift',       'Single-leg deadlift',          'legs',     'reps',          2, 'Hinge on one leg with the other extended behind. Hamstring and balance work.'),
    ('sissy_squat',               'Sissy squat',                  'legs',     'reps',          4, 'Knees travelling forward, hips extended, leaning back through the quads.'),

    -- mobility: the positions the skills are held in
    ('deep_squat_hold',           'Deep squat hold',              'mobility', 'static_hold',   1, 'Sit at the bottom of a squat, heels down, and stay there.'),
    ('bridge',                    'Bridge',                       'mobility', 'static_hold',   3, 'Full bridge on hands and feet. Shoulder and thoracic extension for overhead work.'),
    ('jefferson_curl',            'Jefferson curl',               'mobility', 'reps',          3, 'Roll down one vertebra at a time under a light load and back up. Loaded spinal and hamstring range.')
on conflict (slug) do update set
    name        = excluded.name,
    category    = excluded.category,
    measure     = excluded.measure,
    difficulty  = excluded.difficulty,
    description = excluded.description;
