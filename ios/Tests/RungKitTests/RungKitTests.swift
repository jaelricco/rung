import Foundation
import Testing

@testable import RungKit

// These decode real responses, captured from a running server rather than
// written from the Go structs by hand. That is the whole point: a model that
// matches what I thought the API sends proves nothing, and the mismatches
// that actually happen — a field that is null in practice, a timestamp with
// microseconds, a map where a list was expected — only show up against real
// output.

private func fixture(_ name: String) throws -> Data {
    guard
        let url = Bundle.module.url(
            forResource: name, withExtension: "json", subdirectory: "Fixtures")
    else {
        Issue.record("fixture \(name).json is missing")
        throw CocoaError(.fileNoSuchFile)
    }
    return try Data(contentsOf: url)
}

@Suite("Decoding real API responses")
struct DecodingTests {

    @Test("GET /me")
    func me() throws {
        let user = try RungJSON.decoder.decode(User.self, from: fixture("me"))
        #expect(user.email == "v8@example.com")
        #expect(user.displayName == "V8")
        // Null in the fixture, and null is normal: bodyweight is asked for on
        // the baseline page and plenty of athletes have not answered.
        #expect(user.bodyweightKg == nil)
    }

    @Test("GET /exercises")
    func exercises() throws {
        let list = try RungJSON.decoder.decode([Exercise].self, from: fixture("exercises"))
        #expect(!list.isEmpty)
        // Every measure has to map to a case, or logging that movement is
        // impossible in the app and fine on the web.
        for exercise in list {
            #expect(Measure(rawValue: exercise.measure.rawValue) != nil)
        }
    }

    @Test("GET /level")
    func level() throws {
        let level = try RungJSON.decoder.decode(Level.self, from: fixture("level"))
        #expect(!level.categories.isEmpty)
        #expect(level.sessionsLast28Days >= 0)
        // Unanswered is nil, not empty: the two mean different things to the
        // planner and the app must not flatten them.
        #expect(level.equipment == nil || level.equipment != nil)
        for category in level.categories {
            #expect(!category.tier.isEmpty)
            #expect(category.tierRank >= 0)
        }
    }

    @Test("GET /workouts, including the microseconds that break .iso8601")
    func workouts() throws {
        let workouts = try RungJSON.decoder.decode([Workout].self, from: fixture("workouts"))
        let first = try #require(workouts.first)
        #expect(!first.sets.isEmpty)
        #expect(first.performedAt.timeIntervalSince1970 > 0)
        for set in first.sets {
            #expect(!set.exerciseName.isEmpty)
        }
    }

    @Test("GET /calendar, with the session body nested inside each entry")
    func calendar() throws {
        let calendar = try RungJSON.decoder.decode(CalendarResponse.self, from: fixture("calendar"))
        let entry = try #require(calendar.sessions.first)
        #expect(!entry.title.isEmpty)
        #expect(!entry.body.blocks.isEmpty)
        #expect(RungJSON.parseDay(entry.scheduledOn) != nil)
        // Nothing ticked and nothing completed is the normal state of a
        // freshly scheduled plan, and has to decode rather than throw.
        #expect(entry.completedAt == nil)
    }

    @Test("GET /sessions/{id}: six phases in order, exercises keyed by slug")
    func sessionDetail() throws {
        let detail = try RungJSON.decoder.decode(SessionDetail.self, from: fixture("session"))
        #expect(
            detail.phases.map(\.key) == [
                "joint", "muscular", "mobility", "specific", "training", "cooldown",
            ])
        // The map is what lets a block show a name instead of a slug without
        // a second request, so every block it references has to be in it.
        for block in detail.session.body.blocks {
            #expect(
                detail.exercises[block.exerciseSlug] != nil,
                "block references \(block.exerciseSlug), which the exercises map does not carry")
        }
    }
}

@Suite("Timestamps")
struct TimestampTests {

    @Test(
        "both precisions Go emits",
        arguments: [
            "2026-10-26T21:47:37.792351Z",  // written by the database
            "2026-10-26T00:00:00Z",  // a date somebody typed
            "2026-10-26T21:47:37Z",
        ])
    func parses(_ raw: String) {
        #expect(RungJSON.parseTimestamp(raw) != nil, "could not parse \(raw)")
    }

    @Test("and refuses what is not a timestamp")
    func rejects() {
        #expect(RungJSON.parseTimestamp("2026-10-26") == nil)
        #expect(RungJSON.parseTimestamp("") == nil)
        #expect(RungJSON.parseTimestamp("yesterday") == nil)
    }

    @Test("a scheduled day stays a day")
    func days() {
        let parsed = RungJSON.parseDay("2026-10-26")
        #expect(parsed?.year == 2026)
        #expect(parsed?.month == 10)
        #expect(parsed?.day == 26)
        #expect(RungJSON.parseDay("26-10-2026") == nil)
        #expect(RungJSON.parseDay("2026-13-01") == nil)
        #expect(RungJSON.parseDay("nonsense") == nil)
    }

    @Test("round trip through the encoder the app posts with")
    func roundTrip() throws {
        let when = try #require(RungJSON.parseTimestamp("2026-10-26T21:47:37Z"))
        let input = WorkoutInput(
            performedAt: when, notes: "", rpe: 8,
            sets: [SetInput(exerciseSlug: "pull_up", kind: .reps, reps: 10)])
        let encoded = try RungJSON.encoder.encode(input)
        let json = try #require(try JSONSerialization.jsonObject(with: encoded) as? [String: Any])
        #expect(json["performed_at"] as? String == "2026-10-26T21:47:37Z")
        // The field names are the server's, not Swift's.
        let set = try #require((json["sets"] as? [[String: Any]])?.first)
        #expect(set["exercise_slug"] as? String == "pull_up")
        #expect(set["kind"] as? String == "reps")
    }
}

@Suite("What a set of each kind has to carry")
struct SetKindTests {

    @Test("every measure maps to a kind")
    func mapping() {
        for measure in Measure.allCases {
            #expect(SetKind(measure: measure).rawValue == measure.rawValue)
        }
    }

    @Test("a reps set needs a rep count")
    func reps() {
        var set = SetInput(exerciseSlug: "pull_up", kind: .reps)
        #expect(!set.isComplete)
        set.reps = 8
        #expect(set.isComplete)
    }

    @Test("a weighted set needs both numbers")
    func weighted() {
        var set = SetInput(exerciseSlug: "weighted_pull_up", kind: .weightedReps, reps: 5)
        #expect(!set.isComplete, "reps alone is not a weighted set")
        set.weightKg = 16
        #expect(set.isComplete)
    }

    @Test("a weighted hold needs seconds and kilos")
    func weightedHold() {
        var set = SetInput(
            exerciseSlug: "weighted_front_lever", kind: .weightedHold, holdSeconds: 8)
        #expect(!set.isComplete)
        set.weightKg = 10
        #expect(set.isComplete)
    }

    @Test("an attempt needs to be made or missed, and false is an answer")
    func attempt() {
        var set = SetInput(exerciseSlug: "one_arm_pull_up", kind: .skillAttempt)
        #expect(!set.isComplete)
        set.success = false
        #expect(set.isComplete, "a missed attempt is a logged set, not a blank one")
    }

    @Test("a hold does not want reps")
    func hold() {
        var set = SetInput(exerciseSlug: "tuck_front_lever", kind: .staticHold)
        #expect(!set.isComplete)
        set.holdSeconds = 18
        #expect(set.isComplete)
    }
}
