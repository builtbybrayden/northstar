import SwiftUI

struct NotificationRuleSheet: View {
    @EnvironmentObject private var app: AppState
    @Environment(\.dismiss) private var dismiss

    let rule: NotificationRule
    let onSaved: (NotificationRule) -> Void

    @State private var enabled = true
    @State private var bypassQuiet = false
    @State private var quietEnabled = false
    @State private var quietStart = Date()
    @State private var quietEnd = Date()
    @State private var maxPerDay = 5
    @State private var saving = false
    @State private var saveError: String?

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    Toggle(isOn: $enabled) {
                        Text("Enabled").foregroundStyle(Theme.text)
                    }
                    .tint(Theme.healthGo)
                    .padding(.horizontal, 14).padding(.vertical, 10)
                    .background(Theme.surface)
                    .clipShape(RoundedRectangle(cornerRadius: 12))

                    quietHoursSection
                    capSection

                    Toggle(isOn: $bypassQuiet) {
                        VStack(alignment: .leading) {
                            Text("Bypass quiet hours").foregroundStyle(Theme.text)
                            Text("Critical alerts will fire even during quiet hours")
                                .font(.caption).foregroundStyle(Theme.text3)
                        }
                    }
                    .tint(Theme.financeBad)
                    .padding(.horizontal, 14).padding(.vertical, 10)
                    .background(Theme.surface)
                    .clipShape(RoundedRectangle(cornerRadius: 12))

                    if let saveError {
                        Text(saveError).foregroundStyle(Theme.financeBad).font(.footnote)
                    }
                }
                .padding(20)
            }
            .background(Theme.bg)
            .navigationTitle(prettyName(rule.category))
            .toolbarBackground(Theme.bg, for: .navigationBar)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Button("Cancel") { dismiss() }.foregroundStyle(Theme.text2)
                }
                ToolbarItem(placement: .topBarTrailing) {
                    Button { Task { await save() } } label: {
                        if saving { ProgressView().tint(Theme.ai) }
                        else { Text("Save").bold().foregroundStyle(Theme.ai) }
                    }
                    .disabled(saving)
                }
            }
            .onAppear(perform: loadInitial)
        }
    }

    private var quietHoursSection: some View {
        VStack(alignment: .leading, spacing: 6) {
            Toggle(isOn: $quietEnabled) {
                Text("QUIET HOURS")
                    .font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
            }
            .tint(Theme.ai)
            if quietEnabled {
                HStack {
                    Text("From").foregroundStyle(Theme.text2)
                    Spacer()
                    DatePicker("", selection: $quietStart, displayedComponents: .hourAndMinute)
                        .labelsHidden()
                        .tint(Theme.ai)
                }
                .padding(.horizontal, 14).padding(.vertical, 10)
                .background(Color(hex: 0x0f0f0f))
                .clipShape(RoundedRectangle(cornerRadius: 12))
                HStack {
                    Text("To").foregroundStyle(Theme.text2)
                    Spacer()
                    DatePicker("", selection: $quietEnd, displayedComponents: .hourAndMinute)
                        .labelsHidden()
                        .tint(Theme.ai)
                }
                .padding(.horizontal, 14).padding(.vertical, 10)
                .background(Color(hex: 0x0f0f0f))
                .clipShape(RoundedRectangle(cornerRadius: 12))
                Text("Wrap-around windows like 22:00–07:00 are supported.")
                    .font(.caption).foregroundStyle(Theme.text3)
                    .padding(.horizontal, 4)
            }
        }
    }

    private var capSection: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("DAILY CAP")
                .font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
            HStack {
                Stepper("Max \(maxPerDay) per day", value: $maxPerDay, in: 1...99)
                    .foregroundStyle(Theme.text)
            }
            .padding(.horizontal, 14).padding(.vertical, 10)
            .background(Color(hex: 0x0f0f0f))
            .clipShape(RoundedRectangle(cornerRadius: 12))
        }
    }

    private func loadInitial() {
        enabled = rule.enabled
        bypassQuiet = rule.bypass_quiet
        maxPerDay = max(1, rule.max_per_day)
        let parsed = (parseTime(rule.quiet_hours_start), parseTime(rule.quiet_hours_end))
        if let s = parsed.0, let e = parsed.1 {
            quietEnabled = true
            quietStart = s
            quietEnd = e
        } else {
            quietEnabled = false
            quietStart = setHour(22)
            quietEnd = setHour(7)
        }
    }

    private func parseTime(_ raw: String) -> Date? {
        let parts = raw.split(separator: ":")
        guard parts.count == 2,
              let h = Int(parts[0]), let m = Int(parts[1]) else { return nil }
        var cal = Calendar.current; cal.timeZone = .current
        return cal.date(bySettingHour: h, minute: m, second: 0, of: Date())
    }

    private func setHour(_ h: Int) -> Date {
        var cal = Calendar.current; cal.timeZone = .current
        return cal.date(bySettingHour: h, minute: 0, second: 0, of: Date()) ?? Date()
    }

    private func format(_ d: Date) -> String {
        let f = DateFormatter()
        f.timeZone = .current
        f.dateFormat = "HH:mm"
        return f.string(from: d)
    }

    private func save() async {
        guard let api = app.apiClient() else { return }
        saving = true; saveError = nil
        defer { saving = false }
        let update = APIClient.RuleUpdate(
            enabled: enabled,
            quiet_hours_start: quietEnabled ? format(quietStart) : "",
            quiet_hours_end: quietEnabled ? format(quietEnd) : "",
            bypass_quiet: bypassQuiet,
            max_per_day: maxPerDay
        )
        do {
            try await api.updateRule(category: rule.category, update)
            var updated = rule
            updated.enabled = enabled
            updated.quiet_hours_start = quietEnabled ? format(quietStart) : ""
            updated.quiet_hours_end = quietEnabled ? format(quietEnd) : ""
            updated.bypass_quiet = bypassQuiet
            updated.max_per_day = maxPerDay
            onSaved(updated)
            dismiss()
        } catch {
            saveError = (error as? APIClient.APIError)?.errorDescription ?? error.localizedDescription
        }
    }

    private func prettyName(_ category: String) -> String {
        switch category {
        case "purchase":          return "Purchases"
        case "budget_threshold":  return "Budget thresholds"
        case "anomaly":           return "Anomalies"
        case "daily_brief":       return "Daily brief"
        case "evening_retro":     return "Evening retro"
        case "supplement":        return "Supplements"
        case "health_insight":    return "Health insights"
        case "goal_milestone":    return "Goal milestones"
        case "subscription_new":  return "New subscriptions"
        case "weekly_retro":      return "Weekly retro"
        default:                  return category
        }
    }
}
