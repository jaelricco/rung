# Rung for iOS

A native SwiftUI client against the existing Go API. No new backend: it signs
in, reads and writes through `https://rung.fit/api/v1`, and the web app stays
exactly as it is.

## What is here

```
ios/
  Package.swift          RungKit — models, wire format, API client
  Sources/RungKit/       no SwiftUI, so it builds and tests on Linux
  Tests/RungKitTests/    16 tests, decoding real captured API responses
  App/                   the SwiftUI app
  project.yml            XcodeGen input; generates Rung.xcodeproj
```

The split is deliberate. Everything that can be checked without a Mac lives in
`RungKit`, and it is checked:

```
swift build && swift test     # 16 tests, runs on Linux or macOS
```

`App/` is SwiftUI and needs Xcode. **It has never been compiled.** It was
written against a verified `RungKit` and the real API, but the first `⌘B` on a
Mac should be expected to turn up the usual crop of signature and
availability mistakes. Treat it as a reviewed draft, not as working code.

## Building it

```bash
brew install xcodegen
cd ios
xcodegen generate
open Rung.xcodeproj
```

Then set the signing team in **Signing & Capabilities** — the project file
deliberately carries no team ID.

## What the app covers

Three tabs, chosen as the part of Rung that belongs on a phone in a gym:

- **Standing** — level per category, the last 28 days, records.
- **Log** — the reason for the app. Two things the web form does not do:
  the exercise picker is *searchable* (179 movements is a search, not a
  scroll), and a session carries a **date**, so last night's session logged
  this morning lands on the right day. `performed_at` has always been
  accepted by `POST /workouts`; the browser has simply never sent it.
- **Calendar** — scheduled sessions, and a session in the six parts it is
  performed in, with a tick per block and per warm-up protocol.

Planning, the baseline form, routines and events stay on the web. They are
long forms you fill in once at a desk, and they would be worse on a phone.

## Known gaps

- The SwiftUI layer is uncompiled, as above.
- No offline queue. A set logged without signal fails with the server's own
  message rather than waiting — the obvious next thing, and the one that
  needs most care, since a silently-queued set that never sends is worse
  than one that visibly failed.
- No Keychain, by design: the session is an HttpOnly cookie that
  `URLSession` holds, the same one the browser gets. There is no token to
  store, and sign-out is a request rather than a deletion.
- `Mark done` does not log a workout, exactly as on the web. The API takes a
  `workout_id` on `POST /sessions/{id}/complete` and `RungAPI.markDone`
  passes it through, so joining the two is a UI decision that has not been
  made rather than a missing endpoint.

## CI

`.github/workflows/ios.yml` builds this on the Mac registered as a runner for
the repository, on every change under `ios/`. It runs the RungKit tests
first, on their own, so a failure reads as "the contract moved" rather than
being buried in xcodebuild output — then generates the project and compiles
the app unsigned.

It needs two things installed on that machine:

```bash
brew install xcodegen
xcode-select -p          # a full Xcode, not just the command line tools
```

The job targets `[self-hosted, macOS]`. Those labels are assigned by GitHub
to every self-hosted runner, so no custom label has to be kept in sync. It
refuses to run for pull requests from forks: a self-hosted runner is a real
machine with a real keychain, and fork code has no business on it.

## Getting it to TestFlight

The upload is the second job in the same workflow, run by hand:
**Actions ▸ iOS ▸ Run workflow**, with *Archive and upload to TestFlight*
ticked. It skips with a notice until these four repository secrets exist,
the same way the deploy job skips without `DEPLOY_HOST`:

| Secret | Where it comes from |
| --- | --- |
| `APPLE_TEAM_ID` | Apple Developer ▸ Membership — ten characters |
| `APPSTORE_KEY_ID` | App Store Connect ▸ Users and Access ▸ Integrations ▸ App Store Connect API, the key's ID |
| `APPSTORE_ISSUER_ID` | the same page, above the key list — one per account |
| `APPSTORE_PRIVATE_KEY` | the `.p8` that page downloads **once**, pasted whole, `BEGIN`/`END` lines included |

Give the API key the **App Manager** role; Developer cannot upload builds.

The build number comes from the workflow run number, so it rises on its own
and nothing has to be committed or remembered — App Store Connect rejects a
build whose number it has seen before.

The steps below still have to be done by hand, once, before the first
upload can succeed.

1. **Developer Program** — register the bundle ID `fit.rung.app` under your
   team, or change it in `project.yml` to one that is.
2. **App Store Connect** — create the app record: name, primary language,
   bundle ID, SKU.
3. **Signing** — the workflow archives with `-allowProvisioningUpdates`, so
   the first run creates the distribution certificate and profile itself.
   That needs the Mac to be signed into the Apple ID once, in Xcode ▸
   Settings ▸ Accounts.
4. **Archive and upload** — the workflow, or `Product ▸ Archive` in Xcode if
   you would rather watch it the first time.
5. **Export compliance** — the app makes HTTPS calls and nothing else, so
   the answer is the standard exemption. Setting
   `ITSAppUsesNonExemptEncryption = NO` in the Info.plist stops it being
   asked on every build.
6. **Test information** — internal testing needs none; external testing
   needs a description, contact email, and a Beta App Review pass.
7. **Privacy** — the app collects an email address and training data, both
   linked to the user. That has to be declared on the app's privacy page,
   and App Store Connect will not let the build out to external testers
   until it is.

Steps 1 to 3 are roughly an hour. Step 7 is the one that stalls
submissions, so it is worth filling in first.

## A note on the API

`RungKit` is tested against JSON captured from a running server, in
`Tests/RungKitTests/Fixtures/`. That is what makes the contract real rather
than assumed: it caught, for instance, that `performed_at` carries
microseconds, which Foundation's `.iso8601` strategy silently refuses.

If the API changes shape, recapture the fixtures and the tests will say what
broke:

```bash
curl -sS -b cookies.txt https://rung.fit/api/v1/level > Tests/RungKitTests/Fixtures/level.json
```
