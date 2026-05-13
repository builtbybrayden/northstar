import Foundation

struct Account: Decodable, Identifiable {
    let id: String
    let name: String
    let balance_cents: Int64
    let on_budget: Bool
    let closed: Bool
}

struct Transaction: Decodable, Identifiable {
    let id: String
    let account_id: String
    let account_name: String
    let date: String
    let payee: String
    let category: String
    /// Upstream (from Actual) category, only populated when the user has
    /// applied a local override via PATCH /api/finance/transactions/:id.
    /// Used by the detail sheet to show "Was: X · Reset".
    let category_original: String?
    let amount_cents: Int64
    let notes: String
}

struct CategorySummary: Decodable, Identifiable {
    var id: String { category }
    let category: String
    /// Parent grouping (Living Expenses / Transportation / Dining &
    /// Entertainment / Savings & Income / Miscellaneous). The server seeds
    /// it from a name heuristic; user can override per-category.
    let category_group: String?
    let spent_cents: Int64
    let budgeted_cents: Int64
    let pct: Int
    let over: Bool
}

struct FinanceSummary: Decodable {
    let month: String
    let net_worth_cents: Int64
    let on_budget_cents: Int64
    let off_budget_cents: Int64
    let income_cents: Int64
    let spent_cents: Int64
    let budgeted_cents: Int64
    let saved_cents: Int64
    let categories: [CategorySummary]
}

// ─── Forecast ────────────────────────────────────────────────────────────

struct ForecastEvent: Decodable, Hashable {
    let date: String?
    let label: String
    let amount_cents: Int64?
}

struct ForecastDay: Decodable, Identifiable {
    var id: String { date }
    let date: String
    let balance_cents: Int64
    let events: [ForecastEvent]?
}

struct RecurringCharge: Decodable, Identifiable {
    var id: String { "\(label)|\(amount_cents)|\(day_of_month)" }
    let label: String
    let amount_cents: Int64
    let day_of_month: Int
    let confidence: Double
}

struct Forecast: Decodable {
    let as_of: String
    let horizon_days: Int
    let current_balance_cents: Int64
    let daily_discretionary_cents: Int64
    let projected: [ForecastDay]
    let milestones: [ForecastEvent]
    let recurring: [RecurringCharge]
}

// ─── Investments ────────────────────────────────────────────────────────

struct InvestmentAccount: Decodable, Identifiable {
    let id: String
    let name: String
    let balance_cents: Int64
    let pct_of_total: Double
    let group: String
}

struct Investments: Decodable {
    let total_cents: Int64
    let groups: [String: Int64]
    let accounts: [InvestmentAccount]
}

extension Int64 {
    /// Formats cents as a USD currency string. Negative values keep their sign.
    func asUSD(decimals: Int = 2) -> String {
        let f = NumberFormatter()
        f.numberStyle = .currency
        f.currencyCode = "USD"
        f.maximumFractionDigits = decimals
        f.minimumFractionDigits = decimals
        return f.string(from: NSNumber(value: Double(self) / 100.0)) ?? "$0"
    }
}
