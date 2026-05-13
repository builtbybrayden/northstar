import SwiftUI

/// One collapsible card per parent group on the Finance tab.
///
/// Collapsed (default): name + chevron + total spent / total budgeted.
/// Expanded: each leaf category as a tappable row that drills into a
/// per-category transaction list. Long-press / explicit edit lives on
/// the existing budget-edit sheet via the parent's `onEditBudget` closure.
struct FinanceGroupCard: View {
    let name: String
    let categories: [CategorySummary]
    let summaryMonth: String
    let availableCategories: [String]
    let onEditBudget: (CategorySummary) -> Void

    @State private var expanded = false

    private var totalSpent: Int64   { categories.reduce(0) { $0 + $1.spent_cents } }
    private var totalBudget: Int64  { categories.reduce(0) { $0 + $1.budgeted_cents } }
    private var pct: Int {
        guard totalBudget > 0 else { return 0 }
        return Int((totalSpent * 100) / totalBudget)
    }
    private var over: Bool { totalBudget > 0 && totalSpent > totalBudget }

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            header
            if expanded {
                Divider().overlay(Theme.border)
                    .padding(.horizontal, -18)
                    .padding(.vertical, 10)
                leafRows
            }
        }
        .padding(18)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 20))
    }

    // ─── Header (always visible) ─────────────────────────────────────────

    private var header: some View {
        Button {
            withAnimation(.easeInOut(duration: 0.18)) { expanded.toggle() }
        } label: {
            HStack(alignment: .center, spacing: 12) {
                Image(systemName: expanded ? "chevron.down" : "chevron.right")
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(Theme.text3)
                    .frame(width: 14)

                VStack(alignment: .leading, spacing: 4) {
                    Text(name.uppercased())
                        .font(.caption.bold())
                        .tracking(1.1)
                        .foregroundStyle(groupColor(name))
                    HStack(spacing: 6) {
                        Text("\(categories.count) categor\(categories.count == 1 ? "y" : "ies")")
                            .font(.caption2)
                            .foregroundStyle(Theme.text3)
                        if totalBudget > 0 {
                            Text("· \(pct)% used")
                                .font(.caption2)
                                .foregroundStyle(over ? Theme.financeBad : Theme.text3)
                        }
                    }
                }
                Spacer()
                VStack(alignment: .trailing, spacing: 2) {
                    Text(totalSpent.asUSD(decimals: 0))
                        .font(.system(size: 18, weight: .heavy))
                        .foregroundStyle(over ? Theme.financeBad : Theme.text)
                    if totalBudget > 0 {
                        Text("of \(totalBudget.asUSD(decimals: 0))")
                            .font(.caption2)
                            .foregroundStyle(Theme.text3)
                    }
                }
            }
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }

    // ─── Leaf rows (only when expanded) ──────────────────────────────────

    private var leafRows: some View {
        VStack(spacing: 4) {
            ForEach(categories) { c in
                HStack(spacing: 8) {
                    NavigationLink {
                        CategoryTransactionsView(
                            category: c,
                            month: summaryMonth,
                            availableCategories: availableCategories
                        )
                    } label: {
                        FinanceCategoryBar(category: c)
                            .contentShape(Rectangle())
                    }
                    .buttonStyle(.plain)

                    // Edit-budget shortcut. Wee chevron-arrow so the row's
                    // primary tap target (drilldown) stays the obvious one.
                    Button {
                        onEditBudget(c)
                    } label: {
                        Image(systemName: "slider.horizontal.3")
                            .font(.caption)
                            .foregroundStyle(Theme.text3)
                            .padding(6)
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel("Edit \(c.category) budget")
                }
            }
        }
    }

    // ─── Style ────────────────────────────────────────────────────────────

    private func groupColor(_ name: String) -> Color {
        switch name {
        case "Living Expenses":         return Theme.healthBlue
        case "Transportation":          return Theme.healthMid
        case "Dining & Entertainment":  return Theme.ai
        case "Savings & Income":        return Theme.finance
        default:                        return Theme.text3
        }
    }
}
