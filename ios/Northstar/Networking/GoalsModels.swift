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

// ─── Habits ──────────────────────────────────────────────────────────────

struct Habit: Codable, Identifiable {
    let id: String
    var name: String
    var description_md: String
    var color: String
    var target_per_week: Int
    var active: Bool
    var display_order: Int
    let created_at: Int64
    let updated_at: Int64
}

struct HabitEntry: Codable, Identifiable, Hashable {
    var id: String { "\(habit_id)|\(date)" }
    let habit_id: String
    let date: String
    var count: Int
    var notes: String
    let updated_at: Int64
}

struct HabitWithStats: Codable, Identifiable {
    var id: String { habit.id }

    let habit: Habit
    let streak_days: Int
    let done_last_30: Int
    let entries: [HabitEntry]

    private enum CodingKeys: String, CodingKey {
        case streak_days, done_last_30, entries
        case id, name, description_md, color, target_per_week
        case active, display_order, created_at, updated_at
    }

    // The server flattens Habit fields into the HabitWithStats payload, so
    // decode manually to keep the nested Habit value in Swift while matching
    // the flat JSON shape.
    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        self.habit = Habit(
            id: try c.decode(String.self, forKey: .id),
            name: try c.decode(String.self, forKey: .name),
            description_md: (try? c.decode(String.self, forKey: .description_md)) ?? "",
            color: (try? c.decode(String.self, forKey: .color)) ?? "goals",
            target_per_week: (try? c.decode(Int.self, forKey: .target_per_week)) ?? 7,
            active: (try? c.decode(Bool.self, forKey: .active)) ?? true,
            display_order: (try? c.decode(Int.self, forKey: .display_order)) ?? 0,
            created_at: (try? c.decode(Int64.self, forKey: .created_at)) ?? 0,
            updated_at: (try? c.decode(Int64.self, forKey: .updated_at)) ?? 0
        )
        self.streak_days = (try? c.decode(Int.self, forKey: .streak_days)) ?? 0
        self.done_last_30 = (try? c.decode(Int.self, forKey: .done_last_30)) ?? 0
        self.entries = (try? c.decode([HabitEntry].self, forKey: .entries)) ?? []
    }

    func encode(to encoder: Encoder) throws {
        var c = encoder.container(keyedBy: CodingKeys.self)
        try c.encode(habit.id, forKey: .id)
        try c.encode(habit.name, forKey: .name)
        try c.encode(habit.description_md, forKey: .description_md)
        try c.encode(habit.color, forKey: .color)
        try c.encode(habit.target_per_week, forKey: .target_per_week)
        try c.encode(habit.active, forKey: .active)
        try c.encode(habit.display_order, forKey: .display_order)
        try c.encode(habit.created_at, forKey: .created_at)
        try c.encode(habit.updated_at, forKey: .updated_at)
        try c.encode(streak_days, forKey: .streak_days)
        try c.encode(done_last_30, forKey: .done_last_30)
        try c.encode(entries, forKey: .entries)
    }
}
