import Foundation

struct AppNotification: Decodable, Identifiable, Sendable {
    let id: String
    let category: String
    let title: String
    let body: String
    let priority: Int
    let payload: [String: AnyCodable]?
    let created_at: Int64
    let read_at: Int64?
    let delivery_status: String
}

struct NotificationRule: Decodable, Identifiable {
    var id: String { category }
    let category: String
    var enabled: Bool
    var quiet_hours_start: String
    var quiet_hours_end: String
    var bypass_quiet: Bool
    var delivery: String
    var max_per_day: Int
}

struct BudgetTarget: Decodable, Identifiable {
    var id: String { category }
    let category: String
    var category_group: String?
    var monthly_cents: Int64
    let rationale: String
    var threshold_pcts: [Int]
    var push_enabled: Bool
}

// Minimal type-erased Codable for arbitrary JSON payloads. `value: Any`
// can't be checked Sendable by the compiler, but the only writes happen
// during decode and the value is never mutated afterward — safe to mark
// @unchecked Sendable for cross-actor transport.
struct AnyCodable: Decodable, @unchecked Sendable {
    let value: Any
    init(from decoder: Decoder) throws {
        let c = try decoder.singleValueContainer()
        if let v = try? c.decode(Bool.self)   { value = v; return }
        if let v = try? c.decode(Int64.self)  { value = v; return }
        if let v = try? c.decode(Double.self) { value = v; return }
        if let v = try? c.decode(String.self) { value = v; return }
        if c.decodeNil() { value = NSNull(); return }
        value = NSNull()
    }
}
