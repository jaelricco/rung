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
[Teambuildr](https://blog.teambuildr.com/understanding-and-implementing-the-ramp-protocol)) —
which maps onto the app's protocol library: general raise, then the region
protocol for the day, then ramp-up sets of the first main movement.

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

## 6. What this means for an algorithm

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
11. Anything it cannot do — diagnose, prescribe a diet, promise a full planche
    in eight weeks — it declines to do, in writing (§1, §3, §5).
