import SwiftUI

struct RootTabView: View {
    var body: some View {
        TabView {
            HomeView()
                .tabItem { Label("Home", systemImage: "house.fill") }
            FinanceView()
                .tabItem { Label("Finance", systemImage: "dollarsign.circle.fill") }
            GoalsView()
                .tabItem { Label("Goals", systemImage: "star.fill") }
            HealthView()
                .tabItem { Label("Health", systemImage: "heart.fill") }
            AskView()
                .tabItem { Label("Ask", systemImage: "sparkles") }
        }
    }
}
