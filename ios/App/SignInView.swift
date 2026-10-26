import RungKit
import SwiftUI

struct SignInView: View {
    @Environment(SessionStore.self) private var session

    @State private var registering = false
    @State private var email = ""
    @State private var password = ""
    @State private var displayName = ""
    @State private var busy = false
    @State private var failure: String?

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    TextField("Email", text: $email)
                        .textContentType(.emailAddress)
                        .keyboardType(.emailAddress)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()

                    if registering {
                        TextField("Name", text: $displayName)
                            .textContentType(.name)
                    }

                    SecureField("Password", text: $password)
                        .textContentType(registering ? .newPassword : .password)
                } footer: {
                    if registering {
                        Text("At least 10 characters.")
                    }
                }

                if let failure {
                    Section { Text(failure).foregroundStyle(.red) }
                }

                Section {
                    Button(registering ? "Create account" : "Sign in") {
                        Task { await submit() }
                    }
                    .disabled(busy || email.isEmpty || password.isEmpty)

                    Button(registering ? "I already have an account" : "Create one") {
                        registering.toggle()
                        failure = nil
                    }
                    .foregroundStyle(.secondary)
                }
            }
            .navigationTitle("Rung")
            .disabled(busy)
        }
    }

    private func submit() async {
        busy = true
        failure = nil
        defer { busy = false }
        do {
            let user =
                registering
                ? try await session.api.register(
                    email: email, password: password, displayName: displayName)
                : try await session.api.logIn(email: email, password: password)
            session.signedIn(user)
        } catch let error as RungError {
            failure = error.message
        } catch {
            failure = error.localizedDescription
        }
    }
}
