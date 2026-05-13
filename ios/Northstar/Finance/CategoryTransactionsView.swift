import SwiftUI

/// All transactions in a single category for a given month. Reached by
/// tapping a leaf category inside a collapsible group card on the Finance
/// tab. Tapping a row opens the existing TransactionDetailSheet so the
/// user can re-categorize or just inspect.
struct CategoryTransactionsView: View {
    @EnvironmentObject private var app: AppState

    let category: CategorySummary
    let month: String                       // YYYY-MM, matches FinanceSummary.month
    let availableCategories: [String]       // forwarded to the detail sheet

    @State private var transactions: [Transaction] = []
    @State private var loadError: String?
    @State private var loading = true
    @State private var detailTxn: Transaction?

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 14) {
                heroCard
                listCard
            }
            .padding(.horizontal, 16)
            .padding(.top, 8)
            .padding(.bottom, 30)
        }
        .background(Theme.bg)
        .navigationTitle(category.category)
        .toolbarBackground(Theme.bg, for: .navigationBar)
        .task { await load() }
        .refreshable { await load() }
        .sheet(item: $detailTxn) { t in
            TransactionDetailSheet(
                txn: t,
                availableCategories: availableCategories
            ) {
                Task { await load() }
            }
            .presentationDetents([.medium, .large])
        }
    }

    // ─── Subviews ─────────────────────────────────────────────────────────

    private var heroCard: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(category.category.uppercased())
                .font(.caption2).bold().tracking(1)
                .foregroundStyle(Theme.text3)
            HStack(alignment: .firstTextBaseline, spacing: 6) {
                Text(category.spent_cents.asUSD(decimals: 0))
                    .font(.system(size: 36, weight: .heavy))
                    .tracking(-1)
                if category.budgeted_cents > 0 {
                    Text("of \(category.budgeted_cents.asUSD(decimals: 0))")
                        .font(.callout)
                        .foregroundStyle(Theme.text3)
                }
            }
            if category.budgeted_cents > 0 {
                progressBar
                Text("\(category.pct)% used")
                    .font(.caption)
                    .foregroundStyle(category.over ? Theme.financeBad : Theme.text3)
            }
            Text("\(transactions.count) transaction\(transactions.count == 1 ? "" : "s") this month")
                .font(.footnote)
                .foregroundStyle(Theme.text2)
                .padding(.top, 4)
        }
        .padding(20)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 20))
    }

    private var progressBar: some View {
        GeometryReader { proxy in
            let total = proxy.size.width
            let pct = max(0, min(category.pct, 100))
            ZStack(alignment: .leading) {
                RoundedRectangle(cornerRadius: 3)
                    .fill(Color.white.opacity(0.06))
                RoundedRectangle(cornerRadius: 3)
                    .fill(barColor)
                    .frame(width: total * CGFloat(pct) / 100)
            }
        }
        .frame(height: 6)
    }

    private var barColor: Color {
        if category.over { return Theme.financeBad }
        if category.pct >= 90 { return Theme.healthMid }
        return Theme.finance
    }

    private var listCard: some View {
        VStack(alignment: .leading, spacing: 4) {
            if loading && transactions.isEmpty {
                HStack(spacing: 12) {
                    ProgressView().tint(Theme.text2)
                    Text("Loading transactions…").foregroundStyle(Theme.text2)
                }
                .padding(.vertical, 24)
                .frame(maxWidth: .infinity)
            } else if let loadError {
                VStack(spacing: 8) {
                    Image(systemName: "exclamationmark.triangle.fill")
                        .foregroundStyle(Theme.financeBad)
                    Text(loadError)
                        .multilineTextAlignment(.center)
                        .foregroundStyle(Theme.text2)
                        .font(.footnote)
                    Button("Retry") { Task { await load() } }
                        .foregroundStyle(Theme.ai)
                        .font(.system(.footnote, weight: .semibold))
                }
                .padding(.vertical, 20)
                .frame(maxWidth: .infinity)
            } else if transactions.isEmpty {
                Text("No transactions in this category for \(month).")
                    .font(.footnote)
                    .foregroundStyle(Theme.text3)
                    .padding(.vertical, 16)
                    .frame(maxWidth: .infinity)
            } else {
                ForEach(transactions) { t in
                    Button { detailTxn = t } label: {
                        FinanceTransactionRow(txn: t)
                    }
                    .buttonStyle(.plain)
                    if t.id != transactions.last?.id {
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

    // ─── Plumbing ─────────────────────────────────────────────────────────

    private func load() async {
        guard let api = app.apiClient() else {
            loadError = "Not paired."
            loading = false
            return
        }
        loading = true
        defer { loading = false }
        do {
            transactions = try await api.financeTransactions(
                limit: 500,
                category: category.category,
                month: month
            )
            loadError = nil
        } catch let e as APIClient.APIError {
            loadError = e.errorDescription
        } catch let e {
            loadError = e.localizedDescription
        }
    }
}
