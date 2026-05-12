import SwiftUI

struct GoalsView: View {
    @EnvironmentObject private var app: AppState

    @State private var brief: Brief?
    @State private var milestones: [Milestone] = []
    @State private var loadError: String?
    @State private var editingMilestone: Milestone?
    @State private var showingAdd = false

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 14) {
                    if let flagship = flagshipMilestone {
                        flagshipHero(flagship)
                    }
                    if let brief {
                        todaysBrief(brief)
                    }
                    longHorizonCard
                    moreSection
                    if let loadError {
                        Text(loadError).font(.footnote).foregroundStyle(Theme.financeBad)
                    }
                }
                .padding(.horizontal, 16)
                .padding(.vertical, 8)
            }
            .background(Theme.bg)
            .navigationTitle("Goals")
            .toolbarBackground(Theme.bg, for: .navigationBar)
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    HStack(spacing: 12) {
                        if let streak = brief?.streak_count, streak > 0 {
                            HStack(spacing: 4) {
                                Image(systemName: "flame.fill")
                                    .font(.system(size: 12))
                                Text("\(streak)")
                                    .font(.system(.caption, weight: .bold))
                            }
                            .padding(.horizontal, 10).padding(.vertical, 4)
                            .background(Theme.ai.opacity(0.15))
                            .foregroundStyle(Theme.ai)
                            .clipShape(Capsule())
                        }
                        Button { showingAdd = true } label: {
                            Image(systemName: "plus.circle.fill")
                                .foregroundStyle(Theme.text)
                        }
                    }
                }
            }
            .refreshable { await load() }
            .sheet(isPresented: $showingAdd, onDismiss: { Task { await load() } }) {
                MilestoneEditSheet(milestone: nil)
            }
            .sheet(item: $editingMilestone, onDismiss: { Task { await load() } }) { m in
                MilestoneEditSheet(milestone: m)
            }
        }
        .task { await load() }
    }

    // ─── Hero ────────────────────────────────────────────────────────────

    private var flagshipMilestone: Milestone? {
        milestones.first(where: { $0.flagship && $0.status != "archived" && $0.status != "done" })
    }

    private func flagshipHero(_ m: Milestone) -> some View {
        Button { editingMilestone = m } label: {
            VStack(alignment: .leading, spacing: 4) {
                Text("FLAGSHIP MILESTONE")
                    .font(.caption2).bold().tracking(1.5)
                    .foregroundStyle(Theme.goals)
                Text(m.title)
                    .font(.system(size: 22, weight: .heavy))
                    .foregroundStyle(Theme.text)
                    .lineLimit(2)
                    .multilineTextAlignment(.leading)
                if !m.due_date.isEmpty {
                    Text("due \(m.due_date) · \(timeUntil(m.due_date))")
                        .font(.caption).foregroundStyle(Theme.text2)
                }
                progressBar(for: m)
            }
            .padding(20)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(
                LinearGradient(colors: [Color(hex: 0x1c1b3d), Color(hex: 0x0a0a14)],
                               startPoint: .topLeading, endPoint: .bottomTrailing)
            )
            .overlay(RoundedRectangle(cornerRadius: 22)
                .stroke(Theme.goals.opacity(0.2), lineWidth: 1))
            .clipShape(RoundedRectangle(cornerRadius: 22))
        }
        .buttonStyle(.plain)
    }

    private func progressBar(for m: Milestone) -> some View {
        // Rough progress estimate: status-driven for now. Real estimate from
        // sub-tasks lands when we add per-milestone subtasks.
        let pct: CGFloat = m.status == "in_progress" ? 0.5 : m.status == "done" ? 1.0 : 0.1
        return GeometryReader { proxy in
            ZStack(alignment: .leading) {
                RoundedRectangle(cornerRadius: 4).fill(Color.white.opacity(0.06))
                RoundedRectangle(cornerRadius: 4)
                    .fill(LinearGradient(colors: [Theme.goals, Color(hex: 0x8a87ff)],
                                         startPoint: .leading, endPoint: .trailing))
                    .frame(width: proxy.size.width * pct)
            }
        }
        .frame(height: 8)
        .padding(.top, 10)
    }

    // ─── Today's brief ───────────────────────────────────────────────────

    private func todaysBrief(_ b: Brief) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text("TODAY'S BRIEF")
                    .font(.caption2).bold().tracking(1)
                    .foregroundStyle(Theme.text3)
                Spacer()
                let done = b.items.filter(\.done).count
                Text("\(done) / \(b.items.count) done")
                    .font(.caption2)
                    .foregroundStyle(Theme.text3)
                    .padding(.horizontal, 8).padding(.vertical, 2)
                    .background(Color.white.opacity(0.08))
                    .clipShape(Capsule())
            }
            .padding(.bottom, 4)

            if b.items.isEmpty {
                Text("No tasks for today yet.")
                    .foregroundStyle(Theme.text3)
                    .font(.footnote)
                    .padding(.vertical, 12)
            } else {
                ForEach(b.items) { item in
                    DailyItemRow(item: item) { newDone in
                        await toggleItem(item.id, done: newDone)
                    }
                    if item.id != b.items.last?.id {
                        Divider().overlay(Theme.border)
                    }
                }
            }
        }
        .padding(18)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 20))
    }

    // ─── Long horizon ────────────────────────────────────────────────────

    private var longHorizonCard: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text("ALL MILESTONES")
                    .font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
                Spacer()
                Text("\(milestones.count)")
                    .font(.caption2).foregroundStyle(Theme.text3)
                    .padding(.horizontal, 8).padding(.vertical, 2)
                    .background(Color.white.opacity(0.08))
                    .clipShape(Capsule())
            }
            if milestones.isEmpty {
                VStack(spacing: 6) {
                    Text("No milestones yet.")
                        .foregroundStyle(Theme.text3).font(.footnote)
                    Button("Add one") { showingAdd = true }
                        .foregroundStyle(Theme.ai)
                        .font(.system(.footnote, weight: .semibold))
                }
                .frame(maxWidth: .infinity, minHeight: 100)
            }
            ForEach(milestones) { m in
                Button { editingMilestone = m } label: {
                    MilestoneRow(milestone: m)
                }
                .buttonStyle(.plain)
                if m.id != milestones.last?.id {
                    Divider().overlay(Theme.border)
                }
            }
        }
        .padding(18)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 20))
    }

    // ─── More section ────────────────────────────────────────────────────

    private var moreSection: some View {
        VStack(alignment: .leading, spacing: 0) {
            Text("PLANNERS & LOGS")
                .font(.caption2).bold().tracking(1)
                .foregroundStyle(Theme.text3)
                .padding(.bottom, 8).padding(.leading, 4)

            VStack(spacing: 0) {
                navRow(icon: "calendar", color: Theme.goals,
                       title: "Weekly & monthly planner",
                       sub: "Theme, goals, retro") {
                    WeeklyMonthlyView()
                }
                Divider().overlay(Theme.border).padding(.leading, 56)
                navRow(icon: "doc.text.fill", color: Theme.healthBlue,
                       title: "Output log",
                       sub: "CVEs, blogs, talks, tools shipped") {
                    OutputLogView()
                }
                Divider().overlay(Theme.border).padding(.leading, 56)
                navRow(icon: "person.2.fill", color: Theme.ai,
                       title: "Networking log",
                       sub: "People + next actions") {
                    NetworkingLogView()
                }
                Divider().overlay(Theme.border).padding(.leading, 56)
                navRow(icon: "bell.fill", color: Theme.healthMid,
                       title: "Reminders",
                       sub: "Cron-driven nudges") {
                    RemindersView()
                }
            }
            .background(Theme.surface)
            .clipShape(RoundedRectangle(cornerRadius: 20))
        }
    }

    private func navRow<Dest: View>(icon: String, color: Color, title: String, sub: String,
                                    @ViewBuilder destination: () -> Dest) -> some View {
        NavigationLink(destination: destination()) {
            HStack(spacing: 12) {
                ZStack {
                    RoundedRectangle(cornerRadius: 8).fill(color.opacity(0.18))
                    Image(systemName: icon).foregroundStyle(color).font(.system(size: 14))
                }
                .frame(width: 32, height: 32)
                VStack(alignment: .leading, spacing: 2) {
                    Text(title).font(.system(size: 15, weight: .medium))
                        .foregroundStyle(Theme.text)
                    Text(sub).font(.caption).foregroundStyle(Theme.text3)
                }
                Spacer()
                Image(systemName: "chevron.right").foregroundStyle(Theme.text3).font(.caption)
            }
            .padding(.horizontal, 14).padding(.vertical, 12)
        }
        .buttonStyle(.plain)
    }

    // ─── Plumbing ────────────────────────────────────────────────────────

    private func load() async {
        guard let api = app.apiClient() else { loadError = "Not paired."; return }
        do {
            async let br = api.brief()
            async let ms = api.listMilestones()
            self.brief = try await br
            self.milestones = try await ms
            self.loadError = nil
        } catch let e as APIClient.APIError {
            loadError = e.errorDescription
        } catch let e {
            loadError = e.localizedDescription
        }
    }

    private func toggleItem(_ id: String, done: Bool) async {
        guard let api = app.apiClient(), var b = brief else { return }
        if let i = b.items.firstIndex(where: { $0.id == id }) {
            // Optimistic update
            var copy = b
            copy.items = copy.items
            // The above doesn't actually mutate; rebuild explicitly:
            var items = copy.items
            items[i].done = done
            // Rebuild Brief with updated items
            brief = Brief(date: b.date,
                          items: items,
                          streak_count: b.streak_count,
                          milestones_due_soon: b.milestones_due_soon,
                          active_reminders: b.active_reminders)
            do {
                _ = try await api.putDailyLog(date: nil,
                    APIClient.DailyLogInput(items: items, reflection_md: nil))
            } catch {
                // Best-effort: reload on failure to resync state
                await load()
            }
        }
    }

    private func timeUntil(_ dateStr: String) -> String {
        let formatter = DateFormatter()
        formatter.dateFormat = "yyyy-MM-dd"
        guard let d = formatter.date(from: dateStr) else { return "" }
        let days = Calendar.current.dateComponents([.day], from: Date(), to: d).day ?? 0
        if days < 0 { return "\(-days)d overdue" }
        if days == 0 { return "today" }
        if days < 30 { return "\(days)d out" }
        let months = days / 30
        return "\(months)mo out"
    }
}

// ─── Subviews ────────────────────────────────────────────────────────────

private struct DailyItemRow: View {
    let item: DailyItem
    let onToggle: (Bool) async -> Void

    var body: some View {
        Button {
            Task { await onToggle(!item.done) }
        } label: {
            HStack(spacing: 12) {
                ZStack {
                    Circle()
                        .stroke(item.done ? Theme.goals : Theme.text3, lineWidth: 1.5)
                        .frame(width: 22, height: 22)
                    if item.done {
                        Circle().fill(Theme.goals).frame(width: 22, height: 22)
                        Image(systemName: "checkmark")
                            .font(.system(size: 11, weight: .bold))
                            .foregroundStyle(.white)
                    }
                }
                VStack(alignment: .leading, spacing: 2) {
                    Text(item.text)
                        .font(.system(size: 14, weight: .medium))
                        .foregroundStyle(item.done ? Theme.text3 : Theme.text)
                        .strikethrough(item.done, color: Theme.text3)
                        .multilineTextAlignment(.leading)
                    if item.source != "manual" {
                        Text(sourceLabel(item.source))
                            .font(.caption2).foregroundStyle(Theme.text3)
                    }
                }
                Spacer()
            }
            .padding(.vertical, 8)
        }
        .buttonStyle(.plain)
    }

    private func sourceLabel(_ s: String) -> String {
        switch s {
        case "reminder":  return "from reminder"
        case "milestone": return "milestone"
        case "rollover":  return "rolled over from yesterday"
        default:          return s
        }
    }
}

private struct MilestoneRow: View {
    let milestone: Milestone

    var body: some View {
        HStack(spacing: 12) {
            ZStack {
                RoundedRectangle(cornerRadius: 10)
                    .fill(statusColor.opacity(0.18))
                Image(systemName: statusIcon)
                    .foregroundStyle(statusColor)
            }
            .frame(width: 36, height: 36)

            VStack(alignment: .leading, spacing: 2) {
                Text(milestone.title)
                    .font(.system(size: 15, weight: .semibold))
                    .foregroundStyle(Theme.text)
                    .lineLimit(1)
                HStack(spacing: 6) {
                    Text(statusLabel)
                    if !milestone.due_date.isEmpty {
                        Text("·").foregroundStyle(Theme.text3)
                        Text("due \(milestone.due_date)")
                    }
                    if milestone.flagship {
                        Text("·").foregroundStyle(Theme.text3)
                        Text("flagship")
                            .foregroundStyle(Theme.ai)
                    }
                }
                .font(.caption).foregroundStyle(Theme.text3)
            }
            Spacer()
            Image(systemName: "chevron.right")
                .foregroundStyle(Theme.text3)
                .font(.caption)
        }
        .padding(.vertical, 8)
    }

    private var statusIcon: String {
        switch milestone.status {
        case "done":        return "checkmark"
        case "in_progress": return "circle.dashed"
        case "archived":    return "archivebox"
        default:            return "circle"
        }
    }
    private var statusColor: Color {
        switch milestone.status {
        case "done":        return Theme.healthGo
        case "in_progress": return Theme.ai
        case "archived":    return Theme.text3
        default:            return Theme.goals
        }
    }
    private var statusLabel: String {
        switch milestone.status {
        case "done":        return "done"
        case "in_progress": return "in progress"
        case "archived":    return "archived"
        default:            return "pending"
        }
    }
}
