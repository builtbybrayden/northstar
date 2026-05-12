import Foundation

struct Milestone: Codable, Identifiable {
    let id: String
    var title: String
    var description_md: String
    var due_date: String
    var status: String
    var flagship: Bool
    var display_order: Int
    let created_at: Int64
    let updated_at: Int64
}

struct DailyItem: Codable, Identifiable {
    var id: String
    var text: String
    var done: Bool
    var source: String
    var source_ref: String?
}

struct DailyLog: Codable {
    let date: String
    var items: [DailyItem]
    var reflection_md: String
    let streak_count: Int
    let updated_at: Int64
}

struct OutputEntry: Codable, Identifiable {
    let id: String
    let date: String
    let category: String
    let title: String
    let body_md: String
    let url: String
    let created_at: Int64
}

struct NetworkingEntry: Codable, Identifiable {
    let id: String
    let date: String
    let person: String
    let context: String
    let next_action: String
    let next_action_due: String
    let created_at: Int64
}

struct Reminder: Codable, Identifiable {
    let id: String
    var title: String
    var body: String
    var recurrence: String
    let next_fires_at: Int64
    var active: Bool
    let created_at: Int64
}

struct Brief: Codable {
    let date: String
    let items: [DailyItem]
    let streak_count: Int
    let milestones_due_soon: [Milestone]
    let active_reminders: [Reminder]
}
