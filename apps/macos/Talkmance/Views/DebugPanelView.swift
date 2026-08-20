import AppKit
import SwiftUI

/// 디버그 패널 — Cmd+Shift+D 로 열기. 로그 뷰어(복사/저장) + 상태 + 통계/할당량
struct DebugPanelView: View {
    @ObservedObject private var logger = DebugLogger.shared
    @EnvironmentObject private var appState: AppState
    @State private var filter = ""
    @State private var levelFilter = "전체"
    @State private var tab = 0
    @State private var selectedLogID: UUID?
    @State private var autoScroll = true
    @State private var serverStatus = "확인 안 됨"
    @State private var catalogCount = 0
    @State private var checkingServer = false
    @State private var quotaText = "확인 안 됨"
    @State private var checkingQuota = false
    @State private var copyNotice = ""

    private let levels = ["전체", "ERROR", "WARN", "INFO"]

    private var filteredEntries: [DebugLogger.LogEntry] {
        var entries = logger.entries
        if levelFilter != "전체" {
            entries = entries.filter { $0.level == levelFilter }
        }
        if !filter.isEmpty {
            entries = entries.filter {
                $0.message.localizedCaseInsensitiveContains(filter)
                    || $0.tag.localizedCaseInsensitiveContains(filter)
            }
        }
        return entries
    }

    private var selectedEntry: DebugLogger.LogEntry? {
        guard let id = selectedLogID else { return nil }
        return logger.entries.first { $0.id == id }
    }

    private var appVersion: String {
        (Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String) ?? "?"
    }

    var body: some View {
        VStack(spacing: 0) {
            HStack {
                Text("디버그 패널")
                    .font(.headline)
                Spacer()
                if !copyNotice.isEmpty {
                    Text(copyNotice)
                        .font(.caption)
                        .foregroundStyle(.green)
                        .transition(.opacity)
                }
                Text("로그 \(logger.entries.count)건")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Button {
                    logger.clear()
                } label: {
                    Label("로그 지우기", systemImage: "trash")
                }
                .disabled(logger.entries.isEmpty)
            }
            .padding(8)

            Picker("", selection: $tab) {
                Text("로그").tag(0)
                Text("상태").tag(1)
                Text("통계").tag(2)
            }
            .pickerStyle(.segmented)
            .labelsHidden()
            .padding(.horizontal, 8)
            .padding(.bottom, 6)

            switch tab {
            case 1: statusTab
            case 2: statsTab
            default: logTab
            }
        }
        .frame(minWidth: 620, minHeight: 480)
        .onAppear {
            DebugLogger.shared.feature("디버그패널", "디버그 패널 표시됨")
        }
    }

    // MARK: - 로그 탭 (복사/저장 지원)

    private var logTab: some View {
        VStack(spacing: 0) {
            HStack(spacing: 6) {
                Picker("레벨", selection: $levelFilter) {
                    ForEach(levels, id: \.self) { Text($0) }
                }
                .pickerStyle(.menu)
                .frame(width: 100)
                TextField("필터 (기능명/메시지)", text: $filter)
                    .textFieldStyle(.roundedBorder)
                Button {
                    filter = ""
                    levelFilter = "전체"
                } label: {
                    Text("초기화")
                }
                .disabled(filter.isEmpty && levelFilter == "전체")
                Spacer()
                Toggle("자동 스크롤", isOn: $autoScroll)
                    .toggleStyle(.checkbox)
                    .controlSize(.small)
            }
            .padding(.horizontal, 8)
            .padding(.bottom, 4)

            HStack(spacing: 6) {
                Button {
                    copyFullLog()
                } label: {
                    Label("전체 복사", systemImage: "doc.on.doc")
                }
                .disabled(filteredEntries.isEmpty)
                Button {
                    copySelectedLog()
                } label: {
                    Label("선택한 줄 복사", systemImage: "text.cursor")
                }
                .disabled(selectedEntry == nil)
                Button {
                    saveLogFile()
                } label: {
                    Label("파일로 저장", systemImage: "square.and.arrow.down")
                }
                .disabled(filteredEntries.isEmpty)
                Spacer()
                Text("복사 포맷: [시간] [레벨] [태그] 메시지")
                    .font(.caption2)
                    .foregroundStyle(.tertiary)
            }
            .padding(.horizontal, 8)
            .padding(.bottom, 6)

            ScrollViewReader { proxy in
                List(selection: $selectedLogID) {
                    ForEach(filteredEntries) { entry in
                        HStack(alignment: .top, spacing: 6) {
                            Text(logger.formattedTime(entry.time))
                                .font(.system(.caption, design: .monospaced))
                                .foregroundStyle(.secondary)
                                .frame(width: 78, alignment: .leading)
                            Text(entry.level)
                                .font(.system(.caption, design: .monospaced))
                                .foregroundStyle(levelColor(entry.level))
                                .frame(width: 44, alignment: .leading)
                            Text(entry.tag)
                                .font(.system(.caption, design: .monospaced))
                                .foregroundStyle(.secondary)
                                .frame(width: 72, alignment: .leading)
                            Text(entry.message)
                                .font(.system(.caption, design: .monospaced))
                                .frame(maxWidth: .infinity, alignment: .leading)
                        }
                        .padding(.vertical, 2)
                        .contentShape(Rectangle())
                        .contextMenu {
                            Button("한 줄 복사") { copyLine(entry) }
                            Button("전체 복사") { copyFullLog() }
                            Divider()
                            Button("한 줄 저장…") { saveLineFile(entry) }
                        }
                    }
                }
                .listStyle(.plain)
                .onChange(of: filteredEntries.count) {
                    if autoScroll, let last = filteredEntries.last {
                        proxy.scrollTo(last.id, anchor: .bottom)
                    }
                }
            }
        }
    }

    // MARK: - 상태 탭

    private var statusTab: some View {
        VStack(alignment: .leading, spacing: 10) {
            Group {
                Text("서버 연결")
                    .font(.subheadline.bold())
                HStack {
                    Text("서버 대상")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .frame(width: 90, alignment: .leading)
                    Picker("", selection: serverTargetBinding) {
                        ForEach(ServerTarget.allCases) { t in
                            Text(t.label).tag(t)
                        }
                    }
                    .labelsHidden()
                    .frame(width: 160)
                }
                infoRow("베이스 URL", ServerConfig.shared.baseURL)
                infoRow("기기 ID", String(ServerConfig.shared.deviceID.prefix(8)) + "…")
                infoRow("JWT", ServerConfig.shared.token == nil ? "미발급" : "발급됨 (" + ServerConfig.shared.token!.prefix(12) + "…)")
                infoRow("서버 상태", serverStatus)
            }
            Divider()
            Group {
                Text("모델")
                    .font(.subheadline.bold())
                infoRow("기본 모델", "gemini-3-flash-preview (1순위)")
                infoRow("카탈로그", catalogCount == 0 ? "확인 안 됨" : "\(catalogCount)개 (Gemini 전체 + free)")
            }
            Divider()
            Group {
                Text("앱")
                    .font(.subheadline.bold())
                infoRow("버전", appVersion)
                Toggle("Dock에 표시", isOn: $appState.showInDock)
                    .font(.caption)
            }
            Spacer()
            HStack(spacing: 8) {
                Button {
                    Task { await checkServer() }
                } label: {
                    if checkingServer {
                        ProgressView().controlSize(.small)
                    } else {
                        Label("서버 확인", systemImage: "antenna.radiowaves.left.and.right")
                    }
                }
                .disabled(checkingServer)
                Button {
                    (NSApp.delegate as? AppDelegate)?.openMainWindow()
                } label: {
                    Label("메인 창 열기", systemImage: "macwindow")
                }
                Spacer()
            }
        }
        .padding(12)
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    // MARK: - 통계 탭 (할당량 포함)

    private var statsTab: some View {
        VStack(alignment: .leading, spacing: 10) {
            Group {
                Text("로그 통계")
                    .font(.subheadline.bold())
                statsRow("총 로그", logger.entries.count, .primary)
                statsRow("에러 (ERROR)", logger.count(level: "ERROR"), .red)
                statsRow("경고 (WARN)", logger.count(level: "WARN"), .orange)
                statsRow("정보 (INFO)", logger.count(level: "INFO"), .blue)
            }
            Divider()
            Group {
                HStack {
                    Text("OpenRouter 할당량")
                        .font(.subheadline.bold())
                    Spacer()
                    Button {
                        Task { await checkQuota() }
                    } label: {
                        if checkingQuota {
                            ProgressView().controlSize(.small)
                        } else {
                            Text("확인")
                        }
                    }
                    .disabled(checkingQuota)
                }
                Text(quotaText)
                    .font(.caption.monospaced())
                    .textSelection(.enabled)
            }
            Divider()
            Group {
                Text("최근 기능 로그")
                    .font(.subheadline.bold())
                ScrollView {
                    VStack(alignment: .leading, spacing: 4) {
                        ForEach(logger.recentFeatures(limit: 6)) { entry in
                            Text("\(logger.formattedTime(entry.time)) \(entry.message)")
                                .font(.system(.caption, design: .monospaced))
                                .frame(maxWidth: .infinity, alignment: .leading)
                        }
                    }
                }
            }
            Divider()
            Group {
                Text("캐시 관리")
                    .font(.subheadline.bold())
                HStack {
                    Button {
                        ImageCacheManager.shared.clearCache()
                        DebugLogger.shared.feature("디버그패널", "이미지 캐시 지워짐 (메모리+디스크)")
                        flashCopy("캐시 삭제 완료")
                    } label: {
                        Label("이미지 캐시 지우기", systemImage: "photo.badge.xmark")
                    }
                    Spacer()
                }
            }
            Spacer()
        }
        .padding(12)
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    // MARK: - 액션

    /// 전체 복사 — 필터 적용된 로그 전부 (time level tag message)
    private func copyFullLog() {
        let text = filteredEntries.map { entryText($0) }.joined(separator: "\n")
        copyToPasteboard(text)
        flashCopy("전체 로그 \(filteredEntries.count)줄 복사됨")
        DebugLogger.shared.feature("디버그패널", "전체 로그 복사됨 (\(filteredEntries.count)줄)")
    }

    /// 선택한 줄 복사 (List 선택)
    private func copySelectedLog() {
        guard let entry = selectedEntry else { return }
        copyLine(entry)
    }

    /// 한 줄 복사
    private func copyLine(_ entry: DebugLogger.LogEntry) {
        copyToPasteboard(entryText(entry))
        flashCopy("한 줄 복사됨")
    }

    /// 로그 전체를 파일로 저장 (NSSavePanel)
    private func saveLogFile() {
        let text = filteredEntries.map { entryText($0) }.joined(separator: "\n")
        saveTextToFile(text, suggestedName: "talkmance-debug-\(timestamp()).log")
    }

    private func saveLineFile(_ entry: DebugLogger.LogEntry) {
        saveTextToFile(entryText(entry), suggestedName: "talkmance-log-\(timestamp()).txt")
    }

    private func saveTextToFile(_ text: String, suggestedName: String) {
        let panel = NSSavePanel()
        panel.nameFieldStringValue = suggestedName
        panel.canCreateDirectories = true
        guard panel.runModal() == .OK, let url = panel.url else { return }
        do {
            try text.write(to: url, atomically: true, encoding: .utf8)
            flashCopy("저장됨: \(url.lastPathComponent)")
            DebugLogger.shared.feature("디버그패널", "로그 저장됨: \(url.path)")
        } catch {
            DebugLogger.shared.error("디버그패널", "로그 저장 실패: \(error.localizedDescription)")
        }
    }

    private func copyToPasteboard(_ text: String) {
        let pb = NSPasteboard.general
        pb.clearContents()
        pb.setString(text, forType: .string)
    }

    private func flashCopy(_ message: String) {
        withAnimation { copyNotice = message }
        DispatchQueue.main.asyncAfter(deadline: .now() + 2) {
            withAnimation { copyNotice = "" }
        }
    }

    private func entryText(_ entry: DebugLogger.LogEntry) -> String {
        "[\(logger.formattedTime(entry.time))] [\(entry.level)] [\(entry.tag)] \(entry.message)"
    }

    private func timestamp() -> String {
        let f = DateFormatter()
        f.dateFormat = "yyyyMMdd-HHmmss"
        return f.string(from: Date())
    }

    /// 서버 연결 확인 — 인증 + 모델 목록 로드로 응답 검증
    private func checkServer() async {
        checkingServer = true
        defer { checkingServer = false }
        do {
            let models = try await TalkmanceAPI.shared.listModels()
            catalogCount = models.catalog.count
            serverStatus = "연결됨 (\(models.catalog.count)개 모델)"
            DebugLogger.shared.feature("디버그패널", "서버 확인 성공: 모델 \(models.catalog.count)개")
        } catch {
            serverStatus = "연결 실패: \(error.localizedDescription)"
            DebugLogger.shared.error("디버그패널", "서버 확인 실패: \(error.localizedDescription)")
        }
    }

    /// OpenRouter 할당량 확인
    private func checkQuota() async {
        checkingQuota = true
        defer { checkingQuota = false }
        do {
            let quota = try await TalkmanceAPI.shared.fetchQuota()
            let d = quota.data
            let limitText = d.limit.map { String(format: "$%.4f", $0) } ?? "무제한"
            let rateText = d.rateLimit.map { "\($0.requests)req/\($0.interval)" } ?? "-"
            quotaText = "\(d.label) | 사용 \(String(format: "$%.4f", d.usage)) / \(limitText) | \(d.isFreeTier ? "무료 티어" : "유료") | 제한 \(rateText)"
            DebugLogger.shared.feature("디버그패널", "할당량 확인: \(d.label) usage=\(d.usage)")
        } catch {
            quotaText = "조회 실패: \(error.localizedDescription)"
            DebugLogger.shared.error("디버그패널", "할당량 조회 실패: \(error.localizedDescription)")
        }
    }

    // MARK: - 헬퍼

    private var serverTargetBinding: Binding<ServerTarget> {
        Binding(
            get: { ServerConfig.shared.target },
            set: { ServerConfig.shared.target = $0 }
        )
    }

    private func infoRow(_ label: String, _ value: String) -> some View {
        HStack(alignment: .top) {
            Text(label)
                .font(.caption)
                .foregroundStyle(.secondary)
                .frame(width: 90, alignment: .leading)
            Text(value)
                .font(.caption.monospaced())
                .textSelection(.enabled)
            Spacer()
        }
    }

    private func statsRow(_ label: String, _ value: Int, _ color: Color) -> some View {
        HStack {
            Text(label).font(.caption)
            Spacer()
            Text("\(value)")
                .font(.caption.bold().monospaced())
                .foregroundStyle(color)
        }
    }

    private func levelColor(_ level: String) -> Color {
        switch level {
        case "ERROR": return .red
        case "WARN": return .orange
        case "INFO": return .primary
        default: return .blue
        }
    }
}