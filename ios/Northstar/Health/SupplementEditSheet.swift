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

    // Reminder schedule (server reads schedule_json with shape
    // {"times":["07:00",...],"days":["mon","wed",...]}). Empty `days` means
    // every day; explicit list narrows firing to those weekdays only.
    @State private var reminderTimes: [Date] = []
    @State private var reminderDays: Set<String> = []   // empty = every day

    private let categories = ["supplement", "peptide", "medication"]
    private let weekdays: [(key: String, label: String)] = [
        ("mon", "M"), ("tue", "T"), ("wed", "W"), ("thu", "T"),
        ("fri", "F"), ("sat", "S"), ("sun", "S"),
    ]

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
                        daysSection
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

    private var daysSection: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack {
                Text("DAYS")
                    .font(.caption2).bold().tracking(1).foregroundStyle(Theme.text3)
                Spacer()
                Button(reminderDays.isEmpty ? "Daily" : "Custom") {
                    if reminderDays.isEmpty {
                        reminderDays = ["mon", "wed", "fri"]
                    } else {
                        reminderDays = []
                    }
                }
                .font(.caption).foregroundStyle(Theme.ai)
            }
            HStack(spacing: 6) {
                ForEach(weekdays, id: \.key) { d in
                    let on = reminderDays.isEmpty || reminderDays.contains(d.key)
                    Button {
                        if reminderDays.isEmpty {
                            // First tap converts "daily" to explicit selection
                            // so the user can subtract from the full set.
                            reminderDays = Set(weekdays.map(\.key))
                        }
                        if reminderDays.contains(d.key) {
                            reminderDays.remove(d.key)
                        } else {
                            reminderDays.insert(d.key)
                        }
                        // Collapse "all selected" back to empty = daily.
                        if reminderDays.count == weekdays.count {
                            reminderDays = []
                        }
                    } label: {
                        Text(d.label)
                            .font(.system(size: 13, weight: .semibold))
                            .frame(maxWidth: .infinity, minHeight: 36)
                            .background(on ? Theme.ai.opacity(0.18) : Color(hex: 0x0f0f0f))
                            .foregroundStyle(on ? Theme.ai : Theme.text3)
                            .clipShape(RoundedRectangle(cornerRadius: 10))
                            .overlay(
                                RoundedRectangle(cornerRadius: 10)
                                    .strokeBorder(on ? Theme.ai : Color.clear, lineWidth: 1)
                            )
                    }
                    .buttonStyle(.plain)
                }
            }
            Text(reminderDays.isEmpty
                 ? "Reminder fires every day at the times above."
                 : "Fires only on selected days at the times above.")
                .font(.caption).foregroundStyle(Theme.text3)
                .padding(.horizontal, 4)
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
        let parsed = parseSchedule(s.schedule_json)
        reminderTimes = parsed.times
        reminderDays = parsed.days
    }

    private struct ParsedSchedule {
        var times: [Date] = []
        var days: Set<String> = []
    }

    private func parseSchedule(_ raw: String) -> ParsedSchedule {
        var out = ParsedSchedule()
        guard let data = raw.data(using: .utf8),
              let any = try? JSONSerialization.jsonObject(with: data)
        else { return out }
        let timeStrings: [String]
        if let obj = any as? [String: Any], let t = obj["times"] as? [String] {
            timeStrings = t
            if let d = obj["days"] as? [String] {
                out.days = Set(d.map { $0.lowercased() })
            }
        } else if let arr = any as? [String] {
            timeStrings = arr
        } else {
            return out
        }
        var cal = Calendar.current; cal.timeZone = .current
        out.times = timeStrings.compactMap { hhmm in
            let parts = hhmm.split(separator: ":")
            guard parts.count == 2,
                  let h = Int(parts[0]), let m = Int(parts[1]) else { return nil }
            return cal.date(bySettingHour: h, minute: m, second: 0, of: Date())
        }
        return out
    }

    private func serializeSchedule() -> String? {
        guard reminderEnabled, !reminderTimes.isEmpty else { return nil }
        let fmt = DateFormatter()
        fmt.timeZone = .current
        fmt.dateFormat = "HH:mm"
        let times = reminderTimes.map(fmt.string(from:)).sorted()
        var payload: [String: Any] = ["times": times]
        // Only include `days` when narrowing — empty = "every day" per server.
        if !reminderDays.isEmpty {
            let order = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"]
            payload["days"] = order.filter { reminderDays.contains($0) }
        }
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
