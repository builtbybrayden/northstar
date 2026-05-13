import Foundation

struct Conversation: Codable, Identifiable {
    let id: String
    let title: String
    let started_at: Int64
    let pillar_scope: [String]
}

/// Decoded server-sent event from /api/ai/conversations/:id/messages.
/// Sendable so it can cross the SSE callback boundary into a @MainActor Task
/// without the strict-concurrency checker complaining.
struct AIStreamEvent: Codable, Sendable {
    let type: String          // text | tool_call | done | error
    let text: String?
    let tool_name: String?
    let error: String?
}

/// In-memory chat turn (after the stream completes).
struct ChatMessage: Identifiable {
    let id = UUID()
    let role: Role
    var text: String
    var toolCalls: [String]   // names of tools the assistant invoked during this turn

    enum Role { case user, assistant }
}

/// Persisted-message DTO returned by /api/ai/conversations/:id/messages (GET).
/// Server stores Anthropic-shaped content blocks; we flatten to plain text + tool names.
struct StoredMessage: Codable {
    let id: String
    let role: String
    let content: [StoredContent]
    let created_at: Int64

    struct StoredContent: Codable {
        let type: String
        let text: String?
        let name: String?
    }
}
