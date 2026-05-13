import Foundation
import Security

/// Thin Keychain wrapper for the device's bearer token. Stored under the app's
/// access group with `whenUnlockedThisDeviceOnly` so an iCloud restore to a new
/// device requires re-pairing.
struct Keychain {
    private let service = "dev.northstar.app"
    private let account = "bearer-token"

    var bearerToken: String? {
        get { read() }
        nonmutating set {
            if let v = newValue { write(v) } else { delete() }
        }
    }

    private func read() -> String? {
        let q: [String: Any] = [
            kSecClass as String:       kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecReturnData as String:  true,
            kSecMatchLimit as String:  kSecMatchLimitOne
        ]
        var item: CFTypeRef?
        guard SecItemCopyMatching(q as CFDictionary, &item) == errSecSuccess,
              let data = item as? Data,
              let s = String(data: data, encoding: .utf8) else { return nil }
        return s
    }

    private func write(_ value: String) {
        let data = Data(value.utf8)
        let q: [String: Any] = [
            kSecClass as String:       kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account
        ]
        let attrs: [String: Any] = [
            kSecValueData as String:   data,
            kSecAttrAccessible as String: kSecAttrAccessibleWhenUnlockedThisDeviceOnly
        ]
        let status = SecItemUpdate(q as CFDictionary, attrs as CFDictionary)
        if status == errSecItemNotFound {
            var add = q
            add.merge(attrs) { _, n in n }
            SecItemAdd(add as CFDictionary, nil)
        }
    }

    private func delete() {
        let q: [String: Any] = [
            kSecClass as String:       kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account
        ]
        SecItemDelete(q as CFDictionary)
    }
}
