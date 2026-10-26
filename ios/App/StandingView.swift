import RungKit
import SwiftUI

/// Where you stand, which is the screen the web app opens on: the level per
/// category, computed from logged sets, and what the last four weeks held.
struct StandingView: View {
    @Environment(SessionStore.self) private var session

    @State private var level: Level?
    @State private var failure: String?

    var body: some View {
        NavigationStack {
            List {
                if let level {
                    Section("Level by category") {
                        ForEach(level.categories) { category in
                            LabeledContent {
                                Text(category.tier).monospaced()
                            } label: {
                                VStack(alignment: .leading, spacing: 2) {
                                    Text(category.category.capitalized)
                                    Text(
                                        category.basedOn.isEmpty
                                            ? "Nothing logged yet" : category.basedOn
                                    )
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                                }
                            }
                        }
                    }

                    Section("Last 28 days") {
                        LabeledContent("Sessions this week", value: "\(level.sessionsLast7Days)")
                        LabeledContent("Sessions", value: "\(level.sessionsLast28Days)")
                        LabeledContent("Sets", value: "\(level.setsLast28Days)")
                        LabeledContent("Open injuries", value: "\(level.openInjuries.count)")
                    }

                    let trained = level.records.filter { ($0.tier ?? "untested") != "untested" }
                    if !trained.isEmpty {
                        Section("Records") {
                            ForEach(trained) { record in
                                LabeledContent {
                                    Text(best(record)).monospaced()
                                } label: {
                                    VStack(alignment: .leading, spacing: 2) {
                                        Text(record.name)
                                        Text(record.tier ?? "")
                                            .font(.caption)
                                            .foregroundStyle(.secondary)
                                    }
                                }
                            }
                        }
                    }
                } else if let failure {
                    ContentUnavailableView(
                        "Could not load", systemImage: "exclamationmark.triangle",
                        description: Text(failure))
                } else {
                    ProgressView()
                }
            }
            .navigationTitle("Where you stand")
            .toolbar {
                Button("Sign out") { Task { await session.signOut() } }
            }
            .refreshable { await load() }
            .task { await load() }
        }
    }

    /// Whichever figure this movement is measured in. Only one is ever set.
    private func best(_ record: Record) -> String {
        if let kg = record.bestAddedKg { return "+\(format(kg)) kg" }
        if let hold = record.bestHoldSeconds { return "\(format(hold))s" }
        if let reps = record.bestReps { return "\(reps) reps" }
        return "—"
    }

    private func format(_ value: Double) -> String {
        value == value.rounded() ? String(Int(value)) : String(format: "%.1f", value)
    }

    private func load() async {
        do {
            level = try await session.api.level()
            failure = nil
        } catch let error as RungError {
            failure = error.message
        } catch {
            failure = error.localizedDescription
        }
    }
}
