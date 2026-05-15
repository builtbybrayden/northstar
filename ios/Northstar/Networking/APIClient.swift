import Foundation

/// Minimal API client for Northstar's REST surface. v1 endpoints are health,
/// pairing, /me, and /pillars; per-pillar resource endpoints land in Phase 1+.
struct APIClient {
    let baseURL: URL
    let bearer: String?

    init(baseURL: URL, bearer: String? = nil) {
        self.baseURL = baseURL
        self.bearer = bearer
    }

    enum APIError: Error, LocalizedError {
        case http(status: Int, body: String)
        case decode(Error)
        case transport(Error)

        var errorDescription: String? {
            switch self {
            case .http(let s, let b): return "HTTP \(s): \(b)"
            case .decode(let e):      return "decode: \(e.localizedDescription)"
            case .transport(let e):   return "transport: \(e.localizedDescription)"
            }
        }
    }

    // ─── Public endpoints ─────────────────────────────────────────────────

    struct HealthResponse: Decodable {
        let ok: Bool
        let service: String
        let version: String
        let db: Bool
    }
    func health() async throws -> HealthResponse {
        try await get(path: "/api/health")
    }

    struct PairRedeemRequest: Encodable {
        let code: String
        let device_name: String
    }
    struct PairRedeemResponse: Decodable {
        let device_id: String
        let bearer_token: String
        struct ServerInfo: Decodable { let version: String }
        let server_info: ServerInfo
    }
    func pairRedeem(code: String, deviceName: String) async throws -> PairRedeemResponse {
        try await post(path: "/api/pair/redeem",
                       body: PairRedeemRequest(code: code, device_name: deviceName))
    }

    // ─── Authenticated endpoints ──────────────────────────────────────────

    struct MeResponse: Decodable {
        let device_id: String
        let device_name: String
        let user_id: String
        let paired_at: Int64
    }
    func me() async throws -> MeResponse {
        try await get(path: "/api/me")
    }

    struct PillarsResponse: Decodable {
        let finance: Bool
        let goals: Bool
        let health: Bool
        let ai: Bool
    }
    func pillars() async throws -> PillarsResponse {
        try await get(path: "/api/pillars")
    }

    struct RegisterAPNSRequest: Encodable { let apns_token: String }
    func registerAPNS(token: String) async throws {
        try await postVoid(path: "/api/devices/register-apns",
                           body: RegisterAPNSRequest(apns_token: token))
    }

    // ─── Finance ──────────────────────────────────────────────────────────

    func financeSummary(month: String? = nil) async throws -> FinanceSummary {
        let path = month.map { "/api/finance/summary?month=\($0)" } ?? "/api/finance/summary"
        return try await get(path: path)
    }
    func financeTransactions(limit: Int = 50,
                             category: String? = nil,
                             month: String? = nil) async throws -> [Transaction] {
        var qs = ["limit=\(limit)"]
        if let c = category {
            let enc = c.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? c
            qs.append("category=\(enc)")
        }
        if let m = month {
            qs.append("month=\(m)")
        }
        return try await get(path: "/api/finance/transactions?" + qs.joined(separator: "&"))
    }
    func financeAccounts() async throws -> [Account] {
        try await get(path: "/api/finance/accounts")
    }
    func financeForecast(days: Int = 90) async throws -> Forecast {
        try await get(path: "/api/finance/forecast?days=\(days)")
    }
    func financeInvestments() async throws -> Investments {
        try await get(path: "/api/finance/investments")
    }
    func financeBalanceHistory(days: Int = 90) async throws -> BalanceHistory {
        try await get(path: "/api/finance/balance-history?days=\(days)")
    }

    /// Apply a user category override to a single transaction.
    /// Pass `nil` to clear the override and revert to the upstream value.
    struct TransactionUpdate: Encodable {
        var category: String? = nil
        /// When true, encodes `"category": null` so the server treats it as
        /// an explicit reset. Otherwise the field is encoded as a string.
        var resetCategory: Bool = false

        func encode(to encoder: Encoder) throws {
            var c = encoder.container(keyedBy: CodingKeys.self)
            if resetCategory {
                try c.encodeNil(forKey: .category)
            } else if let category {
                try c.encode(category, forKey: .category)
            }
        }
        enum CodingKeys: String, CodingKey { case category }
    }
    func updateTransactionCategory(id: String, category: String) async throws {
        let e = id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? id
        try await patchVoid(path: "/api/finance/transactions/\(e)",
                            body: TransactionUpdate(category: category))
    }
    func resetTransactionCategory(id: String) async throws {
        let e = id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? id
        try await patchVoid(path: "/api/finance/transactions/\(e)",
                            body: TransactionUpdate(resetCategory: true))
    }

    // ─── Budget targets (edit) ───────────────────────────────────────────

    func budgetTargets() async throws -> [BudgetTarget] {
        try await get(path: "/api/finance/budget-targets")
    }

    struct BudgetTargetUpdate: Encodable {
        var monthly_cents: Int64?
        var threshold_pcts: [Int]?
        var push_enabled: Bool?
        var category_group: String?
    }
    func updateBudgetTarget(category: String, _ u: BudgetTargetUpdate) async throws {
        let encoded = category.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? category
        try await patchVoid(path: "/api/finance/budget-targets/\(encoded)", body: u)
    }

    // ─── Notifications ───────────────────────────────────────────────────

    func notifications(limit: Int = 50, unreadOnly: Bool = false) async throws -> [AppNotification] {
        var path = "/api/notifications/feed?limit=\(limit)"
        if unreadOnly { path += "&unread=1" }
        return try await get(path: path)
    }
    struct UnreadCountResponse: Decodable { let unread: Int }
    func notificationUnreadCount() async throws -> Int {
        let r: UnreadCountResponse = try await get(path: "/api/notifications/unread-count")
        return r.unread
    }
    func markNotificationRead(id: String) async throws {
        try await postVoid(path: "/api/notifications/\(id)/read", body: EmptyBody())
    }
    func notificationRules() async throws -> [NotificationRule] {
        try await get(path: "/api/notifications/rules")
    }
    struct RuleUpdate: Encodable {
        var enabled: Bool?
        var quiet_hours_start: String?
        var quiet_hours_end: String?
        var bypass_quiet: Bool?
        var delivery: String?
        var max_per_day: Int?
    }
    func updateRule(category: String, _ u: RuleUpdate) async throws {
        let encoded = category.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? category
        try await patchVoid(path: "/api/notifications/rules/\(encoded)", body: u)
    }

    // ─── Goals ────────────────────────────────────────────────────────────

    func listMilestones(includeArchived: Bool = false) async throws -> [Milestone] {
        try await get(path: "/api/goals/milestones" + (includeArchived ? "?archived=1" : ""))
    }

    struct MilestoneInput: Encodable {
        var title: String?
        var description_md: String?
        var due_date: String?
        var status: String?
        var flagship: Bool?
        var display_order: Int?
    }
    func createMilestone(_ in_: MilestoneInput) async throws -> Milestone {
        try await post(path: "/api/goals/milestones", body: in_)
    }
    func updateMilestone(id: String, _ in_: MilestoneInput) async throws -> Milestone {
        let e = id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? id
        return try await patch(path: "/api/goals/milestones/\(e)", body: in_)
    }
    func archiveMilestone(id: String) async throws {
        let e = id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? id
        _ = try await send(method: "DELETE", path: "/api/goals/milestones/\(e)", body: nil)
    }

    // ─── Habits ───────────────────────────────────────────────────────────

    func listHabits(days: Int = 90, includeInactive: Bool = false) async throws -> [HabitWithStats] {
        var path = "/api/goals/habits?days=\(days)"
        if includeInactive { path += "&inactive=1" }
        return try await get(path: path)
    }

    struct HabitInput: Encodable {
        var name: String?
        var description_md: String?
        var color: String?
        var target_per_week: Int?
        var active: Bool?
        var display_order: Int?
    }
    func createHabit(_ in_: HabitInput) async throws -> Habit {
        try await post(path: "/api/goals/habits", body: in_)
    }
    func updateHabit(id: String, _ in_: HabitInput) async throws {
        let e = id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? id
        try await patchVoid(path: "/api/goals/habits/\(e)", body: in_)
    }
    func deleteHabit(id: String) async throws {
        let e = id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? id
        _ = try await send(method: "DELETE", path: "/api/goals/habits/\(e)", body: nil)
    }
    /// Toggle done/skip for (habit, date). Passing nil body lets the server
    /// flip the current count.
    func toggleHabitLog(habitID: String, date: String) async throws -> HabitEntry {
        let e = habitID.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? habitID
        return try await putWithOptionalBody(path: "/api/goals/habits/\(e)/log/\(date)")
    }

    func dailyLog(date: String? = nil) async throws -> DailyLog {
        let path = date.map { "/api/goals/daily/\($0)" } ?? "/api/goals/daily"
        return try await get(path: path)
    }
    struct DailyLogInput: Encodable {
        let items: [DailyItem]
        var reflection_md: String?
    }
    func putDailyLog(date: String? = nil, _ in_: DailyLogInput) async throws -> DailyLog {
        let path = date.map { "/api/goals/daily/\($0)" } ?? "/api/goals/daily"
        let encoded = try JSONEncoder().encode(in_)
        let data = try await send(method: "PUT", path: path, body: encoded)
        return try decode(data)
    }

    func brief() async throws -> Brief {
        try await get(path: "/api/goals/brief")
    }

    // Weekly / Monthly trackers
    struct WeeklyTracker: Codable {
        let week_of: String
        var theme: String
        var weekly_goals: [DailyItem]
        var retro_md: String
        let updated_at: Int64?
    }
    struct WeeklyInput: Encodable {
        var theme: String?
        var weekly_goals: [DailyItem]?
        var retro_md: String?
    }
    func weeklyTracker(weekOf: String) async throws -> WeeklyTracker {
        try await get(path: "/api/goals/weekly/\(weekOf)")
    }
    func putWeekly(weekOf: String, _ in_: WeeklyInput) async throws {
        let encoded = try JSONEncoder().encode(in_)
        _ = try await send(method: "PUT", path: "/api/goals/weekly/\(weekOf)", body: encoded)
    }

    struct MonthlyGoals: Codable {
        let month: String
        var monthly_goals: [DailyItem]
        var retro_md: String
        let updated_at: Int64?
    }
    struct MonthlyInput: Encodable {
        var monthly_goals: [DailyItem]?
        var retro_md: String?
    }
    func monthlyGoals(month: String) async throws -> MonthlyGoals {
        try await get(path: "/api/goals/monthly/\(month)")
    }
    func putMonthly(month: String, _ in_: MonthlyInput) async throws {
        let encoded = try JSONEncoder().encode(in_)
        _ = try await send(method: "PUT", path: "/api/goals/monthly/\(month)", body: encoded)
    }

    // Output log
    func listOutput() async throws -> [OutputEntry] {
        try await get(path: "/api/goals/output")
    }
    struct OutputInput: Encodable {
        var date: String?
        var category: String?
        var title: String?
        var body_md: String?
        var url: String?
    }
    func createOutput(_ in_: OutputInput) async throws -> OutputEntry {
        try await post(path: "/api/goals/output", body: in_)
    }

    // Networking log
    func listNetworking() async throws -> [NetworkingEntry] {
        try await get(path: "/api/goals/networking")
    }
    struct NetworkingInput: Encodable {
        var date: String?
        var person: String?
        var context: String?
        var next_action: String?
        var next_action_due: String?
    }
    func createNetworking(_ in_: NetworkingInput) async throws -> NetworkingEntry {
        try await post(path: "/api/goals/networking", body: in_)
    }

    // Reminders
    func listReminders() async throws -> [Reminder] {
        try await get(path: "/api/goals/reminders")
    }
    struct ReminderInput: Encodable {
        var title: String?
        var body: String?
        var recurrence: String?
        var active: Bool?
    }
    func createReminder(_ in_: ReminderInput) async throws -> Reminder {
        try await post(path: "/api/goals/reminders", body: in_)
    }
    func updateReminder(id: String, _ in_: ReminderInput) async throws {
        let e = id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? id
        try await patchVoid(path: "/api/goals/reminders/\(e)", body: in_)
    }
    func deleteReminder(id: String) async throws {
        let e = id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? id
        _ = try await send(method: "DELETE", path: "/api/goals/reminders/\(e)", body: nil)
    }

    // ─── Health ───────────────────────────────────────────────────────────

    func healthToday() async throws -> HealthToday {
        try await get(path: "/api/health/today")
    }
    func healthRecovery(days: Int = 14) async throws -> [RecoveryRow] {
        try await get(path: "/api/health/recovery?days=\(days)")
    }
    func healthSleep(days: Int = 14) async throws -> [SleepRow] {
        try await get(path: "/api/health/sleep?days=\(days)")
    }
    func healthStrain(days: Int = 14) async throws -> [StrainRow] {
        try await get(path: "/api/health/strain?days=\(days)")
    }

    func supplementDefs(includeInactive: Bool = false) async throws -> [SupplementDef] {
        let path = "/api/health/supplements/defs" + (includeInactive ? "?inactive=1" : "")
        return try await get(path: path)
    }
    struct SupplementDefInput: Encodable {
        var name: String?
        var dose: String?
        var category: String?
        var schedule_json: String?
        var cycle_days_on: Int?
        var cycle_days_off: Int?
        var reminder_enabled: Bool?
        var active: Bool?
        var prescribing_doc: String?
        var notes: String?
    }
    func createSupplementDef(_ in_: SupplementDefInput) async throws -> [String: String] {
        let data = try JSONEncoder().encode(in_)
        let resp = try await send(method: "POST", path: "/api/health/supplements/defs", body: data)
        let raw = (try JSONSerialization.jsonObject(with: resp) as? [String: Any]) ?? [:]
        var out: [String: String] = [:]
        for (k, v) in raw { out[k] = "\(v)" }
        return out
    }
    func updateSupplementDef(id: String, _ in_: SupplementDefInput) async throws {
        let e = id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? id
        try await patchVoid(path: "/api/health/supplements/defs/\(e)", body: in_)
    }
    func archiveSupplementDef(id: String) async throws {
        let e = id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? id
        _ = try await send(method: "DELETE", path: "/api/health/supplements/defs/\(e)", body: nil)
    }

    struct LogDoseInput: Encodable {
        let def_id: String
        var taken_at: Int64?
        var notes: String?
    }
    func logSupplementDose(_ in_: LogDoseInput) async throws -> SupplementDose {
        try await post(path: "/api/health/supplements/log", body: in_)
    }
    func supplementDoses(days: Int = 7) async throws -> [SupplementDose] {
        try await get(path: "/api/health/supplements/log?days=\(days)")
    }

    // ─── AI / Ask ────────────────────────────────────────────────────────

    func aiConversations() async throws -> [Conversation] {
        try await get(path: "/api/ai/conversations")
    }
    struct CreateConvInput: Encodable {
        var title: String?
        var pillar_scope: [String]?
    }
    func aiCreateConversation(title: String? = nil,
                              pillarScope: [String]? = nil) async throws -> Conversation {
        try await post(path: "/api/ai/conversations",
                       body: CreateConvInput(title: title, pillar_scope: pillarScope))
    }
    func aiDeleteConversation(id: String) async throws {
        let e = id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? id
        _ = try await send(method: "DELETE", path: "/api/ai/conversations/\(e)", body: nil)
    }
    struct ConversationPatch: Encodable {
        var title: String?
        var pillar_scope: [String]?
    }
    func aiRenameConversation(id: String, title: String) async throws {
        let e = id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? id
        try await patchVoid(path: "/api/ai/conversations/\(e)",
                            body: ConversationPatch(title: title, pillar_scope: nil))
    }
    func aiUpdateConversationScope(id: String, pillarScope: [String]) async throws {
        let e = id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? id
        try await patchVoid(path: "/api/ai/conversations/\(e)",
                            body: ConversationPatch(title: nil, pillar_scope: pillarScope))
    }
    func aiMessages(convID: String) async throws -> [StoredMessage] {
        let e = convID.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? convID
        return try await get(path: "/api/ai/conversations/\(e)/messages")
    }

    /// Streams the assistant reply via SSE. Emits each parsed event via `onEvent`.
    /// `onEvent` is `@Sendable` so the caller can hop to MainActor inside it
    /// without the strict-concurrency checker flagging the closure as a
    /// non-Sendable value crossing into our nonisolated body.
    func aiSendMessageStream(
        convID: String,
        text: String,
        onEvent: @escaping @Sendable (AIStreamEvent) -> Void
    ) async throws {
        let e = convID.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? convID
        var req = URLRequest(url: url(for: "/api/ai/conversations/\(e)/messages"))
        req.httpMethod = "POST"
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.setValue("text/event-stream", forHTTPHeaderField: "Accept")
        if let bearer { req.setValue("Bearer \(bearer)", forHTTPHeaderField: "Authorization") }
        req.timeoutInterval = 300
        req.httpBody = try JSONEncoder().encode(["text": text])

        let (bytes, resp) = try await URLSession.shared.bytes(for: req)
        guard let http = resp as? HTTPURLResponse, (200..<300).contains(http.statusCode) else {
            let code = (resp as? HTTPURLResponse)?.statusCode ?? -1
            throw APIError.http(status: code, body: "stream request failed")
        }
        for try await line in bytes.lines {
            guard line.hasPrefix("data: ") else { continue }
            let payload = String(line.dropFirst(6))
            guard let data = payload.data(using: .utf8) else { continue }
            if let ev = try? JSONDecoder().decode(AIStreamEvent.self, from: data) {
                onEvent(ev)
                if ev.type == "done" || ev.type == "error" { return }
            }
        }
    }

    // ─── Live notification stream (SSE) ───────────────────────────────────

    /// One push event off /api/notifications/stream.
    struct LiveNotification: Decodable, Sendable {
        let type: String          // "notification" — heartbeats and "ready" use a different shape
        let id: String
        let category: String
        let title: String
        let body: String
        let priority: Int
        let created_at: Int64
    }

    /// Long-lived SSE subscription. Yields once per fired notification.
    /// Returns when the server closes the stream or the task is cancelled.
    /// Caller is responsible for reconnect on failure (typically via `Task`).
    func notificationsStream(onEvent: @escaping @Sendable (LiveNotification) -> Void) async throws {
        var req = URLRequest(url: url(for: "/api/notifications/stream"))
        req.httpMethod = "GET"
        req.setValue("text/event-stream", forHTTPHeaderField: "Accept")
        if let bearer { req.setValue("Bearer \(bearer)", forHTTPHeaderField: "Authorization") }
        req.timeoutInterval = 0 // long-lived

        let (bytes, resp) = try await URLSession.shared.bytes(for: req)
        guard let http = resp as? HTTPURLResponse, (200..<300).contains(http.statusCode) else {
            let code = (resp as? HTTPURLResponse)?.statusCode ?? -1
            throw APIError.http(status: code, body: "stream request failed")
        }
        for try await line in bytes.lines {
            // Skip comments (": ping"), event-name lines, and blank lines.
            guard line.hasPrefix("data: ") else { continue }
            let payload = String(line.dropFirst(6))
            guard let data = payload.data(using: .utf8) else { continue }
            if let ev = try? JSONDecoder().decode(LiveNotification.self, from: data) {
                onEvent(ev)
            }
        }
    }

    // Settings
    func userSettings() async throws -> [String: String] {
        let data = try await send(method: "GET", path: "/api/me/settings", body: nil)
        let raw = try JSONSerialization.jsonObject(with: data) as? [String: Any] ?? [:]
        var out: [String: String] = [:]
        for (k, v) in raw { out[k] = "\(v)" }
        return out
    }
    func updateUserSettings(_ patch: [String: String]) async throws {
        let encoded = try JSONEncoder().encode(patch)
        _ = try await send(method: "PATCH", path: "/api/me/settings", body: encoded)
    }

    // Plumbing for chunks that need send() directly
    private struct EmptyBody: Encodable {}
    private struct EmptyResp: Decodable {}
    private func patch<Req: Encodable, Resp: Decodable>(path: String, body: Req) async throws -> Resp {
        let encoded = try JSONEncoder().encode(body)
        let data = try await send(method: "PATCH", path: path, body: encoded)
        return try decode(data)
    }

    // ─── Plumbing ─────────────────────────────────────────────────────────

    private func get<Resp: Decodable>(path: String) async throws -> Resp {
        let data = try await send(method: "GET", path: path, body: nil)
        return try decode(data)
    }

    private func post<Req: Encodable, Resp: Decodable>(path: String, body: Req) async throws -> Resp {
        let encoded = try JSONEncoder().encode(body)
        let data = try await send(method: "POST", path: path, body: encoded)
        return try decode(data)
    }

    private func postVoid<Req: Encodable>(path: String, body: Req) async throws {
        let encoded = try JSONEncoder().encode(body)
        _ = try await send(method: "POST", path: path, body: encoded)
    }

    private func patchVoid<Req: Encodable>(path: String, body: Req) async throws {
        let encoded = try JSONEncoder().encode(body)
        _ = try await send(method: "PATCH", path: path, body: encoded)
    }

    /// PUT with no body, decoding the response. Used for endpoints that
    /// act as toggles (e.g., habit log) where state is server-derived.
    private func putWithOptionalBody<Resp: Decodable>(path: String) async throws -> Resp {
        let data = try await send(method: "PUT", path: path, body: nil)
        return try decode(data)
    }

    /// Resolve a path (e.g. `/api/finance/transactions?limit=25`) against the
    /// base URL. `appendingPathComponent` percent-encodes `?` as `%3F`,
    /// which turns query strings into 404s — string concatenation through
    /// URL() handles both paths and query strings correctly.
    private func url(for path: String) -> URL {
        if let u = URL(string: baseURL.absoluteString + path) { return u }
        return baseURL.appendingPathComponent(path)
    }

    private func send(method: String, path: String, body: Data?) async throws -> Data {
        var req = URLRequest(url: url(for: path))
        req.httpMethod = method
        req.timeoutInterval = 20
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        if let bearer { req.setValue("Bearer \(bearer)", forHTTPHeaderField: "Authorization") }
        if let body { req.httpBody = body }

        let (data, resp): (Data, URLResponse)
        do {
            (data, resp) = try await URLSession.shared.data(for: req)
        } catch {
            throw APIError.transport(error)
        }
        guard let http = resp as? HTTPURLResponse else {
            throw APIError.transport(URLError(.badServerResponse))
        }
        guard (200..<300).contains(http.statusCode) else {
            throw APIError.http(status: http.statusCode,
                                body: String(data: data, encoding: .utf8) ?? "")
        }
        return data
    }

    private func decode<Resp: Decodable>(_ data: Data) throws -> Resp {
        do { return try JSONDecoder().decode(Resp.self, from: data) }
        catch { throw APIError.decode(error) }
    }
}
