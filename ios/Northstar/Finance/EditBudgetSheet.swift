import SwiftUI

/// Bottom sheet for editing one category's budget cap, threshold ladder, and
/// push toggle. Mirrors mockup screen 8.
struct EditBudgetSheet: View {
    @EnvironmentObject private var app: AppState
    @Environment(\.dismiss) private var dismiss

    let target: BudgetTarget
    let onSaved: () -> Void

    @State private var dollarsString: String
    @State private var thresholdSelections: Set<Int>
    @State private var pushEnabled: Bool
    @State private var selectedGroup: String
    @State private var saving = false
    @State private var saveError: String?

    private static let availableThresholds = [50, 75, 90, 100]

    /// Must match `finance.AllGroups` on the server. Order is intentional —
    /// fixed needs first, discretionary next, income, catch-all last.
    private static let availableGroups = [
        "Living Expenses",
        "Transportation",
        "Dining & Entertainment",
        "Savings & Income",
        "Miscellaneous",
    ]

    init(target: BudgetTarget, onSaved: @escaping () -> Void) {
        self.target = target
        self.onSaved = onSaved
        _dollarsString = State(initialValue: String(target.monthly_cents / 100))
        _thresholdSelections = State(initialValue: Set(target.threshold_pcts))
        _pushEnabled = State(initialValue: target.push_enabled)
        let initialGroup = target.category_group ?? "Miscellaneous"
        _selectedGroup = State(initialValue:
            Self.availableGroups.contains(initialGroup) ? initialGroup : "Miscellaneous")
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            Capsule()
                .fill(Color.gray.opacity(0.5))
                .frame(width: 38, height: 4)
                .frame(maxWidth: .infinity)
                .padding(.top, 8)

            VStack(alignment: .leading, spacing: 4) {
                Text(target.category)
                    .font(.system(size: 22, weight: .heavy))
                Text("Edit budget, thresholds, and alerts")
                    .font(.footnote)
                    .foregroundStyle(Theme.text3)
            }

            monthlyCapField
            thresholdsField
            groupField
            behaviorCard

            if let saveError {
                Text(saveError)
                    .font(.footnote)
                    .foregroundStyle(Theme.financeBad)
            }

            VStack(spacing: 8) {
                Button(action: { Task { await save() } }) {
                    HStack {
                        if saving { ProgressView().tint(.black) }
                        Text(saving ? "Saving…" : "Save")
                            .font(.system(.body, weight: .semibold))
                    }
                    .frame(maxWidth: .infinity)
                    .padding(.vertical, 14)
                    .background(Theme.ai)
                    .foregroundStyle(.black)
                    .clipShape(RoundedRectangle(cornerRadius: 16))
                }
                .disabled(saving)

                Button("Cancel") { dismiss() }
                    .foregroundStyle(Theme.text2)
                    .padding(.vertical, 8)
            }
            Spacer(minLength: 0)
        }
        .padding(.horizontal, 22)
        .padding(.bottom, 24)
        .background(Color(hex: 0x1c1c1c))
        .presentationDetents([.medium, .large])
        .presentationDragIndicator(.hidden)
    }

    // ─── Subviews ────────────────────────────────────────────────────────

    private var monthlyCapField: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("MONTHLY CAP")
                .font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
            HStack {
                Text("$").foregroundStyle(Theme.text3).font(.system(size: 22, weight: .semibold))
                TextField("500", text: $dollarsString)
                    .keyboardType(.numberPad)
                    .font(.system(size: 28, weight: .heavy))
                    .foregroundStyle(Theme.text)
                    .tint(Theme.ai)
            }
            .padding(.horizontal, 16).padding(.vertical, 14)
            .background(Color(hex: 0x0f0f0f))
            .clipShape(RoundedRectangle(cornerRadius: 14))
        }
    }

    private var thresholdsField: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("ALERT THRESHOLDS")
                .font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
            HStack(spacing: 8) {
                ForEach(Self.availableThresholds, id: \.self) { t in
                    Button(action: { toggle(t) }) {
                        Text("\(t)%")
                            .font(.system(size: 13, weight: .semibold))
                            .padding(.horizontal, 14).padding(.vertical, 8)
                            .background(thresholdSelections.contains(t)
                                        ? Theme.ai.opacity(0.12) : Color(hex: 0x0f0f0f))
                            .foregroundStyle(thresholdSelections.contains(t)
                                             ? Theme.ai : Theme.text2)
                            .overlay(
                                RoundedRectangle(cornerRadius: 20)
                                    .stroke(thresholdSelections.contains(t) ? Theme.ai : Color.clear,
                                            lineWidth: 1))
                            .clipShape(Capsule())
                    }
                }
            }
        }
    }

    private var groupField: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("GROUP")
                .font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
            ScrollView(.horizontal, showsIndicators: false) {
                HStack(spacing: 8) {
                    ForEach(Self.availableGroups, id: \.self) { g in
                        Button(action: { selectedGroup = g }) {
                            Text(g)
                                .font(.system(size: 13, weight: .semibold))
                                .padding(.horizontal, 14).padding(.vertical, 8)
                                .background(selectedGroup == g
                                            ? Theme.ai.opacity(0.12) : Color(hex: 0x0f0f0f))
                                .foregroundStyle(selectedGroup == g
                                                 ? Theme.ai : Theme.text2)
                                .overlay(
                                    RoundedRectangle(cornerRadius: 20)
                                        .stroke(selectedGroup == g ? Theme.ai : Color.clear,
                                                lineWidth: 1))
                                .clipShape(Capsule())
                        }
                    }
                }
            }
        }
    }

    private var behaviorCard: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("BEHAVIOR")
                .font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
            HStack {
                VStack(alignment: .leading) {
                    Text("Push notification")
                        .font(.system(size: 14, weight: .medium))
                        .foregroundStyle(Theme.text)
                    Text("Lock-screen alert when a threshold hits")
                        .font(.caption).foregroundStyle(Theme.text3)
                }
                Spacer()
                Toggle("", isOn: $pushEnabled)
                    .labelsHidden().tint(Theme.healthGo)
            }
            .padding(.horizontal, 16).padding(.vertical, 14)
            .background(Theme.surface)
            .clipShape(RoundedRectangle(cornerRadius: 14))
        }
    }

    // ─── Plumbing ────────────────────────────────────────────────────────

    private func toggle(_ t: Int) {
        if thresholdSelections.contains(t) {
            thresholdSelections.remove(t)
        } else {
            thresholdSelections.insert(t)
        }
    }

    private func save() async {
        guard let api = app.apiClient() else { return }
        guard let dollars = Int64(dollarsString.filter(\.isNumber)) else {
            saveError = "Cap must be a whole-dollar number."
            return
        }
        saving = true; saveError = nil
        defer { saving = false }
        do {
            let update = APIClient.BudgetTargetUpdate(
                monthly_cents: dollars * 100,
                threshold_pcts: thresholdSelections.sorted(),
                push_enabled: pushEnabled,
                category_group: selectedGroup
            )
            try await api.updateBudgetTarget(category: target.category, update)
            onSaved()
            dismiss()
        } catch let e as APIClient.APIError {
            saveError = e.errorDescription
        } catch let e {
            saveError = e.localizedDescription
        }
    }
}
