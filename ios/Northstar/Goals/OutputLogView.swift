import SwiftUI

struct OutputLogView: View {
    @EnvironmentObject private var app: AppState
    @State private var entries: [OutputEntry] = []
    @State private var loadError: String?
    @State private var showingAdd = false

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 10) {
                if entries.isEmpty && loadError == nil {
                    emptyState
                }
                if let loadError {
                    Text(loadError).foregroundStyle(Theme.financeBad).font(.footnote)
                        .padding(.horizontal, 16)
                }
                ForEach(entries) { e in
                    OutputRow(entry: e)
                        .padding(.horizontal, 16)
                }
            }
            .padding(.vertical, 8)
        }
        .background(Theme.bg)
        .navigationTitle("Output Log")
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
            AddOutputSheet()
        }
        .task { await load() }
    }

    private var emptyState: some View {
        VStack(spacing: 8) {
            Image(systemName: "doc.text").font(.title).foregroundStyle(Theme.text3)
            Text("No entries yet.").foregroundStyle(Theme.text3).font(.footnote)
            Text("Track CVEs, blog posts, talks, tools shipped, certs earned.")
                .multilineTextAlignment(.center)
                .foregroundStyle(Theme.text3).font(.caption)
                .padding(.horizontal, 30)
            Button("Add one") { showingAdd = true }
                .foregroundStyle(Theme.ai)
                .font(.system(.footnote, weight: .semibold))
                .padding(.top, 4)
        }
        .frame(maxWidth: .infinity, minHeight: 220)
    }

    private func load() async {
        guard let api = app.apiClient() else { loadError = "Not paired."; return }
        do {
            entries = try await api.listOutput()
            loadError = nil
        } catch {
            loadError = (error as? APIClient.APIError)?.errorDescription ?? error.localizedDescription
        }
    }
}

private struct OutputRow: View {
    let entry: OutputEntry

    var body: some View {
        HStack(alignment: .top, spacing: 12) {
            ZStack {
                RoundedRectangle(cornerRadius: 10).fill(categoryColor.opacity(0.18))
                Image(systemName: categoryIcon).foregroundStyle(categoryColor)
            }
            .frame(width: 36, height: 36)

            VStack(alignment: .leading, spacing: 2) {
                Text(entry.title)
                    .font(.system(size: 15, weight: .semibold))
                    .foregroundStyle(Theme.text)
                    .lineLimit(2)
                HStack(spacing: 6) {
                    Text(entry.category.uppercased())
                        .font(.caption2).bold()
                        .padding(.horizontal, 6).padding(.vertical, 1)
                        .background(categoryColor.opacity(0.18))
                        .foregroundStyle(categoryColor)
                        .clipShape(Capsule())
                    Text(entry.date).font(.caption).foregroundStyle(Theme.text3)
                }
                if !entry.url.isEmpty {
                    Text(entry.url)
                        .font(.caption).foregroundStyle(Theme.text3)
                        .lineLimit(1).truncationMode(.middle)
                }
            }
            Spacer()
        }
        .padding(12)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 14))
    }

    private var categoryIcon: String {
        switch entry.category.lowercased() {
        case "cve":    return "exclamationmark.shield.fill"
        case "blog":   return "doc.richtext"
        case "talk":   return "mic.fill"
        case "tool":   return "hammer.fill"
        case "cert":   return "rosette"
        case "pr":     return "arrow.triangle.merge"
        case "report": return "doc.text.fill"
        default:       return "doc"
        }
    }
    private var categoryColor: Color {
        switch entry.category.lowercased() {
        case "cve":    return Theme.financeBad
        case "blog":   return Theme.healthBlue
        case "talk":   return Theme.ai
        case "tool":   return Theme.finance
        case "cert":   return Theme.healthGo
        case "pr":     return Theme.goals
        case "report": return Theme.healthMid
        default:       return Theme.text2
        }
    }
}

private struct AddOutputSheet: View {
    @EnvironmentObject private var app: AppState
    @Environment(\.dismiss) private var dismiss

    @State private var title = ""
    @State private var category = "blog"
    @State private var url = ""
    @State private var notes = ""
    @State private var date = Date()
    @State private var saving = false
    @State private var saveError: String?

    private let categories = ["cve", "blog", "talk", "tool", "cert", "pr", "report"]

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    field("TITLE") {
                        TextField("e.g. CVE-2026-1234 advisory", text: $title)
                            .textInputAutocapitalization(.sentences)
                    }
                    field("CATEGORY") {
                        Picker("Category", selection: $category) {
                            ForEach(categories, id: \.self) { c in
                                Text(c.uppercased()).tag(c)
                            }
                        }
                        .pickerStyle(.segmented)
                    }
                    field("DATE") {
                        DatePicker("", selection: $date, displayedComponents: .date)
                            .labelsHidden().colorScheme(.dark).tint(Theme.ai)
                    }
                    field("URL (optional)") {
                        TextField("https://…", text: $url)
                            .keyboardType(.URL)
                            .textInputAutocapitalization(.never)
                            .autocorrectionDisabled()
                    }
                    field("NOTES (optional)") {
                        TextEditor(text: $notes)
                            .scrollContentBackground(.hidden)
                            .frame(minHeight: 80)
                    }
                    if let saveError {
                        Text(saveError).foregroundStyle(Theme.financeBad).font(.footnote)
                    }
                }
                .padding(20)
            }
            .background(Theme.bg)
            .navigationTitle("New entry")
            .toolbarBackground(Theme.bg, for: .navigationBar)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Button("Cancel") { dismiss() }.foregroundStyle(Theme.text2)
                }
                ToolbarItem(placement: .topBarTrailing) {
                    Button { Task { await save() } } label: {
                        if saving { ProgressView().tint(Theme.ai) } else { Text("Save").bold().foregroundStyle(Theme.ai) }
                    }
                    .disabled(saving || title.trimmingCharacters(in: .whitespaces).isEmpty)
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
            _ = try await api.createOutput(APIClient.OutputInput(
                date: f.string(from: date),
                category: category,
                title: title.trimmingCharacters(in: .whitespaces),
                body_md: notes,
                url: url.trimmingCharacters(in: .whitespaces)
            ))
            dismiss()
        } catch {
            saveError = (error as? APIClient.APIError)?.errorDescription ?? error.localizedDescription
        }
    }
}
