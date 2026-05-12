import SwiftUI

struct ContentView: View {
    @EnvironmentObject private var app: AppState

    var body: some View {
        Group {
            if app.paired {
                RootTabView()
            } else {
                PairingView()
            }
        }
    }
}
