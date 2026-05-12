import SwiftUI

struct HomeView: View {
    @EnvironmentObject private var app: AppState
    @State private var summary: FinanceSummary?
    @State private var unread = 0
    @State private var showSettings = false
    @State private var showNotifications = false
    @State private var liveStreamTask: Task<Void, Never>?

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    todayHero
                    pillarGrid
                    Spacer(minLength: 0)
                }
                .padding(.horizontal, 16)
                .padding(.top, 8)
            }
            .background(Theme.bg)
            .navigationTitle("Today")
            .toolbarBackground(Theme.bg, for: .navigationBar)
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    HStack(spacing: 8) {
                        Button { showNotifications = true } label: {
                            ZStack(alignment: .topTrailing) {
                                Image(systemName: "bell")
                                    .foregroundStyle(Theme.text)
                                if unread > 0 {
                                    Text("\(min(unread, 99))")
                                        .font(.system(size: 9, weight: .bold))
                                        .padding(.horizontal, 5).padding(.vertical, 2)
                                        .background(Theme.ai)
                                        .foregroundStyle(.black)
                                        .clipShape(Capsule())
                                        .offset(x: 8, y: -8)
                                }
                            }
                        }
                        Button { showSettings = true } label: {
                            Image(systemName: "gearshape")
                                .foregroundStyle(Theme.text)
                        }
                    }
                }
            }
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

    private func startLiveStream() {
        guard liveStreamTask == nil, let api = app.apiClient() else { return }
        liveStreamTask = Task {
            while !Task.isCancelled {
                do {
                    try await api.notificationsStream { _ in
                        // Any push from the server bumps the unread count.
                        Task { @MainActor in unread += 1 }
                    }
                } catch {
                    try? await Task.sleep(nanoseconds: 3_000_000_000)
                }
            }
        }
    }

    // ─── Hero ────────────────────────────────────────────────────────────

    private var todayHero: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("Today's call")
                .font(.footnote).foregroundStyle(Theme.text3)
            Text(verdictText)
                .font(.system(size: 28, weight: .heavy))
                .foregroundStyle(Theme.text)
            Text(subText)
                .font(.subheadline)
                .foregroundStyle(Theme.text2)
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

    private var verdictText: String {
        guard let s = summary else { return "Loading…" }
        // Phase 2: simple verdict driven by budget pace. Health verdict lands Phase 4.
        if let worst = s.categories.first(where: { $0.over }) {
            return "Pull back on \(worst.category)"
        }
        return "Pace is healthy"
    }
    private var subText: String {
        guard let s = summary else { return "Pulling your latest." }
        let over = s.categories.filter(\.over).count
        let near = s.categories.filter { !$0.over && $0.pct >= 90 }.count
        if over == 0 && near == 0 {
            return "\(s.categories.count) tracked categories, none above 90% yet."
        }
        var parts: [String] = []
        if over > 0 { parts.append("\(over) over") }
        if near > 0 { parts.append("\(near) near 90%") }
        return parts.joined(separator: " · ")
    }

    // ─── Pillars ─────────────────────────────────────────────────────────

    private var pillarGrid: some View {
        let cols = [GridItem(.flexible(), spacing: 10), GridItem(.flexible(), spacing: 10)]
        return LazyVGrid(columns: cols, spacing: 10) {
            pillarCard(icon: "dollarsign.circle.fill", color: Theme.finance,
                       label: "Spend MTD",
                       value: summary.map { $0.spent_cents.asUSD(decimals: 0) } ?? "—",
                       sub: summary.map { "of \($0.budgeted_cents.asUSD(decimals: 0)) budgeted" } ?? "")
            pillarCard(icon: "heart.fill", color: Theme.healthGo,
                       label: "Recovery",
                       value: "—", sub: "WHOOP — Phase 4")
            pillarCard(icon: "star.fill", color: Theme.goals,
                       label: "Today",
                       value: "—", sub: "Goals — Phase 3")
            pillarCard(icon: "sparkles", color: Theme.ai,
                       label: "Ask",
                       value: "—", sub: "Claude — Phase 5")
        }
    }

    private func pillarCard(icon: String, color: Color, label: String, value: String, sub: String) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            ZStack {
                RoundedRectangle(cornerRadius: 8)
                    .fill(color.opacity(0.15))
                Image(systemName: icon).foregroundStyle(color).font(.system(size: 13))
            }
            .frame(width: 26, height: 26)
            Text(label)
                .font(.caption2).bold().tracking(0.5).foregroundStyle(Theme.text3)
            Text(value)
                .font(.system(size: 20, weight: .heavy))
                .foregroundStyle(Theme.text)
            Spacer(minLength: 0)
            Text(sub)
                .font(.caption2).foregroundStyle(Theme.text3)
        }
        .padding(14)
        .frame(maxWidth: .infinity, minHeight: 100, alignment: .topLeading)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 16))
    }

    // ─── Plumbing ────────────────────────────────────────────────────────

    private func loadAll() async {
        await loadSummary()
        await loadUnread()
    }
    private func loadSummary() async {
        guard let api = app.apiClient() else { return }
        summary = try? await api.financeSummary()
    }
    private func loadUnread() async {
        guard let api = app.apiClient() else { return }
        if let n = try? await api.notificationUnreadCount() {
            unread = n
        }
    }
}
