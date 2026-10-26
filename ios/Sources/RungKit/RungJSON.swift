import Foundation

/// The one encoder and the one decoder the whole app uses.
///
/// The date handling is the reason this is not a one-liner. Go's `time.Time`
/// marshals to RFC 3339 and keeps whatever precision the value had, so the
/// same API answers `2026-10-26T21:47:37.792351Z` for a workout written by
/// the database and `2026-10-26T00:00:00Z` for a date somebody typed.
/// Foundation's `.iso8601` strategy parses the second and rejects the first,
/// which fails on the common case and passes on the rare one — the worst way
/// round for something that only shows up once there is real data in it.
public enum RungJSON {
    public static let decoder: JSONDecoder = {
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .custom { decoder in
            let raw = try decoder.singleValueContainer().decode(String.self)
            guard let date = parseTimestamp(raw) else {
                throw DecodingError.dataCorrupted(
                    .init(
                        codingPath: decoder.codingPath,
                        debugDescription: "expected an RFC 3339 timestamp, got \(raw)"
                    ))
            }
            return date
        }
        return decoder
    }()

    public static let encoder: JSONEncoder = {
        let encoder = JSONEncoder()
        // Whole seconds on the way out. Go parses RFC 3339 either way, and a
        // timestamp an athlete chose has no business carrying microseconds.
        encoder.dateEncodingStrategy = .custom { date, encoder in
            var container = encoder.singleValueContainer()
            try container.encode(plain.format(date))
        }
        return encoder
    }()

    // Value types rather than ISO8601DateFormatter, which is a class and not
    // Sendable, so a shared instance of it is a concurrency error under Swift
    // 6 and a data race under any compiler that let it through.
    private static let fractional = Date.ISO8601FormatStyle(includingFractionalSeconds: true)
    private static let plain = Date.ISO8601FormatStyle()

    /// Both shapes, fractional first because that is what the database writes.
    public static func parseTimestamp(_ raw: String) -> Date? {
        if let date = try? fractional.parse(raw) { return date }
        return try? plain.parse(raw)
    }

    /// A `YYYY-MM-DD` day, which is what a session is scheduled on. Kept as a
    /// string on the wire deliberately — a session belongs to a day, not to an
    /// instant, and turning it into one would drag a time zone in.
    public static func parseDay(_ raw: String) -> DateComponents? {
        let parts = raw.split(separator: "-")
        guard parts.count == 3,
            let year = Int(parts[0]), let month = Int(parts[1]), let day = Int(parts[2]),
            (1...12).contains(month), (1...31).contains(day)
        else { return nil }
        return DateComponents(year: year, month: month, day: day)
    }

    /// The inverse, for asking the calendar for a window.
    public static func day(from date: Date, calendar: Calendar = .current) -> String {
        let c = calendar.dateComponents([.year, .month, .day], from: date)
        return String(format: "%04d-%02d-%02d", c.year ?? 0, c.month ?? 0, c.day ?? 0)
    }
}
