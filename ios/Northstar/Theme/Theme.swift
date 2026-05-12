import SwiftUI

// Design tokens lifted from mockups/styles.css §:root.
enum Theme {
    static let bg          = Color(hex: 0x0a0a0a)
    static let surface     = Color(hex: 0x141414)
    static let surfaceHi   = Color(hex: 0x1c1c1c)
    static let border      = Color(hex: 0x2a2a2a)

    static let text        = Color.white
    static let text2       = Color(hex: 0xa8a8a8)
    static let text3       = Color(hex: 0x6b6b6b)

    static let finance     = Color(hex: 0x00E676)
    static let financeBad  = Color(hex: 0xFF3B30)
    static let goals       = Color(hex: 0x5E5CE6)
    static let healthGo    = Color(hex: 0x16EC06)
    static let healthMid   = Color(hex: 0xFFD60A)
    static let healthStop  = Color(hex: 0xFF0026)
    static let healthBlue  = Color(hex: 0x00B6FF)
    static let ai          = Color(hex: 0xFF9F0A)
}

extension Color {
    init(hex: Int, alpha: Double = 1) {
        self.init(
            .sRGB,
            red: Double((hex >> 16) & 0xFF) / 255,
            green: Double((hex >> 8) & 0xFF) / 255,
            blue: Double(hex & 0xFF) / 255,
            opacity: alpha)
    }
}
