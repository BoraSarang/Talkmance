import SwiftUI

/// 메뉴바 드롭다운 메뉴 (기본 기능)
struct MenuBarView: View {
    @EnvironmentObject private var appState: AppState
    @Environment(\.openWindow) private var openWindow
    @Environment(\.openSettings) private var openSettings

    /// 제목 있는 메인 창이 이미 열려 있으면 앞으로 가져오고, 없으면 새로 연다
    private func showMainWindow() {
        let hasMain = NSApp.windows.contains { $0.styleMask.contains(.titled) && $0.title != "디버그 패널" && $0.title != "설정" }
        if hasMain {
            NSApp.activate(ignoringOtherApps: true)
            DebugLogger.shared.feature("메뉴바", "기존 메인 창 활성화")
        } else {
            openWindow(id: "main")
            DebugLogger.shared.feature("메뉴바", "메인 창 열림 (openWindow)")
        }
    }

    var body: some View {
        Button {
            showMainWindow()
            DebugLogger.shared.feature("메뉴바", "톡맨스 열기 클릭됨")
        } label: {
            Label("톡맨스 열기", systemImage: "bubble.left.and.bubble.right.fill")
        }

        Divider()

        Toggle(isOn: $appState.showInDock) {
            Label("Dock에 표시", systemImage: "dock.rectangle")
        }

        Divider()

        Button {
            openSettings()
            DebugLogger.shared.feature("메뉴바", "설정 열기 클릭됨")
        } label: {
            Label("설정…", systemImage: "gearshape")
        }

        Divider()

        Button {
            (NSApp.delegate as? AppDelegate)?.toggleDebugPanel()
            DebugLogger.shared.feature("메뉴바", "디버그 패널 열기 클릭됨")
        } label: {
            Label("디버그 패널", systemImage: "ladybug")
        }

        Divider()

        Button {
            DebugLogger.shared.feature("메뉴바", "종료 클릭됨")
            NSApplication.shared.terminate(nil)
        } label: {
            Label("종료", systemImage: "power")
        }
    }
}