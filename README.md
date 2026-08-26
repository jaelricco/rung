# Calisthenics training app

A training log that computes your level from what you actually lift and hold, and
uses that to write plans. Go API, SvelteKit frontend, Postgres with PostGIS, all
behind Caddy on a single server.

No local development environment is needed. The server is the only place
anything runs.

---

## The server

| | |
| --- | --- |
| Type | `cx33` — 4 vCPU x86, 8 GB RAM, 80 GB |
| Location | `nbg1` (Nuremberg) or `fsn1` (Falkenstein) |
| Image | Ubuntu 24.04 |
| Backups | on |

Images are built in CI and pulled by tag, so the server never compiles
anything and 4 GB would now be enough. The 8 GB box is kept for headroom:
Postgres, the working set, and the local-build fallback in `infra/setup.sh` for
the case where the registry is unreachable.

x86 over ARM for two reasons: Hetzner's June 2026 price adjustment removed the
ARM price advantage, and `postgis/postgis` ARM64 availability is inconsistent.
On x86 that question doesn't arise.

### Setting it up

Setup is defined once, in `infra/setup.sh`. Two entry points reach it:

**Server doesn't exist yet** — edit the `/root/setup.env` values in
`infra/cloud-init.yaml` and paste the file into the **Cloud config** field on
Hetzner's creation page. The server configures itself on first boot. Note that
cloud config is read only at creation; it cannot be added to a running server,
and a rebuild replays whatever was stored at creation time.

**Server already exists** — run the bootstrap over SSH:

```bash
scp infra/bootstrap.sh root@YOUR-IP:/root/
ssh root@YOUR-IP
bash /root/bootstrap.sh https://github.com/YOU/YOUR-REPO.git training.example.com you@example.com
```

Either path ends the same way: Docker installed, `deploy` user created, ufw and
fail2ban on, SSH passwords disabled, 2 GB of swap so builds don't get OOM-killed,
repo cloned, database password generated, stack built and running, nightly
backup cron installed with 14-day retention.

Do not put your Anthropic API key in the cloud config. It stays readable from
inside the server at `169.254.169.254` for the life of the machine. The stack
runs fine without the key — coaching endpoints return 503 — and the login
banner tells you the one command to add it afterwards.

### Creating it: scripted

If you'd rather have it reproducible from your own machine:

```bash
hcloud context create calisthenics   # prompts for the token, stores it locally
bash infra/provision.sh
```

Never paste a Hetzner API token into a chat, an issue, or a commit. It can
create and destroy servers and spend money.

---

## First deployment, the manual way

Only needed if you skipped the cloud config above.

**1. Push this repo to GitHub.**

Public is fine and slightly better: the repo holds no secrets — `.env` is
generated on the server and gitignored — and a public repo means unlimited
Actions minutes and no clone credential sitting in the server's cloud config.

**2. On the fresh Hetzner box, as root:**

```bash
curl -fsSL https://raw.githubusercontent.com/YOU/YOUR-REPO/main/infra/bootstrap.sh -o bootstrap.sh
bash bootstrap.sh https://github.com/YOU/YOUR-REPO.git
```

That installs Docker, creates a `deploy` user, turns on the firewall, disables
SSH password login, clones the repo into `/srv/calisthenics`, and generates a
database password.

**3. Point your domain at the server.** An `A` record for `APP_DOMAIN` to the
server's IPv4. Caddy asks Let's Encrypt for a certificate the first time it
starts, so DNS has to resolve before step 5.

**4. Fill in the rest of `.env`:**

```bash
ssh deploy@YOUR-SERVER
nano /srv/calisthenics/.env     # APP_DOMAIN, ACME_EMAIL, ANTHROPIC_API_KEY
```

**5. Bring it up:**

```bash
cd /srv/calisthenics && make deploy
```

First build takes a few minutes. Migrations run automatically at API startup —
there is no separate migrate step.

---

## Deploying

Push to `main`. That is the whole procedure.

```
push ──▶ CI: go build/vet/test, svelte build, compose + shellcheck
          │
          ▼
     build images ──▶ ghcr.io/jaelricco/rung/{api,web}:<commit-sha>
          │
          ▼
     deploy ──▶ ssh ──▶ infra/deploy.sh <sha>
                          pull, up -d, wait for health
                          healthy ─▶ record the tag, done
                          not     ─▶ restore the previous tag, fail the run
          │
          ▼
     verify https://rung.fit/healthz from outside
```

Two properties worth stating plainly, because the previous pipeline had
neither:

**What CI proved green is exactly what ships.** The deploy job `needs:` the
build job and deploys that job's image digest. It is not a second workflow that
re-checks out the branch and hopes it is still the same commit.

**A failed deploy leaves the last good release running.** `infra/deploy.sh`
records the tag it replaced, and if the new one does not come up healthy it
puts the old one back before failing the run. The server is never left on a
broken build, and `make rollback` (or `bin/ops rollback`) undoes a bad release
that only misbehaves later.

### Driving it from anywhere

`bin/deploy` and `bin/ops` need a GitHub token and nothing else — no Docker, no
SSH key, no route to the server. They dispatch workflows and read the results
back, which means they work from a laptop, a phone tunnel, or an agent sandbox
whose only reachable host is `api.github.com`.

```bash
bin/deploy -m "add the parks map"   # commit, push, follow CI and the deploy
bin/ops status                      # containers, health, disk, live tag
bin/ops logs api 200
bin/ops restart web
bin/ops rollback
bin/ops deploy-tag 1a2b3c4d5e6f
bin/ops backup
```

The token is read from `../.secrets/deploy.env` — outside the working tree, so
no `git add -A` can ever pick it up:

```
GITHUB_TOKEN=github_pat_...
```

Server output comes back through an issue titled **ops log**, not through the
Actions log download. Actions logs are served from a storage host that
restricted networks routinely block, while the issue is plain API. It doubles
as an audit trail of every deploy and every restart.

### Repository secrets

| Secret | Value |
| --- | --- |
| `DEPLOY_HOST` | the server's IP |
| `DEPLOY_USER` | `deploy` |
| `DEPLOY_SSH_KEY` | private key whose public half is in `/home/deploy/.ssh/authorized_keys` |
| `APP_DOMAIN` | `rung.fit` — enables the external check after each deploy |

With `DEPLOY_HOST` unset the pipeline still builds and publishes images and
skips the deploy with a notice, so a fresh fork is never a wall of red runs.

Registry credentials are deliberately absent from that table. Every deploy logs
in with the run's own `GITHUB_TOKEN` and logs out afterwards, so no long-lived
registry credential sits on the server.

### On the server

Run from `/srv/calisthenics`, for when CI is unavailable or you are already
logged in:

| Command | What it does |
| --- | --- |
| `make version` | Which tag and commit are live |
| `make history` | Recent deploys, with rollbacks marked |
| `make deploy` | Pull and restart, health-checked |
| `make rollback` | Return to the previous tag |
| `make build-deploy` | Emergency: build the images here instead of pulling |
| `make logs` / `make ps` / `make health` | Follow logs, status, health |
| `make psql` / `make migrate-status` / `make backup` | Database |

`.github/dependabot.yml` watches Go modules, npm, the Dockerfiles and the
workflow actions. Since there's no local environment where you'd notice an
advisory, those PRs are how you find out.

### Why GitHub rather than GitLab

Two reasons, both about this project specifically. GitHub's free tier gives
2,000 private-repo CI minutes a month against GitLab's 400 — which matters now
that image builds run in CI at four to six minutes a run. And Dependabot covers
private repos free, where GitLab's dependency scanning is an Ultimate feature;
with no local environment, automated dependency PRs are the only thing that
will ever tell you about a CVE.

GitLab is the better answer if you later want to self-host the forge — CE is
free and its runners are unlimited — or if the source itself has to stay in the
EU.

---

## How it fits together

```
                 ┌─────────┐
    :443 ────────│  Caddy  │  TLS, one certificate per domain
                 └────┬────┘
             /api/*   │   everything else
              ┌───────┴───────┐
              ▼               ▼
        ┌──────────┐    ┌──────────┐
        │ api (Go) │    │ web (SK) │
        └────┬─────┘    └──────────┘
             ▼
      ┌─────────────┐
      │  Postgres   │  + PostGIS for park search
      └─────────────┘
```

Only Caddy publishes ports. The API and database are reachable solely on the
compose network, so nothing has to be firewalled off by hand.

### Where the design decisions live

**Level is computed, never asked of the model.** `internal/training/level.go`
holds a threshold table mapping records to tiers. Edit the numbers there and the
whole app — dashboards, prompts, plan difficulty — follows. Two athletes with
the same log always get the same level.

**The model chooses from libraries; it does not invent, and it is checked.**
Every coaching call is handed the exercise table and the protocol list in
`internal/training/injuries.go`, and is told those are the only things it may
prescribe. Then `validatePlan` in `internal/ai/plans.go` holds it to that: a
block naming a slug that does not exist is dropped, a session that loses every
block goes with it, and what was dropped is returned to the browser as a
warning. A slug the athlete cannot log is not a small error — it never reaches
the level calculation and the block is silent about what it wanted. Growing the
app's vocabulary means adding rows, not loosening the prompt, which is why the
library carries the progressions each skill is actually built from: negatives,
band-assisted work, tuck and straddle steps, lean drills and joint preparation.

**A plan starts with research, not with recall.** Before writing anything, the
coach searches for how the named skill is actually trained: the progression
ladder, the standard each rung is held to, how the week is usually laid out,
the accessory work, and what tends to go wrong. Findings come back mapped onto
library slugs, anything invented is pruned, and the pages the search really
retrieved are shown to the athlete under the plan. `skill_research` caches it
per skill for 60 days — what a front lever needs does not change between two
athletes, only what *this* athlete needs from it does, and that comes from the
snapshot. `no_research: true` skips the pass for a faster, cheaper plan.

**Open injuries are a hard filter.** They are part of the snapshot passed into
every prompt, and the model is required to say what it removed and why.

**Every model call is logged.** `ai_calls` records the prompt, the completion,
token counts and duration. When a plan comes out wrong you can see exactly what
was asked.

**Model calls are streamed, and the wait is shown as a real percentage.** Every
completion goes through `internal/ai/stream.go`, which reads the API's
server-sent events. Two things come out of that. The turn's own accounting is
visible, so a call that ends without an answer says why instead of "the model
returned an empty response" — the usual cause is a token ceiling spent on
reasoning, since current models think before they write and both come out of
the same budget, which is why `planTokens` sizes the ceiling to the plan being
asked for. And the deltas drive the progress bar: the browser sends
`Accept: text/event-stream` to `/ai/skill-plan`, `/ai/review` or `/ai/recovery`
and gets `progress` events
(`{stage, label, percent, detail, done, total, indeterminate}`) until a final
`done` event carrying the same JSON body the endpoint has always
returned. The percentage is measured work — reasoning received, then sessions
written out of the number asked for — not a timer. Any client that does not ask
for the stream gets the single JSON body, unchanged. The one phase that cannot
report a fraction of itself is the research search — a single request that says
nothing until it returns — so it sets `indeterminate` and the bar sweeps while
the elapsed time ticks, rather than inventing a number.

**Opus writes the training.** `ANTHROPIC_MODEL` defaults to `claude-opus-5`.
Plan writing is the hardest call the app makes — a ladder of progressions
weighed against one athlete's records, with an injury as a hard constraint —
and it is where the difference between model tiers shows up in the output.
Set `claude-sonnet-5` in `.env` for a cheaper, faster server; nothing else
changes.

**The week is the unit, so the week is what you see.** Both the plan preview
and the calendar lay a week out as four rows of two — Monday and Tuesday, then
a break, and so on — with rest days drawn rather than omitted, because the gaps
between hard sessions are part of the programme. The eighth square carries what
the week adds up to. `src/lib/week.js` deals the sessions into the squares;
both pages use it, so a session looks the same whether it is being considered
or being done.

**Generating a plan and committing to it are separate.** `/ai/skill-plan`
answers with a plan; `POST /plans` is what puts it on the calendar, and the
plan is re-checked against the library on the way in rather than trusted for
having been ours a minute ago. A plan is worth reading before it becomes eight
weeks of appointments.

**Event dates are verified, not trusted.** See the section below.

**Parks come from OpenStreetMap.** `internal/parks` queries the Overpass API for
`leisure=fitness_station` and related tags, caches results in PostGIS, and
serves them by distance. The map has real data on day one instead of an empty
table.

---

## How event discovery is kept honest

Web search grounds the model in real pages and, critically, returns **citations**:
every answer carries the URLs the API actually fetched. That turns the problem
from "trust the model" into "check its work", and the checking happens in Go.

A candidate event has to clear three gates before anyone sees it as fact:

1. **Its source must be a page the search really retrieved.** The model's cited
   `source_url` is looked up in the set of URLs the API returned. A URL that
   isn't in that set was invented, and the candidate is discarded outright. This
   is the single highest-value check in the pipeline — it catches plausible-looking
   URLs that don't exist.
2. **That page must still answer us.** The server fetches it directly and
   requires a 200.
3. **The date and the name must literally appear in the page text.** The date is
   matched in the many forms event pages use — `2026-09-12`, `12.09.2026`,
   `12. September 2026`, `September 12, 2026` — across English, German, French,
   Italian and Spanish month names, with the year required nearby so
   "12 September" can't match the wrong year. The name check requires a
   distinctive word, ignoring filler like "cup", "open" and "calisthenics".

The result of those gates is stored as `confidence`:

| Value | Meaning | Shown by default |
| --- | --- | --- |
| `date_confirmed` | Date and name both found on a live source page | yes |
| `human_confirmed` | Someone checked it by hand | yes |
| `source_live` | Page loads but the date or name wasn't found | review queue only |
| `rejected` | Fabricated source, dead link, or no date stated | never |

A human decision always outranks a machine check and is never downgraded by a
later re-run. Every upcoming event is re-verified weekly, because event pages
move dates and quietly disappear; a source that stops confirming drops out of
the verified list on its own.

Discovery runs are cached for 24 hours per query shape in `discovery_runs`.
Search bills at $10 per 1,000 searches on top of tokens, and a run uses up to
eight, so a hundred users browsing the same region must not trigger a hundred
runs.

### What this still can't do

- **Instagram-only events stay invisible.** A lot of smaller competitions are
  announced in a story or a post and nowhere else. Search doesn't index that
  well, and there's no page to verify against.
- **JavaScript-rendered dates fail the text check.** A site that loads its
  schedule client-side will come back as `source_live` with a note saying so.
  It's a false negative, not a wrong date, and the review queue catches it.
- **A wrong date on the organiser's own page verifies successfully.** The check
  proves the app is faithfully reporting the source, not that the source is
  right.
- **Nothing here is a substitute for the entry deadline.** Always link out.

---

## API

Public:

```
GET    /healthz
POST   /api/v1/auth/register     {email, password, display_name}
POST   /api/v1/auth/login        {email, password}
POST   /api/v1/auth/logout
GET    /api/v1/exercises
GET    /api/v1/protocols?region=wrist
GET    /api/v1/parks?lat=&lng=&radius_km=
```

Requires a session cookie:

```
GET    /api/v1/me
PATCH  /api/v1/me                {display_name?, bodyweight_kg?}
GET    /api/v1/workouts?limit=30
POST   /api/v1/workouts          {performed_at?, notes, rpe, sets[]}
DELETE /api/v1/workouts/{id}
GET    /api/v1/level
GET    /api/v1/injuries
POST   /api/v1/injuries          {region, severity, description}
POST   /api/v1/injuries/{id}/resolve
GET    /api/v1/calendar?from=&to=
GET    /api/v1/calendar.ics
POST   /api/v1/sessions/{id}/complete
DELETE /api/v1/sessions/{id}/complete
GET    /api/v1/plans
POST   /api/v1/plans            {plan, goal, starts_on}
DELETE /api/v1/plans/{id}
POST   /api/v1/ai/skill-plan     {skill, weeks, days_per_week, starts_on?, notes?,
                                  save, no_research?}
POST   /api/v1/ai/review
POST   /api/v1/ai/recovery
       ^ these three also answer with a progress stream instead of one JSON
         body when the request carries Accept: text/event-stream
POST   /api/v1/parks/refresh?lat=&lng=
GET    /api/v1/events?discipline=&country=&from=&to=&include_unconfirmed=
POST   /api/v1/events/discover        {discipline, country, from, to, force}
POST   /api/v1/events/{id}/confirm    {confirmed, note}
POST   /api/v1/events/{id}/recheck
POST   /api/v1/events/{id}/register   {goal}
DELETE /api/v1/events/{id}/register
```

A set carries one of four shapes, and the database enforces it:

| `kind` | Required fields |
| --- | --- |
| `reps` | `reps` |
| `weighted_reps` | `reps`, `weight_kg` (added load only) |
| `static_hold` | `hold_seconds` |
| `skill_attempt` | `success` |

---

## Status

Built and working:

- Accounts, sessions, argon2id passwords
- Exercise library, session logging, records, computed level tiers
- Injury tracking and the curated protocol library
- AI skill plans, training review, recovery and nutrition guidance
- Calendar storage, planned sessions, ICS export, plan deletion
- Park search backed by OpenStreetMap
- Event discovery via web search, with source verification and a review queue
- Frontend: sign in, overview, log a session, generate a plan, the calendar,
  browse events

Not built yet:

- **Frontend for parks and injuries** — the endpoints are all live. The parks
  page needs MapLibre GL, which needs no API token.
- **Event-specific plans** — `savePlan` already accepts an `event_id`; the
  prompt needs a variant that periodises backwards from a competition date.
- **Scheduled discovery** — `RunDiscovery` is ready to be called from a cron
  loop for a fixed set of regions, rather than only on user demand.
- **Password reset** — needs an email sender wired up.
- **Rate limiting on the AI endpoints** — right now a user can generate plans in
  a loop. Add a per-user cap before opening signups.

---

## Notes

- `go.sum` is committed and CI runs `go mod verify`, so a build resolves
  nothing at image-build time. Adding an import means running `go mod tidy`
  and committing the result, or CI fails.
- `frontend/` has no `package-lock.json` yet, so npm resolves the dependency
  tree on every build. Committing a lockfile would make the frontend image
  reproducible the way the backend already is.
- The API returns human-readable error strings; the frontend shows them
  verbatim, so write them as messages a user should see.
- Cookies are `Secure` by default. Only set `INSECURE_COOKIES=true` if you are
  testing over plain HTTP on an IP address.
