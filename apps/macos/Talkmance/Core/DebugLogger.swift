import Foundation
import SwiftUI

/// 톡맨스 디버그 로거 (디버그 패널 노출용)
/// 형식: [HH:mm:ss.SSS] [LEVEL] [TAG] message
@MainActor
final class DebugLogger: ObservableObject {
    static let shared = DebugLogger()

    @Published private(set) var entries: [LogEntry] = []
    private let maxEntries = 500
    private let dateFormatter: DateFormatter

    struct LogEntry: Identifiable {
        let id = UUID()
        let time: Date
        let level: String
        let tag: String
        let message: String
    }

    private init() {
        dateFormatter = DateFormatter()
        dateFormatter.dateFormat = "HH:mm:ss.SSS"
    }

    func log(_ level: String, _ tag: String, _ message: String) {
        let entry = LogEntry(time: Date(), level: level, tag: tag, message: message)
        entries.append(entry)
        if entries.count > maxEntries {
            entries.removeFirst(entries.count - maxEntries)
        }
        #if DEBUG
        NSLog("[%@] [%@] %@", level, tag, message)
        #endif
    }

    func info(_ tag: String, _ message: String) { log("INFO", tag, message) }
    func warn(_ tag: String, _ message: String) { log("WARN", tag, message) }
    func error(_ tag: String, _ message: String) { log("ERROR", tag, message) }
    func feature(_ tag: String, _ message: String) { log("INFO", "FEATURE", "[\(tag)] \(message)") }

    /// 로그 초기화 (디버그 패널)
    func clear() {
        entries.removeAll()
        log("INFO", "디버그패널", "로그가 초기화되었습니다")
    }

    /// 레벨별 카운트 (디버그 패널 통계)
    func count(level: String) -> Int {
        entries.filter { $0.level == level }.count
    }

    /// 최근 기능 로그 (tag == FEATURE)
    func recentFeatures(limit: Int) -> [LogEntry] {
        entries.filter { $0.tag == "FEATURE" }.suffix(limit).reversed()
    }

    func formattedTime(_ date: Date) -> String {
        dateFormatter.string(from: date)
    }
}