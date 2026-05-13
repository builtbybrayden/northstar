import SwiftUI
import Charts

/// Cash-flow projection screen. Pulls /api/finance/forecast and renders:
///   - Hero with current balance + lowest projected balance + recovery date
///   - SwiftUI Charts line chart of the projection
///   - Upcoming recurring inflows/outflows in the next 30 days
///   - Detected recurring stack with day-of-month + confidence
struct ForecastView: View {
    @EnvironmentObject private var app: AppState

    @State private var horizon: Int = 90
    @State private var forecast: Forecast?
    @State private var loadError: String?

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 14) {
                if let f = forecast {
                    heroCard(f)
                    horizonPicker
                    chartCard(f)
                    upcomingCard(f)
                    recurringCard(f)
                } else if loadError != nil {
                    errorCard
                } else {
                    loadingCard
                }
            }
            .padding(.horizontal, 16)
            .padding(.top, 8)
            .padding(.bottom, 30)
        }
        .background(Theme.bg)
        .navigationTitle("Forecast")
        .toolbarBackground(Theme.bg, for: .navigationBar)
        .task { await load() }
        .refreshable { await load() }
    }

    // ─── Subviews ────────────────────────────────────────────────────────

    private func heroCard(_ f: Forecast) -> some View {
        let lowest = f.projected.min(by: { $0.balance_cents < $1.balance_cents })
        let recovery = lowest.flatMap { low in
            f.projected.first { $0.date > low.date && $0.balance_cents >= f.current_balance_cents }
        }
        return VStack(alignment: .leading, spacing: 6) {
            Text("PROJECTED BALANCE · \(f.horizon_days)d")
                .font(.caption2).bold().tracking(1)
                .foregroundStyle(Theme.text3)
            Text(f.current_balance_cents.asUSD(decimals: 0))
                .font(.system(size: 36, weight: .heavy))
                .tracking(-1)
                .lineLimit(1)
                .minimumScaleFactor(0.6)
            if let low = lowest {
                HStack(spacing: 6) {
                    Image(systemName: "arrow.down.right.circle.fill")
                        .foregroundStyle(low.balance_cents < f.current_balance_cents ? Theme.financeBad : Theme.finance)
                    Text("Low \(low.balance_cents.asUSD(decimals: 0))")
                        .foregroundStyle(Theme.text2)
                    Text("· \(prettyDate(low.date))")
                        .foregroundStyle(Theme.text3)
                    Spacer()
                }
                .font(.footnote)
            }
            if let r = recovery {
                Text("Back above today's balance by \(prettyDate(r.date))")
                    .font(.caption)
                    .foregroundStyle(Theme.text3)
            }
            Text("Daily burn ~\(f.daily_discretionary_cents.asUSD(decimals: 0))")
                .font(.caption)
                .foregroundStyle(Theme.text3)
        }
        .padding(22)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            LinearGradient(colors: [Color(hex: 0x14201a), Theme.bg],
                           startPoint: .topLeading, endPoint: .bottomTrailing)
        )
        .clipShape(RoundedRectangle(cornerRadius: 24))
    }

    private var horizonPicker: some View {
        HStack(spacing: 8) {
            ForEach([30, 60, 90, 180], id: \.self) { d in
                Button("\(d)d") {
                    horizon = d
                    Task { await load() }
                }
                .font(.system(size: 12, weight: .semibold))
                .padding(.horizontal, 12).padding(.vertical, 7)
                .background(horizon == d ? Theme.finance : Theme.surfaceHi)
                .foregroundStyle(horizon == d ? .white : Theme.text2)
                .clipShape(Capsule())
            }
        }
    }

    private func chartCard(_ f: Forecast) -> some View {
        let parsed: [(Date, Double)] = f.projected.compactMap { d in
            guard let date = DateFormatter.iso.date(from: d.date) else { return nil }
            return (date, Double(d.balance_cents) / 100.0)
        }
        return VStack(alignment: .leading, spacing: 10) {
            Text("PROJECTION")
                .font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
            Chart {
                ForEach(parsed, id: \.0) { date, balance in
                    LineMark(
                        x: .value("Date", date),
                        y: .value("Balance", balance)
                    )
                    .interpolationMethod(.monotone)
                    .foregroundStyle(Theme.finance)
                }
                RuleMark(y: .value("Today", Double(f.current_balance_cents) / 100.0))
                    .foregroundStyle(Theme.text3.opacity(0.4))
                    .lineStyle(StrokeStyle(lineWidth: 1, dash: [4, 4]))
                    .annotation(position: .top, alignment: .leading) {
                        Text("today")
                            .font(.caption2)
                            .foregroundStyle(Theme.text3)
                    }
            }
            .chartXAxis {
                AxisMarks(values: .stride(by: .day, count: max(1, f.horizon_days / 6))) { v in
                    AxisGridLine().foregroundStyle(Theme.border)
                    if let d = v.as(Date.self) {
                        AxisValueLabel(DateFormatter.shortMonth.string(from: d))
                            .foregroundStyle(Theme.text3)
                    }
                }
            }
            .chartYAxis {
                AxisMarks { v in
                    AxisGridLine().foregroundStyle(Theme.border)
                    if let d = v.as(Double.self) {
                        AxisValueLabel(currencyShort(d))
                            .foregroundStyle(Theme.text3)
                    }
                }
            }
            .frame(height: 180)
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 20))
    }

    private func upcomingCard(_ f: Forecast) -> some View {
        let upcoming = f.milestones.prefix(8)
        return VStack(alignment: .leading, spacing: 10) {
            Text("UPCOMING")
                .font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
            if upcoming.isEmpty {
                Text("No notable events in this window.")
                    .font(.footnote)
                    .foregroundStyle(Theme.text3)
            } else {
                ForEach(Array(upcoming.enumerated()), id: \.offset) { _, ev in
                    HStack(spacing: 10) {
                        Text(prettyDate(ev.date ?? ""))
                            .font(.system(size: 12, weight: .semibold))
                            .foregroundStyle(Theme.text3)
                            .frame(width: 64, alignment: .leading)
                        Text(ev.label)
                            .font(.system(size: 14))
                            .lineLimit(1)
                        Spacer()
                        if let amount = ev.amount_cents, amount != 0 {
                            Text(amount.asUSD(decimals: 0))
                                .font(.system(size: 14, weight: .semibold))
                                .foregroundStyle(amount < 0 ? Theme.financeBad : Theme.finance)
                        }
                    }
                }
            }
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 20))
    }

    private func recurringCard(_ f: Forecast) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack {
                Text("DETECTED RECURRING")
                    .font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
                Spacer()
                Text("\(f.recurring.count)")
                    .font(.caption2)
                    .foregroundStyle(Theme.text3)
                    .padding(.horizontal, 8).padding(.vertical, 2)
                    .background(Color.white.opacity(0.08))
                    .clipShape(Capsule())
            }
            if f.recurring.isEmpty {
                Text("Not enough history yet — recurring detection needs 3+ months of data.")
                    .font(.footnote)
                    .foregroundStyle(Theme.text3)
            }
            ForEach(f.recurring) { r in
                HStack(spacing: 10) {
                    VStack(alignment: .leading, spacing: 2) {
                        Text(r.label)
                            .font(.system(size: 14, weight: .medium))
                            .lineLimit(1)
                        Text("Day \(r.day_of_month) of each month · confidence \(Int(r.confidence * 100))%")
                            .font(.caption)
                            .foregroundStyle(Theme.text3)
                    }
                    Spacer()
                    Text(r.amount_cents.asUSD(decimals: 0))
                        .font(.system(size: 14, weight: .semibold))
                        .foregroundStyle(r.amount_cents < 0 ? Theme.financeBad : Theme.finance)
                }
                .padding(.vertical, 4)
            }
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 20))
    }

    private var loadingCard: some View {
        HStack(spacing: 12) {
            ProgressView().tint(Theme.text2)
            Text("Computing forecast…").foregroundStyle(Theme.text2)
        }
        .frame(maxWidth: .infinity, minHeight: 140)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 20))
    }

    private var errorCard: some View {
        VStack(spacing: 8) {
            Image(systemName: "exclamationmark.triangle.fill")
                .foregroundStyle(Theme.financeBad)
            Text(loadError ?? "Couldn't load forecast.")
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

    // ─── Plumbing ────────────────────────────────────────────────────────

    private func load() async {
        guard let api = app.apiClient() else {
            loadError = "Not paired."
            return
        }
        do {
            self.forecast = try await api.financeForecast(days: horizon)
            self.loadError = nil
        } catch let e as APIClient.APIError {
            loadError = e.errorDescription
        } catch let e {
            loadError = e.localizedDescription
        }
    }

    private func prettyDate(_ s: String) -> String {
        guard let d = DateFormatter.iso.date(from: s) else { return s }
        return DateFormatter.pretty.string(from: d)
    }

    private func prettyShortDate(_ s: String) -> String {
        guard let d = DateFormatter.iso.date(from: s) else { return s }
        return DateFormatter.shortMonth.string(from: d)
    }

    private func currencyShort(_ d: Double) -> String {
        if abs(d) >= 1_000_000 { return String(format: "$%.1fM", d / 1_000_000) }
        if abs(d) >= 1_000 { return String(format: "$%.0fk", d / 1_000) }
        return String(format: "$%.0f", d)
    }
}

private extension DateFormatter {
    static let iso: DateFormatter = {
        let f = DateFormatter()
        f.dateFormat = "yyyy-MM-dd"
        f.locale = Locale(identifier: "en_US_POSIX")
        return f
    }()
    static let pretty: DateFormatter = {
        let f = DateFormatter()
        f.dateFormat = "MMM d"
        return f
    }()
    static let shortMonth: DateFormatter = {
        let f = DateFormatter()
        f.dateFormat = "M/d"
        return f
    }()
}
