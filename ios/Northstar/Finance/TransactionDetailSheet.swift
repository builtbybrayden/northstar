import SwiftUI

/// Bottom sheet shown when the user taps a transaction row in Finance.
/// Read-only details (payee, amount, account, date, notes) plus a category
/// picker that PATCHes the row's `category_user` server-side. Selecting the
/// same category as the upstream Actual value clears the override.
struct TransactionDetailSheet: View {
    @EnvironmentObject private var app: AppState
    @Environment(\.dismiss) private var dismiss

    let txn: Transaction
    /// Categories drawn from `FinanceSummary.categories` — the budgeted set.
    /// We also splice in an "Uncategorized" option so the user can strip a
    /// stale category off a transaction.
    let availableCategories: [String]
    let onSaved: () -> Void

    @State private var selectedCategory: String
    @State private var saving = false
    @State private var saveError: String?

    init(txn: Transaction, availableCategories: [String], onSaved: @escaping () -> Void) {
        self.txn = txn
        self.availableCategories = availableCategories
        self.onSaved = onSaved
        _selectedCategory = State(initialValue: txn.category)
    }

    private var pickerOptions: [String] {
        // Stable, deduped union of the budgeted set plus whatever this txn
        // is currently in (so the picker can render the current value even
        // if it's not in the budget list yet — e.g., "Transfer").
        var seen = Set<String>()
        var out: [String] = []
        for c in availableCategories + [txn.category] {
            let trimmed = c.trimmingCharacters(in: .whitespaces)
            if trimmed.isEmpty || seen.contains(trimmed) { continue }
            seen.insert(trimmed)
            out.append(trimmed)
        }
        return out
    }

    private var hasOverride: Bool {
        if let orig = txn.category_original, !orig.isEmpty { return true }
        return false
    }

    private var hasChange: Bool {
        selectedCategory.trimmingCharacters(in: .whitespaces) != txn.category
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            Capsule()
                .fill(Color.gray.opacity(0.5))
                .frame(width: 38, height: 4)
                .frame(maxWidth: .infinity)
                .padding(.top, 8)

            header
            detailsCard
            categoryCard

            if let saveError {
                Text(saveError)
                    .font(.footnote)
                    .foregroundStyle(Theme.financeBad)
            }

            actionRow
        }
        .padding(.horizontal, 20)
        .padding(.bottom, 28)
        .background(Theme.bg)
    }

    // ─── Subviews ────────────────────────────────────────────────────────

    private var header: some View {
        HStack(alignment: .top) {
            VStack(alignment: .leading, spacing: 4) {
                Text(txn.payee.isEmpty ? "—" : txn.payee)
                    .font(.system(size: 22, weight: .heavy))
                    .lineLimit(2)
                Text(txn.date)
                    .font(.footnote)
                    .foregroundStyle(Theme.text3)
            }
            Spacer()
            Text(txn.amount_cents.asUSD(decimals: 2))
                .font(.system(size: 24, weight: .heavy))
                .foregroundStyle(txn.amount_cents < 0 ? Theme.financeBad : Theme.finance)
        }
    }

    private var detailsCard: some View {
        VStack(alignment: .leading, spacing: 10) {
            detailRow(label: "Account", value: txn.account_name.isEmpty ? txn.account_id : txn.account_name)
            if !txn.notes.isEmpty {
                detailRow(label: "Notes", value: txn.notes)
            }
            detailRow(label: "Txn ID", value: txn.id, monospaced: true)
        }
        .padding(14)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 14))
    }

    private func detailRow(label: String, value: String, monospaced: Bool = false) -> some View {
        HStack(alignment: .top) {
            Text(label.uppercased())
                .font(.caption2).bold().tracking(1)
                .foregroundStyle(Theme.text3)
                .frame(width: 70, alignment: .leading)
            Text(value)
                .font(.system(size: 14, design: monospaced ? .monospaced : .default))
                .foregroundStyle(Theme.text2)
                .lineLimit(3)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
    }

    private var categoryCard: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack {
                Text("CATEGORY")
                    .font(.caption2).bold().tracking(1)
                    .foregroundStyle(Theme.text3)
                Spacer()
                if hasOverride {
                    Text("OVERRIDDEN")
                        .font(.caption2).bold().tracking(1)
                        .foregroundStyle(Theme.ai)
                        .padding(.horizontal, 8).padding(.vertical, 2)
                        .background(Theme.ai.opacity(0.18))
                        .clipShape(Capsule())
                }
            }

            // Chips wrap to two rows on small screens.
            FlowLayout(spacing: 8) {
                ForEach(pickerOptions, id: \.self) { c in
                    Button {
                        selectedCategory = c
                    } label: {
                        Text(c)
                            .font(.system(size: 13, weight: .medium))
                            .padding(.horizontal, 12).padding(.vertical, 7)
                            .background(c == selectedCategory ? Theme.finance : Theme.surfaceHi)
                            .foregroundStyle(c == selectedCategory ? .white : Theme.text2)
                            .clipShape(Capsule())
                    }
                    .buttonStyle(.plain)
                }
            }

            if hasOverride, let original = txn.category_original {
                HStack(spacing: 6) {
                    Text("Was \(original) (from Actual)")
                        .font(.caption)
                        .foregroundStyle(Theme.text3)
                    Spacer()
                    Button("Reset") {
                        Task { await resetOverride() }
                    }
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(Theme.ai)
                    .disabled(saving)
                }
            }
        }
        .padding(14)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 14))
    }

    private var actionRow: some View {
        HStack(spacing: 12) {
            Button("Cancel") { dismiss() }
                .buttonStyle(.bordered)
                .tint(Theme.text2)
                .disabled(saving)

            Spacer()

            Button {
                Task { await save() }
            } label: {
                if saving {
                    ProgressView().tint(.white)
                } else {
                    Text("Save").bold()
                }
            }
            .buttonStyle(.borderedProminent)
            .tint(Theme.finance)
            .disabled(!hasChange || saving)
        }
    }

    // ─── Actions ─────────────────────────────────────────────────────────

    private func save() async {
        guard let api = app.apiClient() else { return }
        saving = true
        defer { saving = false }
        do {
            try await api.updateTransactionCategory(id: txn.id, category: selectedCategory)
            onSaved()
            dismiss()
        } catch let e as APIClient.APIError {
            saveError = e.errorDescription
        } catch let e {
            saveError = e.localizedDescription
        }
    }

    private func resetOverride() async {
        guard let api = app.apiClient() else { return }
        saving = true
        defer { saving = false }
        do {
            try await api.resetTransactionCategory(id: txn.id)
            onSaved()
            dismiss()
        } catch let e as APIClient.APIError {
            saveError = e.errorDescription
        } catch let e {
            saveError = e.localizedDescription
        }
    }
}

// MARK: - FlowLayout

/// Wraps child views onto multiple rows, like CSS flex-wrap. Used here for
/// the category chip cloud so it adapts to phone width without horizontal
/// scrolling.
struct FlowLayout: Layout {
    var spacing: CGFloat = 8

    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        let maxWidth = proposal.width ?? .infinity
        let rows = arrange(subviews: subviews, maxWidth: maxWidth)
        let height = rows.reduce(0) { $0 + $1.height } + spacing * CGFloat(max(0, rows.count - 1))
        return CGSize(width: maxWidth, height: height)
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        let rows = arrange(subviews: subviews, maxWidth: bounds.width)
        var y = bounds.minY
        for row in rows {
            var x = bounds.minX
            for item in row.items {
                let size = subviews[item.index].sizeThatFits(.unspecified)
                subviews[item.index].place(at: CGPoint(x: x, y: y), proposal: .init(size))
                x += size.width + spacing
            }
            y += row.height + spacing
        }
    }

    private struct Row {
        var items: [(index: Int, width: CGFloat)] = []
        var height: CGFloat = 0
    }

    private func arrange(subviews: Subviews, maxWidth: CGFloat) -> [Row] {
        var rows: [Row] = [Row()]
        var rowWidth: CGFloat = 0
        for i in subviews.indices {
            let s = subviews[i].sizeThatFits(.unspecified)
            let needed = rowWidth == 0 ? s.width : rowWidth + spacing + s.width
            if needed > maxWidth && rowWidth > 0 {
                rows.append(Row())
                rowWidth = 0
            }
            rows[rows.count - 1].items.append((i, s.width))
            rows[rows.count - 1].height = max(rows[rows.count - 1].height, s.height)
            rowWidth = rowWidth == 0 ? s.width : rowWidth + spacing + s.width
        }
        return rows
    }
}
