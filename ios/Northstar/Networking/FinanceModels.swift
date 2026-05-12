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
    let amount_cents: Int64
    let notes: String
}

struct CategorySummary: Decodable, Identifiable {
    var id: String { category }
    let category: String
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
