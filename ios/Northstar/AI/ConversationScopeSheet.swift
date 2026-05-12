import SwiftUI

/// Lets the user narrow a conversation's tool surface to one or more pillars.
/// Empty scope (default) = all pillars, which is the current Phase-5 behavior.
struct ConversationScopeSheet: View {
    @EnvironmentObject private var app: AppState
    @Environment(\.dismiss) private var dismiss

    let conversationID: String
    let initial: [String]
    let onSaved: ([String]) -> Void

    @State private var selected: Set<String> = []
    @State private var saving = false
    @State private var saveError: String?

    private let pillars: [(key: String, label: String, color: Color, icon: String)] = [
        ("finance", "Finance", Theme.finance, "dollarsign.circle.fill"),
        ("goals",   "Goals",   Theme.goals,   "star.fill"),
        ("health",  "Health",  Theme.healthGo, "heart.fill"),
    ]

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    Text("Limit which tools Claude can call in this conversation. Leave all selected for the full cross-pillar view.")
                        .font(.footnote)
                        .foregroundStyle(Theme.text3)

                    ForEach(pillars, id: \.key) { p in
                        pillarRow(p)
                    }

                    if !selected.isEmpty && selected.count < pillars.count {
                        Text("Claude won't be able to pull data from the other pillars in this conversation — start a new one if you need cross-pillar context.")
                            .font(.caption)
                            .foregroundStyle(Theme.text3)
                            .padding(.top, 4)
                    }
                    if let saveError {
                        Text(saveError).foregroundStyle(Theme.financeBad).font(.footnote)
                    }
                }
                .padding(20)
            }
            .background(Theme.bg)
            .navigationTitle("Conversation scope")
            .toolbarBackground(Theme.bg, for: .navigationBar)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Button("Cancel") { dismiss() }.foregroundStyle(Theme.text2)
                }
                ToolbarItem(placement: .topBarTrailing) {
                    Button { Task { await save() } } label: {
                        if saving { ProgressView().tint(Theme.ai) }
                        else { Text("Save").bold().foregroundStyle(Theme.ai) }
                    }
                    .disabled(saving)
                }
            }
            .onAppear { selected = Set(initial) }
        }
    }

    private func pillarRow(_ p: (key: String, label: String, color: Color, icon: String)) -> some View {
        let isOn = selected.contains(p.key) || selected.isEmpty
        return Button {
            toggle(p.key)
        } label: {
            HStack {
                ZStack {
                    Circle().fill(p.color.opacity(0.18)).frame(width: 32, height: 32)
                    Image(systemName: p.icon).foregroundStyle(p.color)
                }
                Text(p.label).foregroundStyle(Theme.text)
                Spacer()
                Image(systemName: isOn ? "checkmark.circle.fill" : "circle")
                    .foregroundStyle(isOn ? p.color : Theme.text3)
                    .font(.system(size: 22))
            }
            .padding(.horizontal, 14).padding(.vertical, 12)
            .background(Theme.surface)
            .clipShape(RoundedRectangle(cornerRadius: 12))
        }
        .buttonStyle(.plain)
    }

    private func toggle(_ key: String) {
        // selected = empty means "all on" — first tap converts to explicit
        // selection of the other pillars so the user-visible state matches
        // what the server will store.
        if selected.isEmpty {
            selected = Set(pillars.map(\.key))
        }
        if selected.contains(key) {
            selected.remove(key)
        } else {
            selected.insert(key)
        }
        // If everything is selected, fall back to empty = "all pillars".
        if selected.count == pillars.count {
            selected = []
        }
    }

    private func save() async {
        guard let api = app.apiClient() else { return }
        saving = true; saveError = nil
        defer { saving = false }
        let scope = Array(selected).sorted()
        do {
            try await api.aiUpdateConversationScope(id: conversationID, pillarScope: scope)
            onSaved(scope)
            dismiss()
        } catch {
            saveError = (error as? APIClient.APIError)?.errorDescription ?? error.localizedDescription
        }
    }
}
