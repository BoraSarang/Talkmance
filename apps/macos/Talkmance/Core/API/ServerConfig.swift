import Foundation

/// 서버 연결 대상 (로컬 / Render / 커스텀)
enum ServerTarget: String, CaseIterable, Identifiable {
    case local
    case render
    case custom

    var id: String { rawValue }

    var label: String {
        switch self {
        case .local: return "로컬 개발 서버"
        case .render: return "Render 배포 서버"
        case .custom: return "커스텀 URL"
        }
    }
}

/// 서버 연결 설정 (UserDefaults 저장)
@MainActor
final class ServerConfig: ObservableObject {
    static let shared = ServerConfig()

    /// 서버 변경 알림 (목록 재로드용)
    static let serverChangedNotification = Notification.Name("serverChanged")

private let targetKey = "serverTarget"
	private let renderURLKey = "serverRenderURL"
	private let customURLKey = "serverCustomURL"
	private let deviceIDKey = "deviceId"
	private let polishKey = "polishEnabled"

    /// 서버 대상 (기본: 로컬)
    @Published var target: ServerTarget {
        didSet {
            if target != oldValue {
                applyTargetChange()
            }
        }
    }

    /// Render 배포 URL (예: https://talkmance.onrender.com)
    @Published var renderBaseURL: String {
        didSet {
            UserDefaults.standard.set(renderBaseURL, forKey: renderURLKey)
        }
    }

/// 커스텀 서버 URL
	@Published var customBaseURL: String {
		didSet {
			UserDefaults.standard.set(customBaseURL, forKey: customURLKey)
		}
	}

	/// 한국어 다듬기(B) 후처리 토글 (기본 끔 — 서버 polish 파라미터로 전달)
	@Published var polishEnabled: Bool {
		didSet {
			UserDefaults.standard.set(polishEnabled, forKey: polishKey)
			DebugLogger.shared.feature("서버설정", "한국어 다듬기(B) \(polishEnabled ? "켬" : "끔")")
		}
	}

    /// 실제 사용할 베이스 URL
    var baseURL: String {
        switch target {
        case .local:
            return "http://localhost:8080"
        case .render:
            return renderBaseURL.isEmpty ? "https://talkmance.onrender.com" : renderBaseURL
        case .custom:
            return customBaseURL.isEmpty ? "http://localhost:8080" : customBaseURL
        }
    }

    /// 기기 ID (설치당 1회 생성 — 익명 인증용)
    let deviceID: String

    /// 인증 JWT (메모리 유지)
    @Published var token: String? {
        didSet {
            if token != nil {
                DebugLogger.shared.feature("인증", "JWT 발급됨 (\(token!.prefix(12))…)")
            }
        }
    }

    private init() {
        let storedTarget = UserDefaults.standard.string(forKey: targetKey)
        target = ServerTarget(rawValue: storedTarget ?? "") ?? .local
renderBaseURL = UserDefaults.standard.string(forKey: renderURLKey) ?? ""
		customBaseURL = UserDefaults.standard.string(forKey: customURLKey) ?? ""
		polishEnabled = UserDefaults.standard.bool(forKey: polishKey)
        if let existing = UserDefaults.standard.string(forKey: deviceIDKey) {
            deviceID = existing
        } else {
            let newID = UUID().uuidString
            UserDefaults.standard.set(newID, forKey: deviceIDKey)
            deviceID = newID
        }
        DebugLogger.shared.feature("서버설정", "초기화됨 (target=\(target.rawValue), baseURL=\(baseURL), deviceID=\(deviceID.prefix(8))…)")
    }

    /// 대상 변경 시: 저장 + 토큰 무효화 + 재로드 알림
    private func applyTargetChange() {
        UserDefaults.standard.set(target.rawValue, forKey: targetKey)
        token = nil
        DebugLogger.shared.feature("서버설정", "서버 대상 변경됨: \(target.rawValue) → \(baseURL)")
        NotificationCenter.default.post(name: ServerConfig.serverChangedNotification, object: nil)
    }

    /// 인증 헤더 (JWT 있으면 Bearer)
    var authHeader: [String: String] {
        var headers: [String: String] = [:]
        if let token {
            headers["Authorization"] = "Bearer \(token)"
        }
        return headers
    }
}
