import RungKit
import SwiftUI

struct CalendarView: View {
    @Environment(SessionStore.self) private var session

    @State private var entries: [CalendarEntry] = []
    @State private var failure: String?
    @State private var loaded = false

    var body: some View {
        NavigationStack {
            List {
                if entries.isEmpty && loaded {
                    ContentUnavailableView(
                        "Nothing scheduled", systemImage: "calendar",
                        description: Text("Build a plan on the web and it lands here."))
                }
                ForEach(byDay, id: \.day) { group in
                    Section(group.day) {
                        ForEach(group.entries) { entry in
                            NavigationLink(value: entry.id) {
                                HStack {
                                    VStack(alignment: .leading, spacing: 2) {
                                        Text(entry.title)
                                        Text(shape(entry))
                                            .font(.caption)
                                            .monospaced()
                                            .foregroundStyle(.secondary)
                                    }
                                    Spacer()
                                    if entry.isComplete {
                                        Image(systemName: "checkmark.circle.fill")
                                            .foregroundStyle(.green)
                                    }
                                }
                            }
                        }
                    }
                }
            }
            .navigationTitle("Calendar")
            .navigationDestination(for: String.self) { SessionView(sessionId: $0) }
            .refreshable { await load() }
            .task { await load() }
            .overlay {
                if let failure, entries.isEmpty {
                    ContentUnavailableView(
                        "Could not load", systemImage: "exclamationmark.triangle",
                        description: Text(failure))
                }
            }
        }
    }

    private var byDay: [(day: String, entries: [CalendarEntry])] {
        Dictionary(grouping: entries, by: \.scheduledOn)
            .map { (day: $0.key, entries: $0.value) }
            .sorted { $0.day < $1.day }
    }

    private func shape(_ entry: CalendarEntry) -> String {
        let blocks = entry.body.blocks.count
        let sets = entry.body.blocks.reduce(0) { $0 + $1.sets }
        return "\(blocks) blocks · \(sets) sets"
    }

    private func load() async {
        let today = Date()
        let from = RungJSON.day(from: today.addingTimeInterval(-7 * 86400))
        let to = RungJSON.day(from: today.addingTimeInterval(35 * 86400))
        do {
            entries = try await session.api.calendar(from: from, to: to).sessions
            failure = nil
        } catch let error as RungError {
            failure = error.message
        } catch {
            failure = error.localizedDescription
        }
        loaded = true
    }
}

/// One session, in the six parts it is performed in, with a tick per block —
/// the same shape the web page uses, because the running order is the point.
struct SessionView: View {
    let sessionId: String

    @Environment(SessionStore.self) private var session
    @State private var detail: SessionDetail?
    @State private var ticks = TickState()
    @State private var failure: String?

    var body: some View {
        Group {
            if let detail {
                List {
                    ForEach(detail.phases) { phase in
                        Section(phase.label) {
                            let protocols = detail.protocols.filter { $0.phase == phase.key }
                            let blocks = phase.key == "training" ? numberedBlocks(detail) : []

                            if protocols.isEmpty && blocks.isEmpty {
                                Text(phase.note ?? "Nothing in this phase.")
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }

                            ForEach(protocols) { proto in
                                Toggle(isOn: protocolBinding(proto.slug)) {
                                    VStack(alignment: .leading, spacing: 2) {
                                        Text(proto.title)
                                        Text(proto.purpose)
                                            .font(.caption)
                                            .foregroundStyle(.secondary)
                                    }
                                }
                            }

                            ForEach(blocks, id: \.position) { item in
                                Toggle(isOn: blockBinding(item.position)) {
                                    BlockRow(block: item.block, exercises: detail.exercises)
                                }
                            }
                        }
                    }
                }
                .navigationTitle(detail.session.title)
                .navigationBarTitleDisplayMode(.inline)
                .toolbar {
                    Button(detail.session.isComplete ? "Done — undo" : "Mark done") {
                        Task { await toggleDone() }
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
        .task { await load() }
    }

    private func numberedBlocks(_ detail: SessionDetail) -> [(position: Int, block: SessionBlock)] {
        // Position is the block's index in the body, which is what a tick is
        // recorded against — never the index within a filtered view.
        detail.session.body.blocks.enumerated().map { (position: $0.offset, block: $0.element) }
    }

    private func protocolBinding(_ slug: String) -> Binding<Bool> {
        Binding(
            get: { ticks.doneProtocols.contains(slug) },
            set: { on in
                if on {
                    ticks.doneProtocols.append(slug)
                } else {
                    ticks.doneProtocols.removeAll { $0 == slug }
                }
                Task { await pushTicks() }
            })
    }

    private func blockBinding(_ position: Int) -> Binding<Bool> {
        Binding(
            get: { ticks.doneBlocks.contains(position) },
            set: { on in
                if on {
                    ticks.doneBlocks.append(position)
                } else {
                    ticks.doneBlocks.removeAll { $0 == position }
                }
                Task { await pushTicks() }
            })
    }

    private func load() async {
        do {
            let loaded = try await session.api.session(id: sessionId)
            detail = loaded
            ticks = TickState(
                doneProtocols: loaded.session.doneProtocols ?? [],
                doneBlocks: loaded.session.doneBlocks ?? [])
        } catch let error as RungError {
            failure = error.message
        } catch {
            failure = error.localizedDescription
        }
    }

    /// The tick lands before the request does. An athlete knows what they
    /// just did, and a box that un-ticks itself because the network blinked
    /// is worse than one that is briefly ahead of the server.
    private func pushTicks() async {
        // The server prunes ticks it does not recognise, so take its
        // answer back rather than trusting what we sent.
        if let stored = try? await session.api.saveProgress(sessionId: sessionId, ticks) {
            ticks = TickState(
                doneProtocols: stored.doneProtocols ?? [],
                doneBlocks: stored.doneBlocks ?? [])
        }
    }

    private func toggleDone() async {
        guard let detail else { return }
        do {
            if detail.session.isComplete {
                try await session.api.markNotDone(sessionId: sessionId)
            } else {
                try await session.api.markDone(sessionId: sessionId)
            }
            await load()
        } catch let error as RungError {
            failure = error.message
        } catch {
            failure = error.localizedDescription
        }
    }
}

struct BlockRow: View {
    let block: SessionBlock
    let exercises: [String: Exercise]

    var body: some View {
        VStack(alignment: .leading, spacing: 3) {
            Text(exercises[block.exerciseSlug]?.name ?? block.exerciseSlug)
            Text("\(block.sets) × \(block.prescription)")
                .font(.caption)
                .monospaced()
            if let rest = block.restSeconds, rest > 0 {
                Text("\(rest)s rest")
                    .font(.caption2)
                    .foregroundStyle(.secondary)
            }
            if let notes = block.notes, !notes.isEmpty {
                Text(notes)
                    .font(.caption2)
                    .foregroundStyle(.secondary)
            }
        }
    }
}
