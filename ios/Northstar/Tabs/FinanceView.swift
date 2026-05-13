import SwiftUI

struct FinanceView: View {
    @EnvironmentObject private var app: AppState

    @State private var summary: FinanceSummary?
    @State private var transactions: [Transaction] = []
    @State private var budgetTargets: [BudgetTarget] = []
    @State private var loadError: String?
    @State private var refreshing = false
    @State private var editingTarget: BudgetTarget?
    @State private var detailTxn: Transaction?

    private let monthFormatter: DateFormatter = {
        let f = DateFormatter()
        f.dateFormat = "MMMM yyyy"
        return f
    }()

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 14) {
                    if let summary {
                        netWorthCard(summary)
                        ringsRow(summary)
                        groupCards(summary)
                    } else if loadError != nil {
                        errorCard
                    } else {
                        loadingCard
                    }
                    recentTransactionsCard
                }
                .padding(.horizontal, 16)
                .padding(.top, 8)
                .padding(.bottom, 30)
            }
            .background(Theme.bg)
            .navigationTitle("Finance")
            .toolbarBackground(Theme.bg, for: .navigationBar)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    HStack(spacing: 16) {
                        NavigationLink {
                            ForecastView()
                        } label: {
                            Image(systemName: "chart.line.uptrend.xyaxis")
                                .foregroundStyle(Theme.text2)
                        }
                        .accessibilityLabel("Cash-flow forecast")
                        NavigationLink {
                            InvestmentsView()
                        } label: {
                            Image(systemName: "chart.pie.fill")
                                .foregroundStyle(Theme.text2)
                        }
                        .accessibilityLabel("Investments")
                    }
                }
                ToolbarItem(placement: .topBarTrailing) {
                    Text(displayMonth)
                        .font(.footnote)
                        .foregroundStyle(Theme.text3)
                }
            }
            .refreshable { await load() }
        }
        .task { await load() }
        .sheet(item: $editingTarget) { t in
            EditBudgetSheet(target: t) {
                Task { await load() }
            }
        }
        .sheet(item: $detailTxn) { t in
            TransactionDetailSheet(
                txn: t,
                availableCategories: summary?.categories.map(\.category) ?? []
            ) {
                Task { await load() }
            }
            .presentationDetents([.medium, .large])
        }
    }

    // ─── Subviews ────────────────────────────────────────────────────────

    private func netWorthCard(_ s: FinanceSummary) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("NET WORTH")
                .font(.caption2).bold()
                .tracking(1)
                .foregroundStyle(Theme.text3)
            Text(s.net_worth_cents.asUSD(decimals: 0))
                .font(.system(size: 44, weight: .heavy))
                .tracking(-1.5)
                .lineLimit(1)
                .minimumScaleFactor(0.6)
            HStack(spacing: 16) {
                Text("On-budget \(s.on_budget_cents.asUSD(decimals: 0))")
                Text("Off-budget \(s.off_budget_cents.asUSD(decimals: 0))")
            }
            .font(.footnote)
            .foregroundStyle(Theme.text2)
        }
        .padding(22)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            LinearGradient(colors: [Color(hex: 0x14201a), Theme.bg],
                           startPoint: .topLeading, endPoint: .bottomTrailing)
        )
        .clipShape(RoundedRectangle(cornerRadius: 24))
    }

    private func ringsRow(_ s: FinanceSummary) -> some View {
        HStack(spacing: 10) {
            ring(label: "Spent",
                 value: s.spent_cents.asUSD(decimals: 0),
                 of: s.budgeted_cents.asUSD(decimals: 0),
                 pct: percent(s.spent_cents, of: s.budgeted_cents),
                 color: Theme.finance)
            ring(label: "Saved",
                 value: s.saved_cents.asUSD(decimals: 0),
                 of: s.income_cents.asUSD(decimals: 0),
                 pct: percent(s.saved_cents, of: s.income_cents),
                 color: Theme.goals)
            ring(label: "Income",
                 value: s.income_cents.asUSD(decimals: 0),
                 of: nil, pct: 100, color: Theme.healthMid)
        }
    }

    private func ring(label: String, value: String, of: String?, pct: Int, color: Color) -> some View {
        VStack(spacing: 8) {
            ZStack {
                Circle()
                    .stroke(Theme.border, lineWidth: 6)
                Circle()
                    .trim(from: 0, to: CGFloat(max(0, min(pct, 100))) / 100)
                    .stroke(color, style: StrokeStyle(lineWidth: 6, lineCap: .round))
                    .rotationEffect(.degrees(-90))
                Text("\(pct)%")
                    .font(.system(size: 13, weight: .bold))
            }
            .frame(width: 70, height: 70)
            Text(label).font(.caption2).bold()
            Group {
                if let of {
                    Text("\(value) / \(of)")
                } else {
                    Text(value)
                }
            }
            .font(.system(size: 10))
            .foregroundStyle(Theme.text3)
            .lineLimit(1)
            .minimumScaleFactor(0.7)
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, 12)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 16))
    }

    // Ordered the same as the server's AllGroups so sections render in the
    // expected sequence. Anything that comes back with an unknown group
    // falls into the catch-all at the bottom.
    static let groupOrder = [
        "Living Expenses",
        "Transportation",
        "Dining & Entertainment",
        "Savings & Income",
        "Miscellaneous",
    ]

    /// Builds one collapsible card per group, in `groupOrder`. Replaces the
    /// older single "BY CATEGORY" card so the user can scan group totals
    /// first and only expand the ones they care about.
    @ViewBuilder
    private func groupCards(_ s: FinanceSummary) -> some View {
        let grouped = Dictionary(grouping: s.categories) { cat in
            cat.category_group ?? "Miscellaneous"
        }
        let sectionedGroups = Self.groupOrder
            .filter { grouped[$0] != nil }
            + grouped.keys.filter { !Self.groupOrder.contains($0) }.sorted()

        ForEach(sectionedGroups, id: \.self) { groupName in
            if let cats = grouped[groupName], !cats.isEmpty {
                FinanceGroupCard(
                    name: groupName,
                    categories: cats,
                    summaryMonth: s.month,
                    availableCategories: s.categories.map(\.category),
                    onEditBudget: { c in
                        editingTarget = budgetTargets.first { $0.category == c.category }
                    }
                )
            }
        }
    }

    private var recentTransactionsCard: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text("RECENT")
                    .font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
                Spacer()
                if transactions.isEmpty == false {
                    Text("\(transactions.count) shown")
                        .font(.caption2)
                        .foregroundStyle(Theme.text3)
                        .padding(.horizontal, 8).padding(.vertical, 2)
                        .background(Color.white.opacity(0.08))
                        .clipShape(Capsule())
                }
            }
            .padding(.bottom, 6)
            ForEach(transactions) { t in
                Button { detailTxn = t } label: {
                    FinanceTransactionRow(txn: t)
                }
                .buttonStyle(.plain)
                if t.id != transactions.last?.id {
                    Divider().overlay(Theme.border)
                }
            }
            if transactions.isEmpty && summary != nil {
                Text("No recent transactions.")
                    .font(.footnote)
                    .foregroundStyle(Theme.text3)
                    .padding(.vertical, 12)
            }
        }
        .padding(18)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 20))
    }

    private var loadingCard: some View {
        HStack(spacing: 12) {
            ProgressView().tint(Theme.text2)
            Text("Loading…").foregroundStyle(Theme.text2)
        }
        .frame(maxWidth: .infinity, minHeight: 140)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 20))
    }

    private var errorCard: some View {
        VStack(spacing: 8) {
            Image(systemName: "exclamationmark.triangle.fill")
                .foregroundStyle(Theme.financeBad)
            Text(loadError ?? "Couldn't load finance data.")
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

    private var displayMonth: String {
        guard let s = summary,
              let d = DateFormatter.month.date(from: s.month) else { return "" }
        return monthFormatter.string(from: d)
    }

    private func percent(_ a: Int64, of b: Int64) -> Int {
        guard b > 0 else { return 0 }
        return Int((a * 100) / b)
    }

    private func load() async {
        guard let api = app.apiClient() else {
            loadError = "Not paired."
            return
        }
        refreshing = true
        defer { refreshing = false }
        do {
            async let sum = api.financeSummary()
            async let tx  = api.financeTransactions(limit: 25)
            async let bts = api.budgetTargets()
            let (s, t, b) = try await (sum, tx, bts)
            self.summary = s
            self.transactions = t
            self.budgetTargets = b
            self.loadError = nil
        } catch let e as APIClient.APIError {
            loadError = e.errorDescription
        } catch let e {
            loadError = e.localizedDescription
        }
    }
}

// Promoted out of `private` so `FinanceGroupCard` and
// `CategoryTransactionsView` in sibling files can reuse the same rendering.
struct FinanceCategoryBar: View {
    let category: CategorySummary

    var body: some View {
        VStack(spacing: 6) {
            HStack {
                Text(category.category)
                    .font(.system(size: 14, weight: .medium))
                Spacer()
                HStack(spacing: 4) {
                    Text(category.spent_cents.asUSD(decimals: 0))
                        .foregroundStyle(Theme.text2)
                    if category.budgeted_cents > 0 {
                        Text("/ \(category.budgeted_cents.asUSD(decimals: 0))")
                            .foregroundStyle(Theme.text3)
                    }
                }
                .font(.system(size: 13, weight: .semibold))
            }
            GeometryReader { proxy in
                let total = proxy.size.width
                ZStack(alignment: .leading) {
                    RoundedRectangle(cornerRadius: 3)
                        .fill(Color.white.opacity(0.06))
                    RoundedRectangle(cornerRadius: 3)
                        .fill(barColor)
                        .frame(width: barWidth(total: total))
                }
            }
            .frame(height: 5)
        }
        .padding(.vertical, 6)
    }

    private var barColor: Color {
        if category.over { return Theme.financeBad }
        if category.pct >= 90 { return Theme.healthMid }
        return Theme.finance
    }

    private func barWidth(total: CGFloat) -> CGFloat {
        let pct = max(0, min(category.pct, 100))
        return total * CGFloat(pct) / 100
    }
}

// Shared between the Recent list and the category drilldown.
struct FinanceTransactionRow: View {
    let txn: Transaction

    var body: some View {
        HStack(spacing: 12) {
            ZStack {
                RoundedRectangle(cornerRadius: 10)
                    .fill(Theme.surfaceHi)
                Text(categoryGlyph)
            }
            .frame(width: 36, height: 36)

            VStack(alignment: .leading, spacing: 2) {
                Text(txn.payee.isEmpty ? "—" : txn.payee)
                    .font(.system(size: 15, weight: .medium))
                    .lineLimit(1)
                HStack(spacing: 6) {
                    if !txn.category.isEmpty {
                        Text(txn.category)
                    }
                    Text("· \(txn.date)")
                }
                .font(.system(size: 12))
                .foregroundStyle(Theme.text3)
                .lineLimit(1)
            }
            Spacer()
            Text(txn.amount_cents.asUSD(decimals: 2))
                .font(.system(size: 15, weight: .semibold))
                .foregroundStyle(txn.amount_cents < 0 ? Theme.financeBad : Theme.finance)
        }
        .padding(.vertical, 8)
    }

    private var categoryGlyph: String {
        switch txn.category.lowercased() {
        case "restaurants": return "🍽"
        case "groceries":   return "🛒"
        case "gas":         return "⛽"
        case "utilities":   return "💡"
        case "mortgage":    return "🏠"
        case "subscriptions": return "📺"
        case "travel":      return "✈️"
        case "medical":     return "🩺"
        case "gifts":       return "🎁"
        case "salary":      return "💰"
        case "transfer":    return "🔄"
        default:            return "•"
        }
    }
}

private extension DateFormatter {
    static let month: DateFormatter = {
        let f = DateFormatter()
        f.dateFormat = "yyyy-MM"
        return f
    }()
}
