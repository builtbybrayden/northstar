import SwiftUI

/// Weekly + Monthly planner. Swappable scope via segmented control.
struct WeeklyMonthlyView: View {
    @EnvironmentObject private var app: AppState
    enum Scope: String, CaseIterable { case week = "Week", month = "Month" }
    @State private var scope: Scope = .week

    @State private var weekly: APIClient.WeeklyTracker?
    @State private var monthly: APIClient.MonthlyGoals?
    @State private var loadError: String?
    @State private var saving = false

    @State private var theme: String = ""
    @State private var retro: String = ""
    @State private var items: [DailyItem] = []
    @State private var newItemText = ""

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            Picker("Scope", selection: $scope) {
                ForEach(Scope.allCases, id: \.self) { Text($0.rawValue).tag($0) }
            }
            .pickerStyle(.segmented)
            .padding(.horizontal, 16)

            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    Text(scope == .week ? "Week of \(weeklyMondayDisplay)" : monthDisplay)
                        .font(.system(.headline))
                        .foregroundStyle(Theme.text3)
                        .padding(.horizontal, 16)

                    if scope == .week {
                        themeField
                    }

                    goalsCard
                    retroCard

                    if let loadError {
                        Text(loadError).font(.footnote).foregroundStyle(Theme.financeBad)
                            .padding(.horizontal, 16)
                    }

                    Button(action: { Task { await save() } }) {
                        HStack {
                            if saving { ProgressView().tint(.black) }
                            Text(saving ? "Saving…" : "Save")
                                .font(.system(.body, weight: .semibold))
                        }
                        .frame(maxWidth: .infinity).padding(.vertical, 14)
                        .background(Theme.ai)
                        .foregroundStyle(.black)
                        .clipShape(RoundedRectangle(cornerRadius: 16))
                    }
                    .disabled(saving)
                    .padding(.horizontal, 16)
                    .padding(.bottom, 30)
                }
            }
            .background(Theme.bg)
        }
        .navigationTitle(scope == .week ? "Weekly Tracker" : "Monthly Goals")
        .toolbarBackground(Theme.bg, for: .navigationBar)
        .background(Theme.bg)
        .task { await load() }
        .onChange(of: scope) { _, _ in Task { await load() } }
    }

    // ─── Subviews ────────────────────────────────────────────────────────

    private var themeField: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("THEME").font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
            TextField("e.g. Bug bounty backlog clearance", text: $theme)
                .padding(.horizontal, 14).padding(.vertical, 12)
                .background(Color(hex: 0x0f0f0f))
                .clipShape(RoundedRectangle(cornerRadius: 12))
                .foregroundStyle(Theme.text)
                .tint(Theme.ai)
        }
        .padding(.horizontal, 16)
    }

    private var goalsCard: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("GOALS").font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
            ForEach(items) { item in
                HStack(spacing: 12) {
                    Button { toggleItem(item) } label: {
                        Image(systemName: item.done ? "checkmark.circle.fill" : "circle")
                            .foregroundStyle(item.done ? Theme.goals : Theme.text3)
                    }
                    Text(item.text)
                        .strikethrough(item.done)
                        .foregroundStyle(item.done ? Theme.text3 : Theme.text)
                    Spacer()
                    Button { removeItem(item) } label: {
                        Image(systemName: "minus.circle")
                            .foregroundStyle(Theme.text3)
                    }
                }
                .padding(.vertical, 6)
            }
            HStack {
                TextField("Add a goal", text: $newItemText)
                    .tint(Theme.ai)
                Button("Add") { addItem() }
                    .foregroundStyle(Theme.ai)
                    .disabled(newItemText.trimmingCharacters(in: .whitespaces).isEmpty)
            }
            .padding(.horizontal, 12).padding(.vertical, 8)
            .background(Color(hex: 0x0f0f0f))
            .clipShape(RoundedRectangle(cornerRadius: 12))
        }
        .padding(16)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 16))
        .padding(.horizontal, 16)
    }

    private var retroCard: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("RETRO").font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
            TextEditor(text: $retro)
                .scrollContentBackground(.hidden)
                .frame(minHeight: 120)
                .padding(8)
                .background(Color(hex: 0x0f0f0f))
                .clipShape(RoundedRectangle(cornerRadius: 12))
                .foregroundStyle(Theme.text)
                .tint(Theme.ai)
        }
        .padding(.horizontal, 16)
    }

    // ─── Plumbing ────────────────────────────────────────────────────────

    private var weeklyMondayDisplay: String {
        let mon = Self.mondayOf(Date())
        let f = DateFormatter(); f.dateFormat = "yyyy-MM-dd"
        return f.string(from: mon)
    }
    private var monthDisplay: String {
        let f = DateFormatter(); f.dateFormat = "MMMM yyyy"
        return f.string(from: Date())
    }

    private func addItem() {
        let trimmed = newItemText.trimmingCharacters(in: .whitespaces)
        guard !trimmed.isEmpty else { return }
        items.append(DailyItem(id: UUID().uuidString, text: trimmed, done: false,
                               source: "manual", source_ref: nil))
        newItemText = ""
    }
    private func toggleItem(_ item: DailyItem) {
        if let i = items.firstIndex(where: { $0.id == item.id }) {
            items[i].done.toggle()
        }
    }
    private func removeItem(_ item: DailyItem) {
        items.removeAll { $0.id == item.id }
    }

    private func load() async {
        guard let api = app.apiClient() else { loadError = "Not paired."; return }
        loadError = nil
        do {
            if scope == .week {
                let w = try await api.weeklyTracker(weekOf: weeklyMondayDisplay)
                weekly = w
                theme = w.theme
                retro = w.retro_md
                items = w.weekly_goals
            } else {
                let m = try await api.monthlyGoals(month: String(monthDisplay.prefix(0)) + monthIsoKey)
                monthly = m
                retro = m.retro_md
                items = m.monthly_goals
            }
        } catch {
            loadError = (error as? APIClient.APIError)?.errorDescription ?? error.localizedDescription
        }
    }

    private var monthIsoKey: String {
        let f = DateFormatter(); f.dateFormat = "yyyy-MM"
        return f.string(from: Date())
    }

    private func save() async {
        guard let api = app.apiClient() else { return }
        saving = true; defer { saving = false }
        do {
            if scope == .week {
                try await api.putWeekly(weekOf: weeklyMondayDisplay,
                    APIClient.WeeklyInput(theme: theme, weekly_goals: items, retro_md: retro))
            } else {
                try await api.putMonthly(month: monthIsoKey,
                    APIClient.MonthlyInput(monthly_goals: items, retro_md: retro))
            }
            await load()
        } catch {
            loadError = (error as? APIClient.APIError)?.errorDescription ?? error.localizedDescription
        }
    }

    static func mondayOf(_ d: Date) -> Date {
        var cal = Calendar(identifier: .iso8601)
        cal.timeZone = TimeZone(identifier: "UTC") ?? .current
        let weekday = cal.component(.weekday, from: d) // 1=Sun..7=Sat
        let daysBack = (weekday + 5) % 7 // Mon=0
        return cal.startOfDay(for: cal.date(byAdding: .day, value: -daysBack, to: d) ?? d)
    }
}
