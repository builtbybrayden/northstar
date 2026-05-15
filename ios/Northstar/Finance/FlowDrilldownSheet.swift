import SwiftUI

/// Tapping any of the three Finance donuts (Spent / Saved / Income)
/// opens this sheet. It re-fetches the transactions that make up the
/// headline number (using `?flow=` on the transactions endpoint so the
/// list reconciles exactly to the donut). The `saved` variant adds a
/// stepper for the savings target percentage at the top.
enum FinanceFlow: String, CaseIterable, Identifiable {
    case spent, saved, income
    var id: String { rawValue }

    var title: String {
        switch self {
        case .spent:  return "Spent"
        case .saved:  return "Saved"
        case .income: return "Income"
        }
    }
    var tint: Color {
        switch self {
        case .spent:  return Theme.finance
        case .saved:  return Theme.goals
        case .income: return Theme.healthMid
        }
    }
}

struct FlowDrilldownSheet: View {
    @EnvironmentObject private var app: AppState
    @Environment(\.dismiss) private var dismiss

    let flow: FinanceFlow
    let initialSummary: FinanceSummary
    let onSettingsChanged: () -> Void

    /// Mutable copy so the header re-renders after the user changes an
    /// account flag or a per-transaction override. `load()` always
    /// re-fetches the summary alongside the transaction list so the
    /// donut headline + list are guaranteed to reconcile.
    @State private var summary: FinanceSummary
    @State private var transactions: [Transaction] = []
    @State private var loading = true
    @State private var loadError: String?
    @State private var savingsTargetPct: Int = 25
    @State private var savingTarget = false
    @State private var targetError: String?
    @State private var detailTxn: Transaction?
    @State private var showingAccountSettings = false

    init(flow: FinanceFlow, summary: FinanceSummary, onSettingsChanged: @escaping () -> Void) {
        self.flow = flow
        self.initialSummary = summary
        self.onSettingsChanged = onSettingsChanged
        _summary = State(initialValue: summary)
    }

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 14) {
                    headerCard
                    if flow == .saved {
                        targetCard
                    } else {
                        manageAccountsCard
                    }
                    txList
                }
                .padding(.horizontal, 16)
                .padding(.top, 8)
                .padding(.bottom, 30)
            }
            .background(Theme.bg)
            .navigationTitle(flow.title)
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    Button("Done") { dismiss() }
                        .foregroundStyle(Theme.ai)
                }
            }
        }
        .task { await load() }
        .sheet(item: $detailTxn) { t in
            TransactionDetailSheet(
                txn: t,
                availableCategories: summary.categories.map(\.category)
            ) {
                Task { await load() }
                onSettingsChanged()
            }
            .presentationDetents([.medium, .large])
        }
        .sheet(isPresented: $showingAccountSettings) {
            AccountFlagsSheet {
                Task { await load() }
                onSettingsChanged()
            }
            .presentationDetents([.medium, .large])
        }
    }

    // ─── Subviews ────────────────────────────────────────────────────────

    private var headerCard: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(flow.title.uppercased())
                .font(.caption2).bold().tracking(1)
                .foregroundStyle(Theme.text3)
            Text(headlineCents.asUSD(decimals: 0))
                .font(.system(size: 36, weight: .heavy))
                .tracking(-1)
                .foregroundStyle(flow.tint)
            if let denomLabel {
                Text(denomLabel)
                    .font(.footnote)
                    .foregroundStyle(Theme.text3)
            }
        }
        .padding(18)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 18))
    }

    private var manageAccountsCard: some View {
        Button { showingAccountSettings = true } label: {
            HStack {
                Image(systemName: "slider.horizontal.3")
                VStack(alignment: .leading, spacing: 1) {
                    Text("Manage accounts")
                        .font(.system(.footnote, weight: .semibold))
                    Text("Pick which accounts roll into \(flow.title.lowercased()).")
                        .font(.caption2)
                        .foregroundStyle(Theme.text3)
                }
                Spacer()
                Image(systemName: "chevron.right")
                    .font(.caption)
                    .foregroundStyle(Theme.text3)
            }
            .padding(14)
            .frame(maxWidth: .infinity)
            .background(Theme.surface)
            .clipShape(RoundedRectangle(cornerRadius: 16))
            .foregroundStyle(Theme.ai)
        }
        .buttonStyle(.plain)
    }

    private var targetCard: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack {
                Text("SAVINGS TARGET")
                    .font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
                Spacer()
                if savingTarget { ProgressView().tint(Theme.text2) }
            }
            HStack(spacing: 16) {
                Text("\(savingsTargetPct)%")
                    .font(.system(size: 32, weight: .heavy))
                    .frame(minWidth: 78, alignment: .leading)
                Stepper("",
                        value: $savingsTargetPct,
                        in: 0...100, step: 5)
                    .labelsHidden()
                    .onChange(of: savingsTargetPct) { _, new in
                        Task { await persistTarget(new) }
                    }
            }
            Text("Target this month: \(targetCentsForCurrentPct.asUSD(decimals: 0)) "
                 + "(of \(summary.income_cents.asUSD(decimals: 0)) income)")
                .font(.footnote)
                .foregroundStyle(Theme.text2)
            if let targetError {
                Text(targetError)
                    .font(.footnote)
                    .foregroundStyle(Theme.financeBad)
            }
            Divider().overlay(Theme.border).padding(.vertical, 4)
            Button {
                showingAccountSettings = true
            } label: {
                HStack {
                    Image(systemName: "slider.horizontal.3")
                    Text("Manage savings accounts")
                        .font(.system(.footnote, weight: .semibold))
                    Spacer()
                    Image(systemName: "chevron.right")
                        .font(.caption)
                        .foregroundStyle(Theme.text3)
                }
                .foregroundStyle(Theme.ai)
            }
            .buttonStyle(.plain)
        }
        .padding(18)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 18))
    }

    private var txList: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text("TRANSACTIONS")
                    .font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
                Spacer()
                if !loading {
                    Text("\(transactions.count) shown")
                        .font(.caption2)
                        .foregroundStyle(Theme.text3)
                        .padding(.horizontal, 8).padding(.vertical, 2)
                        .background(Color.white.opacity(0.08))
                        .clipShape(Capsule())
                }
            }
            .padding(.bottom, 6)
            if loading {
                HStack(spacing: 12) {
                    ProgressView().tint(Theme.text2)
                    Text("Loading…").foregroundStyle(Theme.text2)
                }
                .frame(maxWidth: .infinity, minHeight: 100)
            } else if let loadError {
                VStack(spacing: 6) {
                    Text(loadError)
                        .font(.footnote)
                        .foregroundStyle(Theme.financeBad)
                        .multilineTextAlignment(.center)
                    Button("Retry") { Task { await load() } }
                        .foregroundStyle(Theme.ai)
                        .font(.system(.footnote, weight: .semibold))
                }
                .frame(maxWidth: .infinity, minHeight: 100)
            } else if transactions.isEmpty {
                Text("No \(flow.title.lowercased()) transactions this month.")
                    .font(.footnote)
                    .foregroundStyle(Theme.text3)
                    .padding(.vertical, 18)
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
        .clipShape(RoundedRectangle(cornerRadius: 18))
    }

    // ─── State helpers ───────────────────────────────────────────────────

    private var headlineCents: Int64 {
        switch flow {
        case .spent:  return summary.spent_cents
        case .saved:  return summary.saved_cents
        case .income: return summary.income_cents
        }
    }

    private var denomLabel: String? {
        switch flow {
        case .spent:
            if summary.budgeted_cents > 0 {
                return "of \(summary.budgeted_cents.asUSD(decimals: 0)) budgeted"
            }
            return "of \(summary.income_cents.asUSD(decimals: 0)) income"
        case .saved:
            return "target \(targetCentsForCurrentPct.asUSD(decimals: 0))"
        case .income:
            return nil
        }
    }

    private var targetCentsForCurrentPct: Int64 {
        Int64(Double(summary.income_cents) * Double(savingsTargetPct) / 100)
    }

    // ─── Networking ──────────────────────────────────────────────────────

    private func load() async {
        guard let api = app.apiClient() else {
            loadError = "Not paired."
            loading = false
            return
        }
        loading = true
        loadError = nil
        do {
            // Refetch summary alongside the list so the header reflects
            // any account-flag / per-tx override changes the user just
            // made. Otherwise the donut headline appears unchanged even
            // though the list below it updates.
            async let s = api.financeSummary(month: summary.month)
            async let list = api.financeTransactionsByFlow(
                flow: flow.rawValue,
                month: summary.month,
                limit: 200)
            let (newSummary, newList) = try await (s, list)
            self.summary = newSummary
            self.transactions = newList
            if flow == .saved, let pct = newSummary.savings_target_pct {
                savingsTargetPct = pct
            }
            self.loading = false
        } catch let e as APIClient.APIError {
            loadError = e.errorDescription
            loading = false
        } catch let e {
            loadError = e.localizedDescription
            loading = false
        }
    }

    private func persistTarget(_ pct: Int) async {
        guard let api = app.apiClient() else { return }
        savingTarget = true
        targetError = nil
        defer { savingTarget = false }
        do {
            _ = try await api.updateFinanceSettings(
                FinanceSettings(savings_target_pct: pct))
            onSettingsChanged()
        } catch let e as APIClient.APIError {
            targetError = e.errorDescription
        } catch let e {
            targetError = e.localizedDescription
        }
    }
}
