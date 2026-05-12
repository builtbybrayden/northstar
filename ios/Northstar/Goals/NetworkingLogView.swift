import SwiftUI

struct NetworkingLogView: View {
    @EnvironmentObject private var app: AppState
    @State private var entries: [NetworkingEntry] = []
    @State private var loadError: String?
    @State private var showingAdd = false

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 10) {
                if entries.isEmpty && loadError == nil {
                    VStack(spacing: 8) {
                        Image(systemName: "person.2").font(.title).foregroundStyle(Theme.text3)
                        Text("No conversations logged yet.").foregroundStyle(Theme.text3).font(.footnote)
                        Button("Add one") { showingAdd = true }
                            .foregroundStyle(Theme.ai).font(.system(.footnote, weight: .semibold))
                    }
                    .frame(maxWidth: .infinity, minHeight: 220)
                }
                if let loadError {
                    Text(loadError).foregroundStyle(Theme.financeBad).font(.footnote).padding(.horizontal, 16)
                }
                ForEach(entries) { e in
                    NetworkingRow(entry: e)
                        .padding(.horizontal, 16)
                }
            }
            .padding(.vertical, 8)
        }
        .background(Theme.bg)
        .navigationTitle("Networking")
        .toolbarBackground(Theme.bg, for: .navigationBar)
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                Button { showingAdd = true } label: {
                    Image(systemName: "plus.circle.fill").foregroundStyle(Theme.text)
                }
            }
        }
        .refreshable { await load() }
        .sheet(isPresented: $showingAdd, onDismiss: { Task { await load() } }) {
            AddNetworkingSheet()
        }
        .task { await load() }
    }

    private func load() async {
        guard let api = app.apiClient() else { loadError = "Not paired."; return }
        do {
            entries = try await api.listNetworking()
            loadError = nil
        } catch {
            loadError = (error as? APIClient.APIError)?.errorDescription ?? error.localizedDescription
        }
    }
}

private struct NetworkingRow: View {
    let entry: NetworkingEntry

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack {
                Text(entry.person)
                    .font(.system(size: 15, weight: .semibold))
                    .foregroundStyle(Theme.text)
                Spacer()
                Text(entry.date).font(.caption).foregroundStyle(Theme.text3)
            }
            if !entry.context.isEmpty {
                Text(entry.context)
                    .font(.footnote).foregroundStyle(Theme.text2)
                    .lineLimit(2)
            }
            if !entry.next_action.isEmpty {
                HStack(spacing: 6) {
                    Image(systemName: "arrow.right.circle.fill")
                        .foregroundStyle(Theme.ai).font(.caption)
                    Text(entry.next_action)
                        .font(.caption).foregroundStyle(Theme.text)
                    if !entry.next_action_due.isEmpty {
                        Text("by \(entry.next_action_due)")
                            .font(.caption2).foregroundStyle(Theme.ai)
                    }
                }
            }
        }
        .padding(12)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 14))
    }
}

private struct AddNetworkingSheet: View {
    @EnvironmentObject private var app: AppState
    @Environment(\.dismiss) private var dismiss

    @State private var person = ""
    @State private var date = Date()
    @State private var context = ""
    @State private var nextAction = ""
    @State private var nextDue = Date()
    @State private var hasNextDue = false
    @State private var saving = false
    @State private var saveError: String?

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    field("PERSON") {
                        TextField("Name or handle", text: $person)
                            .textInputAutocapitalization(.words)
                    }
                    field("DATE") {
                        DatePicker("", selection: $date, displayedComponents: .date)
                            .labelsHidden().colorScheme(.dark).tint(Theme.ai)
                    }
                    field("CONTEXT") {
                        TextEditor(text: $context)
                            .scrollContentBackground(.hidden)
                            .frame(minHeight: 80)
                    }
                    field("NEXT ACTION") {
                        TextField("e.g. follow up about pentesting tooling", text: $nextAction)
                    }
                    VStack(alignment: .leading, spacing: 6) {
                        Toggle(isOn: $hasNextDue) {
                            Text("NEXT ACTION DUE")
                                .font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
                        }
                        .tint(Theme.ai)
                        if hasNextDue {
                            DatePicker("", selection: $nextDue, displayedComponents: .date)
                                .labelsHidden().colorScheme(.dark).tint(Theme.ai)
                        }
                    }
                    if let saveError {
                        Text(saveError).foregroundStyle(Theme.financeBad).font(.footnote)
                    }
                }
                .padding(20)
            }
            .background(Theme.bg)
            .navigationTitle("New contact")
            .toolbarBackground(Theme.bg, for: .navigationBar)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Button("Cancel") { dismiss() }.foregroundStyle(Theme.text2)
                }
                ToolbarItem(placement: .topBarTrailing) {
                    Button { Task { await save() } } label: {
                        if saving { ProgressView().tint(Theme.ai) } else { Text("Save").bold().foregroundStyle(Theme.ai) }
                    }
                    .disabled(saving || person.trimmingCharacters(in: .whitespaces).isEmpty)
                }
            }
        }
    }

    private func field<C: View>(_ label: String, @ViewBuilder _ content: () -> C) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(label).font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
            content()
                .padding(.horizontal, 14).padding(.vertical, 10)
                .background(Color(hex: 0x0f0f0f))
                .clipShape(RoundedRectangle(cornerRadius: 12))
                .foregroundStyle(Theme.text)
                .tint(Theme.ai)
        }
    }

    private func save() async {
        guard let api = app.apiClient() else { return }
        saving = true; defer { saving = false }
        let f = DateFormatter(); f.dateFormat = "yyyy-MM-dd"
        do {
            _ = try await api.createNetworking(APIClient.NetworkingInput(
                date: f.string(from: date),
                person: person.trimmingCharacters(in: .whitespaces),
                context: context,
                next_action: nextAction,
                next_action_due: hasNextDue ? f.string(from: nextDue) : ""
            ))
            dismiss()
        } catch {
            saveError = (error as? APIClient.APIError)?.errorDescription ?? error.localizedDescription
        }
    }
}
