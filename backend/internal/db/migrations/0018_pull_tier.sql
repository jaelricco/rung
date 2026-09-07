-- The tier above the full front lever.
--
-- Migration 0013 added the elite tier and 0014 rebuilt its maltese on the
-- floor. Both were push: maltese, iron cross, planche press. The pull side of
-- the same tier was never added at all, which is why the app's "maximal" list
-- had exactly three entries and none of them was something you hang from.
--
-- Two skills fill it, and they are the front lever's two directions out.
--
-- The SAT is the front lever with the arms opened to the maximum on a straight
-- bar — the bar's answer to the rings Victorian, and to the front lever
-- exactly what the maltese is to the planche: the same skill with the hands
-- travelling outward. So it gets the same shape of ladder 0014 gave the
-- maltese, including the half-way rung that tradition runs through: a
-- wide-grip front lever, which is to the SAT what the wide planche is to the
-- maltese.
--
-- The one-arm front lever is the other direction: the same hands, one of them.
-- It follows the ordinary lever progression — tuck, advanced tuck, straddle,
-- full — because that is what shortening a lever looks like whichever arm is
-- holding it.
insert into exercises (slug, name, category, measure, difficulty, description) values
    -- The SAT line, on the bar.
    ('box_victorian',       'Box victorian',              'static', 'static_hold',  8, 'Victorian position with the forearms resting on a box or a low bar. The earliest honest loading of the shoulder angle, and where this ladder starts once the front lever is held.'),
    ('wide_front_lever',    'Wide-grip front lever',      'static', 'static_hold',  9, 'Front lever with the hands set wider than the shoulders. The rung between a front lever and a SAT, and the one that teaches the shoulder the angle.'),
    ('band_sat',            'Band-assisted SAT',          'static', 'static_hold',  9, 'SAT with a band under the hips. Assistance comes down between blocks, never inside a session.'),
    ('tuck_sat',            'Tuck SAT',                   'static', 'static_hold',  9, 'SAT with the legs tucked. Arms fully open, lever shortened.'),
    ('adv_tuck_sat',        'Advanced tuck SAT',          'static', 'static_hold',  9, 'Tuck SAT with the back flat and the knees carried further out.'),
    ('straddle_sat',        'Straddle SAT',               'static', 'static_hold', 10, 'SAT with the legs straddled. The last rung before the full hold.'),
    ('sat',                 'SAT',                        'static', 'static_hold', 10, 'The front lever with the arms opened to the maximum, on a straight bar. Hips at the bar, arms locked, shoulders depressed.'),
    ('front_lever_touch',   'Front lever touch',          'pull',   'reps',         9, 'Pull from the front lever until the bar touches the hips, and return to the line. One of the two marks of a front lever that is owned rather than reached.'),

    -- The rings version of the same position, which is what an angry wrist
    -- trains instead: the ring turns with the forearm, so nothing is pressed
    -- against a bar.
    ('tuck_victorian',      'Tuck victorian',             'static', 'static_hold',  9, 'Victorian on rings with the legs tucked. The neutral-wrist form of the tuck SAT.'),
    ('victorian',           'Victorian',                  'static', 'static_hold', 10, 'The rings version of the SAT: front lever with the arms opened to the maximum, rings turned out.'),

    -- The one-arm front lever line.
    ('assisted_one_arm_front_lever', 'Assisted one-arm front lever', 'static', 'static_hold',  8, 'One-arm front lever with the second hand on a band, a strap or a lower grip. The assistance is what comes off, one step per block.'),
    ('one_arm_tuck_front_lever',     'One-arm tuck front lever',     'static', 'static_hold',  9, 'Front lever on one arm with the legs tucked.'),
    ('one_arm_adv_tuck_front_lever', 'One-arm advanced tuck front lever', 'static', 'static_hold', 9, 'One-arm tuck front lever with the back flat and the knees carried out.'),
    ('one_arm_straddle_front_lever', 'One-arm straddle front lever', 'static', 'static_hold', 10, 'One-arm front lever with the legs straddled. Bending the working elbow is the regression inside this rung, not a rung of its own.'),
    ('one_arm_front_lever',          'One-arm front lever',          'static', 'static_hold', 10, 'The front lever held on one arm. Grip, lat and the anti-rotation the second arm used to provide, all at once.')
on conflict (slug) do update set
    name        = excluded.name,
    category    = excluded.category,
    measure     = excluded.measure,
    difficulty  = excluded.difficulty,
    description = excluded.description;
