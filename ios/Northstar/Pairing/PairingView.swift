import SwiftUI

/// First-launch pairing flow.
///
/// QR payload format (produced by the server's admin web page):
///   northstar://pair?server=https://host:8080&code=123456
struct PairingView: View {
    @EnvironmentObject private var app: AppState

    @State private var serverURL: String = "http://localhost:8080"
    @State private var code: String = ""
    @State private var busy = false
    @State private var error: String?
    @State private var showScanner = false

    var body: some View {
        ZStack {
            Theme.bg.ignoresSafeArea()
            VStack(spacing: 20) {
                Spacer()
                Image(systemName: "sparkles")
                    .font(.system(size: 56, weight: .bold))
                    .foregroundStyle(Theme.ai)
                Text("Northstar")
                    .font(.system(size: 40, weight: .heavy, design: .default))
                Text("Pair this device with your server.")
                    .foregroundStyle(Theme.text2)
                    .font(.subheadline)

                VStack(spacing: 12) {
                    field("Server URL", text: $serverURL, kind: .url)
                    field("Pairing code (6 digits)", text: $code, kind: .number)
                }
                .padding(.horizontal, 24)
                .padding(.top, 24)

                if let error {
                    Text(error)
                        .foregroundStyle(Theme.financeBad)
                        .font(.footnote)
                        .padding(.horizontal, 24)
                        .multilineTextAlignment(.center)
                }

                VStack(spacing: 12) {
                    Button(action: redeem) {
                        HStack {
                            if busy { ProgressView().tint(.black) }
                            Text(busy ? "Pairing…" : "Pair")
                                .font(.system(.body, weight: .semibold))
                        }
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 14)
                        .background(Theme.ai)
                        .foregroundStyle(.black)
                        .clipShape(RoundedRectangle(cornerRadius: 16))
                    }
                    .disabled(busy || code.count != 6 || URL(string: serverURL) == nil)

                    Button { showScanner = true } label: {
                        Label("Scan QR from server", systemImage: "qrcode.viewfinder")
                            .frame(maxWidth: .infinity)
                            .padding(.vertical, 14)
                            .foregroundStyle(Theme.text2)
                            .overlay(
                                RoundedRectangle(cornerRadius: 16)
                                    .stroke(Theme.border, lineWidth: 1))
                    }
                }
                .padding(.horizontal, 24)

                Spacer()
            }
        }
        .sheet(isPresented: $showScanner) {
            ZStack(alignment: .top) {
                QRScannerView { payload in
                    handleScannedPayload(payload)
                    showScanner = false
                }
                .ignoresSafeArea()
                HStack {
                    Button("Cancel") { showScanner = false }
                        .foregroundStyle(.white)
                        .padding(.horizontal, 18)
                        .padding(.vertical, 10)
                        .background(.black.opacity(0.6))
                        .clipShape(Capsule())
                    Spacer()
                }
                .padding()
            }
        }
    }

    enum FieldKind { case url, number }
    private func field(_ label: String, text: Binding<String>, kind: FieldKind) -> some View {
        TextField("", text: text, prompt: Text(label).foregroundStyle(Theme.text3))
            .keyboardType(kind == .url ? .URL : .numberPad)
            .textInputAutocapitalization(.never)
            .autocorrectionDisabled()
            .padding(.horizontal, 16).padding(.vertical, 14)
            .background(Theme.surface)
            .clipShape(RoundedRectangle(cornerRadius: 14))
            .foregroundStyle(Theme.text)
    }

    private func handleScannedPayload(_ s: String) {
        // northstar://pair?server=...&code=...
        guard let comps = URLComponents(string: s),
              comps.scheme == "northstar", comps.host == "pair" else {
            error = "QR not recognized."
            return
        }
        for item in comps.queryItems ?? [] {
            if item.name == "server", let v = item.value { serverURL = v }
            if item.name == "code",   let v = item.value { code = v }
        }
        if code.count == 6 && URL(string: serverURL) != nil { redeem() }
    }

    private func redeem() {
        guard let url = URL(string: serverURL) else { return }
        busy = true; error = nil
        let deviceName = UIDevice.current.model       // "iPhone" — privacy-safe fallback
        Task {
            defer { busy = false }
            do {
                let api = APIClient(baseURL: url)
                let resp = try await api.pairRedeem(code: code, deviceName: deviceName)
                app.completePairing(serverURL: url,
                                    bearerToken: resp.bearer_token,
                                    deviceID: resp.device_id)
            } catch let e as APIClient.APIError {
                error = e.errorDescription
            } catch let e {
                error = e.localizedDescription
            }
        }
    }
}
