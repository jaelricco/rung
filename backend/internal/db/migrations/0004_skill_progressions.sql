-- The library is what the coach may prescribe from, so a thin library is a
-- hard ceiling on plan quality: asked for a front lever with only four static
-- holds available, the model has no honest way to fill twenty-four sessions
-- and pads them instead. These are the progressions actually used to build the
-- headline skills — the intermediate steps, the specific pulls and presses,
-- and the joint preparation that straight-arm work needs.
insert into exercises (slug, name, category, measure, difficulty, description) values
    -- pull: getting to the first pull-up, then past it
    ('dead_hang',               'Dead hang',                    'pull',     'static_hold',   1, 'Passive full hang from the bar. Builds grip and shoulder tolerance.'),
    ('active_hang',             'Active hang',                  'pull',     'static_hold',   1, 'Hang with shoulders pulled down and back, chest lifted. The starting position of every pull.'),
    ('negative_pull_up',        'Negative pull-up',             'pull',     'reps',          1, 'Jump to the top, lower under control for 5 seconds. The main driver before a first pull-up.'),
    ('band_assisted_pull_up',   'Band-assisted pull-up',        'pull',     'reps',          1, 'Foot or knee in a band. Reduce band thickness as reps come.'),
    ('wide_pull_up',            'Wide-grip pull-up',            'pull',     'reps',          3, 'Grip well outside the shoulders. Loads the lats harder, shortens the range.'),
    ('l_sit_pull_up',           'L-sit pull-up',                'pull',     'reps',          5, 'Pull-up holding an L-sit. Adds compression demand to the pull.'),
    ('typewriter_pull_up',      'Typewriter pull-up',           'pull',     'reps',          6, 'At the top, shift side to side keeping the chin bar height. Bridges toward one-arm work.'),
    ('assisted_one_arm_pull_up','Assisted one-arm pull-up',     'pull',     'reps',          7, 'One arm on the bar, the other holding a towel, band or wrist. Reduce the assist over time.'),
    ('one_arm_negative',        'One-arm negative pull-up',     'pull',     'reps',          8, 'Top position on one arm, lowered as slowly as control allows.'),
    ('one_arm_dead_hang',       'One-arm dead hang',            'pull',     'static_hold',   4, 'Single-arm hang. Grip and elbow preparation for one-arm pulling.'),
    ('false_grip_hang',         'False-grip hang',              'pull',     'static_hold',   3, 'Wrists over the rings or bar. The position the muscle-up transition needs.'),
    ('false_grip_row',          'False-grip ring row',          'pull',     'reps',          4, 'Horizontal row on rings in false grip, pulling the rings to the sternum.'),

    -- front and back lever
    ('tuck_front_lever_row',    'Tuck front lever row',         'pull',     'reps',          5, 'Row while holding a tuck front lever. Builds the bent-arm strength behind the hold.'),
    ('front_lever_raise',       'Front lever raise',            'pull',     'reps',          7, 'From a hang, raise to the lever position with straight arms and lower under control.'),
    ('front_lever_row',         'Front lever row',              'pull',     'reps',          9, 'Row while holding a full front lever.'),
    ('ice_cream_maker',         'Ice cream maker',              'pull',     'reps',          7, 'From an inverted hang, roll out to the lever line and pull back up.'),
    ('tuck_back_lever',         'Tuck back lever',              'static',   'static_hold',   3, 'Face down, knees tucked, shoulders open. The first honest back lever step.'),
    ('adv_tuck_back_lever',     'Advanced tuck back lever',     'static',   'static_hold',   4, 'Hips opened past 90 degrees, back flat.'),
    ('straddle_back_lever',     'Straddle back lever',          'static',   'static_hold',   5, 'Legs straight and wide, body horizontal.'),
    ('skin_the_cat',            'Skin the cat',                 'mobility', 'reps',          3, 'Slow rotation through inverted hang to German hang and back. Shoulder preparation for levers.'),
    ('inverted_hang',           'Inverted hang',                'static',   'static_hold',   2, 'Hips over the bar, body vertical. The entry into every lever.'),

    -- planche
    ('planche_lean',            'Planche lean',                 'static',   'static_hold',   3, 'Push-up position, shoulders driven forward of the hands, body straight. Scales by lean angle.'),
    ('frog_stand',              'Frog stand',                   'static',   'static_hold',   2, 'Knees resting on the elbows, feet off the floor. The first straight-arm balance.'),
    ('pseudo_planche_push_up',  'Pseudo planche push-up',       'push',     'reps',          4, 'Push-up with hands at the hips and shoulders forward. The main planche strength driver.'),
    ('adv_tuck_planche',        'Advanced tuck planche',        'static',   'static_hold',   6, 'Hips level with the shoulders, knees still tucked.'),
    ('tuck_planche_push_up',    'Tuck planche push-up',         'push',     'reps',          7, 'Press up and down while holding a tuck planche.'),
    ('planche_push_up',         'Planche push-up',              'push',     'reps',          9, 'Full planche, bent and straightened arms.'),

    -- handstand and overhead pressing
    ('wall_handstand',          'Chest-to-wall handstand',      'static',   'static_hold',   2, 'Stomach facing the wall, body stacked. Teaches the line the freestanding hold needs.'),
    ('wall_walk',               'Wall walk',                    'mobility', 'reps',          2, 'Feet up the wall, hands walking in until the body is vertical.'),
    ('handstand_shoulder_taps', 'Handstand shoulder taps',      'static',   'reps',          5, 'In a handstand, shift weight and tap the opposite shoulder. Builds the balance corrections.'),
    ('crow_pose',               'Crow pose',                    'static',   'static_hold',   2, 'Bent-arm balance with knees on the triceps. An easy entry to hand balancing.'),
    ('elevated_pike_push_up',   'Elevated pike push-up',        'push',     'reps',          4, 'Feet raised, hips stacked over the shoulders. The step before wall handstand push-ups.'),
    ('wall_hspu',               'Wall handstand push-up',       'push',     'reps',          5, 'Handstand push-up with the wall for balance, head to the floor.'),
    ('negative_hspu',           'Negative handstand push-up',   'push',     'reps',          4, 'Lower from a handstand to the floor over 5 seconds, then come down and reset.'),
    ('deficit_hspu',            'Deficit handstand push-up',    'push',     'reps',          8, 'Hands raised on parallettes or plates so the head passes below them.'),
    ('press_to_handstand',      'Press to handstand',           'static',   'skill_attempt', 7, 'Straight-arm press from a pike or straddle to a handstand.'),

    -- muscle-up and dips
    ('straight_bar_dip',        'Straight bar dip',             'push',     'reps',          3, 'Dip on a single bar, torso leaning slightly forward. The top half of a muscle-up.'),
    ('jumping_muscle_up',       'Jumping muscle-up',            'dynamic',  'reps',          3, 'Jump from a low bar into the transition to learn the path.'),
    ('muscle_up_negative',      'Muscle-up negative',           'dynamic',  'reps',          4, 'Start in support, lower slowly through the transition to a hang.'),
    ('russian_dip',             'Russian dip',                  'push',     'reps',          6, 'From a dip, lower onto the forearms and press back out.'),

    -- compression, core and the L-sit family
    ('tuck_l_sit',              'Tuck L-sit',                   'static',   'static_hold',   2, 'Support hold with the knees tucked. The first L-sit step.'),
    ('one_leg_l_sit',           'One-leg L-sit',                'static',   'static_hold',   2, 'One leg straight, one tucked. Halves the lever without losing the position.'),
    ('v_sit',                   'V-sit',                        'static',   'static_hold',   7, 'Legs raised above hip height in a support hold.'),
    ('compression_leg_lift',    'Seated compression lift',      'core',     'reps',          3, 'Sitting with legs straight, lift the heels off the floor and hold. Builds active compression.'),
    ('hanging_knee_raise',      'Hanging knee raise',           'core',     'reps',          1, 'Knees to chest from a hang, controlled on the way down.'),
    ('toes_to_bar',             'Toes to bar',                  'core',     'reps',          4, 'Straight legs from a hang until the toes touch the bar.'),
    ('dragon_flag_negative',    'Dragon flag negative',         'core',     'reps',          5, 'From shoulders on the bench, lower the straight body slowly.'),
    ('dragon_flag',             'Dragon flag',                  'core',     'reps',          7, 'Full body straight, pivoting only at the shoulders.'),
    ('hollow_rock',             'Hollow rock',                  'core',     'reps',          2, 'Rock in the hollow position without losing the flat back.'),
    ('arch_body_hold',          'Arch body hold',               'core',     'static_hold',   1, 'Face down, chest and legs lifted. The posterior half of the hollow.'),
    ('plank',                   'Plank',                        'core',     'static_hold',   1, 'Forearms down, ribs tucked, glutes on.'),
    ('side_plank',             'Side plank',                    'core',     'static_hold',   1, 'Stacked hips, body in one line.'),
    ('copenhagen_plank',        'Copenhagen plank',             'core',     'static_hold',   3, 'Side plank with the top leg on a bench. Adductor strength for flags and levers.'),

    -- human flag
    ('clutch_flag',             'Clutch flag',                  'static',   'static_hold',   5, 'Pole clamped between the upper arm and chest, body horizontal. The accessible flag.'),
    ('tuck_human_flag',         'Tuck human flag',              'static',   'static_hold',   6, 'Press-pull grip on the pole with the knees tucked.'),
    ('straddle_human_flag',     'Straddle human flag',          'static',   'static_hold',   7, 'Flag with the legs straight and wide.'),
    ('flag_negative',           'Human flag negative',          'static',   'reps',          6, 'From the top of the pole, lower to horizontal under control.'),

    -- legs
    ('assisted_pistol_squat',   'Assisted pistol squat',        'legs',     'reps',          2, 'Hold a support or counterweight to reach full depth on one leg.'),
    ('shrimp_squat',            'Shrimp squat',                 'legs',     'reps',          5, 'Rear foot held behind, knee to the floor and back up.'),
    ('jump_squat',              'Jump squat',                   'legs',     'reps',          2, 'Full-depth squat into a maximal jump, landing soft.'),
    ('single_leg_calf_raise',   'Single-leg calf raise',        'legs',     'reps',          1, 'Full range off a step. Ankle and achilles resilience for jumping work.'),
    ('reverse_nordic',          'Reverse nordic curl',          'legs',     'reps',          3, 'Kneeling, lean back with straight hips. Loads the quads at long length.'),

    -- preparation and prehab, which is what keeps straight-arm work sustainable
    ('scapular_push_up',        'Scapular push-up',             'mobility', 'reps',          1, 'Push-up position, protract and retract without bending the elbows.'),
    ('elbow_prep_circuit',      'Elbow preparation circuit',    'mobility', 'reps',          1, 'Slow banded curls, straight-arm holds and pronation work. Run before every straight-arm session.'),
    ('band_face_pull',          'Band face pull',               'mobility', 'reps',          1, 'Pull to the face with external rotation. Balances heavy pressing.'),
    ('seated_pike_stretch',     'Seated pike stretch',          'mobility', 'static_hold',   1, 'Straight legs, hinge from the hips. Compression and hamstring length.'),
    ('pancake_stretch',         'Pancake stretch',              'mobility', 'static_hold',   2, 'Straddle sit, chest toward the floor. Needed for straddle levers and planche.'),
    ('wrist_extensor_curl',     'Wrist extensor curl',          'mobility', 'reps',          1, 'Light load, full range through wrist extension. Tendon work for hand balancing.'),
    ('shoulder_dislocate_hold', 'Shoulder extension hold',      'mobility', 'static_hold',   2, 'Hands behind on a low bar, walk the feet forward. Opens the shoulder for German hangs and levers.')
on conflict (slug) do update set
    name        = excluded.name,
    category    = excluded.category,
    measure     = excluded.measure,
    difficulty  = excluded.difficulty,
    description = excluded.description;
