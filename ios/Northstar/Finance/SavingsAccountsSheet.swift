import SwiftUI

/// Lists every open account and lets the user toggle whether it counts
/// as a savings destination. Override wins over the server's name
/// heuristic; clearing the override reverts to the heuristic.
///
/// Used from the saved-donut drilldown when the user notices the
/// heuristic missed (or over-matched) one of their accounts.
struct SavingsAccountsSheet: View {
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
                        accountList
                    }
                }
                .padding(.horizontal, 16)
                .padding(.top, 8)
                .padding(.bottom, 30)
            }
            .background(Theme.bg)
            .navigationTitle("Savings accounts")
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
            Text("Toggle which accounts count as savings destinations. "
                 + "The saved donut sums transfers INTO any account marked here.")
                .font(.footnote)
                .foregroundStyle(Theme.text2)
            Text("Tap the chevron on an overridden row to revert to the heuristic.")
                .font(.caption)
                .foregroundStyle(Theme.text3)
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 16))
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
        let isDest = a.is_savings_destination ?? false
        let hasOverride = a.savings_destination_override != nil
        return HStack {
            VStack(alignment: .leading, spacing: 2) {
                Text(a.name)
                    .font(.system(size: 15, weight: .semibold))
                HStack(spacing: 6) {
                    Text(a.balance_cents.asUSD(decimals: 0))
                        .foregroundStyle(Theme.text3)
                    if hasOverride {
                        Text("· custom")
                            .foregroundStyle(Theme.ai)
                    } else {
                        Text("· auto")
                            .foregroundStyle(Theme.text3)
                    }
                }
                .font(.caption2)
            }
            Spacer()
            if saving.contains(a.id) {
                ProgressView().tint(Theme.text2)
            }
            if hasOverride {
                Button {
                    Task { await update(account: a, to: nil) }
                } label: {
                    Image(systemName: "arrow.uturn.backward.circle")
                        .foregroundStyle(Theme.text3)
                }
                .buttonStyle(.plain)
            }
            Toggle("", isOn: Binding(
                get: { isDest },
                set: { new in Task { await update(account: a, to: new) } }))
                .labelsHidden()
                .tint(Theme.goals)
        }
        .padding(.horizontal, 16).padding(.vertical, 10)
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
            let list = try await api.financeAccounts()
            self.accounts = list
            self.loading = false
        } catch let e as APIClient.APIError {
            loadError = e.errorDescription
            loading = false
        } catch let e {
            loadError = e.localizedDescription
            loading = false
        }
    }

    private func update(account: Account, to value: Bool?) async {
        guard let api = app.apiClient() else { return }
        saving.insert(account.id)
        defer { saving.remove(account.id) }
        do {
            try await api.setAccountSavingsDestination(id: account.id, value: value)
            await load()
            onChanged()
        } catch {
            loadError = "Couldn't save: \(error.localizedDescription)"
        }
    }
}
