import SwiftUI

/// Two-toggle account manager: per-account "Income" + "Saved" flags.
/// Override wins over the server-side heuristic; the chevron next to an
/// overridden row reverts that flag back to auto.
///
/// Accessible from the income and saved drilldown sheets so the user
/// can fix the classifier at the source whenever a number looks wrong.
struct AccountFlagsSheet: View {
    @EnvironmentObject private var app: AppState
    @Environment(\.dismiss) private var dismiss

    let onChanged: () -> Void

    @State private var accounts: [Account] = []
    @State private var loading = true
    @State private var loadError: String?
    @State private var saving: Set<String> = []

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 14) {
                    intro
                    if loading {
                        loadingCard
                    } else if let loadError {
                        errorCard(loadError)
                    } else {
                        legend
                        accountList
                    }
                }
                .padding(.horizontal, 16)
                .padding(.top, 8)
                .padding(.bottom, 30)
            }
            .background(Theme.bg)
            .navigationTitle("Accounts")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    Button("Done") { dismiss() }
                        .foregroundStyle(Theme.ai)
                }
            }
        }
        .task { await load() }
    }

    // ─── Subviews ────────────────────────────────────────────────────────

    private var intro: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("Pick which accounts roll up into the Income and Saved "
                 + "donuts. Defaults come from each account's type — "
                 + "flip a toggle to override.")
                .font(.footnote)
                .foregroundStyle(Theme.text2)
            Text("Tap the revert chevron on an overridden row to fall back to the default.")
                .font(.caption)
                .foregroundStyle(Theme.text3)
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 16))
    }

    private var legend: some View {
        HStack(spacing: 12) {
            Spacer()
            legendChip("Income", color: Theme.healthMid)
            legendChip("Saved", color: Theme.goals)
        }
        .padding(.horizontal, 8)
    }
    private func legendChip(_ s: String, color: Color) -> some View {
        Text(s)
            .font(.caption2).bold().tracking(0.5)
            .foregroundStyle(color)
            .frame(width: 56, alignment: .center)
    }

    private var accountList: some View {
        VStack(alignment: .leading, spacing: 0) {
            ForEach(accounts) { a in
                row(a)
                if a.id != accounts.last?.id {
                    Divider().overlay(Theme.border)
                }
            }
        }
        .padding(.vertical, 4)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 16))
    }

    private func row(_ a: Account) -> some View {
        HStack(spacing: 14) {
            VStack(alignment: .leading, spacing: 2) {
                Text(a.name)
                    .font(.system(size: 15, weight: .semibold))
                HStack(spacing: 6) {
                    Text(a.balance_cents.asUSD(decimals: 0))
                    if let type = a.type, !type.isEmpty {
                        Text("· \(type)")
                    }
                    if hasAnyOverride(a) {
                        Text("· custom")
                            .foregroundStyle(Theme.ai)
                    }
                }
                .font(.caption2)
                .foregroundStyle(Theme.text3)
            }
            Spacer()
            if saving.contains(a.id) {
                ProgressView().tint(Theme.text2)
            }
            if hasAnyOverride(a) {
                Button {
                    Task { await revert(a) }
                } label: {
                    Image(systemName: "arrow.uturn.backward.circle")
                        .foregroundStyle(Theme.text3)
                }
                .buttonStyle(.plain)
            }
            Toggle("", isOn: Binding(
                get: { a.include_in_income ?? false },
                set: { new in
                    Task { await update(a, income: .set(new)) }
                }))
                .labelsHidden()
                .tint(Theme.healthMid)
                .frame(width: 56)
            Toggle("", isOn: Binding(
                get: { a.is_savings_destination ?? false },
                set: { new in
                    Task { await update(a, savings: .set(new)) }
                }))
                .labelsHidden()
                .tint(Theme.goals)
                .frame(width: 56)
        }
        .padding(.horizontal, 16).padding(.vertical, 10)
    }

    private func hasAnyOverride(_ a: Account) -> Bool {
        a.savings_destination_override != nil || a.include_in_income_override != nil
    }

    private var loadingCard: some View {
        HStack(spacing: 12) {
            ProgressView().tint(Theme.text2)
            Text("Loading…").foregroundStyle(Theme.text2)
        }
        .frame(maxWidth: .infinity, minHeight: 100)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 16))
    }

    private func errorCard(_ msg: String) -> some View {
        VStack(spacing: 6) {
            Text(msg)
                .font(.footnote)
                .foregroundStyle(Theme.financeBad)
            Button("Retry") { Task { await load() } }
                .foregroundStyle(Theme.ai)
                .font(.system(.footnote, weight: .semibold))
        }
        .frame(maxWidth: .infinity, minHeight: 100)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 16))
    }

    // ─── Networking ──────────────────────────────────────────────────────

    private func load() async {
        guard let api = app.apiClient() else {
            loadError = "Not paired."
            loading = false
            return
        }
        loading = true; loadError = nil
        do {
            self.accounts = try await api.financeAccounts()
            self.loading = false
        } catch let e as APIClient.APIError {
            loadError = e.errorDescription
            loading = false
        } catch let e {
            loadError = e.localizedDescription
            loading = false
        }
    }

    private func update(_ account: Account,
                        savings: APIClient.AccountFlagChange = .unchanged,
                        income: APIClient.AccountFlagChange = .unchanged) async {
        guard let api = app.apiClient() else { return }
        saving.insert(account.id)
        defer { saving.remove(account.id) }
        do {
            try await api.updateAccountFlags(
                id: account.id,
                savingsDestination: savings,
                includeInIncome: income)
            await load()
            onChanged()
        } catch {
            loadError = "Couldn't save: \(error.localizedDescription)"
        }
    }

    private func revert(_ account: Account) async {
        await update(account, savings: .clear, income: .clear)
    }
}
