import SwiftUI

struct HealthView: View {
    @EnvironmentObject private var app: AppState

    @State private var today: HealthToday?
    @State private var recoveryWindow: [RecoveryRow] = []
    @State private var supplements: [SupplementDef] = []
    @State private var todaysDoses: [SupplementDose] = []
    @State private var loadError: String?
    @State private var loggingID: String?
    @State private var editingDef: SupplementDef?
    @State private var showingAdd = false

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 14) {
                    recoveryHero
                    metricsGrid
                    sevenDaySpark
                    supplementsCard
                    if let loadError {
                        Text(loadError).foregroundStyle(Theme.financeBad).font(.footnote)
                    }
                }
                .padding(.horizontal, 16)
                .padding(.vertical, 8)
            }
            .background(Theme.bg)
            .navigationTitle("Health")
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
                SupplementEditSheet(supplement: nil)
            }
            .sheet(item: $editingDef, onDismiss: { Task { await load() } }) { def in
                SupplementEditSheet(supplement: def)
            }
        }
        .task { await load() }
    }

    // ─── Hero ────────────────────────────────────────────────────────────

    private var recoveryHero: some View {
        VStack(spacing: 12) {
            let score = today?.recovery?.score ?? 0
            ZStack {
                Circle()
                    .stroke(Color(hex: 0x1a1a1a), lineWidth: 14)
                Circle()
                    .trim(from: 0, to: CGFloat(score) / 100)
                    .stroke(recoveryColor(score: score),
                            style: StrokeStyle(lineWidth: 14, lineCap: .round))
                    .rotationEffect(.degrees(-90))
                VStack(spacing: 2) {
                    Text("\(score)")
                        .font(.system(size: 64, weight: .heavy))
                        .foregroundStyle(recoveryColor(score: score))
                    Text("% RECOVERED")
                        .font(.caption2).bold().tracking(1.5)
                        .foregroundStyle(Theme.text3)
                }
            }
            .frame(width: 180, height: 180)
            .padding(.top, 6)

            if let t = today, !t.verdict.isEmpty {
                Text(verdictLine(verdict: t.verdict))
                    .font(.system(size: 22, weight: .bold))
                    .foregroundStyle(recoveryColor(score: today?.recovery?.score ?? 0))
            }
            if let t = today, !t.strain_goal.isEmpty {
                Text("Strain target today: \(t.strain_goal)")
                    .font(.footnote).foregroundStyle(Theme.text2)
            }
        }
        .padding(20)
        .frame(maxWidth: .infinity)
        .background(
            RadialGradient(colors: [recoveryColor(score: today?.recovery?.score ?? 0).opacity(0.2),
                                    Color.clear],
                           center: .top, startRadius: 0, endRadius: 200)
            .background(Theme.surface)
        )
        .clipShape(RoundedRectangle(cornerRadius: 24))
    }

    // ─── Metrics grid ────────────────────────────────────────────────────

    private var metricsGrid: some View {
        let cols = [GridItem(.flexible(), spacing: 8),
                    GridItem(.flexible(), spacing: 8),
                    GridItem(.flexible(), spacing: 8)]
        return LazyVGrid(columns: cols, spacing: 8) {
            metric(label: "HRV",
                   value: today?.recovery?.hrv_ms.map { "\(Int($0))" } ?? "—",
                   unit: "ms",
                   delta: hrvDelta())
            metric(label: "RHR",
                   value: today?.recovery?.rhr.map { "\($0)" } ?? "—",
                   unit: "bpm",
                   delta: rhrDelta())
            metric(label: "Sleep",
                   value: sleepDisplay,
                   unit: today?.sleep?.score.map { "\($0)% score" } ?? "",
                   delta: sleepDelta())
        }
    }

    private func metric(label: String, value: String, unit: String, delta: String? = nil) -> some View {
        // Always reserve the bottom delta line so all three boxes render at
        // the same height regardless of whether a delta is available.
        let deltaText = delta ?? "—"
        let deltaColor: Color = {
            guard let delta else { return Theme.text3 }
            return delta.hasPrefix("▼") ? Theme.financeBad : Theme.healthGo
        }()
        return VStack(spacing: 2) {
            Text(label).font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
            HStack(alignment: .firstTextBaseline, spacing: 2) {
                Text(value).font(.system(size: 22, weight: .bold))
                    .foregroundStyle(Theme.text)
                if !unit.isEmpty {
                    Text(unit).font(.caption2).foregroundStyle(Theme.text3)
                }
            }
            Text(deltaText).font(.caption2).bold()
                .foregroundStyle(deltaColor)
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, 12)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 14))
    }

    // ─── 7-day spark ─────────────────────────────────────────────────────

    private var sevenDaySpark: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("7-DAY RECOVERY")
                .font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
            HStack(alignment: .bottom, spacing: 3) {
                let sorted = recoveryWindow.prefix(7).reversed()
                ForEach(Array(sorted), id: \.date) { row in
                    let score = row.score ?? 0
                    RoundedRectangle(cornerRadius: 2)
                        .fill(recoveryColor(score: score))
                        .frame(height: max(8, CGFloat(score) * 0.4))
                        .frame(maxWidth: .infinity)
                        .opacity(0.85)
                }
            }
            .frame(height: 50)
            HStack {
                Text("oldest").font(.caption2).foregroundStyle(Theme.text3)
                Spacer()
                Text("today").font(.caption2).foregroundStyle(Theme.text3)
            }
        }
        .padding(14)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 14))
    }

    // ─── Supplements card ────────────────────────────────────────────────

    private var supplementsCard: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text("SUPPLEMENTS & PEPTIDES")
                    .font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
                Spacer()
                let logged = doseCountToday()
                Text("\(logged) / \(supplements.count) today")
                    .font(.caption2)
                    .foregroundStyle(Theme.text3)
                    .padding(.horizontal, 8).padding(.vertical, 2)
                    .background(Color.white.opacity(0.08))
                    .clipShape(Capsule())
            }
            .padding(.bottom, 4)

            if supplements.isEmpty {
                VStack(spacing: 4) {
                    Text("No supplements tracked yet.")
                        .foregroundStyle(Theme.text3).font(.footnote)
                    Button("Add one") { showingAdd = true }
                        .foregroundStyle(Theme.ai)
                        .font(.system(.footnote, weight: .semibold))
                }
                .frame(maxWidth: .infinity, minHeight: 80)
            }
            ForEach(supplements) { def in
                SupplementRow(
                    def: def,
                    doneToday: dosedToday(def.id),
                    logging: loggingID == def.id,
                    onTap: { editingDef = def },
                    onLog: { Task { await logDose(def.id) } }
                )
                if def.id != supplements.last?.id {
                    Divider().overlay(Theme.border)
                }
            }
        }
        .padding(18)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 20))
    }

    // ─── Plumbing ────────────────────────────────────────────────────────

    private var sleepDisplay: String {
        guard let s = today?.sleep, let dur = s.duration_min else { return "—" }
        let h = dur / 60
        let m = dur % 60
        return String(format: "%d:%02d", h, m)
    }

    private func hrvDelta() -> String? {
        guard let today = today?.recovery?.hrv_ms,
              recoveryWindow.count >= 2,
              let yesterday = recoveryWindow.dropFirst().first?.hrv_ms else { return nil }
        let pct = ((today - yesterday) / yesterday) * 100
        let arrow = pct >= 0 ? "▲" : "▼"
        return "\(arrow) \(Int(abs(pct.rounded())))%"
    }
    private func rhrDelta() -> String? {
        guard let today = today?.recovery?.rhr,
              recoveryWindow.count >= 2,
              let yesterday = recoveryWindow.dropFirst().first?.rhr else { return nil }
        let d = today - yesterday
        if d == 0 { return "—" }
        return d < 0 ? "▼ \(abs(d))" : "▲ \(d)"
    }
    private func sleepDelta() -> String? {
        // Show sleep debt if any; falls back to score-change later if we
        // start pulling a window of sleep history.
        guard let debt = today?.sleep?.debt_min else { return nil }
        if debt <= 0 { return "no debt" }
        let h = debt / 60
        let m = debt % 60
        return h > 0 ? "▼ \(h)h\(m > 0 ? " \(m)m" : "") debt" : "▼ \(m)m debt"
    }

    private func recoveryColor(score: Int) -> Color {
        if score >= 67 { return Theme.healthGo }
        if score >= 34 { return Theme.healthMid }
        return Theme.healthStop
    }
    private func verdictLine(verdict: String) -> String {
        switch verdict {
        case "push":     return "Green — push hard"
        case "maintain": return "Yellow — maintain"
        case "recover":  return "Red — recover"
        default:         return verdict.capitalized
        }
    }

    private func dosedToday(_ defID: String) -> Bool {
        let start = Calendar.current.startOfDay(for: Date()).timeIntervalSince1970
        return todaysDoses.contains { $0.def_id == defID && Double($0.taken_at) >= start }
    }
    private func doseCountToday() -> Int {
        supplements.filter { dosedToday($0.id) }.count
    }

    private func logDose(_ defID: String) async {
        guard let api = app.apiClient() else { return }
        loggingID = defID
        defer { loggingID = nil }
        do {
            _ = try await api.logSupplementDose(.init(def_id: defID, taken_at: nil, notes: nil))
            todaysDoses = (try? await api.supplementDoses(days: 1)) ?? todaysDoses
        } catch {
            loadError = (error as? APIClient.APIError)?.errorDescription ?? error.localizedDescription
        }
    }

    private func load() async {
        guard let api = app.apiClient() else { loadError = "Not paired."; return }
        loadError = nil
        do {
            async let t = api.healthToday()
            async let w = api.healthRecovery(days: 7)
            async let s = api.supplementDefs()
            async let d = api.supplementDoses(days: 1)
            self.today          = try await t
            self.recoveryWindow = try await w
            self.supplements    = try await s
            self.todaysDoses    = try await d
        } catch {
            loadError = (error as? APIClient.APIError)?.errorDescription ?? error.localizedDescription
        }
    }
}

private struct SupplementRow: View {
    let def: SupplementDef
    let doneToday: Bool
    let logging: Bool
    let onTap: () -> Void
    let onLog: () -> Void

    var body: some View {
        HStack(spacing: 12) {
            ZStack {
                RoundedRectangle(cornerRadius: 10).fill(iconBg)
                Image(systemName: iconName).foregroundStyle(iconFg)
            }
            .frame(width: 36, height: 36)
            Button(action: onTap) {
                VStack(alignment: .leading, spacing: 2) {
                    Text(def.name).font(.system(size: 15, weight: .medium))
                        .foregroundStyle(Theme.text)
                    HStack(spacing: 6) {
                        Text(def.dose.isEmpty ? def.category : def.dose)
                        if let on = def.cycle_days_on, let off = def.cycle_days_off {
                            Text("· cycle \(on)/\(off)").foregroundStyle(Theme.text3)
                        }
                    }
                    .font(.caption).foregroundStyle(Theme.text3)
                }
            }
            .buttonStyle(.plain)
            Spacer()
            Button(action: onLog) {
                if logging {
                    ProgressView().tint(Theme.text2)
                } else if doneToday {
                    Image(systemName: "checkmark.circle.fill")
                        .foregroundStyle(Theme.healthGo)
                        .font(.title3)
                } else {
                    Image(systemName: "circle")
                        .foregroundStyle(Theme.text3)
                        .font(.title3)
                }
            }
            .buttonStyle(.plain)
            .disabled(logging)
        }
        .padding(.vertical, 8)
    }

    private var iconName: String {
        switch def.category.lowercased() {
        case "peptide":    return "syringe"
        case "medication": return "pills.fill"
        default:           return "pill"
        }
    }
    private var iconBg: Color {
        switch def.category.lowercased() {
        case "peptide":    return Theme.ai.opacity(0.18)
        case "medication": return Theme.financeBad.opacity(0.18)
        default:           return Theme.healthBlue.opacity(0.18)
        }
    }
    private var iconFg: Color {
        switch def.category.lowercased() {
        case "peptide":    return Theme.ai
        case "medication": return Theme.financeBad
        default:           return Theme.healthBlue
        }
    }
}
