import SwiftUI
import Charts

/// Compact line chart of recent net-worth snapshots. Embedded in the
/// Finance tab's net-worth card so the headline number has context.
/// Hides itself when fewer than 2 days of history have accumulated —
/// drawing a single point is misleading.
struct NetWorthSparkline: View {
    let days: [BalanceHistoryDay]

    var body: some View {
        if days.count < 2 {
            EmptyView()
        } else {
            Chart {
                ForEach(days) { d in
                    AreaMark(
                        x: .value("Date", parseDate(d.date) ?? Date.distantPast),
                        y: .value("NW", Double(d.net_worth_cents) / 100.0)
                    )
                    .foregroundStyle(
                        LinearGradient(
                            colors: [Theme.finance.opacity(0.35), Theme.finance.opacity(0.02)],
                            startPoint: .top, endPoint: .bottom)
                    )
                    LineMark(
                        x: .value("Date", parseDate(d.date) ?? Date.distantPast),
                        y: .value("NW", Double(d.net_worth_cents) / 100.0)
                    )
                    .foregroundStyle(Theme.finance)
                    .lineStyle(StrokeStyle(lineWidth: 2))
                }
            }
            .chartXAxis(.hidden)
            .chartYAxis(.hidden)
            .chartPlotStyle { $0.background(Color.clear) }
            .frame(height: 56)
        }
    }

    private func parseDate(_ s: String) -> Date? {
        let f = DateFormatter()
        f.dateFormat = "yyyy-MM-dd"
        f.timeZone = TimeZone(identifier: "UTC")
        return f.date(from: s)
    }
}
