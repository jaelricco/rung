import Foundation

// The wire types, one per JSON object the API answers with. Field names match
// the Go structs they decode, so the mapping is readable against
// backend/internal/training rather than inferred.

public struct User: Codable, Sendable, Identifiable, Equatable {
    public let id: String
    public let email: String
    public let displayName: String
    public let bodyweightKg: Double?

    enum CodingKeys: String, CodingKey {
        case id, email
        case displayName = "display_name"
        case bodyweightKg = "bodyweight_kg"
    }
}

// MARK: - The exercise library

/// How a movement is measured, which decides what a set of it has to carry.
/// The raw values are the `measure` column; `SetKind` mirrors them because the
/// API names the same distinction twice, once per direction.
public enum Measure: String, Codable, Sendable, CaseIterable {
    case reps
    case weightedReps = "weighted_reps"
    case staticHold = "static_hold"
    case weightedHold = "weighted_hold"
    case skillAttempt = "skill_attempt"
}

public struct Exercise: Codable, Sendable, Identifiable, Equatable {
    public var id: String { slug }
    public let slug: String
    public let name: String
    public let category: String
    public let measure: Measure
    public let difficulty: Int
    public let description: String
}

// MARK: - Logging

/// What a logged set is. Named `kind` on the wire in both directions; the
/// cases are the same five the database has a check constraint for.
public enum SetKind: String, Codable, Sendable {
    case reps
    case weightedReps = "weighted_reps"
    case staticHold = "static_hold"
    case weightedHold = "weighted_hold"
    case skillAttempt = "skill_attempt"

    public init(measure: Measure) {
        switch measure {
        case .reps: self = .reps
        case .weightedReps: self = .weightedReps
        case .staticHold: self = .staticHold
        case .weightedHold: self = .weightedHold
        case .skillAttempt: self = .skillAttempt
        }
    }

    /// Which fields the server will insist on. Checking it here means a set
    /// that cannot be saved is caught while it is still being typed, rather
    /// than as a 400 after the athlete has put the phone down.
    public var requiredFields: Set<SetField> {
        switch self {
        case .reps: [.reps]
        case .weightedReps: [.reps, .weight]
        case .staticHold: [.hold]
        case .weightedHold: [.hold, .weight]
        case .skillAttempt: [.success]
        }
    }
}

public enum SetField: Sendable, Hashable {
    case reps, weight, hold, success
}

public struct LoggedSet: Codable, Sendable, Identifiable, Equatable {
    public let id: String
    public let exerciseSlug: String
    public let exerciseName: String
    public let category: String
    public let kind: SetKind
    public let position: Int
    public let reps: Int?
    public let weightKg: Double?
    public let holdSeconds: Double?
    public let success: Bool?

    enum CodingKeys: String, CodingKey {
        case id, category, kind, position, reps, success
        case exerciseSlug = "exercise_slug"
        case exerciseName = "exercise_name"
        case weightKg = "weight_kg"
        case holdSeconds = "hold_seconds"
    }
}

public struct Workout: Codable, Sendable, Identifiable, Equatable {
    public let id: String
    public let performedAt: Date
    public let notes: String
    public let rpe: Int?
    public let sets: [LoggedSet]

    enum CodingKeys: String, CodingKey {
        case id, notes, rpe, sets
        case performedAt = "performed_at"
    }
}

/// One set on its way out. Only the fields the kind requires are sent: the
/// server rejects a weighted set with no weight, and sending a null where it
/// wants a number is the same thing.
public struct SetInput: Codable, Sendable, Equatable {
    public let exerciseSlug: String
    public let kind: SetKind
    public var reps: Int?
    public var weightKg: Double?
    public var holdSeconds: Double?
    public var success: Bool?

    public init(
        exerciseSlug: String, kind: SetKind, reps: Int? = nil,
        weightKg: Double? = nil, holdSeconds: Double? = nil, success: Bool? = nil
    ) {
        self.exerciseSlug = exerciseSlug
        self.kind = kind
        self.reps = reps
        self.weightKg = weightKg
        self.holdSeconds = holdSeconds
        self.success = success
    }

    enum CodingKeys: String, CodingKey {
        case kind, reps, success
        case exerciseSlug = "exercise_slug"
        case weightKg = "weight_kg"
        case holdSeconds = "hold_seconds"
    }

    /// Whether the server will accept this set, by the same rule it applies.
    public var isComplete: Bool {
        for field in kind.requiredFields {
            switch field {
            case .reps where reps == nil: return false
            case .weight where weightKg == nil: return false
            case .hold where holdSeconds == nil: return false
            case .success where success == nil: return false
            default: continue
            }
        }
        return true
    }
}

public struct WorkoutInput: Codable, Sendable, Equatable {
    /// Nil means now, which is what the web app always sends. The iOS app can
    /// fill it in, which is the one thing logging on a phone should be able to
    /// do that logging in a browser here cannot.
    public var performedAt: Date?
    public var notes: String
    public var rpe: Int?
    public var sets: [SetInput]

    public init(performedAt: Date? = nil, notes: String = "", rpe: Int? = nil, sets: [SetInput]) {
        self.performedAt = performedAt
        self.notes = notes
        self.rpe = rpe
        self.sets = sets
    }

    enum CodingKeys: String, CodingKey {
        case notes, rpe, sets
        case performedAt = "performed_at"
    }
}

// MARK: - Where you stand

public struct Record: Codable, Sendable, Identifiable, Equatable {
    public var id: String { slug }
    public let slug: String
    public let name: String
    public let category: String
    public let bestReps: Int?
    public let bestAddedKg: Double?
    public let bestHoldSeconds: Double?
    public let totalSets: Int
    public let tier: String?
    public let lastTrained: String?
    /// `logged`, `declared` or `both` — whether the figure was performed here
    /// or stated on the baseline page. Shown, because the plan quotes it.
    public let source: String

    enum CodingKeys: String, CodingKey {
        case slug, name, category, tier, source
        case bestReps = "best_reps"
        case bestAddedKg = "best_added_kg"
        case bestHoldSeconds = "best_hold_seconds"
        case totalSets = "total_sets"
        case lastTrained = "last_trained"
    }
}

public struct CategoryLevel: Codable, Sendable, Identifiable, Equatable {
    public var id: String { category }
    public let category: String
    public let tier: String
    public let tierRank: Int
    public let basedOn: String

    enum CodingKeys: String, CodingKey {
        case category, tier
        case tierRank = "tier_rank"
        case basedOn = "based_on"
    }
}

public struct Injury: Codable, Sendable, Identifiable, Equatable {
    public let id: String
    public let region: String
    public let severity: Int
    public let description: String
    public let startedOn: Date
    public let resolvedOn: Date?

    enum CodingKeys: String, CodingKey {
        case id, region, severity, description
        case startedOn = "started_on"
        case resolvedOn = "resolved_on"
    }
}

/// What `GET /level` answers: the athlete's standing, computed from logged
/// sets rather than estimated.
public struct Level: Codable, Sendable, Equatable {
    public let bodyweightKg: Double?
    public let categories: [CategoryLevel]
    public let records: [Record]
    public let sessionsLast7Days: Int
    public let sessionsLast28Days: Int
    public let setsLast28Days: Int
    public let openInjuries: [Injury]
    public let trainsPerWeek: Int?
    public let sleepHours: Double?
    /// Nil until the question has been answered; an empty array is a real
    /// answer and means floor only.
    public let equipment: [String]?
    public let learning: [String]?

    enum CodingKeys: String, CodingKey {
        case categories, records, equipment, learning
        case bodyweightKg = "bodyweight_kg"
        case sessionsLast7Days = "sessions_last_7_days"
        case sessionsLast28Days = "sessions_last_28_days"
        case setsLast28Days = "sets_last_28_days"
        case openInjuries = "open_injuries"
        case trainsPerWeek = "trains_per_week"
        case sleepHours = "sleep_hours"
    }
}

// MARK: - The calendar and a session in it

public struct SessionBlock: Codable, Sendable, Equatable {
    public let exerciseSlug: String
    public let intent: String?
    public let sets: Int
    public let prescription: String
    public let intensity: String?
    public let tempo: String?
    public let restSeconds: Int?
    public let progression: String?
    public let notes: String?

    enum CodingKeys: String, CodingKey {
        case intent, sets, prescription, intensity, tempo, progression, notes
        case exerciseSlug = "exercise_slug"
        case restSeconds = "rest_seconds"
    }
}

public struct SessionBody: Codable, Sendable, Equatable {
    public let title: String
    public let focus: String
    public let load: String?
    public let durationMinutes: Int?
    public let warmupProtocols: [String]?
    public let blocks: [SessionBlock]
    public let cooldown: String?

    enum CodingKeys: String, CodingKey {
        case title, focus, load, blocks, cooldown
        case durationMinutes = "duration_minutes"
        case warmupProtocols = "warmup_protocols"
    }
}

public struct CalendarEntry: Codable, Sendable, Identifiable, Equatable {
    public let id: String
    public let planId: String?
    public let routineId: String?
    /// `plan`, `routine` or `manual`.
    public let source: String
    /// A plain `YYYY-MM-DD`, not a timestamp: a session belongs to a day.
    public let scheduledOn: String
    public let title: String
    public let focus: String
    public let body: SessionBody
    public let completedAt: Date?
    public let workoutId: String?
    public let doneProtocols: [String]?
    public let doneBlocks: [Int]?

    public var isComplete: Bool { completedAt != nil }

    enum CodingKeys: String, CodingKey {
        case id, source, title, focus, body
        case planId = "plan_id"
        case routineId = "routine_id"
        case scheduledOn = "scheduled_on"
        case completedAt = "completed_at"
        case workoutId = "workout_id"
        case doneProtocols = "done_protocols"
        case doneBlocks = "done_blocks"
    }
}

public struct CalendarResponse: Codable, Sendable, Equatable {
    public let from: String
    public let to: String
    public let sessions: [CalendarEntry]
}

/// The six parts a session is performed in, in order. The server names them,
/// so the app does not get to disagree about the order.
public struct SessionPhase: Codable, Sendable, Identifiable, Equatable {
    public var id: String { key }
    public let key: String
    public let label: String
    public let note: String?
}

public struct Protocol_: Codable, Sendable, Identifiable, Equatable {
    public var id: String { slug }
    public let slug: String
    public let region: String
    public let title: String
    public let purpose: String
    public let phase: String?
    public let steps: [String]
    public let avoidWhile: [String]?
    public let seeClinician: String?

    enum CodingKeys: String, CodingKey {
        case slug, region, title, purpose, phase, steps
        case avoidWhile = "avoid_while"
        case seeClinician = "see_clinician"
    }
}

/// `GET /sessions/{id}` — everything the page needs in one request, because
/// the thing reading it is open in a gym.
public struct SessionDetail: Codable, Sendable, Equatable {
    public let session: CalendarEntry
    public let phases: [SessionPhase]
    public let protocols: [Protocol_]
    /// Keyed by slug, so a block's `exercise_slug` resolves to a real name
    /// without a second request.
    public let exercises: [String: Exercise]
}

/// What `POST /sessions/{id}/ticks` carries. Protocols are named by slug;
/// blocks by their position, which only means anything against the body they
/// were ticked against.
public struct TickState: Codable, Sendable, Equatable {
    public var doneProtocols: [String]
    public var doneBlocks: [Int]

    public init(doneProtocols: [String] = [], doneBlocks: [Int] = []) {
        self.doneProtocols = doneProtocols
        self.doneBlocks = doneBlocks
    }

    enum CodingKeys: String, CodingKey {
        case doneProtocols = "done_protocols"
        case doneBlocks = "done_blocks"
    }
}

// MARK: - Errors

/// What the API answers with when it refuses: `{"error": "..."}`.
struct APIErrorBody: Decodable {
    let error: String
}

public enum RungError: Error, Sendable, Equatable {
    /// The server said no, and said why in words meant for the athlete.
    case api(status: Int, message: String)
    case notAuthenticated
    case transport(String)
    case undecodable(String)

    public var message: String {
        switch self {
        case .api(_, let message): message
        case .notAuthenticated: "Sign in to see this."
        case .transport(let detail): "Could not reach the server: \(detail)"
        case .undecodable(let detail): "The server's answer could not be read: \(detail)"
        }
    }
}
