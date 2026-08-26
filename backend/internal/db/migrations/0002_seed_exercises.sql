-- Exercise library. Safe to re-run: conflicts on slug update the row.
insert into exercises (slug, name, category, measure, difficulty, description) values
    -- pull
    ('pull_up',              'Pull-up',                    'pull',     'reps',          2, 'Strict, full hang to chin over bar.'),
    ('chin_up',              'Chin-up',                    'pull',     'reps',          2, 'Supinated grip.'),
    ('archer_pull_up',       'Archer pull-up',             'pull',     'reps',          5, 'One arm straightens along the bar.'),
    ('one_arm_pull_up',      'One-arm pull-up',            'pull',     'skill_attempt', 9, 'Single-arm, no assistance.'),
    ('australian_row',       'Australian row',             'pull',     'reps',          1, 'Horizontal row under a low bar.'),

    -- push
    ('dip',                  'Dip',                        'push',     'reps',          2, 'Parallel bars, shoulders below elbows.'),
    ('push_up',              'Push-up',                    'push',     'reps',          1, 'Chest to floor, body in one line.'),
    ('pike_push_up',         'Pike push-up',               'push',     'reps',          3, 'Hips high, head to floor.'),
    ('handstand_push_up',    'Handstand push-up',          'push',     'reps',          6, 'Against wall or freestanding.'),
    ('ring_dip',             'Ring dip',                   'push',     'reps',          4, 'Dips on rings, turned out at the top.'),

    -- static holds
    ('hollow_body_hold',     'Hollow body hold',           'core',     'static_hold',   1, 'Lower back pressed flat.'),
    ('l_sit',                'L-sit',                      'static',   'static_hold',   3, 'Legs straight and parallel to the ground.'),
    ('tuck_front_lever',     'Tuck front lever',           'static',   'static_hold',   4, 'Knees tucked, back parallel to ground.'),
    ('adv_tuck_front_lever', 'Advanced tuck front lever',  'static',   'static_hold',   5, 'Hips open past 90 degrees.'),
    ('straddle_front_lever', 'Straddle front lever',       'static',   'static_hold',   6, 'Legs straight and wide.'),
    ('front_lever',          'Front lever',                'static',   'static_hold',   8, 'Full body horizontal, supinated or neutral grip.'),
    ('back_lever',           'Back lever',                 'static',   'static_hold',   6, 'Face down, body horizontal.'),
    ('tuck_planche',         'Tuck planche',               'static',   'static_hold',   5, 'Knees to chest, shoulders forward of hands.'),
    ('straddle_planche',     'Straddle planche',           'static',   'static_hold',   8, 'Legs straight and wide, hips level.'),
    ('full_planche',         'Full planche',               'static',   'static_hold',  10, 'Body straight and horizontal.'),
    ('handstand',            'Freestanding handstand',     'static',   'static_hold',   4, 'No wall.'),
    ('human_flag',           'Human flag',                 'static',   'static_hold',   8, 'Body horizontal off a vertical pole.'),

    -- dynamics
    ('muscle_up',            'Bar muscle-up',              'dynamic',  'reps',          5, 'Pull to transition over the bar.'),
    ('ring_muscle_up',       'Ring muscle-up',             'dynamic',  'reps',          6, 'Muscle-up on rings.'),
    ('explosive_pull_up',    'Explosive pull-up',          'dynamic',  'reps',          4, 'Pull to sternum or higher.'),
    ('360',                  '360',                        'dynamic',  'skill_attempt', 7, 'Full rotation around the bar.'),
    ('shrimp_flip',          'Shrimp flip',                'dynamic',  'skill_attempt', 8, 'Reverse rotation release move.'),

    -- weighted
    ('weighted_pull_up',     'Weighted pull-up',           'weighted', 'weighted_reps', 4, 'Added load on a belt. Record added kg only.'),
    ('weighted_dip',         'Weighted dip',               'weighted', 'weighted_reps', 4, 'Added load on a belt. Record added kg only.'),
    ('weighted_muscle_up',   'Weighted muscle-up',         'weighted', 'weighted_reps', 7, 'Added load on a belt.'),
    ('weighted_push_up',     'Weighted push-up',           'weighted', 'weighted_reps', 2, 'Plate on the back.'),

    -- legs
    ('pistol_squat',         'Pistol squat',               'legs',     'reps',          4, 'Single-leg squat to full depth.'),
    ('bodyweight_squat',     'Bodyweight squat',           'legs',     'reps',          1, 'Full depth, heels down.'),
    ('nordic_curl',          'Nordic hamstring curl',      'legs',     'reps',          5, 'Eccentric control on the way down.'),

    -- core
    ('hanging_leg_raise',    'Hanging leg raise',          'core',     'reps',          3, 'Straight legs to bar.'),
    ('ab_wheel_rollout',     'Ab wheel rollout',           'core',     'reps',          3, 'Standing or from knees.'),

    -- mobility / prehab
    ('wrist_prep',           'Wrist preparation circuit',  'mobility', 'static_hold',   1, 'Flexion, extension, rotation and side-to-side loading.'),
    ('scapular_pull_up',     'Scapular pull-up',           'mobility', 'reps',          1, 'Depress and retract from a dead hang.'),
    ('band_dislocate',       'Band shoulder dislocate',    'mobility', 'reps',          1, 'Straight arms, wide grip, slow.'),
    ('german_hang',          'German hang',                'mobility', 'static_hold',   3, 'Passive shoulder extension hang.'),
    ('bulgarian_split',      'Bulgarian split squat',      'legs',     'reps',          2, 'Rear foot elevated.')
on conflict (slug) do update set
    name        = excluded.name,
    category    = excluded.category,
    measure     = excluded.measure,
    difficulty  = excluded.difficulty,
    description = excluded.description;
