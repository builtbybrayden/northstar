import SwiftUI

struct SettingsView: View {
    @EnvironmentObject private var app: AppState
    @Environment(\.dismiss) private var dismiss

    @State private var rules: [NotificationRule] = []
    @State private var loading = true
    @State private var loadError: String?
    @State private var savingCategory: String?
    @State private var editingRule: NotificationRule?

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 24) {
                    connectionsSection
                    notificationRulesSection
                    pillarsSection
                    dataSection
                    signOutSection
                }
                .padding(.horizontal, 16)
                .padding(.vertical, 12)
            }
            .background(Theme.bg)
            .navigationTitle("Settings")
            .toolbarBackground(Theme.bg, for: .navigationBar)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Button("Done") { dismiss() }
                        .foregroundStyle(Theme.ai)
                }
            }
            .task { await load() }
            .sheet(item: $editingRule) { rule in
                NotificationRuleSheet(rule: rule) { updated in
                    if let i = rules.firstIndex(where: { $0.category == updated.category }) {
                        rules[i] = updated
                    }
                }
                .environmentObject(app)
            }
        }
    }

    private func load() async {
        guard let api = app.apiClient() else {
            loadError = "Not paired."; loading = false; return
        }
        do {
            rules = try await api.notificationRules()
            loadError = nil
        } catch let e as APIClient.APIError {
            loadError = e.errorDescription
        } catch let e {
            loadError = e.localizedDescription
        }
        loading = false
    }

    // ─── Sections ────────────────────────────────────────────────────────

    private var connectionsSection: some View {
        section("CONNECTIONS") {
            connectionRow(icon: "dollarsign.circle.fill", color: Theme.finance,
                          title: "Actual Budget", status: "connected (mock)")
            connectionRow(icon: "heart.fill", color: Theme.healthGo,
                          title: "WHOOP", status: "not yet (Phase 4)")
            connectionRow(icon: "sparkles", color: Theme.ai,
                          title: "Claude API", status: "not yet (Phase 5)")
            connectionRow(icon: "star.fill", color: Theme.goals,
                          title: "Goals", status: "native — no external connection",
                          statusColor: Theme.text3)
        }
    }

    private var notificationRulesSection: some View {
        section("NOTIFICATION RULES") {
            if loading { ProgressView().tint(Theme.text2).padding(.vertical, 12) }
            if let loadError {
                Text(loadError).foregroundStyle(Theme.financeBad).font(.footnote)
            }
            ForEach(rules) { rule in
                ruleRow(rule)
                if rule.id != rules.last?.id {
                    Divider().overlay(Theme.border).padding(.leading, 16)
                }
            }
        }
    }

    private var pillarsSection: some View {
        section("PILLARS") {
            pillarRow("Finance", icon: "dollarsign.circle.fill", color: Theme.finance, enabled: true)
            pillarRow("Goals", icon: "star.fill", color: Theme.goals, enabled: true)
            pillarRow("Health", icon: "heart.fill", color: Theme.healthGo, enabled: true)
            pillarRow("Ask Claude", icon: "sparkles", color: Theme.ai, enabled: true)
        }
    }

    private var dataSection: some View {
        section("DATA") {
            HStack {
                Text("Server URL").foregroundStyle(Theme.text)
                Spacer()
                Text(app.serverURL?.absoluteString ?? "—")
                    .foregroundStyle(Theme.text3)
                    .font(.footnote)
                    .lineLimit(1)
                    .truncationMode(.middle)
            }
            .padding(.horizontal, 16).padding(.vertical, 12)
            HStack {
                Text("Device ID").foregroundStyle(Theme.text)
                Spacer()
                Text(app.deviceID ?? "—")
                    .foregroundStyle(Theme.text3)
                    .font(.footnote)
                    .lineLimit(1)
                    .truncationMode(.middle)
            }
            .padding(.horizontal, 16).padding(.vertical, 12)
        }
    }

    private var signOutSection: some View {
        Button(role: .destructive) {
            app.signOut()
            dismiss()
        } label: {
            Text("Sign out")
                .frame(maxWidth: .infinity)
                .padding(.vertical, 14)
                .foregroundStyle(Theme.financeBad)
        }
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 16))
    }

    // ─── Row helpers ─────────────────────────────────────────────────────

    private func section<Content: View>(_ title: String,
                                        @ViewBuilder _ content: () -> Content) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(title)
                .font(.caption2).bold().tracking(1.5)
                .foregroundStyle(Theme.text3)
                .padding(.horizontal, 4)
            VStack(spacing: 0) { content() }
                .background(Theme.surface)
                .clipShape(RoundedRectangle(cornerRadius: 16))
        }
    }

    private func connectionRow(icon: String, color: Color, title: String,
                               status: String, statusColor: Color = Theme.healthGo) -> some View {
        HStack {
            Image(systemName: icon).foregroundStyle(color).frame(width: 28)
            Text(title).foregroundStyle(Theme.text)
            Spacer()
            Text(status).font(.footnote).foregroundStyle(statusColor)
        }
        .padding(.horizontal, 16).padding(.vertical, 12)
        .overlay(Divider().overlay(Theme.border).padding(.leading, 56),
                 alignment: .bottom)
    }

    private func ruleRow(_ rule: NotificationRule) -> some View {
        let label = displayLabel(for: rule.category)
        let saving = savingCategory == rule.category
        let quietSummary: String = {
            if !rule.quiet_hours_start.isEmpty && !rule.quiet_hours_end.isEmpty {
                return "quiet \(rule.quiet_hours_start)–\(rule.quiet_hours_end)"
            }
            return "no quiet hours"
        }()
        return Button { editingRule = rule } label: {
            HStack {
                VStack(alignment: .leading, spacing: 2) {
                    Text(label.title).foregroundStyle(Theme.text)
                    Text("\(label.sub) · \(quietSummary)")
                        .font(.caption).foregroundStyle(Theme.text3)
                }
                Spacer()
                if saving { ProgressView().tint(Theme.text2) }
                Toggle("", isOn: Binding(
                    get: { rule.enabled },
                    set: { newValue in Task { await toggleRule(rule, enabled: newValue) } }
                ))
                .labelsHidden()
                .tint(Theme.healthGo)
                Image(systemName: "chevron.right")
                    .font(.caption).foregroundStyle(Theme.text3)
            }
            .padding(.horizontal, 16).padding(.vertical, 12)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }

    private func pillarRow(_ name: String, icon: String, color: Color, enabled: Bool) -> some View {
        HStack {
            Image(systemName: icon).foregroundStyle(color).frame(width: 28)
            Text(name).foregroundStyle(Theme.text)
            Spacer()
            Toggle("", isOn: .constant(enabled))
                .labelsHidden()
                .disabled(true) // server-controlled in v1
                .tint(Theme.healthGo)
        }
        .padding(.horizontal, 16).padding(.vertical, 12)
        .overlay(Divider().overlay(Theme.border).padding(.leading, 56),
                 alignment: .bottom)
    }

    // ─── Mutations ───────────────────────────────────────────────────────

    private func toggleRule(_ rule: NotificationRule, enabled: Bool) async {
        guard let api = app.apiClient() else { return }
        savingCategory = rule.category
        defer { savingCategory = nil }
        do {
            try await api.updateRule(category: rule.category,
                                     APIClient.RuleUpdate(enabled: enabled))
            if let i = rules.firstIndex(where: { $0.category == rule.category }) {
                rules[i].enabled = enabled
            }
        } catch {
            // Reload to revert
            await load()
        }
    }

    // ─── Label dictionary ────────────────────────────────────────────────

    private struct DisplayLabel { let title: String; let sub: String }
    private func displayLabel(for category: String) -> DisplayLabel {
        switch category {
        case "purchase":          return .init(title: "Purchases", sub: "Every transaction")
        case "budget_threshold":  return .init(title: "Budget thresholds", sub: "50 / 75 / 90 / 100%")
        case "anomaly":           return .init(title: "Anomalies", sub: "New merchant + spend spikes")
        case "daily_brief":       return .init(title: "Daily brief", sub: "Morning push")
        case "evening_retro":     return .init(title: "Evening retro", sub: "Daily log nudge")
        case "supplement":        return .init(title: "Supplements", sub: "Scheduled doses")
        case "health_insight":    return .init(title: "Health insights", sub: "Recovery dips, HRV outliers")
        case "goal_milestone":    return .init(title: "Goal milestones", sub: "Status changes")
        case "subscription_new":  return .init(title: "New subscriptions", sub: "Recurring charge detected")
        case "weekly_retro":      return .init(title: "Weekly retro", sub: "Friday evening")
        default:                  return .init(title: category, sub: "")
        }
    }
}
