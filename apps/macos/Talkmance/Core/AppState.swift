import AppKit
import Foundation

/// 앱 전역 상태 (Dock 표시 토글 등)
@MainActor
final class AppState: ObservableObject {
    static let shared = AppState()

    private let showInDockKey = "showInDock"

    /// Dock에 아이콘 표시 여부 (기본: 표시)
    @Published var showInDock: Bool {
        didSet {
            UserDefaults.standard.set(showInDock, forKey: showInDockKey)
            applyDockPolicy()
            DebugLogger.shared.feature("Dock토글", showInDock ? "Dock에 표시 활성화됨" : "Dock에 표시 비활성화됨")
        }
    }

    private init() {
        let stored = UserDefaults.standard.object(forKey: showInDockKey) as? Bool
        showInDock = stored ?? true
        applyDockPolicy()
        DebugLogger.shared.feature("AppState", "앱 상태 초기화됨 (showInDock=\(showInDock))")
    }

    /// Dock 정책 적용: .regular → Dock 표시, .accessory → 메뉴바 전용
    func applyDockPolicy() {
        NSApp.setActivationPolicy(showInDock ? .regular : .accessory)
        if showInDock {
            NSApp.activate(ignoringOtherApps: true)
        }
    }
}