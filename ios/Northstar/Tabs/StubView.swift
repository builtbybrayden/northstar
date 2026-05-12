import SwiftUI

struct StubView: View {
    let title: String
    let phase: String
    let note: String

    var body: some View {
        NavigationStack {
            VStack(spacing: 16) {
                Spacer()
                Image(systemName: "hammer.fill")
                    .font(.system(size: 32))
                    .foregroundStyle(Theme.text3)
                Text(phase)
                    .font(.caption)
                    .foregroundStyle(Theme.ai)
                    .padding(.horizontal, 10).padding(.vertical, 4)
                    .background(Theme.ai.opacity(0.12))
                    .clipShape(Capsule())
                Text(note)
                    .multilineTextAlignment(.center)
                    .foregroundStyle(Theme.text2)
                    .padding(.horizontal, 30)
                Spacer()
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            .background(Theme.bg)
            .navigationTitle(title)
            .toolbarBackground(Theme.bg, for: .navigationBar)
        }
    }
}
