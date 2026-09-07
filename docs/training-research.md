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

## 10. What this means for an algorithm

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
