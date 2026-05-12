import SwiftUI

struct SupplementEditSheet: View {
    @EnvironmentObject private var app: AppState
    @Environment(\.dismiss) private var dismiss

    let supplement: SupplementDef?

    @State private var name = ""
    @State private var dose = ""
    @State private var category = "supplement"
    @State private var prescribingDoc = ""
    @State private var notes = ""
    @State private var cycleEnabled = false
    @State private var cycleOn = 5
    @State private var cycleOff = 2
    @State private var reminderEnabled = true
    @State private var active = true
    @State private var saving = false
    @State private var saveError: String?

    // Reminder schedule (server reads schedule_json with shape {"times":["07:00", ...]})
    @State private var reminderTimes: [Date] = []

    private let categories = ["supplement", "peptide", "medication"]

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    field("NAME") {
                        TextField("e.g. BPC-157", text: $name)
                            .textInputAutocapitalization(.sentences)
                    }
                    field("DOSE") {
                        TextField("e.g. 250mcg SQ morning", text: $dose)
                    }
                    field("CATEGORY") {
                        Picker("Category", selection: $category) {
                            ForEach(categories, id: \.self) { c in
                                Text(c.capitalized).tag(c)
                            }
                        }
                        .pickerStyle(.segmented)
                    }
                    cycleSection
                    Toggle(isOn: $reminderEnabled) {
                        VStack(alignment: .leading) {
                            Text("Reminder").foregroundStyle(Theme.text)
                            Text("Push when due — uses your supplement notification rule")
                                .font(.caption).foregroundStyle(Theme.text3)
                        }
                    }
                    .tint(Theme.ai)
                    .padding(.horizontal, 14).padding(.vertical, 10)
                    .background(Theme.surface)
                    .clipShape(RoundedRectangle(cornerRadius: 12))

                    if reminderEnabled {
                        timesSection
                    }

                    if category == "medication" {
                        field("PRESCRIBING DOC (optional)") {
                            TextField("Name", text: $prescribingDoc)
                        }
                    }
                    field("NOTES (optional)") {
                        TextEditor(text: $notes)
                            .scrollContentBackground(.hidden).frame(minHeight: 70)
                    }
                    if let saveError {
                        Text(saveError).foregroundStyle(Theme.financeBad).font(.footnote)
                    }
                    if supplement != nil {
                        Button(role: .destructive) { Task { await archive() } } label: {
                            Text("Archive").frame(maxWidth: .infinity).padding(.vertical, 10)
                        }
                        .padding(.top, 8)
                    }
                }
                .padding(20)
            }
            .background(Theme.bg)
            .navigationTitle(supplement == nil ? "Add" : "Edit")
            .toolbarBackground(Theme.bg, for: .navigationBar)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Button("Cancel") { dismiss() }.foregroundStyle(Theme.text2)
                }
                ToolbarItem(placement: .topBarTrailing) {
                    Button { Task { await save() } } label: {
                        if saving { ProgressView().tint(Theme.ai) } else { Text("Save").bold().foregroundStyle(Theme.ai) }
                    }
                    .disabled(saving || name.trimmingCharacters(in: .whitespaces).isEmpty)
                }
            }
            .onAppear(perform: loadInitial)
        }
    }

    private var timesSection: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack {
                Text("TIMES")
                    .font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
                Spacer()
                Button {
                    var c = Calendar.current
                    c.timeZone = .current
                    let d = c.date(bySettingHour: 7, minute: 0, second: 0, of: Date()) ?? Date()
                    reminderTimes.append(d)
                } label: {
                    Image(systemName: "plus.circle.fill").foregroundStyle(Theme.ai)
                }
            }
            if reminderTimes.isEmpty {
                Text("No reminder times set — tap + to add one")
                    .font(.caption).foregroundStyle(Theme.text3)
                    .padding(.horizontal, 14).padding(.vertical, 10)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .background(Color(hex: 0x0f0f0f))
                    .clipShape(RoundedRectangle(cornerRadius: 12))
            } else {
                ForEach(Array(reminderTimes.enumerated()), id: \.offset) { idx, _ in
                    HStack {
                        DatePicker("",
                                   selection: Binding(
                                       get: { reminderTimes[idx] },
                                       set: { reminderTimes[idx] = $0 }),
                                   displayedComponents: .hourAndMinute)
                            .labelsHidden()
                            .tint(Theme.ai)
                        Spacer()
                        Button(role: .destructive) {
                            reminderTimes.remove(at: idx)
                        } label: {
                            Image(systemName: "minus.circle.fill").foregroundStyle(Theme.financeBad)
                        }
                    }
                    .padding(.horizontal, 14).padding(.vertical, 10)
                    .background(Color(hex: 0x0f0f0f))
                    .clipShape(RoundedRectangle(cornerRadius: 12))
                }
            }
        }
    }

    private var cycleSection: some View {
        VStack(alignment: .leading, spacing: 6) {
            Toggle(isOn: $cycleEnabled) {
                Text("CYCLE")
                    .font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
            }
            .tint(Theme.ai)
            if cycleEnabled {
                HStack {
                    Stepper("On: \(cycleOn) days", value: $cycleOn, in: 1...60)
                        .foregroundStyle(Theme.text)
                    Spacer()
                }
                .padding(.horizontal, 14).padding(.vertical, 10)
                .background(Color(hex: 0x0f0f0f))
                .clipShape(RoundedRectangle(cornerRadius: 12))
                HStack {
                    Stepper("Off: \(cycleOff) days", value: $cycleOff, in: 0...60)
                        .foregroundStyle(Theme.text)
                    Spacer()
                }
                .padding(.horizontal, 14).padding(.vertical, 10)
                .background(Color(hex: 0x0f0f0f))
                .clipShape(RoundedRectangle(cornerRadius: 12))
            }
        }
    }

    private func field<C: View>(_ label: String, @ViewBuilder _ content: () -> C) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(label).font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
            content()
                .padding(.horizontal, 14).padding(.vertical, 10)
                .background(Color(hex: 0x0f0f0f))
                .clipShape(RoundedRectangle(cornerRadius: 12))
                .foregroundStyle(Theme.text).tint(Theme.ai)
        }
    }

    private func loadInitial() {
        guard let s = supplement else { return }
        name = s.name; dose = s.dose; category = s.category
        prescribingDoc = s.prescribing_doc; notes = s.notes
        reminderEnabled = s.reminder_enabled; active = s.active
        if let on = s.cycle_days_on, let off = s.cycle_days_off, on > 0 {
            cycleEnabled = true; cycleOn = on; cycleOff = off
        }
        reminderTimes = parseScheduleTimes(s.schedule_json)
    }

    private func parseScheduleTimes(_ raw: String) -> [Date] {
        guard let data = raw.data(using: .utf8),
              let any = try? JSONSerialization.jsonObject(with: data)
        else { return [] }
        let strings: [String]
        if let obj = any as? [String: Any], let t = obj["times"] as? [String] { strings = t }
        else if let arr = any as? [String] { strings = arr }
        else { return [] }
        var cal = Calendar.current; cal.timeZone = .current
        return strings.compactMap { hhmm in
            let parts = hhmm.split(separator: ":")
            guard parts.count == 2,
                  let h = Int(parts[0]), let m = Int(parts[1]) else { return nil }
            return cal.date(bySettingHour: h, minute: m, second: 0, of: Date())
        }
    }

    private func serializeSchedule() -> String? {
        guard reminderEnabled, !reminderTimes.isEmpty else { return nil }
        let fmt = DateFormatter()
        fmt.timeZone = .current
        fmt.dateFormat = "HH:mm"
        let times = reminderTimes.map(fmt.string(from:)).sorted()
        let payload: [String: Any] = ["times": times]
        if let data = try? JSONSerialization.data(withJSONObject: payload),
           let s = String(data: data, encoding: .utf8) {
            return s
        }
        return nil
    }

    private func save() async {
        guard let api = app.apiClient() else { return }
        saving = true; saveError = nil
        defer { saving = false }
        let input = APIClient.SupplementDefInput(
            name: name.trimmingCharacters(in: .whitespaces),
            dose: dose,
            category: category,
            schedule_json: serializeSchedule(),
            cycle_days_on: cycleEnabled ? cycleOn : nil,
            cycle_days_off: cycleEnabled ? cycleOff : nil,
            reminder_enabled: reminderEnabled,
            active: active,
            prescribing_doc: prescribingDoc,
            notes: notes
        )
        do {
            if let s = supplement {
                try await api.updateSupplementDef(id: s.id, input)
            } else {
                _ = try await api.createSupplementDef(input)
            }
            dismiss()
        } catch {
            saveError = (error as? APIClient.APIError)?.errorDescription ?? error.localizedDescription
        }
    }

    private func archive() async {
        guard let api = app.apiClient(), let s = supplement else { return }
        saving = true; defer { saving = false }
        do {
            try await api.archiveSupplementDef(id: s.id)
            dismiss()
        } catch {
            saveError = (error as? APIClient.APIError)?.errorDescription ?? error.localizedDescription
        }
    }
}
