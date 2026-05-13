import SwiftUI

struct HomeView: View {
    @EnvironmentObject private var app: AppState

    @State private var summary: FinanceSummary?
    @State private var healthToday: HealthToday?
    @State private var brief: Brief?
    @State private var milestones: [Milestone] = []
    @State private var recentNotifs: [AppNotification] = []
    @State private var supplements: [SupplementDef] = []
    @State private var doseToday: [SupplementDose] = []
    @State private var unread = 0
    @State private var showSettings = false
    @State private var showNotifications = false
    @State private var liveStreamTask: Task<Void, Never>?

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 14) {
                    todayHero
                    quickStatsRow
                    if let flagship = flagshipMilestone {
                        flagshipCard(flagship)
                    }
                    if let b = brief, !b.items.isEmpty {
                        todayBriefCard(b)
                    }
                    if !pendingSupplements.isEmpty {
                        supplementsDueCard
                    }
                    if !recentNotifs.isEmpty {
                        recentNotificationsCard
                    }
                    Spacer(minLength: 8)
                }
                .padding(.horizontal, 16)
                .padding(.top, 8)
            }
            .background(Theme.bg)
            .navigationTitle("Today")
            .toolbarBackground(Theme.bg, for: .navigationBar)
            .toolbar { toolbarButtons }
            .sheet(isPresented: $showSettings) { SettingsView() }
            .sheet(isPresented: $showNotifications, onDismiss: { Task { await loadUnread() } }) {
                NotificationsView()
            }
            .refreshable { await loadAll() }
        }
        .task { await loadAll() }
        .onAppear { startLiveStream() }
        .onDisappear { liveStreamTask?.cancel(); liveStreamTask = nil }
    }

    @ToolbarContentBuilder
    private var toolbarButtons: some ToolbarContent {
        ToolbarItem(placement: .topBarTrailing) {
            HStack(spacing: 8) {
                Button { showNotifications = true } label: {
                    ZStack(alignment: .topTrailing) {
                        Image(systemName: "bell").foregroundStyle(Theme.text)
                        if unread > 0 {
                            Text("\(min(unread, 99))")
                                .font(.system(size: 9, weight: .bold))
                                .padding(.horizontal, 5).padding(.vertical, 2)
                                .background(Theme.ai).foregroundStyle(.black)
                                .clipShape(Capsule()).offset(x: 8, y: -8)
                        }
                    }
                }
                Button { showSettings = true } label: {
                    Image(systemName: "gearshape").foregroundStyle(Theme.text)
                }
            }
        }
    }

    // ─── Hero ────────────────────────────────────────────────────────────

    private var todayHero: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(greeting)
                .font(.footnote).foregroundStyle(Theme.text3)
            Text(verdictText)
                .font(.system(size: 26, weight: .heavy))
                .foregroundStyle(Theme.text)
                .lineLimit(2)
            HStack(spacing: 10) {
                if let s = healthToday?.recovery?.score {
                    miniBadge(icon: "heart.fill", color: recoveryColor(s),
                              text: "\(s)% recovery")
                }
                if let b = brief, b.streak_count > 0 {
                    miniBadge(icon: "flame.fill", color: Theme.goals,
                              text: "\(b.streak_count)-day streak")
                }
                if let s = summary {
                    let pct = s.budgeted_cents > 0
                        ? Int((Double(s.spent_cents) / Double(s.budgeted_cents)) * 100)
                        : 0
                    miniBadge(icon: "dollarsign.circle.fill",
                              color: pct >= 90 ? Theme.financeBad : Theme.finance,
                              text: "\(pct)% MTD")
                }
            }
        }
        .padding(20)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            LinearGradient(colors: [Theme.surfaceHi, Theme.bg],
                           startPoint: .topLeading, endPoint: .bottomTrailing)
        )
        .overlay(
            RoundedRectangle(cornerRadius: 22)
                .stroke(Theme.border.opacity(0.4), lineWidth: 1))
        .clipShape(RoundedRectangle(cornerRadius: 22))
    }

    private func miniBadge(icon: String, color: Color, text: String) -> some View {
        HStack(spacing: 4) {
            Image(systemName: icon).font(.caption2).foregroundStyle(color)
            Text(text).font(.caption).foregroundStyle(Theme.text2)
        }
        .padding(.horizontal, 8).padding(.vertical, 4)
        .background(Theme.surface)
        .clipShape(Capsule())
    }

    private var greeting: String {
        let hour = Calendar.current.component(.hour, from: Date())
        switch hour {
        case 5..<12:  return "Good morning"
        case 12..<17: return "Good afternoon"
        case 17..<22: return "Good evening"
        default:      return "Late night"
        }
    }

    private var verdictText: String {
        if let h = healthToday {
            switch h.verdict {
            case "push":     return "Green light — push today."
            case "maintain": return "Steady — maintain effort."
            case "recover":  return "Recover — pull back today."
            default:         break
            }
        }
        if let s = summary, let worst = s.categories.first(where: { $0.over }) {
            return "Pull back on \(worst.category)"
        }
        return "Pace is healthy"
    }

    // ─── Quick stats row ─────────────────────────────────────────────────

    private var quickStatsRow: some View {
        let cols = [GridItem(.flexible(), spacing: 8),
                    GridItem(.flexible(), spacing: 8)]
        return LazyVGrid(columns: cols, spacing: 8) {
            statCard(icon: "dollarsign.circle.fill", color: Theme.finance,
                     label: "SPEND MTD",
                     value: summary.map { $0.spent_cents.asUSD(decimals: 0) } ?? "—",
                     sub: summaryRemainingSub)
            statCard(icon: "heart.fill", color: Theme.healthGo,
                     label: "RECOVERY",
                     value: healthToday?.recovery?.score.map { "\($0)%" } ?? "—",
                     sub: healthToday?.sleep?.duration_min.map(formatHours) ?? "no sleep data")
            statCard(icon: "star.fill", color: Theme.goals,
                     label: "MILESTONES",
                     value: "\(activeMilestoneCount)",
                     sub: nearestMilestoneSub)
            statCard(icon: "sparkles", color: Theme.ai,
                     label: "ASK",
                     value: "Chat",
                     sub: "Tap to open")
        }
    }

    private var summaryRemainingSub: String {
        guard let s = summary, s.budgeted_cents > 0 else { return "—" }
        let remaining = s.budgeted_cents - s.spent_cents
        if remaining < 0 {
            return "\((-remaining).asUSD(decimals: 0)) over"
        }
        return "\(remaining.asUSD(decimals: 0)) left"
    }
    private var activeMilestoneCount: Int {
        milestones.filter { $0.status != "done" && $0.status != "archived" }.count
    }
    private var nearestMilestoneSub: String {
        let active = milestones
            .filter { $0.status != "done" && $0.status != "archived" && !$0.due_date.isEmpty }
            .sorted { $0.due_date < $1.due_date }
        if let next = active.first {
            return "next: \(next.due_date)"
        }
        return activeMilestoneCount > 0 ? "none with due dates" : "no active"
    }

    private func statCard(icon: String, color: Color, label: String, value: String, sub: String) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            ZStack {
                RoundedRectangle(cornerRadius: 8).fill(color.opacity(0.15))
                Image(systemName: icon).foregroundStyle(color).font(.system(size: 13))
            }
            .frame(width: 26, height: 26)
            Text(label).font(.caption2).bold().tracking(0.5).foregroundStyle(Theme.text3)
            Text(value).font(.system(size: 20, weight: .heavy)).foregroundStyle(Theme.text)
            Spacer(minLength: 0)
            Text(sub).font(.caption2).foregroundStyle(Theme.text3).lineLimit(1)
        }
        .padding(14)
        .frame(maxWidth: .infinity, minHeight: 110, alignment: .topLeading)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 16))
    }

    // ─── Flagship milestone ──────────────────────────────────────────────

    private var flagshipMilestone: Milestone? {
        milestones.first {
            $0.flagship && $0.status != "done" && $0.status != "archived"
        }
    }

    private func flagshipCard(_ m: Milestone) -> some View {
        let pct: CGFloat = m.status == "in_progress" ? 0.5 : 0.1
        return VStack(alignment: .leading, spacing: 10) {
            HStack {
                Image(systemName: "star.fill").foregroundStyle(.white).font(.caption)
                Text("FLAGSHIP").font(.caption2).bold().tracking(1).foregroundStyle(.white)
                Spacer()
                if !m.due_date.isEmpty {
                    Text(m.due_date).font(.caption).foregroundStyle(.white.opacity(0.7))
                }
            }
            Text(m.title)
                .font(.system(size: 22, weight: .heavy))
                .foregroundStyle(.white)
                .lineLimit(2)
            if !m.description_md.isEmpty {
                Text(m.description_md)
                    .font(.footnote).foregroundStyle(.white.opacity(0.8))
                    .lineLimit(2)
            }
            GeometryReader { proxy in
                ZStack(alignment: .leading) {
                    Capsule().fill(Color.white.opacity(0.18)).frame(height: 6)
                    Capsule().fill(.white).frame(width: proxy.size.width * pct, height: 6)
                }
            }
            .frame(height: 6)
        }
        .padding(18)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            LinearGradient(colors: [Theme.goals, Color(hex: 0x4338ca)],
                           startPoint: .topLeading, endPoint: .bottomTrailing)
        )
        .clipShape(RoundedRectangle(cornerRadius: 20))
    }

    // ─── Today's brief ───────────────────────────────────────────────────

    private func todayBriefCard(_ b: Brief) -> some View {
        let items = Array(b.items.prefix(4))
        return VStack(alignment: .leading, spacing: 10) {
            HStack {
                Text("TODAY").font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
                Spacer()
                Text("\(b.items.filter { !$0.done }.count) open")
                    .font(.caption2).foregroundStyle(Theme.text3)
            }
            ForEach(items) { item in
                HStack(spacing: 10) {
                    Image(systemName: item.done ? "checkmark.circle.fill" : "circle")
                        .foregroundStyle(item.done ? Theme.healthGo : Theme.text3)
                    VStack(alignment: .leading, spacing: 1) {
                        Text(item.text)
                            .font(.subheadline)
                            .foregroundStyle(item.done ? Theme.text3 : Theme.text)
                            .strikethrough(item.done, color: Theme.text3)
                        if item.source != "user" {
                            Text(item.source.replacingOccurrences(of: "_", with: " "))
                                .font(.caption2).foregroundStyle(Theme.text3)
                        }
                    }
                    Spacer()
                }
            }
            if b.items.count > items.count {
                Text("+ \(b.items.count - items.count) more on the Goals tab")
                    .font(.caption2).foregroundStyle(Theme.text3)
            }
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 18))
    }

    // ─── Supplements due now ─────────────────────────────────────────────

    private var pendingSupplements: [SupplementDef] {
        let active = supplements.filter { $0.active && $0.reminder_enabled }
        guard !active.isEmpty else { return [] }
        let loggedIds = Set(doseToday.map(\.def_id))
        return active.filter { !loggedIds.contains($0.id) }.prefix(3).map { $0 }
    }

    private var supplementsDueCard: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack {
                Image(systemName: "pills.fill")
                    .foregroundStyle(Theme.healthBlue).font(.caption)
                Text("STACK").font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
                Spacer()
                Text("\(pendingSupplements.count) not logged today")
                    .font(.caption2).foregroundStyle(Theme.text3)
            }
            ForEach(pendingSupplements) { s in
                HStack(spacing: 10) {
                    Circle().fill(Theme.healthBlue.opacity(0.18))
                        .frame(width: 28, height: 28)
                        .overlay(Image(systemName: "circle")
                                    .foregroundStyle(Theme.healthBlue).font(.caption))
                    VStack(alignment: .leading, spacing: 1) {
                        Text(s.name).font(.subheadline).foregroundStyle(Theme.text)
                        if !s.dose.isEmpty {
                            Text(s.dose).font(.caption2).foregroundStyle(Theme.text3)
                        }
                    }
                    Spacer()
                    Text(s.category).font(.caption2).foregroundStyle(Theme.text3)
                }
            }
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 18))
    }

    // ─── Recent notifications preview ────────────────────────────────────

    private var recentNotificationsCard: some View {
        let items = Array(recentNotifs.prefix(3))
        return VStack(alignment: .leading, spacing: 10) {
            HStack {
                Image(systemName: "bell.fill")
                    .foregroundStyle(Theme.ai).font(.caption)
                Text("RECENT")
                    .font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
                Spacer()
                Button("See all") { showNotifications = true }
                    .font(.caption).foregroundStyle(Theme.ai)
            }
            ForEach(items) { n in
                HStack(spacing: 10) {
                    Circle().fill(notifColor(n.category).opacity(0.18))
                        .frame(width: 8, height: 8)
                    VStack(alignment: .leading, spacing: 1) {
                        Text(n.title).font(.subheadline).foregroundStyle(Theme.text).lineLimit(1)
                        Text(n.body).font(.caption2).foregroundStyle(Theme.text3).lineLimit(1)
                    }
                    Spacer()
                }
            }
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 18))
    }

    private func notifColor(_ category: String) -> Color {
        switch category {
        case "purchase":          return Theme.finance
        case "budget_threshold":  return Theme.financeBad
        case "anomaly":           return Theme.ai
        case "supplement":        return Theme.healthBlue
        case "health_insight":    return Theme.healthGo
        case "goal_milestone":    return Theme.goals
        default:                  return Theme.text2
        }
    }

    // ─── Helpers ─────────────────────────────────────────────────────────

    private func recoveryColor(_ score: Int) -> Color {
        switch score {
        case 70...: return Theme.healthGo
        case 40...: return Theme.healthMid
        default:    return Theme.financeBad
        }
    }
    private func formatHours(_ min: Int) -> String {
        let h = min / 60, m = min % 60
        return "\(h)h\(m > 0 ? " \(m)m" : "") sleep"
    }

    // ─── Plumbing ────────────────────────────────────────────────────────

    private func startLiveStream() {
        guard liveStreamTask == nil, let api = app.apiClient() else { return }
        liveStreamTask = Task {
            while !Task.isCancelled {
                do {
                    try await api.notificationsStream { _ in
                        Task { @MainActor in
                            unread += 1
                            await loadRecentNotifs()
                        }
                    }
                } catch {
                    try? await Task.sleep(nanoseconds: 3_000_000_000)
                }
            }
        }
    }

    private func loadAll() async {
        await withTaskGroup(of: Void.self) { g in
            g.addTask { await loadSummary() }
            g.addTask { await loadHealth() }
            g.addTask { await loadGoals() }
            g.addTask { await loadUnread() }
            g.addTask { await loadRecentNotifs() }
            g.addTask { await loadSupplements() }
        }
    }
    private func loadSummary() async {
        guard let api = app.apiClient() else { return }
        summary = try? await api.financeSummary()
    }
    private func loadHealth() async {
        guard let api = app.apiClient() else { return }
        healthToday = try? await api.healthToday()
    }
    private func loadGoals() async {
        guard let api = app.apiClient() else { return }
        async let br = api.brief()
        async let ms = api.listMilestones()
        brief = (try? await br)
        milestones = (try? await ms) ?? []
    }
    private func loadUnread() async {
        guard let api = app.apiClient() else { return }
        if let n = try? await api.notificationUnreadCount() {
            unread = n
        }
    }
    private func loadRecentNotifs() async {
        guard let api = app.apiClient() else { return }
        if let list = try? await api.notifications(limit: 5) {
            recentNotifs = list
        }
    }
    private func loadSupplements() async {
        guard let api = app.apiClient() else { return }
        async let defs = api.supplementDefs()
        async let doses = api.supplementDoses(days: 1)
        supplements = (try? await defs) ?? []
        doseToday = (try? await doses) ?? []
    }
}
