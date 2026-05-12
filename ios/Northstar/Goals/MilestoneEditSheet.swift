import SwiftUI

/// Add or edit a milestone. Passed nil = create mode; passed an existing
/// Milestone = edit mode (which also exposes an Archive button).
struct MilestoneEditSheet: View {
    @EnvironmentObject private var app: AppState
    @Environment(\.dismiss) private var dismiss

    let milestone: Milestone?

    @State private var title = ""
    @State private var description = ""
    @State private var dueDate = Date()
    @State private var hasDueDate = false
    @State private var status = "pending"
    @State private var flagship = false
    @State private var saving = false
    @State private var saveError: String?

    private let statuses = ["pending", "in_progress", "done"]

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            Capsule()
                .fill(Color.gray.opacity(0.5))
                .frame(width: 38, height: 4)
                .frame(maxWidth: .infinity)
                .padding(.top, 8)

            VStack(alignment: .leading, spacing: 4) {
                Text(milestone == nil ? "New milestone" : "Edit milestone")
                    .font(.system(size: 22, weight: .heavy))
                Text(milestone == nil ? "What are you working toward?" : milestone!.title)
                    .font(.footnote).foregroundStyle(Theme.text3)
                    .lineLimit(1)
            }

            ScrollView {
                VStack(alignment: .leading, spacing: 18) {
                    field("TITLE") {
                        TextField("Pass OSCP exam", text: $title)
                            .font(.system(size: 16, weight: .medium))
                            .textInputAutocapitalization(.sentences)
                            .foregroundStyle(Theme.text)
                            .tint(Theme.ai)
                    }
                    field("DESCRIPTION") {
                        TextEditor(text: $description)
                            .scrollContentBackground(.hidden)
                            .frame(minHeight: 80)
                            .foregroundStyle(Theme.text)
                            .tint(Theme.ai)
                    }
                    VStack(alignment: .leading, spacing: 6) {
                        Toggle(isOn: $hasDueDate) {
                            Text("DUE DATE")
                                .font(.caption2).bold().tracking(1)
                                .foregroundStyle(Theme.text3)
                        }
                        .tint(Theme.ai)
                        if hasDueDate {
                            DatePicker("", selection: $dueDate, displayedComponents: .date)
                                .labelsHidden()
                                .colorScheme(.dark)
                                .tint(Theme.ai)
                        }
                    }
                    field("STATUS") {
                        Picker("Status", selection: $status) {
                            ForEach(statuses, id: \.self) { s in
                                Text(s.replacingOccurrences(of: "_", with: " ").capitalized)
                                    .tag(s)
                            }
                        }
                        .pickerStyle(.segmented)
                    }
                    Toggle(isOn: $flagship) {
                        VStack(alignment: .leading) {
                            Text("Flagship")
                                .font(.system(size: 14, weight: .medium))
                                .foregroundStyle(Theme.text)
                            Text("Pin to the top of the Goals tab")
                                .font(.caption).foregroundStyle(Theme.text3)
                        }
                    }
                    .tint(Theme.ai)
                    .padding(.horizontal, 16).padding(.vertical, 12)
                    .background(Theme.surface)
                    .clipShape(RoundedRectangle(cornerRadius: 14))
                }
            }

            if let saveError {
                Text(saveError)
                    .font(.footnote)
                    .foregroundStyle(Theme.financeBad)
            }

            VStack(spacing: 8) {
                Button(action: { Task { await save() } }) {
                    HStack {
                        if saving { ProgressView().tint(.black) }
                        Text(saving ? "Saving…" : "Save")
                            .font(.system(.body, weight: .semibold))
                    }
                    .frame(maxWidth: .infinity).padding(.vertical, 14)
                    .background(Theme.ai)
                    .foregroundStyle(.black)
                    .clipShape(RoundedRectangle(cornerRadius: 16))
                }
                .disabled(saving || title.trimmingCharacters(in: .whitespaces).isEmpty)

                if milestone != nil {
                    Button(role: .destructive, action: { Task { await archive() } }) {
                        Text("Archive")
                            .frame(maxWidth: .infinity).padding(.vertical, 10)
                    }
                }
                Button("Cancel") { dismiss() }
                    .foregroundStyle(Theme.text2)
                    .padding(.vertical, 6)
            }
        }
        .padding(.horizontal, 22).padding(.bottom, 24)
        .background(Color(hex: 0x1c1c1c))
        .presentationDetents([.large])
        .presentationDragIndicator(.hidden)
        .onAppear(perform: loadInitial)
    }

    private func field<C: View>(_ label: String, @ViewBuilder _ content: () -> C) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(label)
                .font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
            content()
                .padding(.horizontal, 14).padding(.vertical, 10)
                .background(Color(hex: 0x0f0f0f))
                .clipShape(RoundedRectangle(cornerRadius: 12))
        }
    }

    private func loadInitial() {
        guard let m = milestone else { return }
        title = m.title
        description = m.description_md
        if !m.due_date.isEmpty {
            let formatter = DateFormatter()
            formatter.dateFormat = "yyyy-MM-dd"
            if let d = formatter.date(from: m.due_date) {
                dueDate = d
                hasDueDate = true
            }
        }
        status = m.status
        flagship = m.flagship
    }

    private func save() async {
        guard let api = app.apiClient() else { return }
        saving = true; saveError = nil
        defer { saving = false }

        var dueStr: String? = nil
        if hasDueDate {
            let f = DateFormatter()
            f.dateFormat = "yyyy-MM-dd"
            dueStr = f.string(from: dueDate)
        } else {
            dueStr = ""
        }

        let input = APIClient.MilestoneInput(
            title: title.trimmingCharacters(in: .whitespaces),
            description_md: description,
            due_date: dueStr,
            status: status,
            flagship: flagship,
            display_order: milestone?.display_order
        )
        do {
            if let m = milestone {
                _ = try await api.updateMilestone(id: m.id, input)
            } else {
                _ = try await api.createMilestone(input)
            }
            dismiss()
        } catch let e as APIClient.APIError {
            saveError = e.errorDescription
        } catch let e {
            saveError = e.localizedDescription
        }
    }

    private func archive() async {
        guard let api = app.apiClient(), let m = milestone else { return }
        saving = true; saveError = nil
        defer { saving = false }
        do {
            try await api.archiveMilestone(id: m.id)
            dismiss()
        } catch let e {
            saveError = e.localizedDescription
        }
    }
}
