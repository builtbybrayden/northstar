import Foundation

struct RecoveryRow: Codable, Identifiable {
    var id: String { date }
    let date: String
    let score: Int?
    let hrv_ms: Double?
    let rhr: Int?
}

struct SleepRow: Codable, Identifiable {
    var id: String { date }
    let date: String
    let duration_min: Int?
    let score: Int?
    let debt_min: Int?
}

struct StrainRow: Codable, Identifiable {
    var id: String { date }
    let date: String
    let score: Double?
    let avg_hr: Int?
    let max_hr: Int?
}

struct HealthToday: Codable {
    let date: String
    let recovery: RecoveryRow?
    let sleep: SleepRow?
    let strain: StrainRow?
    let verdict: String
    let strain_goal: String
}

struct SupplementDef: Codable, Identifiable {
    let id: String
    var name: String
    var dose: String
    var category: String              // supplement / peptide / medication
    var schedule_json: String
    var cycle_days_on: Int?
    var cycle_days_off: Int?
    var reminder_enabled: Bool
    var active: Bool
    var prescribing_doc: String
    var notes: String
    let created_at: Int64
}

struct SupplementDose: Codable, Identifiable {
    let id: String
    let def_id: String
    let taken_at: Int64
    let notes: String
}
