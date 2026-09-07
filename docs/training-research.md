# What the planner knows

This is the research the deterministic planner is built from. Every number in
`internal/plan/knowledge.go` traces back to a line in here, and every line in
here names where it came from. It is written to be argued with: if a threshold
is wrong, change it here and in the table it feeds, and the whole app follows.

A note on the sources. Training methodology has two literatures and they are
not equally good. Sets, volume, protein and tendon loading have real trials and
meta-analyses behind them, and those are cited. The progression ladders for
levers, planche and flags do not — there is no randomised trial of tuck versus
advanced tuck. What exists there is coaching consensus, and it is reported as
such, with the disagreements left visible rather than averaged away.

---

## 1. The four things calisthenics training is made of

The sport splits into four qualities that do not train the same way, do not
recover on the same clock, and cannot be programmed with one rule. Almost every
bad plan is a plan that treated them as one thing.

### Statics — straight-arm holds

Front lever, back lever, planche, human flag, L-sit, handstand. The load is an
isometric contraction at a very long lever, taken mostly by connective tissue
and by the shoulder in a position it has no strength curve in until you build
one.

The programming consequences are specific:

- **Train them in short holds, well short of failure.** Coaching sources
  converge on accumulating work at **50–70% of the best hold**, in sets of
  **3–15 seconds**, for **3–6 sets** ([GMB](https://gmb.io/planche/),
  [Calisthenics Association](https://calisthenicsassociation.org/lessons/advanced-tuck-planche),
  [Calisthenics 101](https://www.calisthenics-101.co.uk/how-to-front-lever)).
  A static held to collapse trains the collapse: the line breaks, the hips
  drop, and what gets rehearsed is the broken position.
- **Rest long.** 2.5–4 minutes between maximal-quality holds; up to 3–5 minutes
  when a rung is hard enough that the best hold is only 10–15 seconds
  ([Calisthenics Corner](https://www.calisthenics-corner.com/skills/front-lever/)).
- **Frequency 2–3 sessions a week per skill**, blocks of 8–12 weeks
  ([Calisthenics 101](https://www.calisthenics-101.co.uk/how-to-front-lever)).
- **Progress on a standard, not on a feeling.** The common rule is a clean hold
  of **10–15 seconds for 3+ sets** at the current rung before opening the lever
  ([GMB](https://gmb.io/planche/), [Bodyweight Training Arena](https://bodyweighttrainingarena.com/from-tuck-to-advanced-tuck-planche/)).

Where sources disagree: the exact entry standard. 10 s for 3 sets and 15–20 s
for multiple sets both appear, for the same rung. The planner takes the
stricter end for the lower rungs (where the cost of moving early is an elbow)
and the looser end higher up (where the holds get short for everyone).

Realistic timelines, which matter because an 8-week plan promising a full
planche is lying: planche lean in month one, tuck planche around a year,
straddle at two to three years, full planche at three to five plus
([Fitloop](https://fitloop.app/skills/planche)). The handstand is 3–12 months to
a freestanding hold ([Playthenics](https://playthenics.com/blog/how-long-to-learn-handstand)).

### Dynamics — explosive and release moves

Muscle-ups, 360s, shrimp flips, explosive pull-ups. These are trained as
**skills first and conditioning never**. Attempts belong early in the session,
in low volume, while fresh, because a missed release move is how people land on
their heads.

- **Entry standard for the bar muscle-up: 8–10 strict dead-hang pull-ups**, plus
  a chest-high explosive pull and a stable deep dip
  ([BULLBAR](https://bullbarfit.com/blogs/q-as/what-training-progression-can-lead-from-standard-pull-ups-to-performing-a-muscle-up),
  [Gymnase Tips](https://www.gymnasetips.com/muscle-up-progression/)).
- **The transition is technique, not strength.** ~90% of failed attempts stall
  at the moment the chest passes the bar, which is why the ladder runs
  explosive pull → false grip → low-bar transition → assisted → full, rather
  than "get stronger and try again"
  ([Gymnase Tips](https://www.gymnasetips.com/muscle-up-progression/),
  [Berg Movement](https://www.bergmovement.com/calisthenics-blog/how-to-explosive-muscle-up)).
- Typical time to a first muscle-up from a 8-pull-up base: **8–12 weeks**
  ([Calisthenics Association](https://calisthenicsassociation.org/blog/muscle-up-tutorial-zero-to-hero)).

### Weighted — added load on the basics

Weighted pull-ups, dips, muscle-ups, push-ups. This is the one part of
calisthenics that is ordinary strength training and can be programmed with the
ordinary tools: percentages of a one-rep max, rep-max tables, block
periodisation.

- **%1RM to reps** follows the standard table: ~80% of 1RM ≈ 8 reps, and the
  usable strength range is 75–90%
  ([The Movement System / NSCA Table 17.7](https://www.themovementsystem.com/blog/cscs-program-design-how-to-program-based-on-1rm-with-example-program)).
- **A three-block structure** is the common shape: accumulation (moderate load,
  higher reps) → intensification (load up, reps down) → peak (heavy singles and
  doubles) ([Calisthenics Association](https://calisthenicsassociation.org/blog/weighted-pull-up-program)).
- **The 1RM includes bodyweight.** A 70 kg athlete doing a pull-up with 20 kg is
  moving 90 kg, so estimating a max from added load alone is wrong by the
  weight of the athlete. The planner estimates
  `e1RM_total = (bodyweight + added) × (1 + reps/30)` (Epley) and subtracts
  bodyweight again to get a prescribable added load
  ([Calintellect](https://www.calintellect.com/articles/one-rep-max-calculator-weighted-calisthenics)).
- **Don't add load to a movement that isn't owned yet.** The entry standard used
  here is 8–10 strict reps of the unloaded movement before any belt goes on.

### Conditioning and density formats — EMOM, AMRAP, Tabata, ladders

These are formats, not training qualities. They set *when* work happens rather
than what it is, and the choice of format decides what the session actually
trains ([Sole](https://www.soletreadmills.com/blogs/news/amrap-vs-emom-vs-tabata-differences-benefits-examples),
[Alo](https://blog.alomoves.com/movement/amrap-emom-tabata-explained-why-you-should-switch-up-your-training),
[Adamas](https://adamascrossfit.com/5-types-of-crossfit-workouts-explained/)):

| Format | Shape | What it is good for | What it ruins |
| --- | --- | --- | --- |
| **EMOM** (every minute on the minute) | fixed reps at the top of each minute, the remainder is rest | technique under mild fatigue, honest volume accumulation, submaximal skill work | max-effort work — the clock caps your rest |
| **AMRAP** | as many rounds/reps as possible in a fixed window | work capacity, pacing | anything where form degrades dangerously; never for a release move or a heavy static |
| **Tabata** | 20 s on / 10 s off × 8 | VO2max, four honest minutes | strength; it is a conditioning tool wearing a strength costume |
| **Ladder** | reps climb (or climb and descend) each round | volume at submaximal intensity without a grinding set | precision skill work |

The planner uses **EMOM for submaximal skill and volume work** (its rest cap is
a feature there) and **AMRAP/ladders as finishers on non-hard days only**. It
never puts a static hold, a release move, or a heavy weighted set inside a
timed format — that is the combination that fills the injury tables.

### The fifth thing: grease the groove

High-frequency, low-fatigue practice — sets at **50–70% of max, spread across
the day, never near failure**
([Heavyweight Calisthenics](https://heavyweight-calisthenics.com/grease-the-groove-for-calisthenics/),
[Calisthentials](https://calisthentials.com/grease-the-groove/)). It is the right
tool for a skill that is limited by the nervous system rather than by tissue:
handstand balance, false-grip tolerance, the first pull-up. The planner emits it
as an explicit off-day prescription for skills where frequency beats volume,
which the motor-learning literature supports: **distributed practice beats
massed practice for retention**
([Distributed practice RCT](https://www.ncbi.nlm.nih.gov/pmc/articles/PMC4864846/),
[contextual interference](https://www.ncbi.nlm.nih.gov/pmc/articles/PMC4069194/)).
For handstands specifically, coaching sources put the useful frequency at 3×/week
early and 5+×/week once balance is the limiter
([UMoveSg](https://umovesg.com/blogs/handstands-training/how-often-should-you-practice-handstands)).

---

## 2. How a week is built

### Volume

The best current dose-response evidence is Pelland et al.'s meta-regression of
67 studies and 2,058 subjects: hypertrophy keeps rising with weekly sets well
past the point strength does, and both show diminishing returns — strength much
more sharply ([Sports Medicine](https://link.springer.com/article/10.1007/s40279-025-02344-w),
[SportRxiv preprint](https://sportrxiv.org/index.php/server/preprint/view/460),
[summary](https://biolayne.com/reps/issue-31/the-king-of-volume-metas/)). A
systematic review puts the practical band at **12–20 hard sets per muscle group
per week** for trained young men
([PubMed 35291645](https://pubmed.ncbi.nlm.nih.gov/35291645/)).

Frequency, at equal volume, does little for hypertrophy but does help strength
with diminishing returns — which is exactly the argument for spreading a pattern
over two or three sessions rather than one.

The planner works in **hard sets per movement pattern per week** — pull, push,
static, legs, core — with a floor of 6 and a ceiling that scales with the
athlete's logged session history rather than with ambition. Someone who logged
four sessions in the last 28 days does not get a 20-set week.

### Ordering inside a session

Fixed, and the app already enforces it: joint preparation → skill and
straight-arm work while fresh → heavy strength → accessories → conditioning.
The reason is that a static or a release move performed on fatigue is both a
worse rehearsal and a higher-risk one.

Warm-up follows **RAMP** — Raise, Activate, Mobilise, Potentiate
([Human Kinetics](https://humankinetics.me/2019/03/04/what-is-the-ramp-warm-up/),
[Teambuildr](https://blog.teambuildr.com/understanding-and-implementing-the-ramp-protocol)).

Every protocol carries the phase it belongs to, and a session is performed and
displayed in six named parts: **joint warm-up**, **muscular warm-up** (raise),
**mobility and dynamic stretching** (mobilise), **specific warm-up**
(potentiate — the movements of the session at a fraction of the effort),
**training**, **cool-down**. Joint preparation is named separately from the
rest of RAMP rather than folded into activation, because in a sport loading
wrists and elbows at full extension it is the part that gets skipped and the
part the injury tables are about.

Mobilise means moving through range, not holding an end position: static
stretching immediately before training reduces strength for roughly the next
hour, which is the wrong hour.

### Spacing

48 hours between hard sessions for the same pattern. With more training days
than patterns, alternate rather than repeat. This is in the app's coaching rules
already and the planner implements it as a hard constraint on day placement
rather than as advice.

### Progression and periodisation

- Weeks 1 to N–1 of a block: **add one unit per week** — a second on the hold, a
  rep, 1.25–2.5 kg, a degree of lean. Stated per block, not left to feel.
- **Deload every 4–6 weeks.** Surveyed strength and physique athletes deload
  every **5.6 ± 2.3 weeks** for **6.4 ± 1.7 days**, and do it by cutting volume
  and effort while keeping the movements
  ([Sports Medicine Open](https://link.springer.com/article/10.1186/s40798-024-00691-y),
  [practical recommendations](https://shura.shu.ac.uk/35313/3/Bell-APracticalApproach(AM).pdf)).
  A controlled trial found a one-week deload cost nothing in adaptation
  ([PMC10809978](https://www.ncbi.nlm.nih.gov/pmc/articles/PMC10809978/)) — the
  value is fatigue management over months, not the week itself.
- **Autoregulate with RIR.** Prescribe reps-in-reserve, not just sets and reps:
  2–3 RIR in accumulation, 1–2 in intensification, 0–1 only in a test week
  ([review of deloading and autoregulation practice](https://pmc.ncbi.nlm.nih.gov/articles/PMC9811819/)).
- **The last week tests the goal**, in terms that can be passed or failed.

---

## 3. Injuries: what actually happens, and what to do about it

### The epidemiology

Calisthenics is not a high-injury sport by rate — about **1.29 injuries per
1000 training hours** — but the injuries it does produce are concentrated and
slow ([Ngo et al., OJSM 2021](https://journals.sagepub.com/doi/full/10.1177/2325967121990926);
[German Journal of Sports Medicine](https://www.germanjournalsportsmedicine.com/archive/archive-2018/issue-9/the-epidemiological-profile-of-calisthenics-athletes/);
[Open Access J Sports Med](https://www.tandfonline.com/doi/pdf/10.2147/OAJSM.S394044)):

- **More than three quarters of injuries are upper-extremity.** Shoulder first,
  then upper/mid back, elbow, wrist and hand.
- **Nearly two thirds cause time lost from training.**
- The activities people were doing when hurt: **freestyle/dynamic 20.7%,
  planche 12.1%, muscle-ups 12.1%, front lever 10.3%**.

That list is the design brief. The four things most likely to hurt you are
exactly the four things a skill plan is mostly made of, which is why joint
preparation is not optional decoration in this app and why the planner refuses
to put dynamics or maximal statics late in a fatigued session.

### Loading an irritated tendon

The evidence has moved away from "eccentrics only". Isometric, isotonic,
eccentric and heavy-slow resistance all work, and **HSR (~70% 1RM, roughly a
7RM, slow tempo) is at least as good as isolated eccentrics**
([editorial, PMC8954075](https://www.ncbi.nlm.nih.gov/pmc/articles/PMC8954075/);
[network meta-analysis, PMC11570476](https://www.ncbi.nlm.nih.gov/pmc/articles/PMC11570476/);
[Physiopedia](https://www.physio-pedia.com/Tendinopathy_Rehabilitation)).

The staged shape used by the app's rehab protocols:

1. **Isometrics for pain.** 30–45 s holds, pain-free effort, to settle
   irritability and get an analgesic effect
   ([E3 Rehab, golfer's elbow](https://e3rehab.com/golfers-elbow-rehab/)).
2. **Reduce the provoking load, don't stop training.** Shorten range, change the
   implement (parallettes instead of the floor for an angry wrist), keep the
   rest of the body training.
3. **Heavy slow resistance** once tolerance returns, to build tendon stiffness
   and load capacity.
4. **Return to the provoking movement at half the previous volume.**

Tendon adapts by hypertrophy and increased stiffness driven by collagen
synthesis after each loading bout, and **infrequent loading (one hard session a
week) adapts worse than more frequent, moderate loading**
([German Journal of Sports Medicine](https://www.germanjournalsportsmedicine.com/archive/archive-2019/issue-4/functional-adaptation-of-connective-tissue-by-training/);
[Nature Sci Rep, Achilles loading](https://www.nature.com/articles/s41598-024-56840-6)).
Practically: the elbow prep that goes before straight-arm work every session is
doing more than warming you up.

### The line the app does not cross

Nothing here diagnoses. The rule the whole app follows is that persistent,
worsening, night-waking or numbness-producing pain is an in-person assessment,
not a programming problem.

---

## 4. Recovery

- **Sleep is the largest single lever, and it is measurable.** Meta-analysis of
  nine studies and 1,078 athletes: shorter sleep raises injury risk
  (**OR 1.34, 95% CI 1.08–1.66**). In adolescent athletes the split is stark —
  **65% of those sleeping under 8 h were injured versus 31% of those at 8 h or
  more**, against an actual average of ~6.3 h
  ([meta-analysis](https://www.researchsquare.com/article/rs-7606280/v1);
  [narrative review, PMC10745648](https://www.ncbi.nlm.nih.gov/pmc/articles/PMC10745648/)).
- **Rest days are structural, not leftovers.** They are what makes the 48-hour
  spacing rule real, which is why this app draws them on the calendar instead of
  omitting them.
- **Deload before you need one** (see §2). Stalled performance, accumulated
  joint ache and a rising RPE at the same load are the signals to bring one
  forward.
- **Soreness is not the metric.** Readiness is: bar speed, best hold, and
  whether yesterday's easy set felt easy.

---

## 5. Nutrition

Kept to what is defensible and general. The app gives ranges, not meal plans,
and prescribes no supplements.

- **Protein: ~1.6 g/kg/day**, with individual variation up to ~2.2. Morton et
  al.'s meta-analysis of 49 studies found gains in fat-free mass plateau at
  **1.62 g/kg/day (95% CI 1.03–2.20)**
  ([PMC5867436](https://pmc.ncbi.nlm.nih.gov/articles/PMC5867436/);
  [dose-response meta-analysis](https://www.ncbi.nlm.nih.gov/pmc/articles/PMC9441410/)).
  Higher intakes (2.2–2.6) are worth it mainly in a deficit, to hold muscle.
- **Energy availability is the one with a hard floor.**
  `EA = (intake − exercise expenditure) / kg fat-free mass`. Below roughly
  **30 kcal/kg FFM/day**, endocrine, bone and metabolic function are disturbed;
  optimal is ≥45. This is Relative Energy Deficiency in Sport, and it costs bone
  density, tendon and muscle adaptation, and recovery — which matters
  disproportionately in a sport where being lighter makes every skill easier,
  and where the temptation to cut is therefore permanent
  ([IOC 2023 consensus](https://stillmed.olympics.com/media/Documents/Athletes/Medical-Scientific/Consensus-Statements/REDs/BJSM-IOC-consensus-statement-on-Relative-Energy-Deficiency-in-Sport-REDs.pdf);
  [Endocrine Reviews](https://academic.oup.com/edrv/article/45/5/676/7629683)).
- **Weight loss for skills has a ceiling.** Strength-to-weight ratio is real —
  a lighter athlete holds a lever more easily — but the gain stops well before
  the energy-availability floor, and passing that floor costs the tendon
  adaptation the skill is built on.
- **Around training:** eat carbohydrate before hard skill sessions (they are
  neurally demanding and long), and get protein across the day in several
  feedings rather than one. Timing is a small effect next to daily totals
  ([protein timing meta-analysis](https://www.tandfonline.com/doi/full/10.1186/1550-2783-10-53)).
- **Hydration:** ordinary thirst-led drinking; more when sessions are long, hot
  or grip-limited.

---

## 6. What to ask an athlete before writing them a plan

A planner that works from records has one failure mode that has nothing to do
with training: a person who has trained for five years and joined yesterday has
no records, so it starts them at the bottom of every ladder. The fix is to ask —
but only for the figures that change a prescription. Every extra field costs
answers on the ones that matter.

These are the ones that do, and what each decides:

### Body

| Value | What it changes |
| --- | --- |
| **Bodyweight** | Every added-load percentage. A pull-up one-rep max that ignores bodyweight is wrong by the weight of the athlete (§1), so without this the plan can only say "add a little". |

### Eight benchmarks

The smallest set that resolves every branch the planner takes. Each is a test
someone can do today and write one number against.

| Test | Unit | What it decides |
| --- | --- | --- |
| **Pull-up**, max strict | reps | The whole pulling ladder, and whether a belt is on the table at all (≥10) |
| **Dip**, max strict | reps | The same for pushing, and loaded dips (≥12) |
| **Push-up**, max | reps | What the pushing work is built from when dips are not there yet |
| **Dead hang** | seconds | Grip, the quiet limit on every hanging skill, and the first rung of the pull-up ladder |
| **Hollow body hold** | seconds | The body line every lever is held with; a lever plan over a weak hollow stalls |
| **Hanging leg raise** | reps | Which core progression is used |
| **Bodyweight squat** | reps | Where the leg work starts |
| **Chest-to-wall handstand** | seconds | Where handstand and overhead pressing work begins |

### The rungs of the chosen ladder

The app already knows the ladder to each skill, so it asks about that ladder and
no other: a front lever plan asks for tuck, advanced tuck and straddle holds; a
pistol plan does not. The first rung the athlete cannot fill in is roughly where
they are, which is exactly what placement needs.

### Training context

| Value | What it changes |
| --- | --- |
| **Sessions a week** | The starting volume. With no log here, a declared four sessions a week is better evidence than nothing, and the plan starts a step below full instead of two. |
| **Sleep, hours a night** | Under about 8 h roughly a third more injuries (§4), and volume is the only part of that a plan controls — so under 7 h it adds volume half as fast rather than starting lighter. A ten-percent trim on set counts of three to six rounds away to nothing; slowing the climb does not. |
| **Equipment** | Which movements exist. A plan that prescribes weighted pull-ups to someone with no belt is not a hard plan, it is an impossible one, and the athlete cannot tell which. Bar, parallel bars, parallettes, rings, a way to add load, bands — or none of it, which is a real answer and still a plan. |

### Deliberately not asked

Age, sex, height, body-fat percentage, years training, VO2max. None of them
changes a single number this planner produces, and collecting them would be
pseudo-precision — a form that looks thorough and is not.

Open injuries are asked for elsewhere, on the injuries page, because they are a
running record rather than a one-off baseline.

### The rule that keeps it honest

A declared figure never overwrites a logged one. The record is whichever is
higher, and every prescription built on a declaration says so in as many words
("from your baseline rather than a logged set"). Log the movement once and the
plan stops taking your word for it.

---

## 7. The tier above the full planche

Everything above assumed a ceiling somewhere around a full planche or a full
front lever. That ceiling was in the app, not in the sport. An athlete holding a
twelve-second full planche is past the last rung of every ladder in §1, and the
skills that come next — maltese, iron cross, one-arm handstand, planche press,
front lever pull-up — do not behave like harder versions of what came before.
Three things change.

### Entry standards stop being advice and become a gate

Below this tier, starting a progression early costs you a slow month. Here it
costs a biceps tendon. So these skills carry prerequisites that are checked
rather than suggested.

**Maltese.** The commonly cited entry is a **full planche held 10–15 seconds in
good form**, alongside a **straddle planche at 10–15 seconds**, a **back lever
of at least 10 seconds**, and — because every honest progression is on rings —
ring competence before ring skill
([Calisthenics 101](https://www.calisthenics-101.co.uk/maltese-calisthenics),
[Calisthenics world](https://calisthenicsworld.org/maltese/),
[Caliathletics](https://caliathletics.com/knowledge/how-to-train-maltese/),
[GymnasticBodies forum](https://www.gymnasticbodies.com/forum/topic/5286-maltese-ring-prerequisites-skipping-the-planche)).
The back lever is not decoration: it is the cheapest available test of whether
the biceps tendon tolerates a long lever with the shoulder open, which is
exactly what maltese work asks for and exactly what it damages.

**One-arm handstand.** A **comfortable 30–60 second two-arm handstand**, and
the sources are unusually blunt that this means full mastery rather than a
hard-won maximum: "the alignment, technique, control and comfort must be very
high on two arms to the degree there is little challenge left in the basic
drills". Then **20–30 seconds at each progression** before the next, and
**1.5–3 years** to a first straddle one-arm even with good preparation
([Handstand Factory](https://handstandfactory.com/articles/are-your-ready-for-the-one-arm-handstand/),
[GMB](https://gmb.io/oahs/)).

### The prerequisite does not stop mattering once you are past it

The relationship between planche and maltese is reciprocal in *training effect*
— you are learning one language of straight-arm, hollow-body control, so lever
work and planche work feed each other — but it is **one-directional in
sequencing**: planche is the base that builds the maltese, and the standard
advice is to approach maltese only after the planche is genuinely held
([Calisthenics world](https://calisthenicsworld.org/maltese/),
[Gravgear](https://au.thegravgear.com/blogs/gymnastic-ring-training/maltese-cross-rings-gymnastics)).

The practical consequence is the one most people get wrong: **the planche stays
in the week**. An athlete who parks their planche to chase a maltese loses the
thing that was driving the maltese, and ends up with neither. So a goal in this
tier declares what feeds it, and those skills keep a maintenance slot — held,
not pushed.

### There is one tendon budget, and every skill spends from it

Elite practice splits training by elbow position — straight-arm skills (planche,
levers, cross) on their own days, bent-arm skills (HSPU, muscle-up, dips) on
others — precisely so the straight-arm structures get a full week between
exposures. High-strain statics sit around **3–4 days a week**, and the limiter
is connective tissue rather than muscle
([Coach Bachmann](https://www.coachbachmann.com/knowledgebase/the-ultimate-calisthenics-training-split-for-maximum-gains)).

The framing worth stealing is the **tendon budget**: most calisthenics injuries
appear after *a spike in training stress*, and **a new skill layers load onto
tissue the previous skills did not prepare** — especially when it is added on
top of an already high-volume routine
([BULLBAR](https://bullbarfit.com/blogs/updates/calisthenics-injury-prevention-the-tendon-budget-approach-so-you-can-train-for-years)).
Muscle adapts faster than tendon, so the athlete feels ready before the tissue
is.

There is no dose-response curve for elite statics to lean on, so the planner
uses a deliberately blunt count: each maximal straight-arm skill costs 2–3
units of a week that holds about 5. Four skills in flight is roughly double
what one athlete recovers from, and the answer is to park, not to squeeze.

### Adding a skill without getting hurt

The rule that falls out of all of the above: **a movement with no history starts
small regardless of how strong the athlete is elsewhere.** An experienced
athlete's normal volume, applied to tissue that has never seen the position, is
the spike. First exposures are capped, the first session is treated as finding
the number rather than hitting one, and assistance (bands) comes down in small
steps between blocks rather than inside a session
([Gravgear](https://au.thegravgear.com/blogs/gymnastic-ring-training/maltese-cross-rings-gymnastics)).

---

## 8. Wrists, specifically

This deserves its own section because the evidence points somewhere other than
where the intuition does.

A 2025 cross-sectional survey of **321 adult handstand practitioners** found
chronic wrist pain in **182 of them — 56.7%**. The striking part is what it was
*not* associated with: **no significant association with weekly training hours,
warm-up routines, brace use, or grip device use.** Younger age was associated
with *higher* prevalence
([Martonovich et al., J. Funct. Morphol. Kinesiol. 2025](https://pubmed.ncbi.nlm.nih.gov/41133562/)).

Read that carefully, because it inverts the usual advice. If wrist pain in this
population does not track volume or warm-ups, then cutting sets and adding a
wrist circuit is treating the wrong variable. What it does track is
**how much load arrives through a wrist in extension** — the wrist has limited
tolerance for sustained hyperextension, and axial load in that position readily
exceeds what the carpal ligaments stabilise, with force going through the
radiocarpal and midcarpal joints, the volar radiocarpal ligaments and the TFCC
([StatPearls, TFCC](https://www.ncbi.nlm.nih.gov/books/NBK537055/),
[Warringah Physio](https://www.warringahphysio.com.au/single-post/2020/06/22/wrist-pain-in-gymnastics)).
Sustained end-range extension is also the mechanism behind **dorsal wrist
impingement**, where soft tissue is repeatedly pinched between radius and
carpals until the capsule thickens
([Hooper's Beta](https://www.hoopersbeta.com/library/how-to-fix-ulnar-wrist-pain-tfcc-injury-recovery-guide),
[E3 Rehab](https://e3rehab.com/wrist-pain-rehab/)).

So for a **mild, chronic** wrist complaint the useful lever is the *position*,
not the amount:

| Instead of | Train | Why |
| --- | --- | --- |
| Floor planche | **Ring planche** | Neutral wrist; harder skill, no extension |
| Dips, L-sits on the floor | **Rings, parallettes** | Wrist out of end-range |
| Push-ups, planks | **Fists, knuckles** | Neutral |
| Handstand on flat palms | **Fingertips, wedge, parallettes** | Reduces the extension angle |

Deleting every wrist-loading movement — which is what a naive injury filter
does — removes essentially the entire sport for a complaint that half of all
hand balancers carry. That is not caution, it is uselessness.

**Where the line is.** This substitution logic is for a niggle. Pain that has
lasted months, wakes the athlete at night, or comes with numbness or clicking
on rotation is a wrist that needs imaging and an in-person assessment, and a
severe complaint still clears the region outright. Loading is managed the same
way as any other tendon: isometrics for pain, reduce the provoking position,
heavy slow resistance as tolerance returns (§3).

---

## 9. Two traditions build a maltese, and they are not the same ladder

§7 built the maltese out of rings, from ring support and band-assisted crosses,
because that is what the freely available sources describe. A structured
calisthenics programme for planche and maltese — Cali-Aesthetics, paid, so the
tables are not reproduced here — builds it entirely differently, and the
difference is not cosmetic.

In the **rings-gymnastics** lineage a maltese is a cross derivative: ring
support, iron cross, L-cross, maltese. In the **floor-and-parallel-bars**
lineage it is a planche whose hands keep travelling outward, and the ladder runs

> lean maltese → **wide planche hold** → wide planche press → zanetti →
> maltese elevator → maltese hold → maltese press

The **wide planche** — hands wider than a planche, narrower than a maltese — is
the rung §7 had no equivalent for at all, and it is the one that teaches the
shoulder the angle. Across five documents of that programme, "back lever",
"iron cross", "ring support" and "front lever" appear **zero** times; "rings"
appears three times, as an implement variant at the very top. The prerequisites
§7 took from the rings sources were being applied to a skill from the other
tradition.

This app follows the floor ladder, because that is the sport its athletes are
in. The rings ladder survives as the iron cross, which is genuinely its own
goal.

### Gates belong on rungs, not on goals

The same programme prescribes **lean maltese, banded, as an accessory block in
its beginner workouts** — two sets of 5–10 seconds, at the end of the session.
So "may I train toward a maltese" is the wrong question. Leaning into the
position with a band is available early; *holding* and *pressing* it are not.

A gate on the goal gets the first half wrong, so the prerequisites moved onto
individual rungs. An athlete is then placed by their records and capped by the
gate of the rung above — held one rung below what they have logged, with the
number that would release them.

### What the programme's own parameters are

Counted across its beginner, intermediate, advanced and personal workouts:

| | Values used |
| --- | --- |
| **Rest** | 5 min (64×), 3 min (32×), 2 min (29×), 4 min (18×), 7 min (6×) |
| **Sets** | 5 (66×), 2 (60×), 3 (27×) |
| **Holds** | 2–4s, 3–6s, 3–8s, 4–8s, 5–10s, 8–15s, 10–20s |
| **Reps** | 1–3, 2–4, 3–5, 3–8, 5–10, 8–15 |

Three things fall out of that, and all three contradicted what this app was
doing:

**Rest is 4 to 7 minutes for maximal work, at every level.** The beginner
holding a tuck planche rests as long as the athlete pressing a maltese, because
the rest is for the maximality of the position, not for the experience of the
athlete. This app had been prescribing 2.5 to 3 minutes.

**Everything is a range, never a point.** `5–10s`, `3–8r`. A range is met at the
top on a good day and the bottom on a bad one without the plan being wrong
either time. Computed targets are now snapped onto the nearest band from the
lists above rather than printed as a single number.

**Volume is held constant and difficulty is moved.** Five sets is the workhorse
at every level; what changes between beginner and advanced is the variant and
the assistance — band, neck band, elevated, parallel bars versus floor versus
supinated grip, +2 kg ankle, +4 kg hips, +10 kg vest. That is the reverse of
scaling sets by readiness, and for skill work it is the better way round.

### The session is a span of the ladder

Every workout in the programme has the same four-block shape:

```
2–3 sets   the hardest variant       5–7 min rest
5 sets     the main rung             5 min
5 sets     the rung below it         3–4 min
2–3 sets   the easiest variant       2 min
```

Read against a ladder, that is not four arbitrary exercises — it is a span of
rungs, hardest first while the athlete is completely fresh. The opener is two
or three sets of the *next rung up*, attempted before anything has tired. A
skill attempted at the end of a session is a skill rehearsed badly.

---

## 10. How much of a week one skill may have

§7 asked what a week costs the tendons and §9 asked what one session looks
like. Neither asked the question in between, which is the one an athlete
actually has an opinion about: *how much of my week does this new skill get?*
The planner answered it silently — as much as the session order allowed — and
that answer is wrong in both directions. Someone fitting a maltese into a week
they otherwise like does not want it eating the week. Someone who has cleared
the calendar for it does not want two sets and an apology.

### A note on these sources, and on what could not be read

The material below was gathered from three coaches whose channels were named
as the sources for this pass — [Doctor Yaad](https://www.youtube.com/@DoctorYaad)
(Yaad Mohammad, MD, calisthenics coach and athlete),
[King Dailong](https://www.youtube.com/@kingdailong) (Daï-Long Huynh, whose
maltese programme is the floor ladder §9 is built on) and
[Nicky Lyan](https://www.youtube.com/@NICKYLYAN) — plus the tendon literature
one of them interviewed.

The transcripts themselves could not be read. The environment this pass ran in
has an egress policy that blocks youtube.com along with the coaches' own sites,
so what follows comes from search over that material — the podcast pages,
programme documents, article versions and interviews — rather than from a
verbatim transcript. Several of the numbers below therefore carry a confidence
one step below the papers cited in §2 to §5, and every one of them is
attributed so it can be replaced by a better reading. Where a claim rests on a
single search summary rather than a source read end to end, it says so.

### Tendon adaptation saturates, and it saturates fast

This is the finding that changes the arithmetic, and unlike the rest of this
section it comes from a named researcher rather than from coaching consensus:
Keith Baar (UC Davis), interviewed on the Doctor Yaad podcast and in several
other places
([Doctor Yaad](https://doctoryaad.com/articles/how-to-get-bulletproof-tendons-ft-professor-keith-baar-doctor-yaad-podcast),
[Tim Ferriss #797](https://tim.blog/2025/02/26/dr-keith-baar/),
[Just-Fly Sports](https://www.just-fly-sports.com/podcast-392/),
[summary of the protocol](https://boxlifemagazine.com/secret-static-holds-build-stronger-tendons/)):

- **About ten minutes of loading gives a tendon its maximum anabolic signal.**
  Past that the growth signal does not rise; the wear does.
- **Ten minutes of load, then six to eight hours of rest, then ten minutes
  again, roughly doubles the collagen response** of one continuous bout. Two
  short exposures beat one long one, and the gap between them is the active
  ingredient.
- **Isometrics build a stiffer tendon than the same time under tension spent
  on reps.** Four 30-second holds across an eight-minute window produced a
  bigger *and* stronger tendon; the same total time as dynamic repetitions
  produced a bigger tendon with no strength gain and slightly *lower*
  stiffness.
- **Hold length depends on whether the tendon is healthy.** Short holds
  (1–10 s) for a healthy tendon; 30 s for an injured one, because a damaged
  region is stress-shielded by the tissue around it and needs longer to see
  the load. Ramp on over 3–5 s, hold, ramp off over 3–5 s — "low-jerk".
- **Three days immobilised costs 15–20% of a tendon's collagen.** Tendon is not
  slow to lose, only slow to build.

Read against the rest of this document, that says something blunt: **a maximal
skill day has a ceiling on useful volume, and it is low.** Sets past it are
bought at full tissue cost and no adaptation. It also retrospectively explains
why §1's grease-the-groove prescription works, and why the deload evidence in
§2 does not contradict it — frequency is the lever, duration is not.

### The frequency that actually moves a static

Doctor Yaad's planche material puts it at **two to three sessions a week per
maximal static, at least 48 hours apart** — more often and the wrists and
shoulders start reporting it, less often and the stimulus does not land — with
**4 to 6 sets of 5 to 15 seconds** per session at a fraction of the best hold,
and technique ahead of duration: *a 5-second hold with a correct posterior
pelvic tilt is worth more than a 15-second hold with a banana back*. He also
prescribes keeping the full variant in the week alongside the easier one —
negatives or band-assisted attempts at least weekly — so the nervous system
keeps seeing the goal pattern while the strength is built on the sub-variant
([summary of his planche guidance](https://doctoryaad.com/articles/should-you-skip-the-straddle-planche-i-asked-the-experts),
[FitnessFAQs #65, on programme design](https://creators.spotify.com/pod/profile/fitnessfaqs/episodes/65---How-To-Design-The-Perfect-Calisthenics-Program---Dr-Yaad-Mohammad-e30kaja)).

That is the same 2–3 the elite-split source in §7 gives for high-strain statics,
arrived at from a different direction, and it is the number the focus dial's
top position is capped at.

### The maltese, in its own programme's proportions

The programme this app's floor ladder comes from
([Daï-Long Huynh](https://street-workout.fandom.com/wiki/Da%C3%AF-Long_Huynh);
the programme documents themselves are paid, and are described rather than
reproduced) runs **six training days and one rest day around three skill
sessions: one planche, one maltese, and one mixed**. Work is in short sets —
1 to 8 reps — with **4 to 8 minutes of rest**, and at least 5 minutes between
sets of anything maximal. The progression order is straddle planche → full
planche → maltese on parallel bars → maltese on the floor.

The proportion is the point. **For the athlete whose entire sport is the
maltese, the maltese is one day in six, and a shared day makes two.** That is
where this app's ceiling of 40% comes from: it is a little above what the
sport's own specialists do, so it is a genuine ceiling rather than a
restatement of the average, and nothing in the app is allowed past it.

### A skill day belongs to one line

Across the coaching material the session has the same shape, and it is not four
arbitrary exercises: **the most specific thing first — the isometric itself —
then a dynamic version of the same line (a pseudo planche push-up, a planche
raise, a lean), and only then isolation accessories**
([worked example](https://www.thebodyweighttribe.com/blog/how-to-train-for-the-planche)).
The rest of the week carries the *other* skills, handstands, mobility and
ordinary bent-arm work — separately.

The consequence is the rule this planner was missing. What goes beside a
maltese hold on a maltese day is a **maltese lean, a wide planche, a planche
lean** — the rung above it and the rung below it. It is not an L-sit and not a
back lever. Those are not accessories, they are a second skill in an
accessory's clothes, and they spend from the budget §7 is about while adding
nothing to the position being trained. A skill day has room for exactly one
line.

### Nicky Lyan's frame: specificity, volume, normalisation

Nicky Lyan's material organises the same ideas as principles rather than
protocols — *specificity*, *volume work*, and **normalisation**, the idea that
a skill is owned when the position stops being an event and becomes ordinary
([his channel](https://www.youtube.com/@NICKYLYAN),
[the written version](https://nickylyan.com/products/how-to-actually-get-better-at-calisthenics)).
The detail behind the framework is in paid material and is not reproduced here;
what is taken is the framing, and it maps onto exactly one block: the rung
*below* the one being trained, run for more sets at a comfortable hold. That is
what normalisation costs, and it is the block a plan built only from maximal
work does not have.

### Maintenance is cheap, which is what makes a focused week affordable

Trained adults hold strength and muscle on a fraction of the volume that built
them — around **one third**, and for younger trained lifters as little as
**one ninth**, with roughly **2–5 sets per muscle group per week** and a single
weekly exposure generally enough to hold neuromuscular adaptation
([Bret Contreras' review of the maintenance literature](https://bretcontreras.com/how-much-training-is-necessary-to-maintain-strength-and-muscle/),
[maintenance volume summary](https://bonytobeastly.com/maintenance-training-volume/)).

That is the number that makes §7's rule affordable. Keeping the planche in the
week while chasing a maltese does not cost a second planche programme; it costs
two or three sets. So the fed skill stays, at maintenance, at every level of
the dial — and it is not charged to the new skill's share, because it is not
the new skill's work.

### What the dial does

Three positions, and a ceiling none of them crosses. The share is measured as
the goal's own ladder work against the week's working sets, warm-ups excluded,
on ordinary training weeks — a deload deliberately cuts accessories and keeps
the skill, so measuring it there would trim the one block that week exists to
protect.

| | Share of the week | Skill sessions | Rungs per session | Tendon budget |
| --- | --- | --- | --- | --- |
| **Keep it in the week** | ≤ 20% | 1 | the rung and its drill | its own cost |
| **Train it properly** | ≤ 30% | 2 | + the rung above, as the opener | its own cost |
| **Build the week around it** | ≤ 40% | 3 | + the rung below, as volume | one unit more |

The ceiling is enforced on the finished week rather than assumed from the way
it was built: sets come off the largest skill block first, down to a floor of
two, and only then does a session give up a rung of its span — the drill, then
the rung below, then the opener, in the order they add value. The rung the
session is named after is never removed. A week that still cannot be brought
under says so rather than pretending.

The 48-hour rule outranks the dial. A three-day week gets two skill sessions at
every level above the lowest, because a third would land inside the recovery of
the second, and the top of the dial is a cap rather than a target.

---

## 11. An accessory has a level too

The focus dial in §10 decided how much of the week the skill gets. It said
nothing about what fills the rest of the session, and what filled it was a
fixed list. Fixed lists have one failure mode, and it gets worse the better the
athlete is: they keep prescribing the movement that used to be hard.

The report that surfaced it is worth quoting in shape, because it is the honest
test of every accessory rule: *I can hold a front lever and a straddle planche
and I am training the front lever pull-up. An australian row is not accessory
work for me. A hollow body hold is not accessory work for me. Give me front
lever tuck rows, weighted pull-ups, dragon flags.* Every word of that is right,
and the planner was doing exactly what was described — three of the twenty
accessory lists in the catalogue named `australian_row` and thirteen named
`hollow_body_hold`, and the antagonist block was a fixed `australian_row` or
`push_up` at every level of the sport.

### Why tiers were the wrong fix

The core chain already *had* tiers — three of them — and it still produced the
complaint, because every tier keyed on one movement: a hanging leg raise or an
L-sit hold. An athlete whose log is full of levers and planches and empty of
hanging leg raises fell straight through to the bottom of it and was prescribed
planks. That is not a missing tier, it is the wrong evidence. Any rule that
asks "have you logged *this* movement" fails for the athlete who got strong
some other way.

Two numbers already in the app answer it without any new tiers at all. The
exercise library rates every movement 1 to 10 for difficulty. The athlete's
records name the hardest thing they have done. The band between them is the
tier, and it needs nothing kept in step: a movement added to the library joins
the right level by being rated, and an athlete moves up it by logging a set.

### The rules that fall out

**Level is per pattern, not overall.** Someone holding a maltese is not
therefore ready for a one-arm pull-up, and someone with a front lever is not
therefore a runner. The ceiling a movement is measured against is the hardest
thing on record in the same kind of work — pull, push, legs, core — with
statics measured against the overall figure, because that is what the ladders
are rated on.

**A pattern with nothing logged does not make a beginner.** Somebody with a
full planche has been training for years whatever their pull-up column says. So
the floor under an unlogged category is the overall figure less two: one useful
step down rather than a reset to push-ups.

**Movements that scale with load are exempt from the floor.** A weighted
pull-up is as hard as the plate on the belt, and a band face pull is prescribed
for what it does to a shoulder rather than for how hard it is. Neither is ever
"too easy for this athlete".

**Inside the band, what they have a record for comes first.** The point of
scaling accessories up is to stop prescribing work the athlete outgrew, not to
fill their week with movements nobody has a number for. A block priced from a
rung's standard rather than from a logged set is a guess, and a guess is
tolerable for one block and wrong for three.

**The rep count is capped by the movement, not by the slot that asked.** An
accessory slot asks for twelve reps because twelve is what an accessory is
worth. Hand that number to a one-arm negative and the block reads "8-15 reps"
of something nobody does more than three of. The library's difficulty rating
reads directly as what a set of the thing looks like:

| Difficulty | Most reps prescribed |
| --- | --- |
| 9–10 | 3 |
| 8 | 5 |
| 6–7 | 8 |
| 4–5 | 12 |
| 3 | 15 |
| 1–2 | 20 |

**The antagonist is one step easier, and never the goal's own work.** That
block exists to balance the day. A block that trains the same thing balances
nothing — an ice cream maker is not the counterweight to a front lever day, it
is another front lever day — and a maximal set is not a counterweight either.

**Scaling up must not scale a beginner up.** The first version of this rule
prescribed deficit handstand push-ups in a plan for somebody's first pull-up,
because the band closed over nothing and the fallback handed back the first
entry of a list written hardest-first. Two guards: a level never falls below
one, so the bottom of the library stays reachable; and when nothing in a list
is at the athlete's level, the fallback sorts it easiest-first, because that is
the only safe direction to be wrong in.

### Where the one-line rule ended up

§10 filtered another skill's rungs out of a skill day's supporting slots. Two
things changed once the pools reached into the rest of the catalogue.

It now covers **the whole line rather than just the rungs** — an ice cream
maker is a front lever drill rather than a front lever, and putting one on a
planche day is exactly the mistake the rule is about — and it applies on
**every day of the week rather than only the skill days**. The argument for the
skill day was rehearsal: one line per session. The argument for the rest of the
week is the tendon budget of §7, and it is the stronger one — a back lever on
the day *after* a maltese day is loading the same tissue during the recovery
that day exists to provide.

And it narrowed in the other direction, to **straight-arm work only**. The rule
exists because one maximal static spends from the same account as another. A
hard *bent-arm* movement that happens to sit on somebody else's ladder is not
that: a typewriter pull-up is the antagonist a maltese day wants, and refusing
it because a one-arm pull-up is climbed through one leaves the counterweight to
a heavy push day as a band face pull.

---

## 12. The pull side of the same tier

§7 opened the tier above the full planche and §9 rebuilt its maltese on the
floor. Both were push. Counted afterwards, every skill the app rated as costing
three units of the tendon budget — the group the baseline page draws as
"Maximal" — was a pushing skill: maltese, iron cross, planche press. Nothing
you hang from was in it.

That was not a judgement about what counts as maximal. It was the extent of the
catalogue, and it had two consequences worth naming. The tendon budget
under-counted every athlete training a maximal pull, because the skill they
were training was either absent or rated at one unit. And the focus dial of §10
could not ration a week around a skill it did not know about.

Four skills fill the gap. Three of them are the front lever's directions out —
open the arms on a bar, open them on rings, or take one hand off — and the
fourth is the one skill at this end of the catalogue that is a pull rather than
a hold.

### The SAT is to the front lever what the maltese is to the planche

The **SAT** is the front lever with the arms opened to the maximum, held on a
straight bar — the bar's answer to the rings **Victorian**
([Calisteniapp's statics diagram](https://calisteniapp.com/articles/calisthenics-statics),
[GorNation, straight bar victorian](https://www.gornation.com/blogs/calisthenics-exercises/straight-bar-victorian),
[GorNation, victorian](https://www.gornation.com/blogs/calisthenics-exercises/victorian)).
The relationship is exactly the one §9 described for the maltese: the same
skill, with the hands travelling outward. So it gets the same shape of ladder,
including the rung that tradition runs through — a **wide-grip front lever**,
which is to the SAT what the wide planche is to the maltese, and which no
published progression seems to name.

The prerequisite the sources give is unusually emphatic, and it is not "a front
lever". It is a front lever that is **owned**: a long clean hold, front lever
pull-ups, and a front lever touch. That is the §7 pattern again — the
prerequisite does not stop mattering once you are past it — so the front lever
is what the first rung of this ladder *is*, the gates above it ask for twelve
then twenty seconds, and the tuck SAT is gated on a front lever touch rather
than on more seconds of the same hold.

The ladder: held front lever → **box victorian** (forearms on a box or a low
bar, the earliest honest loading) → **wide-grip front lever** → **tuck SAT** →
**advanced tuck SAT** → **straddle SAT** → **SAT**, with a band-assisted SAT as
the assist beside the top three and the assistance coming down between blocks
rather than inside a session.

There is deliberately no entry gate on the goal itself. §9 settled that
question: gates belong on rungs. Here it falls out for free, because the first
rung *is* the front lever — an athlete who does not have one is placed there
and trains it, and nothing has to refuse them.

**The wrist.** The SAT rests the forearms and wrists on a straight bar and then
loads them with a horizontal body. It is not a planche and nothing goes through
the palm, but the wrist is held in extension against the bar for the length of
every set, which is the mechanism §8 is about. So the SAT rungs are on the
wrist-loading list, and — unlike most of the maltese ladder — they have a real
neutral-wrist substitution: the rings victorian, where the ring turns with the
forearm instead of the forearm being pressed against a bar. It is a harder
skill, not a consolation; it simply asks nothing of the wrist.

### The one-arm front lever is the other direction out

The **one-arm front lever** is the same hands, one of them. The sources agree
on the shape — a strong full front lever first, then assisted work, then the
one-arm progressions, with bending the working elbow as the regression inside a
rung rather than a rung of its own
([Street Workout wiki](https://street-workout.fandom.com/wiki/One_arm_front_lever),
[GorNation tutorial](https://www.gornation.com/blogs/calisthenics-exercises/one-arm-front-lever),
[Calisthenics 101](https://www.calisthenics-101.co.uk/how-to-front-lever)) — and
on the failure mode: rushing it from a lever that is reached rather than held.

The ladder follows the ordinary lever progression, because shortening a lever
looks the same whichever arm is holding it: held front lever → **assisted one
arm** (second hand on a band, a strap or a lower grip) → **one-arm tuck** →
**one-arm advanced tuck** → **one-arm straddle** → **one-arm front lever**.

Two things are specific to it and are gates rather than advice. **Grip fails
first** for most people on this ladder, and a hand that is slipping is a
shoulder taking a jerk — so the one-arm tuck is gated on a twenty-second
one-arm dead hang, which is the cheapest available test of whether the grip
will hold. And **one arm carries what two were carrying**, so the straddle is
gated on a twenty-five-second two-arm lever: the two-arm hold is the only
honest measure of whether there is enough there to halve.

The other risk has no gate because no number expresses it: this is the most
asymmetric loading in the sport, the shoulder is resisting rotation as well as
holding a lever, and a side left two rungs behind is the side that gets hurt.
That one is in the plan's own risk list, in those words.

### Both of them keep the front lever in the week

Both declare the front lever as what feeds them, so it holds its maintenance
slot for the length of the plan rather than being dropped for the new skill —
§7's rule, and the reason the maltese ladder keeps the planche. At three units
each they also mean what they say to the budget of §7: two of these at once is
six units against a ceiling of five, and the planner will name one to park.

### A gate nobody is asked about is a gate nobody can pass

Adding these exposed a fault that was already there. The baseline form derives
its questions from the ladder's *rungs*, so a rung gated on something off the
ladder — the maltese on a full planche, the SAT on a front lever touch — asked
the athlete for nothing, read "nothing logged or declared" for ever, and held
them a rung below where they actually were. The form now asks for every gate
and every entry standard as well, and a test holds it there.

### The victorian is the SAT on rings

They are one skill on two implements — the front lever with the arms opened to
the maximum — and the sources treat them that way, giving the same prerequisite
for both: a front lever that is *owned*, meaning the long hold, the pull-ups
and the touch, not a personal best
([GorNation, victorian](https://www.gornation.com/blogs/calisthenics-exercises/victorian),
[Calisthenics world](https://calisthenicsworld.org/victorian/)).

The rings ladder differs from the bar one in one rung and one risk. It runs
**tuck (20 s) → one leg (10 s) → straddle → full**, banded where the athlete
needs it — the one-leg hold is a rung the bar version has no equivalent for,
the same way the wide-grip front lever is a rung the rings version does not
need. And the wrist question disappears, because the ring turns with the
forearm instead of the forearm being pressed against a bar. That is exactly why
the victorian is what §8's substitution logic trains the SAT as, rung for rung.

Rings punish what a bar forgives, which is the compensating cost: the hold is
unstable in every direction, so a rung is held clean or it is not held.

### The hefesto is the one that is a pull

Every other skill at this end of the catalogue is a hold. The hefesto is a
pull, and it is pulled from the worst position the shoulder has: from a german
hang, arms behind the body, over the bar, finishing in a korean dip
([Calisthenics world](https://calisthenicsworld.org/hefesto/),
[Calisteniapp tutorial](https://calisteniapp.com/articles/hefesto-tutorial),
[BARSTARZZ](https://barstarzz.com/hefesto-backwards-muscle-up-tutorial/)).
The cue the sources agree on is **pull to the armpits, not to the lower back**.

Its ladder is therefore mostly other people's skills, and that is not a
shortcut — it is what the movement is assembled from:

> german hang → back lever → korean dip → hefesto negatives → band-assisted →
> tuck hefesto → hefesto

Two of those are gates with numbers behind them. The korean dip asks for **15
strict dips and a 30-second german hang**, which is the base the sources put
under it, and below that you are asking an unprepared shoulder to work at the
end of its range
([More Than Lifting](https://morethanlifting.com/how-to-do-korean-dips/),
[Caliverse](https://www.caliverse.app/exercises/korean-dips-97)). The negatives
ask for the korean dip itself, because a negative is lowered out of a position
you have to be able to get into — and the sources are unanimous that the
eccentric is where this is actually earned and that skipping it is what hurts
people.

The shoulder position has **a hard anatomical limit at roughly ninety degrees**
of extension. That is not a range to train into; sharp pain there is a stop.
And the korean dip and everything built on it finish with bodyweight on the
palms with the hands behind the body, which is the wrist at the far end of
extension carrying the lot — so those rungs are wrist-loading and, unlike the
SAT, they have no neutral-wrist form at all. The rings do not save this one.

### Two faults the hefesto exposed

**A rung an injury takes away is not the whole ladder.** The planner used to
place an athlete and then discover, block by block, that the rung's movement
was banned — at which point the session stopped being about the skill and said
so. That is right when the whole ladder is gone and wrong when it is not, and
the hefesto makes the difference impossible to miss: its first two rungs are a
german hang and a back lever, neither of which touches a wrist, while
everything above them is wrist-loading. A sore wrist should cost that athlete
the top of the ladder, not the skill. Placement now steps down to the highest
rung that survives the filters, and says which rung it came from and why.

**An opener is a rung, and rungs have gates.** The session's first block is two
or three sets of the rung *above* the one being trained, attempted fresh. For
the hefesto that meant somebody with no shoulder-extension tolerance at all
being shown a back lever. The rung above now carries its own gate — twenty
seconds of german hang — and the opener is simply not written when it is unmet,
which is the machinery that already existed being pointed at the right rung.

---

## 13. What this means for an algorithm

Rules the generator implements directly, each traceable to a section above:

1. Place the athlete on the ladder from **logged records against numeric
   standards**, never from self-assessment (§1).
2. Prescribe statics at **50–70% of best hold, 3–6 sets, 3–15 s, 2–3 min rest**;
   never to failure (§1).
3. Prescribe reps with **RIR**, and weighted work from an **e1RM that includes
   bodyweight** (§1, §2).
4. Skills and dynamics go **early in the session, low volume, fresh** (§2, §3).
5. **48 h between hard sessions of the same pattern**; alternate patterns when
   training days exceed patterns (§2).
6. Keep weekly hard sets per pattern **between 6 and 20**, scaled by the
   athlete's recent logged frequency, not by ambition (§2).
7. **Deload every 4–6 weeks** at roughly half volume with intensity kept (§2).
8. **Joint preparation in every session that contains straight-arm work** (§3).
9. **Open injuries are a hard filter** on the movement list, and the plan says
   what it removed (§3).
10. The plan **ends in a test** stated as pass or fail (§2).
11. Prescribe only what the athlete has the equipment to perform, and say what
    was left out for want of kit (§6).
12. Take a stated figure when there is no logged one, mark every prescription
    built on it as stated, and let a logged set outrank it (§6).
13. **Gate the elite skills on demonstrated prerequisites**, and when they are
    not met, train the gaps instead of the skill and say which numbers are
    missing (§7).
14. **Keep the skills a goal is built on in the week** at maintenance volume,
    rather than dropping them for the new one (§7).
15. **Count every maximal skill against one tendon budget**, and name what to
    park when the total exceeds what one athlete recovers from (§7).
16. **Cap the first exposures to any movement with no history**, however strong
    the athlete is elsewhere, and treat the first session as finding the number
    (§7).
17. For a **mild wrist**, change the position rather than the volume: swap onto
    the neutral-wrist version where one exists, and remove only what has none
    (§8).
18. **Rest maximal straight-arm work for 4 to 7 minutes**, at every level, and
    the accessories for two (§9).
19. **Prescribe ranges, not point values**, snapped onto the bands the coaching
    material actually uses (§9).
20. **Open a straight-arm session with the rung above**, two or three sets,
    before anything has tired (§9).
21. Anything it cannot do — diagnose, prescribe a diet, promise a full planche
    in eight weeks — it declines to do, in writing (§1, §3, §5).
22. **Ask how much of the week the skill gets**, in three steps, and hold the
    week to the answer — measured as the goal's own ladder work against the
    week's working sets, warm-ups excluded (§10).
23. **No skill takes more than 40% of a training week**, whatever was asked
    for. The specialists' own programmes put one maximal skill at one day in
    six, and a tendon's adaptation signal saturates after about ten minutes of
    loading, so the way past the ceiling is another session on another day and
    never a longer one (§10).
24. **A skill day carries one line.** The work beside the main hold is the rung
    above it, the rung below it, or a drill for it. Another skill's rung is not
    accessory work there, and is filtered out of the supporting slots (§10).
25. **The session is a span of the ladder, and the focus decides how wide.**
    One rung at the bottom of the dial; the rung above as the opener at the
    middle; the rung below as the volume block at the top — which is where
    normalisation happens and the block a plan of maximal work alone lacks
    (§9, §10).
26. **Focus costs tendon budget.** A week built around a maximal static claims
    a unit more than one that merely contains it, so the other skills give way
    sooner — the sets land on the same tissue either way (§7, §10).
27. **Maintenance of the skills a goal is built on is not charged to the goal.**
    Holding a planche while chasing a maltese costs two or three sets a week,
    not a second programme (§7, §10).
28. **Choose accessories from the athlete's own level, per pattern**, as a band
    around the hardest thing they have on record rather than from a fixed list
    or a tier keyed on one movement (§11).
29. **Cap a rep count by the movement**, never by the slot that asked for it
    (§11).
30. **The antagonist is one step easier than the athlete's ceiling and never
    the goal's own work**, because a block that trains the same thing balances
    nothing (§11).
31. **Ask the athlete for every gate and entry standard**, not only for the
    rungs: a gate nobody is asked about is a gate nobody can pass (§12).
32. **Keep both sides of the sport at the top of the catalogue.** A "maximal"
    tier made only of pushing skills under-counts the tendon budget of every
    athlete training a maximal pull, and gives the focus dial nothing to
    ration (§12).
33. **Step down the ladder when an injury takes a rung**, to the highest rung
    that survives the filters, and say which one it came from. Losing the skill
    is the answer only when every rung is gone (§12).
