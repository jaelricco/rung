import RungKit
import SwiftUI

@main
struct RungApp: App {
    @State private var session = SessionStore()

    var body: some Scene {
        WindowGroup {
            RootView()
                .environment(session)
                .tint(.rungSignal)
        }
    }
}

/// Who is signed in, and the one API client everything shares.
///
/// The session is a cookie held by URLSession, so there is nothing to persist
/// here: on launch the app asks the server who it is, and a 401 is the answer
/// "nobody" rather than an error worth showing.
@Observable
@MainActor
final class SessionStore {
    enum State: Equatable {
        case loading
        case signedOut
        case signedIn(User)
    }

    private(set) var state: State = .loading
    let api: RungAPI

    init(api: RungAPI = .live()) {
        self.api = api
    }

    func restore() async {
        do {
            state = .signedIn(try await api.me())
        } catch RungError.notAuthenticated {
            state = .signedOut
        } catch {
            // Offline on launch is not signed out. Staying on the loading
            // state would be a spinner forever, so fall back to the sign-in
            // screen, which can show the reason and retry.
            state = .signedOut
        }
    }

    func signedIn(_ user: User) { state = .signedIn(user) }

    func signOut() async {
        try? await api.logOut()
        state = .signedOut
    }
}

struct RootView: View {
    @Environment(SessionStore.self) private var session

    var body: some View {
        switch session.state {
        case .loading:
            ProgressView().task { await session.restore() }
        case .signedOut:
            SignInView()
        case .signedIn:
            MainTabs()
        }
    }
}

struct MainTabs: View {
    // The `Tab` value type and the TabView builder that takes it are iOS 18,
    // and this app's floor is 17 — which still covers every iPhone back to
    // the XS. Nothing here needs what the newer API adds (typed selection,
    // a sidebar adaptation), so the older one is not a workaround, it is the
    // right size for three fixed tabs.
    var body: some View {
        TabView {
            StandingView()
                .tabItem { Label("Standing", systemImage: "chart.bar.fill") }
            LogSessionView()
                .tabItem { Label("Log", systemImage: "plus.circle.fill") }
            CalendarView()
                .tabItem { Label("Calendar", systemImage: "calendar") }
        }
    }
}

extension Color {
    /// The amber the web app uses for anything that acts. Deepened for a
    /// light background, the same way app.css does it.
    static let rungSignal = Color(red: 0.91, green: 0.58, blue: 0.04)
}
