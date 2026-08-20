import SwiftUI

/// 설정 창 — 서버 선택, 대화 규칙, BYOK 키, 커스텀 모델, Dock 토글
struct SettingsView: View {
    @EnvironmentObject private var appState: AppState
    @StateObject private var config = ServerConfig.shared

    @State private var rules: [PromptRule] = []
    @State private var keys: [UserKey] = []
    @State private var customModels: [CustomModel] = []
    @State private var sheet: SettingsSheet?
    @State private var settingsError: String?

    var body: some View {
        Form {
            Section("서버") {
                Picker("서버 대상", selection: $config.target) {
                    ForEach(ServerTarget.allCases) { t in
                        Text(t.label).tag(t)
                    }
                }
                .pickerStyle(.menu)

                if config.target == .render {
                    TextField("Render URL (예: https://talkmance.onrender.com)", text: $config.renderBaseURL)
                        .textFieldStyle(.roundedBorder)
                } else if config.target == .custom {
                    TextField("서버 URL (예: http://192.168.0.10:8080)", text: $config.customBaseURL)
                        .textFieldStyle(.roundedBorder)
                }

                LabeledContent("현재 주소", value: config.baseURL)
                Text("변경하면 인증이 초기화되고 목록을 다시 불러옵니다")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }

            Section("대화 규칙") {
                Toggle("한국어 다듬기(B)", isOn: $config.polishEnabled)
                Text("AI 답변을 사람이 쓴 것처럼 자연스럽게 다듬습니다 (이모지·AI 말투·반복 제거). 끄면 원문 그대로 표시됩니다.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                if settingsError != nil {
                    Text(settingsError!)
                        .font(.caption)
                        .foregroundStyle(.red)
                }
                if rules.isEmpty {
                    Text("규칙이 없습니다 (기본 규칙은 서버에 자동 생성됩니다)")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                ForEach(rules) { rule in
                    HStack {
                        Text(rule.isDefault ? "\(rule.name) (기본)" : rule.name)
                        Spacer()
                        Button("편집") {
                            sheet = .rule(.edit(rule))
                        }
                        .controlSize(.small)
                        Button("삭제") {
                            Task { await deleteRule(rule) }
                        }
                        .controlSize(.small)
                        .disabled(rule.isDefault)
                        .help(rule.isDefault ? "기본 규칙은 삭제할 수 없습니다" : "")
                    }
                }
                Button("새 규칙") {
                    sheet = .rule(.new)
                }
            }

            Section("API 키 (BYOK)") {
                if keys.isEmpty {
                    Text("등록된 키가 없습니다. 키를 등록하면 해당 키로 모델을 호출합니다.")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                ForEach(keys) { key in
                    HStack {
                        Text(key.label)
                        Spacer()
                        Button("삭제") {
                            Task { await deleteKey(key) }
                        }
                        .controlSize(.small)
                    }
                }
                Button("키 등록") {
                    sheet = .key
                }
                Text("키는 서버에 AES-GCM으로 암호화 저장되며 다시 표시되지 않습니다")
                    .font(.caption2)
                    .foregroundStyle(.tertiary)
            }

            Section("커스텀 모델") {
                if customModels.isEmpty {
                    Text("커스텀 모델이 없습니다. 직접 API(모델 ID/Base URL)를 추가할 수 있습니다.")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                ForEach(customModels) { model in
                    HStack {
                        VStack(alignment: .leading, spacing: 1) {
                            Text(model.name)
                            Text(model.modelID)
                                .font(.caption2)
                                .foregroundStyle(.secondary)
                        }
                        Spacer()
                        Text(model.isFree ? "무료" : "유료")
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                        Button("삭제") {
                            Task { await deleteCustomModel(model) }
                        }
                        .controlSize(.small)
                    }
                }
                Button("모델 추가") {
                    sheet = .customModel
                }
            }

            Section("일반") {
                Toggle("Dock에 표시", isOn: $appState.showInDock)
                    .help("끄면 Dock 아이콘이 사라지고 메뉴바 아이콘으로만 사용합니다")
            }

            Section("앱 정보") {
                LabeledContent("버전", value: "1.0.0")
            }
        }
        .formStyle(.grouped)
        .frame(width: 460)
        .padding()
        .sheet(item: $sheet) { target in
            switch target {
            case .rule(let ruleTarget):
                RuleEditSheet(target: ruleTarget) {
                    sheet = nil
                    Task { await loadRules() }
                }
            case .key:
                KeyEditSheet {
                    sheet = nil
                    Task { await loadKeys() }
                }
            case .customModel:
                CustomModelEditSheet {
                    sheet = nil
                    Task { await loadCustomModels() }
                }
            }
        }
        .onAppear {
            DebugLogger.shared.feature("설정화면", "설정 창 표시됨 (target=\(config.target.rawValue), showInDock=\(appState.showInDock))")
            Task {
                await loadRules()
                await loadKeys()
                await loadCustomModels()
            }
        }
    }

    // MARK: - 로드

    private func loadRules() async {
        do {
            rules = try await TalkmanceAPI.shared.listRules()
            settingsError = nil
            DebugLogger.shared.feature("설정화면", "규칙 로드 (\(rules.count)개)")
        } catch {
            DebugLogger.shared.error("설정화면", "규칙 로드 실패: \(error.localizedDescription)")
        }
    }

    private func loadKeys() async {
        do {
            keys = try await TalkmanceAPI.shared.listKeys()
            DebugLogger.shared.feature("설정화면", "키 로드 (\(keys.count)개)")
        } catch {
            DebugLogger.shared.error("설정화면", "키 로드 실패: \(error.localizedDescription)")
        }
    }

    private func loadCustomModels() async {
        do {
            let models = try await TalkmanceAPI.shared.listModels()
            customModels = models.custom
            DebugLogger.shared.feature("설정화면", "커스텀 모델 로드 (\(customModels.count)개)")
        } catch {
            DebugLogger.shared.error("설정화면", "커스텀 모델 로드 실패: \(error.localizedDescription)")
        }
    }

    // MARK: - 삭제

    private func deleteRule(_ rule: PromptRule) async {
        do {
            try await TalkmanceAPI.shared.deleteRule(id: rule.id)
            rules.removeAll { $0.id == rule.id }
            DebugLogger.shared.feature("설정화면", "규칙 삭제됨 (\(rule.name))")
        } catch {
            settingsError = (error as? APIError)?.message ?? error.localizedDescription
            DebugLogger.shared.error("설정화면", "규칙 삭제 실패: \(error.localizedDescription)")
        }
    }

    private func deleteKey(_ key: UserKey) async {
        do {
            try await TalkmanceAPI.shared.deleteKey(id: key.id)
            keys.removeAll { $0.id == key.id }
            DebugLogger.shared.feature("설정화면", "키 삭제됨 (\(key.label))")
        } catch {
            settingsError = (error as? APIError)?.message ?? error.localizedDescription
        }
    }

    private func deleteCustomModel(_ model: CustomModel) async {
        do {
            try await TalkmanceAPI.shared.deleteCustomModel(id: model.id)
            customModels.removeAll { $0.id == model.id }
            DebugLogger.shared.feature("설정화면", "커스텀 모델 삭제됨 (\(model.name))")
        } catch {
            settingsError = (error as? APIError)?.message ?? error.localizedDescription
        }
    }
}

// MARK: - 설정 시트 대상 (하나의 sheet로 통합)

enum SettingsSheet: Identifiable {
    case rule(RuleSheetTarget)
    case key
    case customModel

    var id: String {
        switch self {
        case .rule(let target): return "rule-\(target.id)"
        case .key: return "key"
        case .customModel: return "customModel"
        }
    }
}

// MARK: - 규칙 편집 시트 대상

enum RuleSheetTarget: Identifiable {
    case new
    case edit(PromptRule)

    var id: String {
        switch self {
        case .new: return "new"
        case .edit(let rule): return rule.id
        }
    }
}

// MARK: - 규칙 생성/편집 시트

struct RuleEditSheet: View {
    let target: RuleSheetTarget
    var onDone: () -> Void
    @Environment(\.dismiss) private var dismiss

    @State private var name = ""
    @State private var systemPrompt = ""
    @State private var saving = false
    @State private var errorMessage: String?

    private var isEdit: Bool {
        if case .edit = target { return true }
        return false
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text(isEdit ? "규칙 편집" : "새 규칙")
                .font(.headline)

            TextField("규칙 이름 (예: 반말 톡)", text: $name)
                .textFieldStyle(.roundedBorder)

            Text("시스템 프롬프트 (대화 규칙)")
                .font(.caption)
                .foregroundStyle(.secondary)
            TextEditor(text: $systemPrompt)
                .font(.system(.caption, design: .monospaced))
                .frame(height: 180)
                .overlay(
                    RoundedRectangle(cornerRadius: 6)
                        .stroke(Color.gray.opacity(0.3))
                )

            if let errorMessage {
                Text(errorMessage).font(.caption).foregroundStyle(.red)
            }

            HStack {
                Spacer()
                Button("취소") { dismiss() }
                Button("저장") {
                    Task { await save() }
                }
                .buttonStyle(.borderedProminent)
                .disabled(name.isEmpty || systemPrompt.isEmpty || saving)
            }
        }
        .padding(20)
        .frame(width: 440, height: 340)
        .onAppear {
            DebugLogger.shared.feature("규칙", "편집 시트 표시됨 (mode=\(isEdit ? "edit" : "new"))")
            if case .edit(let rule) = target {
                name = rule.name
                systemPrompt = rule.systemPrompt
            }
        }
    }

    private func save() async {
        saving = true
        defer { saving = false }
        do {
            if case .edit(let rule) = target {
                try await TalkmanceAPI.shared.updateRule(id: rule.id, name: name, systemPrompt: systemPrompt)
                DebugLogger.shared.feature("규칙", "규칙 수정됨 (\(name))")
            } else {
                try await TalkmanceAPI.shared.createRule(name: name, systemPrompt: systemPrompt)
                DebugLogger.shared.feature("규칙", "규칙 생성됨 (\(name))")
            }
            dismiss()
            onDone()
        } catch {
            errorMessage = (error as? APIError)?.message ?? error.localizedDescription
        }
    }
}

// MARK: - BYOK 키 등록 시트

struct KeyEditSheet: View {
    var onDone: () -> Void
    @Environment(\.dismiss) private var dismiss

    @State private var label = ""
    @State private var apiKey = ""
    @State private var saving = false
    @State private var errorMessage: String?

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("API 키 등록 (BYOK)")
                .font(.headline)

            TextField("이름 (예: 내 OpenRouter 키)", text: $label)
                .textFieldStyle(.roundedBorder)
            SecureField("API 키 (sk-...)", text: $apiKey)
                .textFieldStyle(.roundedBorder)

            Text("키는 서버에 AES-GCM 암호화로 저장됩니다. 등록 후에는 키 값이 다시 표시되지 않습니다.")
                .font(.caption2)
                .foregroundStyle(.secondary)

            if let errorMessage {
                Text(errorMessage).font(.caption).foregroundStyle(.red)
            }

            HStack {
                Spacer()
                Button("취소") { dismiss() }
                Button("등록") {
                    Task { await save() }
                }
                .buttonStyle(.borderedProminent)
                .disabled(label.isEmpty || apiKey.isEmpty || saving)
            }
        }
        .padding(20)
        .frame(width: 420)
        .onAppear {
            DebugLogger.shared.feature("BYOK", "키 등록 시트 표시됨")
        }
    }

    private func save() async {
        saving = true
        defer { saving = false }
        do {
            try await TalkmanceAPI.shared.createKey(label: label, apiKey: apiKey)
            DebugLogger.shared.feature("BYOK", "키 등록됨 (\(label))")
            dismiss()
            onDone()
        } catch {
            errorMessage = (error as? APIError)?.message ?? error.localizedDescription
        }
    }
}

// MARK: - 커스텀 모델 추가 시트

struct CustomModelEditSheet: View {
    var onDone: () -> Void
    @Environment(\.dismiss) private var dismiss

    @State private var name = ""
    @State private var modelID = ""
    @State private var baseURL = ""
    @State private var apiKey = ""
    @State private var isFree = true
    @State private var desc = ""
    @State private var saving = false
    @State private var errorMessage: String?

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("커스텀 모델 추가")
                .font(.headline)

            TextField("이름 (예: 내 Gemini)", text: $name)
                .textFieldStyle(.roundedBorder)
            TextField("모델 ID (예: gemini-3-flash-preview)", text: $modelID)
                .textFieldStyle(.roundedBorder)
            TextField("Base URL (OpenAI 호환)", text: $baseURL)
                .textFieldStyle(.roundedBorder)
            SecureField("API 키 (선택 — 비우면 서버 키 사용)", text: $apiKey)
                .textFieldStyle(.roundedBorder)
            Toggle("무료 모델", isOn: $isFree)
            TextField("설명 (선택)", text: $desc)
                .textFieldStyle(.roundedBorder)

            if let errorMessage {
                Text(errorMessage).font(.caption).foregroundStyle(.red)
            }

            HStack {
                Spacer()
                Button("취소") { dismiss() }
                Button("추가") {
                    Task { await save() }
                }
                .buttonStyle(.borderedProminent)
                .disabled(name.isEmpty || modelID.isEmpty || saving)
            }
        }
        .padding(20)
        .frame(width: 440)
        .onAppear {
            DebugLogger.shared.feature("커스텀모델", "추가 시트 표시됨")
        }
    }

    private func save() async {
        saving = true
        defer { saving = false }
        do {
            try await TalkmanceAPI.shared.createCustomModel(
                name: name, modelID: modelID, baseURL: baseURL,
                apiKey: apiKey.isEmpty ? nil : apiKey, isFree: isFree, description: desc
            )
            DebugLogger.shared.feature("커스텀모델", "모델 추가됨 (\(name) / \(modelID))")
            dismiss()
            onDone()
        } catch {
            errorMessage = (error as? APIError)?.message ?? error.localizedDescription
        }
    }
}