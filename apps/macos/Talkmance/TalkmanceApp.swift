import SwiftUI

@main
struct TalkmanceApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate
    @StateObject private var appState = AppState.shared

    var body: some Scene {
        // 메뉴바 아이콘 + 드롭다운 메뉴 (항상 표시)
        MenuBarExtra {
            MenuBarView()
                .environmentObject(appState)
        } label: {
            Image("MenuBarIcon")
        }
        .menuBarExtraStyle(.menu)

        // 메인 창 (Dock 숨김 시에도 메뉴바에서 열기 가능)
        WindowGroup("톡맨스", id: "main") {
            MainWindowView()
                .environmentObject(appState)
        }
        .defaultSize(width: 520, height: 420)

        // 설정 창 (Dock 표시 토글)
        Settings {
            SettingsView()
                .environmentObject(appState)
        }
    }
}

/// 앱 수명주기 + Cmd+Shift+D 디버그 패널 단축키
@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    /// 폴백 메인 창은 앱 시작 시 1회만 생성
    private static var fallbackWindowShown = false

    /// 19세 확인 게이트 (T-39) — 1회 통과 시 UserDefaults 저장
    private static let adultGateKey = "adultGatePassed"

    @MainActor
    func applicationDidFinishLaunching(_ notification: Notification) {
        DebugLogger.shared.feature("앱시작", "톡맨스 macOS 시작됨 (버전 1.0.0)")
        // 이미지 캐시 — AsyncImage(공유 세션)가 URLCache.shared 사용
        URLCache.shared = URLCache(memoryCapacity: 50_000_000, diskCapacity: 200_000_000)
        NSEvent.addLocalMonitorForEvents(matching: .keyDown) { event in
            if event.modifierFlags.contains(.command) && event.modifierFlags.contains(.shift) && event.charactersIgnoringModifiers?.lowercased() == "d" {
                DispatchQueue.main.async { self.toggleDebugPanel() }
                return nil
            }
            return event
        }
        // 19세 확인 게이트 — 미통과 시 앱 종료
        if !UserDefaults.standard.bool(forKey: Self.adultGateKey) {
            showAdultGate()
        }
        // 첫 실행 시 메인 창 자동 열기 — 진단 로그 포함 (폴백은 1회만)
        DispatchQueue.main.asyncAfter(deadline: .now() + 2.0) {
            guard !Self.fallbackWindowShown else { return }
            let windows = NSApp.windows.map { "\($0.title)|\($0.styleMask.rawValue)|\($0.identifier?.rawValue ?? "-")" }
            DebugLogger.shared.feature("앱시작", "창 목록: \(windows)")
            let hasMain = NSApp.windows.contains { $0.styleMask.contains(.titled) }
            if !hasMain {
                DebugLogger.shared.feature("앱시작", "메인 창 없음 — NSHostingController로 직접 생성")
                Self.fallbackWindowShown = true
                let hosting = NSHostingController(rootView: MainWindowView().environmentObject(AppState.shared))
                let window = NSWindow(contentViewController: hosting)
                window.title = "톡맨스"
                window.styleMask = [.titled, .closable, .resizable, .miniaturizable]
                window.identifier = NSUserInterfaceItemIdentifier("MainWindow")
                window.setFrameAutosaveName("MainWindow")
                window.makeKeyAndOrderFront(nil)
                NSApp.activate(ignoringOtherApps: true)
            }
        }
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        false // 창을 닫아도 앱은 메뉴바에 유지
    }

    func applicationSupportsSecureRestorableState(_ app: NSApplication) -> Bool {
        true
    }

    /// 19세 이상 확인 알림 (T-39) — 거부 시 앱 종료
    private func showAdultGate() {
        let alert = NSAlert()
        alert.messageText = "성인 인증 (만 19세 이상)"
        alert.informativeText = "톡맨스는 만 19세 이상 성인을 위한 AI 연애 채팅 앱입니다.\n미성년자는 이용할 수 없으며, 미성년자 캐릭터 설정은 금지됩니다.\n계속하시겠습니까?"
        alert.alertStyle = .warning
        alert.addButton(withTitle: "만 19세 이상입니다 — 계속")
        alert.addButton(withTitle: "취소")
        alert.window.level = .floating
        if alert.runModal() == .alertFirstButtonReturn {
            UserDefaults.standard.set(true, forKey: Self.adultGateKey)
            DebugLogger.shared.feature("성인인증", "19세 이상 확인됨 (게이트 통과)")
        } else {
            DebugLogger.shared.feature("성인인증", "19세 미만/거부 — 앱 종료")
            NSApp.terminate(nil)
        }
    }

    /// 메인 창 열기 — 이미 있으면 활성화만 (디버그 패널/메뉴바 공용)
    func openMainWindow() {
        if let window = NSApp.windows.first(where: { $0.styleMask.contains(.titled) && $0.identifier?.rawValue == "MainWindow" }) {
            window.makeKeyAndOrderFront(nil)
            NSApp.activate(ignoringOtherApps: true)
            return
        }
        let hosting = NSHostingController(rootView: MainWindowView().environmentObject(AppState.shared))
        let window = NSWindow(contentViewController: hosting)
        window.title = "톡맨스"
        window.styleMask = [.titled, .closable, .resizable, .miniaturizable]
        window.identifier = NSUserInterfaceItemIdentifier("MainWindow")
        window.setFrameAutosaveName("MainWindow")
        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }

    func toggleDebugPanel() {
        let key = "debugPanelWindow"
        if let window = NSApp.windows.first(where: { $0.identifier?.rawValue == key }) {
            window.close()
            DebugLogger.shared.feature("디버그패널", "디버그 패널 닫힘")
            return
        }
        let hosting = NSHostingController(rootView: DebugPanelView().environmentObject(AppState.shared))
        let window = NSWindow(contentViewController: hosting)
        window.identifier = NSUserInterfaceItemIdentifier(key)
        window.title = "디버그 패널"
        window.styleMask = [.titled, .closable, .resizable, .miniaturizable]
        window.setContentSize(NSSize(width: 620, height: 480))
        window.setFrameAutosaveName("DebugPanel")
        window.isReleasedWhenClosed = false
        window.makeKeyAndOrderFront(nil)
        window.center()
        NSApp.activate(ignoringOtherApps: true)
    }
}