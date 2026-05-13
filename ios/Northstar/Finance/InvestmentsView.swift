import SwiftUI

/// Read-only view of off-budget accounts grouped by asset class. The grouping
/// is heuristic on account name (see classifyInvestmentAccount on the server).
/// Rename an account in Actual to push it into a different bucket.
struct InvestmentsView: View {
    @EnvironmentObject private var app: AppState

    @State private var data: Investments?
    @State private var loadError: String?

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 14) {
                if let d = data {
                    heroCard(d)
                    groupsCard(d)
                    accountsCard(d)
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
        .navigationTitle("Investments")
        .toolbarBackground(Theme.bg, for: .navigationBar)
        .task { await load() }
        .refreshable { await load() }
    }

    private func heroCard(_ d: Investments) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("OFF-BUDGET VALUE")
                .font(.caption2).bold().tracking(1)
                .foregroundStyle(Theme.text3)
            Text(d.total_cents.asUSD(decimals: 0))
                .font(.system(size: 40, weight: .heavy))
                .tracking(-1)
                .lineLimit(1)
                .minimumScaleFactor(0.6)
            Text("\(d.accounts.count) account\(d.accounts.count == 1 ? "" : "s") · \(d.groups.count) asset class\(d.groups.count == 1 ? "" : "es")")
                .font(.footnote)
                .foregroundStyle(Theme.text2)
        }
        .padding(22)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            LinearGradient(colors: [Color(hex: 0x101a26), Theme.bg],
                           startPoint: .topLeading, endPoint: .bottomTrailing)
        )
        .clipShape(RoundedRectangle(cornerRadius: 24))
    }

    private func groupsCard(_ d: Investments) -> some View {
        let sortedGroups = d.groups.sorted(by: { $0.value > $1.value })
        return VStack(alignment: .leading, spacing: 10) {
            Text("BY ASSET CLASS")
                .font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
            ForEach(sortedGroups, id: \.key) { group, value in
                groupRow(group: group, value: value, total: d.total_cents)
            }
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 20))
    }

    private func groupRow(group: String, value: Int64, total: Int64) -> some View {
        let pct = total != 0 ? Double(value) / Double(total) * 100 : 0
        return VStack(spacing: 6) {
            HStack {
                Text(prettyGroupName(group))
                    .font(.system(size: 14, weight: .medium))
                Spacer()
                Text(value.asUSD(decimals: 0))
                    .font(.system(size: 14, weight: .semibold))
                    .foregroundStyle(value < 0 ? Theme.financeBad : Theme.text)
                Text(String(format: "%.0f%%", abs(pct)))
                    .font(.caption)
                    .foregroundStyle(Theme.text3)
                    .frame(width: 36, alignment: .trailing)
            }
            GeometryReader { proxy in
                ZStack(alignment: .leading) {
                    RoundedRectangle(cornerRadius: 3)
                        .fill(Color.white.opacity(0.06))
                    RoundedRectangle(cornerRadius: 3)
                        .fill(groupColor(group))
                        .frame(width: proxy.size.width * CGFloat(max(0, min(abs(pct), 100))) / 100)
                }
            }
            .frame(height: 5)
        }
        .padding(.vertical, 4)
    }

    private func accountsCard(_ d: Investments) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("ACCOUNTS")
                .font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
                .padding(.bottom, 6)
            ForEach(d.accounts) { a in
                HStack(spacing: 10) {
                    ZStack {
                        RoundedRectangle(cornerRadius: 8).fill(groupColor(a.group).opacity(0.18))
                        Text(groupGlyph(a.group))
                            .font(.system(size: 14))
                    }
                    .frame(width: 36, height: 36)

                    VStack(alignment: .leading, spacing: 2) {
                        Text(a.name)
                            .font(.system(size: 14, weight: .medium))
                            .lineLimit(1)
                        Text("\(prettyGroupName(a.group)) · \(Int(a.pct_of_total))%")
                            .font(.caption)
                            .foregroundStyle(Theme.text3)
                    }
                    Spacer()
                    Text(a.balance_cents.asUSD(decimals: 0))
                        .font(.system(size: 14, weight: .semibold))
                        .foregroundStyle(a.balance_cents < 0 ? Theme.financeBad : Theme.finance)
                }
                .padding(.vertical, 6)
                if a.id != d.accounts.last?.id {
                    Divider().overlay(Theme.border)
                }
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
            Text("Loading investments…").foregroundStyle(Theme.text2)
        }
        .frame(maxWidth: .infinity, minHeight: 140)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 20))
    }

    private var errorCard: some View {
        VStack(spacing: 8) {
            Image(systemName: "exclamationmark.triangle.fill")
                .foregroundStyle(Theme.financeBad)
            Text(loadError ?? "Couldn't load investments.")
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

    private func prettyGroupName(_ group: String) -> String {
        switch group {
        case "retirement":  return "Retirement"
        case "crypto":      return "Crypto"
        case "real_estate": return "Real estate"
        case "equity":      return "Equity"
        case "cash":        return "Cash"
        default:            return "Other"
        }
    }

    private func groupColor(_ group: String) -> Color {
        switch group {
        case "retirement":  return Theme.goals
        case "crypto":      return Theme.ai
        case "real_estate": return Theme.healthMid
        case "equity":      return Theme.finance
        case "cash":        return Theme.healthBlue
        default:            return Theme.text3
        }
    }

    private func groupGlyph(_ group: String) -> String {
        switch group {
        case "retirement":  return "🏦"
        case "crypto":      return "₿"
        case "real_estate": return "🏠"
        case "equity":      return "📈"
        case "cash":        return "💵"
        default:            return "•"
        }
    }

    private func load() async {
        guard let api = app.apiClient() else {
            loadError = "Not paired."
            return
        }
        do {
            data = try await api.financeInvestments()
            loadError = nil
        } catch let e as APIClient.APIError {
            loadError = e.errorDescription
        } catch let e {
            loadError = e.localizedDescription
        }
    }
}
