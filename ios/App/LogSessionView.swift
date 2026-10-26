import RungKit
import SwiftUI

/// Logging, which is the whole reason to have this on a phone: it happens in
/// a gym, between sets, one-handed.
///
/// Two things it does that the web form does not. The exercise picker is
/// searchable, because 179 movements in a flat select is a scroll on a
/// phone and a search on one. And the session carries a date, so an evening
/// session logged the next morning lands on the right day — the API has
/// always taken `performed_at` and the browser has never sent it.
struct LogSessionView: View {
    @Environment(SessionStore.self) private var session

    @State private var exercises: [Exercise] = []
    @State private var drafts: [SetDraft] = []
    @State private var performedAt = Date()
    @State private var notes = ""
    @State private var rpe: Int?
    @State private var busy = false
    @State private var failure: String?
    @State private var saved = false

    var body: some View {
        NavigationStack {
            Form {
                ForEach($drafts) { $draft in
                    SetDraftRow(draft: $draft, exercises: exercises)
                }
                .onDelete { drafts.remove(atOffsets: $0) }

                Section {
                    Button("Add another set", systemImage: "plus") { addSet() }
                        .disabled(exercises.isEmpty)
                }

                Section("The session") {
                    DatePicker("Performed", selection: $performedAt)
                    TextField(
                        "Notes — how it felt, anything that hurt", text: $notes, axis: .vertical
                    )
                    .lineLimit(2...4)
                    Picker("Effort", selection: $rpe) {
                        Text("—").tag(Int?.none)
                        ForEach(1...10, id: \.self) { Text("\($0)").tag(Int?.some($0)) }
                    }
                }

                if let failure {
                    Section { Text(failure).foregroundStyle(.red) }
                }

                Section {
                    Button("Save session") { Task { await save() } }
                        .disabled(busy || !canSave)
                } footer: {
                    if !drafts.isEmpty && !canSave {
                        Text("Every set needs its numbers before it can be saved.")
                    }
                }
            }
            .navigationTitle("Log a session")
            .disabled(busy)
            .task { await loadExercises() }
            .alert("Saved", isPresented: $saved) {
                Button("OK") {}
            } message: {
                Text("Your level and records have moved.")
            }
        }
    }

    private var canSave: Bool {
        !drafts.isEmpty && drafts.allSatisfy { $0.asInput(in: exercises)?.isComplete == true }
    }

    private func loadExercises() async {
        guard exercises.isEmpty else { return }
        do {
            exercises = try await session.api.exercises()
            if drafts.isEmpty { addSet() }
        } catch let error as RungError {
            failure = error.message
        } catch {
            failure = error.localizedDescription
        }
    }

    /// A new set copies the last one, because the common case is three of the
    /// same thing and the second two should be a tap rather than a form.
    private func addSet() {
        if let previous = drafts.last {
            drafts.append(previous.repeated())
        } else if let first = exercises.first {
            drafts.append(SetDraft(exerciseSlug: first.slug))
        }
    }

    private func save() async {
        busy = true
        failure = nil
        defer { busy = false }

        let sets = drafts.compactMap { $0.asInput(in: exercises) }
        do {
            try await session.api.log(
                WorkoutInput(performedAt: performedAt, notes: notes, rpe: rpe, sets: sets))
            drafts = []
            notes = ""
            rpe = nil
            performedAt = Date()
            addSet()
            saved = true
        } catch let error as RungError {
            failure = error.message
        } catch {
            failure = error.localizedDescription
        }
    }
}

/// One set being typed. Kept separate from `SetInput` because a half-filled
/// set is a normal state here and an invalid one on the wire.
struct SetDraft: Identifiable {
    let id = UUID()
    var exerciseSlug: String
    var reps: Int?
    var weightKg: Double?
    var holdSeconds: Double?
    var success = true

    func repeated() -> SetDraft {
        SetDraft(
            exerciseSlug: exerciseSlug, reps: reps, weightKg: weightKg,
            holdSeconds: holdSeconds, success: success)
    }

    func asInput(in exercises: [Exercise]) -> SetInput? {
        guard let exercise = exercises.first(where: { $0.slug == exerciseSlug }) else { return nil }
        let kind = SetKind(measure: exercise.measure)
        var input = SetInput(exerciseSlug: exerciseSlug, kind: kind)
        // Only what this kind asks for. Sending a null where the server wants
        // a number is the same as not sending it.
        for field in kind.requiredFields {
            switch field {
            case .reps: input.reps = reps
            case .weight: input.weightKg = weightKg
            case .hold: input.holdSeconds = holdSeconds
            case .success: input.success = success
            }
        }
        return input
    }
}

struct SetDraftRow: View {
    @Binding var draft: SetDraft
    let exercises: [Exercise]

    private var exercise: Exercise? {
        exercises.first { $0.slug == draft.exerciseSlug }
    }

    var body: some View {
        Section {
            NavigationLink {
                ExercisePicker(selection: $draft.exerciseSlug, exercises: exercises)
            } label: {
                LabeledContent("Exercise", value: exercise?.name ?? draft.exerciseSlug)
            }

            // What is asked for follows the movement, exactly as the server
            // decides it.
            switch exercise?.measure {
            case .reps:
                NumberField("Reps", value: $draft.reps)
            case .weightedReps:
                NumberField("Reps", value: $draft.reps)
                DecimalField("Added kg", value: $draft.weightKg)
            case .staticHold:
                DecimalField("Hold (s)", value: $draft.holdSeconds)
            case .weightedHold:
                DecimalField("Hold (s)", value: $draft.holdSeconds)
                DecimalField("Added kg", value: $draft.weightKg)
            case .skillAttempt:
                Picker("Result", selection: $draft.success) {
                    Text("Made").tag(true)
                    Text("Missed").tag(false)
                }
                .pickerStyle(.segmented)
            case nil:
                EmptyView()
            }
        }
    }
}

/// 179 movements is a search, not a scroll.
struct ExercisePicker: View {
    @Binding var selection: String
    let exercises: [Exercise]

    @Environment(\.dismiss) private var dismiss
    @State private var query = ""

    private var grouped: [(category: String, items: [Exercise])] {
        let matching =
            query.isEmpty
            ? exercises
            : exercises.filter { $0.name.localizedCaseInsensitiveContains(query) }
        return Dictionary(grouping: matching, by: \.category)
            .map { (category: $0.key, items: $0.value) }
            .sorted { $0.category < $1.category }
    }

    var body: some View {
        List {
            ForEach(grouped, id: \.category) { group in
                Section(group.category.capitalized) {
                    ForEach(group.items) { exercise in
                        Button {
                            selection = exercise.slug
                            dismiss()
                        } label: {
                            HStack {
                                Text(exercise.name).foregroundStyle(.primary)
                                Spacer()
                                if exercise.slug == selection {
                                    Image(systemName: "checkmark")
                                }
                            }
                        }
                    }
                }
            }
        }
        .searchable(text: $query, prompt: "Search movements")
        .navigationTitle("Exercise")
    }
}

struct NumberField: View {
    let title: String
    @Binding var value: Int?

    init(_ title: String, value: Binding<Int?>) {
        self.title = title
        self._value = value
    }

    var body: some View {
        LabeledContent(title) {
            TextField(title, value: $value, format: .number)
                .keyboardType(.numberPad)
                .multilineTextAlignment(.trailing)
        }
    }
}

struct DecimalField: View {
    let title: String
    @Binding var value: Double?

    init(_ title: String, value: Binding<Double?>) {
        self.title = title
        self._value = value
    }

    var body: some View {
        LabeledContent(title) {
            TextField(title, value: $value, format: .number)
                .keyboardType(.decimalPad)
                .multilineTextAlignment(.trailing)
        }
    }
}
