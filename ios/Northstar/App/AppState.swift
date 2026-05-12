import Foundation

@MainActor
final class AppState: ObservableObject {
    @Published var paired: Bool
    @Published var deviceID: String?
    @Published var serverURL: URL?

    private let keychain = Keychain()

    init() {
        if let token = keychain.bearerToken,
           let urlStr = UserDefaults.standard.string(forKey: "northstar.serverURL"),
           let url = URL(string: urlStr) {
            self.serverURL = url
            self.paired = !token.isEmpty
            self.deviceID = UserDefaults.standard.string(forKey: "northstar.deviceID")
        } else {
            self.paired = false
        }
    }

    func completePairing(serverURL: URL, bearerToken: String, deviceID: String) {
        keychain.bearerToken = bearerToken
        UserDefaults.standard.set(serverURL.absoluteString, forKey: "northstar.serverURL")
        UserDefaults.standard.set(deviceID, forKey: "northstar.deviceID")
        self.serverURL = serverURL
        self.deviceID = deviceID
        self.paired = true
    }

    func signOut() {
        keychain.bearerToken = nil
        UserDefaults.standard.removeObject(forKey: "northstar.serverURL")
        UserDefaults.standard.removeObject(forKey: "northstar.deviceID")
        self.serverURL = nil
        self.deviceID = nil
        self.paired = false
    }

    /// Returns an authenticated API client if the device is paired, else nil.
    func apiClient() -> APIClient? {
        guard let url = serverURL, let tok = keychain.bearerToken else { return nil }
        return APIClient(baseURL: url, bearer: tok)
    }
}
