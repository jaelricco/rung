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

## Getting it to TestFlight

Everything below needs a Mac and the Apple Developer account, so none of it
was done here.

1. **Developer Program** — the bundle ID `fit.rung.app` has to be registered
   under your team, or changed in `project.yml` to one that is.
2. **App Store Connect** — create the app record: name, primary language,
   bundle ID, SKU.
3. **Signing** — Xcode's automatic signing is enough for TestFlight. The
   first archive will create the distribution certificate and provisioning
   profile.
4. **Archive** — `Product ▸ Archive` with the scheme on *Any iOS Device*,
   then *Distribute App ▸ TestFlight & App Store*.
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

Steps 1 to 4 are roughly an hour once the code compiles. Step 7 is the one
that stalls submissions, so it is worth filling in first.

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
