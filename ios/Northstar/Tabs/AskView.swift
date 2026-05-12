import SwiftUI

struct AskView: View {
    @EnvironmentObject private var app: AppState

    @State private var conversation: Conversation?
    @State private var messages: [ChatMessage] = []
    @State private var draft = ""
    @State private var streaming = false
    @State private var loadError: String?
    @State private var activeToolName: String?
    @State private var showHistory = false
    @State private var streamTask: Task<Void, Never>?
    @State private var showScopeSheet = false

    private let quickPrompts = [
        "Should I push hard today?",
        "Where did my budget break this month?",
        "Am I on track for OSCP?",
        "What supplements correlate with my best HRV?",
        "What did I miss yesterday?",
    ]

    var body: some View {
        NavigationStack {
            VStack(spacing: 0) {
                chatHeader
                Divider().overlay(Theme.border)
                messageStream
                quickPromptsRow
                composer
            }
            .background(Theme.bg)
            .navigationBarHidden(true)
            .sheet(isPresented: $showHistory) {
                ConversationHistoryView(currentID: conversation?.id) { id in
                    Task {
                        await loadConversation(id: id)
                        showHistory = false
                    }
                }
            }
            .sheet(isPresented: $showScopeSheet) {
                if let conv = conversation {
                    ConversationScopeSheet(
                        conversationID: conv.id,
                        initial: conv.pillar_scope
                    ) { newScope in
                        if let api = app.apiClient() {
                            Task {
                                try? await api.aiUpdateConversationScope(id: conv.id, pillarScope: newScope)
                                let list = try? await api.aiConversations()
                                conversation = list?.first(where: { $0.id == conv.id })
                            }
                        }
                    }
                    .environmentObject(app)
                }
            }
        }
        .task { await ensureConversation() }
    }

    // ─── Header ─────────────────────────────────────────────────────────

    private var scopeSubtitle: String {
        let scope = conversation?.pillar_scope ?? []
        if scope.isEmpty {
            return "● All pillars · finance + goals + health"
        }
        return "● " + scope.map { $0.capitalized }.joined(separator: " + ") + " only"
    }

    private var chatHeader: some View {
        HStack(spacing: 12) {
            ZStack {
                RoundedRectangle(cornerRadius: 12)
                    .fill(LinearGradient(colors: [Theme.ai, Color(hex: 0xff5e3a)],
                                         startPoint: .topLeading, endPoint: .bottomTrailing))
                Text("C").font(.system(size: 18, weight: .heavy)).foregroundStyle(.black)
            }
            .frame(width: 36, height: 36)
            VStack(alignment: .leading, spacing: 1) {
                Text("Claude").font(.system(size: 16, weight: .bold)).foregroundStyle(Theme.text)
                Text(scopeSubtitle)
                    .font(.system(size: 11))
                    .foregroundStyle(Theme.healthGo)
            }
            Spacer()
            if conversation != nil {
                Button { showScopeSheet = true } label: {
                    Image(systemName: "slider.horizontal.3")
                        .foregroundStyle(Theme.text2)
                }
            }
            Button { showHistory = true } label: {
                Image(systemName: "clock.arrow.circlepath")
                    .foregroundStyle(Theme.text2)
            }
            Button { Task { await newConversation() } } label: {
                Image(systemName: "square.and.pencil")
                    .foregroundStyle(Theme.text2)
            }
        }
        .padding(.horizontal, 18).padding(.top, 18).padding(.bottom, 12)
        .background(Theme.bg)
    }

    // ─── Message stream ─────────────────────────────────────────────────

    private var messageStream: some View {
        ScrollViewReader { proxy in
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 12) {
                    ForEach(messages) { msg in
                        bubble(for: msg)
                            .id(msg.id)
                    }
                    if let toolName = activeToolName {
                        toolCallChip(name: toolName)
                    }
                    if streaming && messages.last?.role != .assistant {
                        HStack(spacing: 6) {
                            ProgressView().tint(Theme.text3)
                            Text("Thinking…").font(.caption).foregroundStyle(Theme.text3)
                        }
                        .padding(.horizontal, 4)
                    }
                    if let loadError {
                        Text(loadError).foregroundStyle(Theme.financeBad).font(.footnote)
                    }
                    Color.clear.frame(height: 4).id("bottom")
                }
                .padding(.horizontal, 16).padding(.vertical, 12)
            }
            .onChange(of: messages.count) { _, _ in
                withAnimation { proxy.scrollTo("bottom", anchor: .bottom) }
            }
            .onChange(of: messages.last?.text) { _, _ in
                proxy.scrollTo("bottom", anchor: .bottom)
            }
        }
    }

    private func bubble(for msg: ChatMessage) -> some View {
        HStack(alignment: .top) {
            if msg.role == .user { Spacer(minLength: 30) }
            VStack(alignment: .leading, spacing: 6) {
                Text(msg.text)
                    .font(.system(size: 14))
                    .foregroundStyle(msg.role == .user ? .black : Theme.text)
                if !msg.toolCalls.isEmpty {
                    HStack(spacing: 4) {
                        ForEach(msg.toolCalls, id: \.self) { name in
                            Text(name)
                                .font(.system(size: 10, weight: .semibold, design: .monospaced))
                                .padding(.horizontal, 6).padding(.vertical, 2)
                                .background(Color.white.opacity(0.06))
                                .foregroundStyle(Theme.text3)
                                .clipShape(RoundedRectangle(cornerRadius: 6))
                        }
                    }
                }
            }
            .padding(.horizontal, 14).padding(.vertical, 10)
            .background(msg.role == .user ? Theme.ai : Theme.surface)
            .clipShape(BubbleShape(side: msg.role == .user ? .right : .left))
            if msg.role == .assistant { Spacer(minLength: 30) }
        }
    }

    private func toolCallChip(name: String) -> some View {
        HStack(spacing: 8) {
            Circle().fill(Theme.ai).frame(width: 6, height: 6)
            Text(name)
                .font(.system(size: 11, design: .monospaced))
                .foregroundStyle(Theme.text3)
        }
    }

    // ─── Quick prompts ──────────────────────────────────────────────────

    private var quickPromptsRow: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: 8) {
                ForEach(quickPrompts, id: \.self) { p in
                    Button { send(p) } label: {
                        Text(p)
                            .font(.system(size: 12))
                            .padding(.horizontal, 14).padding(.vertical, 8)
                            .background(Theme.surface)
                            .foregroundStyle(Theme.text2)
                            .clipShape(Capsule())
                    }
                }
            }
            .padding(.horizontal, 16).padding(.vertical, 8)
        }
        .background(Theme.bg.opacity(0.92))
        .overlay(Divider().overlay(Theme.border), alignment: .top)
    }

    // ─── Composer ───────────────────────────────────────────────────────

    private var composer: some View {
        HStack(spacing: 8) {
            TextField("Ask anything about your life…", text: $draft, axis: .vertical)
                .padding(.horizontal, 14).padding(.vertical, 10)
                .background(Theme.surface)
                .foregroundStyle(Theme.text)
                .tint(Theme.ai)
                .clipShape(RoundedRectangle(cornerRadius: 22))
                .lineLimit(1...5)
            Button {
                if streaming {
                    streamTask?.cancel()
                    return
                }
                let trimmed = draft.trimmingCharacters(in: .whitespaces)
                guard !trimmed.isEmpty else { return }
                draft = ""
                send(trimmed)
            } label: {
                Image(systemName: streaming ? "stop.fill" : "arrow.up")
                    .foregroundStyle(.black)
                    .font(.system(size: 16, weight: .heavy))
                    .frame(width: 40, height: 40)
                    .background(streaming ? Theme.financeBad : Theme.ai)
                    .clipShape(Circle())
            }
            .disabled(!streaming && draft.trimmingCharacters(in: .whitespaces).isEmpty)
        }
        .padding(.horizontal, 14).padding(.vertical, 10)
        .padding(.bottom, 12)
        .background(Theme.bg)
    }

    // ─── Plumbing ───────────────────────────────────────────────────────

    private func ensureConversation() async {
        guard conversation == nil else { return }
        guard let api = app.apiClient() else { loadError = "Not paired."; return }
        do {
            let list = try await api.aiConversations()
            if let recent = list.first {
                await loadConversation(id: recent.id)
            } else {
                let new = try await api.aiCreateConversation()
                conversation = new
            }
        } catch let e as APIClient.APIError {
            loadError = e.errorDescription
        } catch let e {
            loadError = e.localizedDescription
        }
    }

    private func loadConversation(id: String) async {
        guard let api = app.apiClient() else { return }
        do {
            let stored = try await api.aiMessages(convID: id)
            let list = try await api.aiConversations()
            conversation = list.first(where: { $0.id == id })
            messages = stored.map { sm in
                var text = ""
                var tools: [String] = []
                for c in sm.content {
                    if c.type == "text", let t = c.text { text += t }
                    if c.type == "tool_use", let n = c.name { tools.append(n) }
                }
                return ChatMessage(
                    role: sm.role == "user" ? .user : .assistant,
                    text: text,
                    toolCalls: tools)
            }
        } catch {
            loadError = (error as? APIClient.APIError)?.errorDescription ?? error.localizedDescription
        }
    }

    private func newConversation() async {
        guard let api = app.apiClient() else { return }
        do {
            let new = try await api.aiCreateConversation()
            conversation = new
            messages = []
            loadError = nil
        } catch {
            loadError = (error as? APIClient.APIError)?.errorDescription ?? error.localizedDescription
        }
    }

    private func send(_ text: String) {
        guard let api = app.apiClient(), let conv = conversation, !streaming else { return }
        messages.append(ChatMessage(role: .user, text: text, toolCalls: []))
        messages.append(ChatMessage(role: .assistant, text: "", toolCalls: []))
        streaming = true
        activeToolName = nil
        loadError = nil

        streamTask = Task {
            defer {
                Task { @MainActor in
                    streaming = false
                    activeToolName = nil
                    streamTask = nil
                }
            }
            do {
                try await api.aiSendMessageStream(convID: conv.id, text: text) { ev in
                    Task { @MainActor in
                        switch ev.type {
                        case "text":
                            if let t = ev.text, let lastIdx = messages.indices.last {
                                messages[lastIdx].text += t
                            }
                            activeToolName = nil
                        case "tool_call":
                            if let name = ev.tool_name {
                                if let lastIdx = messages.indices.last {
                                    messages[lastIdx].toolCalls.append(name)
                                }
                                activeToolName = name
                            }
                        case "tool_error":
                            // Surface failed tool calls inline; the assistant keeps streaming.
                            if let name = ev.tool_name, let lastIdx = messages.indices.last {
                                messages[lastIdx].toolCalls.append("\(name) ✗")
                            }
                            activeToolName = nil
                        case "error":
                            loadError = ev.error ?? "stream error"
                        case "done":
                            break
                        default: break
                        }
                    }
                }
            } catch is CancellationError {
                // User tapped stop — leave the partial reply in place, no error banner.
                await MainActor.run {
                    if let lastIdx = messages.indices.last,
                       messages[lastIdx].role == .assistant,
                       messages[lastIdx].text.isEmpty {
                        messages[lastIdx].text = "(stopped)"
                    }
                }
            } catch let e as APIClient.APIError {
                await MainActor.run { loadError = e.errorDescription }
            } catch let e {
                if (e as NSError).code == NSURLErrorCancelled {
                    await MainActor.run {
                        if let lastIdx = messages.indices.last,
                           messages[lastIdx].role == .assistant,
                           messages[lastIdx].text.isEmpty {
                            messages[lastIdx].text = "(stopped)"
                        }
                    }
                } else {
                    await MainActor.run { loadError = e.localizedDescription }
                }
            }
        }
    }
}

// ─── Bubble shape ─────────────────────────────────────────────────────────

private struct BubbleShape: Shape {
    enum Side { case left, right }
    let side: Side

    func path(in rect: CGRect) -> Path {
        let r: CGFloat = 18
        let tail: CGFloat = 6
        var path = Path()
        switch side {
        case .right:
            path.addRoundedRect(in: rect, cornerSize: CGSize(width: r, height: r))
            // Square off bottom-right corner for the "tail"
            path = Path(roundedRect: rect, cornerRadii: .init(
                topLeading: r, bottomLeading: r, bottomTrailing: tail, topTrailing: r))
        case .left:
            path = Path(roundedRect: rect, cornerRadii: .init(
                topLeading: r, bottomLeading: tail, bottomTrailing: r, topTrailing: r))
        }
        return path
    }
}

// ─── Conversation history sheet ──────────────────────────────────────────

private struct ConversationHistoryView: View {
    @EnvironmentObject private var app: AppState
    @Environment(\.dismiss) private var dismiss

    let currentID: String?
    let onPick: (String) -> Void

    @State private var conversations: [Conversation] = []
    @State private var loadError: String?
    @State private var renamingID: String?
    @State private var renameDraft: String = ""

    var body: some View {
        NavigationStack {
            List {
                ForEach(conversations) { c in
                    Button { onPick(c.id) } label: {
                        VStack(alignment: .leading, spacing: 2) {
                            Text(c.title.isEmpty ? "Untitled" : c.title)
                                .font(.system(size: 15, weight: .semibold))
                                .foregroundStyle(Theme.text)
                            Text(relativeDate(c.started_at))
                                .font(.caption).foregroundStyle(Theme.text3)
                        }
                        .padding(.vertical, 4)
                    }
                    .listRowBackground(c.id == currentID ? Theme.surfaceHi : Theme.surface)
                    .swipeActions(edge: .leading) {
                        Button {
                            renamingID = c.id
                            renameDraft = c.title
                        } label: {
                            Label("Rename", systemImage: "pencil")
                        }
                        .tint(Theme.ai)
                    }
                    .contextMenu {
                        Button {
                            renamingID = c.id
                            renameDraft = c.title
                        } label: {
                            Label("Rename", systemImage: "pencil")
                        }
                    }
                }
                .onDelete(perform: deleteAt)
                if let loadError {
                    Text(loadError).foregroundStyle(Theme.financeBad).font(.footnote)
                }
            }
            .scrollContentBackground(.hidden)
            .background(Theme.bg)
            .navigationTitle("Conversations")
            .toolbarBackground(Theme.bg, for: .navigationBar)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Button("Close") { dismiss() }.foregroundStyle(Theme.text2)
                }
            }
        }
        .task { await load() }
        .alert("Rename conversation",
               isPresented: Binding(get: { renamingID != nil },
                                    set: { if !$0 { renamingID = nil } })) {
            TextField("Title", text: $renameDraft)
            Button("Save") { commitRename() }
            Button("Cancel", role: .cancel) { renamingID = nil }
        }
    }

    private func load() async {
        guard let api = app.apiClient() else { return }
        do { conversations = try await api.aiConversations() }
        catch { loadError = (error as? APIClient.APIError)?.errorDescription ?? error.localizedDescription }
    }
    private func commitRename() {
        guard let id = renamingID, let api = app.apiClient() else { return }
        let title = renameDraft.trimmingCharacters(in: .whitespaces)
        renamingID = nil
        Task {
            do {
                try await api.aiRenameConversation(id: id, title: title)
                await load()
            } catch {
                loadError = (error as? APIClient.APIError)?.errorDescription ?? error.localizedDescription
            }
        }
    }
    private func deleteAt(_ offsets: IndexSet) {
        guard let api = app.apiClient() else { return }
        let ids = offsets.map { conversations[$0].id }
        Task {
            for id in ids {
                try? await api.aiDeleteConversation(id: id)
            }
            await load()
        }
    }
    private func relativeDate(_ epoch: Int64) -> String {
        let d = Date(timeIntervalSince1970: TimeInterval(epoch))
        let f = RelativeDateTimeFormatter(); f.unitsStyle = .full
        return f.localizedString(for: d, relativeTo: Date())
    }
}
