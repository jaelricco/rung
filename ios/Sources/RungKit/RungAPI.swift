import Foundation

#if canImport(FoundationNetworking)
    import FoundationNetworking
#endif

/// Everything the app asks the server for.
///
/// The session is an HttpOnly cookie, exactly as it is in the browser, so
/// there is no token to hold: `URLSession` keeps the cookie and sends it back.
/// That also means signing out is a request rather than forgetting a value.
public actor RungAPI {
    public let baseURL: URL
    private let session: URLSession

    public init(baseURL: URL, session: URLSession = .shared) {
        self.baseURL = baseURL
        self.session = session
    }

    /// `https://rung.fit`, the deployed server.
    public static func live() -> RungAPI {
        RungAPI(baseURL: URL(string: "https://rung.fit")!)
    }

    // MARK: - Who you are

    public func register(email: String, password: String, displayName: String) async throws -> User
    {
        try await send(
            .post, "/auth/register",
            body: ["email": email, "password": password, "display_name": displayName])
    }

    public func logIn(email: String, password: String) async throws -> User {
        try await send(.post, "/auth/login", body: ["email": email, "password": password])
    }

    public func logOut() async throws {
        _ = try await sendDiscardingBody(.post, "/auth/logout", body: [String: String]())
    }

    /// Answers 401 when nobody is signed in, which is how the app decides
    /// whether to open on the log or on the sign-in screen.
    public func me() async throws -> User {
        try await send(.get, "/me")
    }

    // MARK: - Training

    public func exercises() async throws -> [Exercise] {
        try await send(.get, "/exercises")
    }

    public func level() async throws -> Level {
        try await send(.get, "/level")
    }

    public func workouts(limit: Int = 20) async throws -> [Workout] {
        try await send(.get, "/workouts?limit=\(limit)")
    }

    @discardableResult
    public func log(_ workout: WorkoutInput) async throws -> Workout {
        try await send(.post, "/workouts", body: workout)
    }

    // MARK: - The calendar

    public func calendar(from: String, to: String) async throws -> CalendarResponse {
        try await send(.get, "/calendar?from=\(from)&to=\(to)")
    }

    public func session(id: String) async throws -> SessionDetail {
        try await send(.get, "/sessions/\(id)")
    }

    /// How far through a session the athlete has got. The server answers with
    /// the entry it stored, having dropped any tick against a protocol or a
    /// block this session does not contain — so what comes back is the truth
    /// and what was sent is only a request.
    @discardableResult
    public func saveProgress(sessionId: String, _ ticks: TickState) async throws -> CalendarEntry {
        try await send(.put, "/sessions/\(sessionId)/progress", body: ticks)
    }

    /// Marking a session done does not log anything — the sets are a separate
    /// write, the same way they are on the web. `workoutId` is what joins the
    /// two, and the API has always accepted it.
    public func markDone(sessionId: String, workoutId: String? = nil) async throws {
        struct Body: Encodable {
            let workoutId: String?
            enum CodingKeys: String, CodingKey { case workoutId = "workout_id" }
        }
        _ = try await sendDiscardingBody(
            .post, "/sessions/\(sessionId)/complete", body: Body(workoutId: workoutId))
    }

    public func markNotDone(sessionId: String) async throws {
        _ = try await sendDiscardingBody(
            .delete, "/sessions/\(sessionId)/complete", body: nil as String?)
    }

    // MARK: - The one place a request is actually made

    enum Method: String {
        case get = "GET"
        case post = "POST"
        case put = "PUT"
        case delete = "DELETE"
    }

    private func send<Response: Decodable>(
        _ method: Method, _ path: String, body: (some Encodable)? = nil as String?
    ) async throws -> Response {
        let data = try await perform(method, path, body: body)
        do {
            return try RungJSON.decoder.decode(Response.self, from: data)
        } catch {
            throw RungError.undecodable(String(describing: error))
        }
    }

    private func sendDiscardingBody(
        _ method: Method, _ path: String, body: (some Encodable)?
    ) async throws -> Data {
        try await perform(method, path, body: body)
    }

    private func perform(
        _ method: Method, _ path: String, body: (some Encodable)?
    ) async throws -> Data {
        guard let url = URL(string: "/api/v1" + path, relativeTo: baseURL) else {
            throw RungError.transport("could not build a URL for \(path)")
        }
        var request = URLRequest(url: url)
        request.httpMethod = method.rawValue
        if let body {
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
            request.httpBody = try RungJSON.encoder.encode(body)
        }

        let data: Data
        let response: URLResponse
        do {
            (data, response) = try await session.data(for: request)
        } catch {
            throw RungError.transport(error.localizedDescription)
        }

        guard let http = response as? HTTPURLResponse else {
            throw RungError.transport("the response was not HTTP")
        }
        guard (200..<300).contains(http.statusCode) else {
            if http.statusCode == 401 { throw RungError.notAuthenticated }
            // The server explains itself in words meant for the athlete.
            // Prefer those over anything this layer could invent.
            let message =
                (try? RungJSON.decoder.decode(APIErrorBody.self, from: data))?.error
                ?? "The request failed (\(http.statusCode))."
            throw RungError.api(status: http.statusCode, message: message)
        }
        return data
    }
}
