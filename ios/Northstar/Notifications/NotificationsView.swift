import SwiftUI

struct NotificationsView: View {
    @EnvironmentObject private var app: AppState
    @Environment(\.dismiss) private var dismiss

    @State private var items: [AppNotification] = []
    @State private var loading = true
    @State private var loadError: String?
    @State private var streamTask: Task<Void, Never>?

    var body: some View {
        NavigationStack {
            ScrollView {
                LazyVStack(spacing: 8) {
                    if items.isEmpty && !loading {
                        VStack(spacing: 6) {
                            Image(systemName: "bell.slash")
                                .font(.title).foregroundStyle(Theme.text3)
                            Text("No notifications yet.")
                                .foregroundStyle(Theme.text3)
                                .font(.footnote)
                        }
                        .frame(maxWidth: .infinity, minHeight: 200)
                    }
                    ForEach(items) { n in
                        NotificationRow(n: n)
                            .onAppear {
                                if n.read_at == nil { markRead(n.id) }
                            }
                    }
                }
                .padding(.horizontal, 16)
                .padding(.vertical, 8)
            }
            .background(Theme.bg)
            .navigationTitle("Notifications")
            .toolbarBackground(Theme.bg, for: .navigationBar)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Button("Done") { dismiss() }
                        .foregroundStyle(Theme.ai)
                }
            }
            .refreshable { await load() }
        }
        .task { await load() }
        .onAppear { startStream() }
        .onDisappear { streamTask?.cancel(); streamTask = nil }
    }

    private func startStream() {
        guard streamTask == nil, let api = app.apiClient() else { return }
        streamTask = Task {
            while !Task.isCancelled {
                do {
                    try await api.notificationsStream { live in
                        Task { @MainActor in
                            // Prepend if not already present.
                            if items.contains(where: { $0.id == live.id }) { return }
                            let injected = AppNotification(
                                id: live.id,
                                category: live.category,
                                title: live.title,
                                body: live.body,
                                priority: live.priority,
                                payload: nil,
                                created_at: live.created_at,
                                read_at: nil,
                                delivery_status: "sent"
                            )
                            items.insert(injected, at: 0)
                        }
                    }
                } catch {
                    // Server closed or transport failed — back off then reconnect.
                    try? await Task.sleep(nanoseconds: 3_000_000_000)
                }
            }
        }
    }

    private func load() async {
        guard let api = app.apiClient() else {
            loadError = "Not paired."; loading = false; return
        }
        do {
            items = try await api.notifications(limit: 50)
        } catch let e as APIClient.APIError {
            loadError = e.errorDescription
        } catch let e {
            loadError = e.localizedDescription
        }
        loading = false
    }

    private func markRead(_ id: String) {
        guard let api = app.apiClient() else { return }
        Task {
            try? await api.markNotificationRead(id: id)
            if let i = items.firstIndex(where: { $0.id == id }) {
                items[i] = withReadStamp(items[i])
            }
        }
    }
    private func withReadStamp(_ n: AppNotification) -> AppNotification {
        // AppNotification is immutable; rebuild via JSON for simplicity since
        // it's a tiny payload — not worth a full Codable rewrite to a struct
        // with var fields.
        guard let data = try? JSONEncoder().encode(NotifEncode(from: n)) else { return n }
        let mutated = NotifEncode(from: n, readAtNow: Int64(Date().timeIntervalSince1970))
        guard let d = try? JSONEncoder().encode(mutated),
              let out = try? JSONDecoder().decode(AppNotification.self, from: d) else {
            _ = data
            return n
        }
        return out
    }
}

/// Encodable mirror of AppNotification so we can rebuild it with read_at set.
private struct NotifEncode: Encodable {
    let id: String
    let category: String
    let title: String
    let body: String
    let priority: Int
    let payload: [String: String]?
    let created_at: Int64
    let read_at: Int64?
    let delivery_status: String

    init(from n: AppNotification, readAtNow: Int64? = nil) {
        id = n.id
        category = n.category
        title = n.title
        body = n.body
        priority = n.priority
        payload = nil
        created_at = n.created_at
        read_at = readAtNow ?? n.read_at
        delivery_status = n.delivery_status
    }
}

private struct NotificationRow: View {
    let n: AppNotification

    var body: some View {
        HStack(alignment: .top, spacing: 12) {
            ZStack {
                RoundedRectangle(cornerRadius: 10)
                    .fill(iconBg)
                Image(systemName: iconName).foregroundStyle(iconFg)
            }
            .frame(width: 38, height: 38)

            VStack(alignment: .leading, spacing: 4) {
                HStack {
                    Text(displayCategory).font(.caption2).bold().tracking(0.5)
                        .foregroundStyle(Theme.text3)
                    Spacer()
                    Text(relative(n.created_at)).font(.caption2)
                        .foregroundStyle(Theme.text3)
                }
                Text(n.title).font(.system(size: 15, weight: .semibold))
                    .foregroundStyle(Theme.text)
                if !n.body.isEmpty {
                    Text(n.body).font(.footnote)
                        .foregroundStyle(Theme.text2)
                        .lineLimit(3)
                }
            }
            if n.read_at == nil {
                Circle().fill(Theme.ai).frame(width: 6, height: 6).padding(.top, 8)
            }
        }
        .padding(14)
        .background(Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 16))
    }

    private var displayCategory: String {
        switch n.category {
        case "purchase": return "PURCHASE"
        case "budget_threshold": return "BUDGET"
        case "anomaly": return "ANOMALY"
        case "daily_brief": return "DAILY BRIEF"
        case "supplement": return "SUPPLEMENT"
        case "health_insight": return "HEALTH"
        case "goal_milestone": return "GOAL"
        case "subscription_new": return "SUBSCRIPTION"
        case "weekly_retro": return "WEEKLY RETRO"
        default: return n.category.uppercased()
        }
    }

    private var iconName: String {
        switch n.category {
        case "purchase":         return "dollarsign.circle.fill"
        case "budget_threshold": return "exclamationmark.triangle.fill"
        case "anomaly":          return "questionmark.diamond.fill"
        case "supplement":       return "pills.fill"
        case "health_insight":   return "heart.fill"
        case "goal_milestone":   return "star.fill"
        case "subscription_new": return "arrow.clockwise"
        default:                 return "bell.fill"
        }
    }
    private var iconBg: Color {
        switch n.category {
        case "purchase":         return Theme.finance.opacity(0.18)
        case "budget_threshold": return Theme.financeBad.opacity(0.18)
        case "anomaly":          return Theme.ai.opacity(0.18)
        case "supplement":       return Theme.healthBlue.opacity(0.18)
        case "health_insight":   return Theme.healthGo.opacity(0.18)
        case "goal_milestone":   return Theme.goals.opacity(0.18)
        case "subscription_new": return Theme.healthMid.opacity(0.18)
        default:                 return Theme.surfaceHi
        }
    }
    private var iconFg: Color {
        switch n.category {
        case "purchase":         return Theme.finance
        case "budget_threshold": return Theme.financeBad
        case "anomaly":          return Theme.ai
        case "supplement":       return Theme.healthBlue
        case "health_insight":   return Theme.healthGo
        case "goal_milestone":   return Theme.goals
        case "subscription_new": return Theme.healthMid
        default:                 return Theme.text2
        }
    }

    private func relative(_ epoch: Int64) -> String {
        let date = Date(timeIntervalSince1970: TimeInterval(epoch))
        let formatter = RelativeDateTimeFormatter()
        formatter.unitsStyle = .abbreviated
        return formatter.localizedString(for: date, relativeTo: Date())
    }
}
