import SwiftUI

struct RemindersView: View {
    @EnvironmentObject private var app: AppState
    @State private var reminders: [Reminder] = []
    @State private var loadError: String?
    @State private var editing: Reminder?
    @State private var showingAdd = false

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 10) {
                if reminders.isEmpty && loadError == nil {
                    VStack(spacing: 8) {
                        Image(systemName: "bell.badge").font(.title).foregroundStyle(Theme.text3)
                        Text("No reminders yet.").foregroundStyle(Theme.text3).font(.footnote)
                        Button("Add one") { showingAdd = true }
                            .foregroundStyle(Theme.ai).font(.system(.footnote, weight: .semibold))
                    }
                    .frame(maxWidth: .infinity, minHeight: 220)
                }
                if let loadError {
                    Text(loadError).foregroundStyle(Theme.financeBad).font(.footnote)
                        .padding(.horizontal, 16)
                }
                ForEach(reminders) { r in
                    Button { editing = r } label: {
                        ReminderRow(reminder: r)
                    }
                    .buttonStyle(.plain)
                    .padding(.horizontal, 16)
                }
            }
            .padding(.vertical, 8)
        }
        .background(Theme.bg)
        .navigationTitle("Reminders")
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
            ReminderEditSheet(reminder: nil)
        }
        .sheet(item: $editing, onDismiss: { Task { await load() } }) { r in
            ReminderEditSheet(reminder: r)
        }
        .task { await load() }
    }

    private func load() async {
        guard let api = app.apiClient() else { loadError = "Not paired."; return }
        do {
            reminders = try await api.listReminders()
            loadError = nil
        } catch {
            loadError = (error as? APIClient.APIError)?.errorDescription ?? error.localizedDescription
        }
    }
}

private struct ReminderRow: View {
    let reminder: Reminder

    var body: some View {
        HStack(spacing: 12) {
            ZStack {
                RoundedRectangle(cornerRadius: 10).fill(Theme.goals.opacity(0.18))
                Image(systemName: reminder.active ? "bell.fill" : "bell.slash")
                    .foregroundStyle(reminder.active ? Theme.goals : Theme.text3)
            }
            .frame(width: 36, height: 36)
            VStack(alignment: .leading, spacing: 2) {
                Text(reminder.title)
                    .font(.system(size: 15, weight: .semibold))
                    .foregroundStyle(reminder.active ? Theme.text : Theme.text3)
                HStack(spacing: 6) {
                    Text(reminder.recurrence)
                        .font(.system(.caption, design: .monospaced))
                        .foregroundStyle(Theme.text3)
                    if reminder.next_fires_at > 0 {
                        Text("· next \(nextDisplay)").font(.caption).foregroundStyle(Theme.text3)
                    }
                }
            }
            Spacer()
            Image(systemName: "chevron.right").foregroundStyle(Theme.text3).font(.caption)
        }
        .padding(12)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 14))
    }

    private var nextDisplay: String {
        let d = Date(timeIntervalSince1970: TimeInterval(reminder.next_fires_at))
        let f = RelativeDateTimeFormatter(); f.unitsStyle = .abbreviated
        return f.localizedString(for: d, relativeTo: Date())
    }
}

private struct ReminderEditSheet: View {
    @EnvironmentObject private var app: AppState
    @Environment(\.dismiss) private var dismiss

    let reminder: Reminder?

    @State private var title = ""
    @State private var body_ = ""
    @State private var recurrence = "0 7 * * *"
    @State private var active = true
    @State private var saving = false
    @State private var saveError: String?

    private struct Preset: Identifiable {
        var id: String { cron }
        let label: String
        let cron: String
    }
    private let presets: [Preset] = [
        .init(label: "Every morning 7am",  cron: "0 7 * * *"),
        .init(label: "Every evening 9pm",  cron: "0 21 * * *"),
        .init(label: "Weekdays 9am",       cron: "0 9 * * 1-5"),
        .init(label: "Monday 8am",         cron: "0 8 * * 1"),
        .init(label: "Friday 5pm",         cron: "0 17 * * 5"),
        .init(label: "Monthly 1st 9am",    cron: "0 9 1 * *"),
    ]

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    field("TITLE") {
                        TextField("e.g. BPC-157 morning dose", text: $title)
                            .textInputAutocapitalization(.sentences)
                    }
                    field("BODY (optional)") {
                        TextEditor(text: $body_)
                            .scrollContentBackground(.hidden).frame(minHeight: 70)
                    }
                    VStack(alignment: .leading, spacing: 6) {
                        Text("PRESETS").font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
                        ScrollView(.horizontal, showsIndicators: false) {
                            HStack(spacing: 8) {
                                ForEach(presets) { p in
                                    Button { recurrence = p.cron } label: {
                                        Text(p.label)
                                            .font(.caption).foregroundStyle(recurrence == p.cron ? Theme.ai : Theme.text2)
                                            .padding(.horizontal, 12).padding(.vertical, 8)
                                            .background(recurrence == p.cron ? Theme.ai.opacity(0.12) : Color(hex: 0x0f0f0f))
                                            .overlay(
                                                RoundedRectangle(cornerRadius: 18)
                                                    .stroke(recurrence == p.cron ? Theme.ai : Color.clear, lineWidth: 1))
                                            .clipShape(Capsule())
                                    }
                                }
                            }
                        }
                    }
                    field("CRON (5 fields: min hour dom month dow)") {
                        TextField("0 7 * * *", text: $recurrence)
                            .font(.system(.body, design: .monospaced))
                            .textInputAutocapitalization(.never)
                            .autocorrectionDisabled()
                    }
                    Toggle(isOn: $active) {
                        Text("Active").foregroundStyle(Theme.text)
                    }
                    .padding(.horizontal, 14).padding(.vertical, 10)
                    .background(Theme.surface)
                    .clipShape(RoundedRectangle(cornerRadius: 12))
                    .tint(Theme.ai)
                    if let saveError {
                        Text(saveError).foregroundStyle(Theme.financeBad).font(.footnote)
                    }
                    if reminder != nil {
                        Button(role: .destructive) { Task { await delete() } } label: {
                            Text("Delete reminder").frame(maxWidth: .infinity).padding(.vertical, 10)
                        }
                        .padding(.top, 8)
                    }
                }
                .padding(20)
            }
            .background(Theme.bg)
            .navigationTitle(reminder == nil ? "New reminder" : "Edit reminder")
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
            .onAppear {
                if let r = reminder {
                    title = r.title; body_ = r.body
                    recurrence = r.recurrence; active = r.active
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
                .foregroundStyle(Theme.text).tint(Theme.ai)
        }
    }

    private func save() async {
        guard let api = app.apiClient() else { return }
        saving = true; defer { saving = false }
        do {
            if let r = reminder {
                try await api.updateReminder(id: r.id, APIClient.ReminderInput(
                    title: title, body: body_, recurrence: recurrence, active: active))
            } else {
                _ = try await api.createReminder(APIClient.ReminderInput(
                    title: title, body: body_, recurrence: recurrence, active: active))
            }
            dismiss()
        } catch {
            saveError = (error as? APIClient.APIError)?.errorDescription ?? error.localizedDescription
        }
    }

    private func delete() async {
        guard let api = app.apiClient(), let r = reminder else { return }
        saving = true; defer { saving = false }
        do {
            try await api.deleteReminder(id: r.id)
            dismiss()
        } catch {
            saveError = (error as? APIClient.APIError)?.errorDescription ?? error.localizedDescription
        }
    }
}
