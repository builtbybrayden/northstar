import SwiftUI

/// Habits screen: list of tracked habits with a 90-day GitHub-style heatmap.
/// Tap a habit cell on a date to toggle done/skip for that day.
struct HabitsView: View {
    @EnvironmentObject private var app: AppState

    @State private var habits: [HabitWithStats] = []
    @State private var loadError: String?
    @State private var showingNew = false

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 14) {
                if !habits.isEmpty {
                    ForEach(habits) { hs in
                        HabitCard(stats: hs) { date in
                            Task { await toggle(habit: hs.habit, date: date) }
                        }
                    }
                } else if loadError != nil {
                    errorCard
                } else {
                    emptyCard
                }
            }
            .padding(.horizontal, 16)
            .padding(.top, 8)
            .padding(.bottom, 30)
        }
        .background(Theme.bg)
        .navigationTitle("Habits")
        .toolbarBackground(Theme.bg, for: .navigationBar)
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                Button { showingNew = true } label: {
                    Image(systemName: "plus")
                        .foregroundStyle(Theme.text2)
                }
            }
        }
        .task { await load() }
        .refreshable { await load() }
        .sheet(isPresented: $showingNew) {
            NewHabitSheet { await load() }
        }
    }

    private var emptyCard: some View {
        VStack(spacing: 8) {
            Image(systemName: "target")
                .font(.title)
                .foregroundStyle(Theme.text3)
            Text("No habits yet.")
                .font(.footnote)
                .foregroundStyle(Theme.text2)
            Button("Add one") { showingNew = true }
                .font(.system(.footnote, weight: .semibold))
                .foregroundStyle(Theme.ai)
        }
        .padding(20)
        .frame(maxWidth: .infinity)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 20))
    }

    private var errorCard: some View {
        VStack(spacing: 8) {
            Image(systemName: "exclamationmark.triangle.fill")
                .foregroundStyle(Theme.financeBad)
            Text(loadError ?? "Couldn't load habits.")
                .multilineTextAlignment(.center)
                .foregroundStyle(Theme.text2)
                .font(.footnote)
            Button("Retry") { Task { await load() } }
                .foregroundStyle(Theme.ai)
                .font(.system(.footnote, weight: .semibold))
        }
        .padding(20)
        .frame(maxWidth: .infinity)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 20))
    }

    private func load() async {
        guard let api = app.apiClient() else {
            loadError = "Not paired."
            return
        }
        do {
            habits = try await api.listHabits()
            loadError = nil
        } catch let e as APIClient.APIError {
            loadError = e.errorDescription
        } catch let e {
            loadError = e.localizedDescription
        }
    }

    private func toggle(habit: Habit, date: String) async {
        guard let api = app.apiClient() else { return }
        do {
            _ = try await api.toggleHabitLog(habitID: habit.id, date: date)
            await load()
        } catch {
            // Surface silently for now — list reload will reveal the
            // canonical state on next refresh anyway.
        }
    }
}

// ─── Habit card ───────────────────────────────────────────────────────────

private struct HabitCard: View {
    let stats: HabitWithStats
    let onCellTap: (String) -> Void

    private static let weeks = 13                 // ~90 days
    private static let cellSize: CGFloat = 14
    private static let cellSpacing: CGFloat = 3

    private static let iso: DateFormatter = {
        let f = DateFormatter()
        f.dateFormat = "yyyy-MM-dd"
        f.locale = Locale(identifier: "en_US_POSIX")
        return f
    }()

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            header
            heatmap
            footer
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 20))
    }

    private var header: some View {
        HStack(alignment: .top) {
            VStack(alignment: .leading, spacing: 2) {
                Text(stats.habit.name)
                    .font(.system(size: 17, weight: .semibold))
                if !stats.habit.description_md.isEmpty {
                    Text(stats.habit.description_md)
                        .font(.caption)
                        .foregroundStyle(Theme.text3)
                        .lineLimit(2)
                }
            }
            Spacer()
            VStack(alignment: .trailing, spacing: 2) {
                Text("\(stats.streak_days)d streak")
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(stats.streak_days > 0 ? Theme.finance : Theme.text3)
                Text("\(stats.done_last_30)/30 last 30d")
                    .font(.caption2)
                    .foregroundStyle(Theme.text3)
            }
        }
    }

    private var heatmap: some View {
        let cells = buildCells()
        let columns = (0..<Self.weeks).map { _ in
            GridItem(.fixed(Self.cellSize), spacing: Self.cellSpacing)
        }
        return LazyHGrid(rows: columns, spacing: Self.cellSpacing) {
            ForEach(0..<(Self.weeks * 7), id: \.self) { i in
                let cell = cells[safe: i]
                Button {
                    if let date = cell?.date { onCellTap(date) }
                } label: {
                    RoundedRectangle(cornerRadius: 3)
                        .fill(cellColor(cell))
                        .frame(width: Self.cellSize, height: Self.cellSize)
                }
                .buttonStyle(.plain)
                .disabled(cell == nil)
            }
        }
        .frame(height: 7 * Self.cellSize + 6 * Self.cellSpacing)
    }

    private var footer: some View {
        HStack(spacing: 8) {
            ForEach(0..<4) { tier in
                RoundedRectangle(cornerRadius: 2)
                    .fill(legendColor(for: tier))
                    .frame(width: 10, height: 10)
            }
            Text("more").font(.caption2).foregroundStyle(Theme.text3)
            Spacer()
            Text("\(stats.habit.target_per_week)/wk target")
                .font(.caption2).foregroundStyle(Theme.text3)
        }
    }

    // ─── data layout ──────────────────────────────────────────────────

    private struct HeatCell {
        let date: String
        let count: Int
    }

    private func buildCells() -> [HeatCell?] {
        // Build a column-major grid: 7 rows × Self.weeks columns, oldest on
        // the left, newest on the right, today in the rightmost column at
        // its weekday row. Each column = one ISO week ending today.
        let totalDays = Self.weeks * 7
        let counts = Dictionary(uniqueKeysWithValues: stats.entries.map { ($0.date, $0.count) })
        let today = Self.iso.string(from: Date())
        guard let todayDate = Self.iso.date(from: today) else { return [] }
        let cal = Calendar(identifier: .gregorian)

        // Column 0 = oldest week, last column = current week. Within each
        // column, row 0 = Monday. Anchor on today; days after today are nil.
        var cells: [HeatCell?] = Array(repeating: nil, count: totalDays)
        for offset in 0..<totalDays {
            let date = cal.date(byAdding: .day, value: -offset, to: todayDate)!
            let key = Self.iso.string(from: date)
            // Place newest (offset=0) at column = weeks-1, row = weekday-of-today.
            // We render via LazyHGrid which is row-major, so map (row, col) → index.
            let weekdayIndex = (cal.component(.weekday, from: date) + 5) % 7 // Mon=0..Sun=6
            let col = (Self.weeks - 1) - (offset / 7)
            let row = weekdayIndex
            if col < 0 { break }
            let idx = row * Self.weeks + col
            if idx >= 0 && idx < cells.count {
                cells[idx] = HeatCell(date: key, count: counts[key] ?? -1)
            }
        }
        return cells
    }

    private func cellColor(_ cell: HeatCell?) -> Color {
        guard let cell else { return Color.clear }
        if cell.count < 0 { return Theme.surfaceHi }          // no record
        if cell.count == 0 { return Theme.text3.opacity(0.2) } // skipped
        if cell.count == 1 { return Theme.goals.opacity(0.55) }
        if cell.count <= 3 { return Theme.goals.opacity(0.75) }
        return Theme.goals
    }

    private func legendColor(for tier: Int) -> Color {
        switch tier {
        case 0: return Theme.surfaceHi
        case 1: return Theme.goals.opacity(0.55)
        case 2: return Theme.goals.opacity(0.75)
        default: return Theme.goals
        }
    }
}

private extension Array {
    subscript(safe i: Int) -> Element? {
        indices.contains(i) ? self[i] : nil
    }
}

// ─── New habit sheet ──────────────────────────────────────────────────────

private struct NewHabitSheet: View {
    let onSaved: () async -> Void
    @EnvironmentObject private var app: AppState
    @Environment(\.dismiss) private var dismiss

    @State private var name = ""
    @State private var description = ""
    @State private var target = 7
    @State private var saving = false
    @State private var saveError: String?

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    TextField("Name", text: $name)
                    TextField("Description (optional)", text: $description, axis: .vertical)
                        .lineLimit(2...4)
                }
                Section("Target") {
                    Stepper("\(target) day\(target == 1 ? "" : "s") per week",
                            value: $target, in: 1...7)
                }
                if let saveError {
                    Section { Text(saveError).foregroundStyle(Theme.financeBad) }
                }
            }
            .navigationTitle("New habit")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Save") { Task { await save() } }
                        .disabled(name.trimmingCharacters(in: .whitespaces).isEmpty || saving)
                }
            }
        }
    }

    private func save() async {
        guard let api = app.apiClient() else { return }
        saving = true
        defer { saving = false }
        do {
            _ = try await api.createHabit(.init(
                name: name,
                description_md: description.isEmpty ? nil : description,
                target_per_week: target
            ))
            await onSaved()
            dismiss()
        } catch let e as APIClient.APIError {
            saveError = e.errorDescription
        } catch let e {
            saveError = e.localizedDescription
        }
    }
}
