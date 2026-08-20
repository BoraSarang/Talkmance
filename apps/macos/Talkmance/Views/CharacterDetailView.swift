import SwiftUI

/// 캐릭터 상세 — 대화방 목록 + 새 대화 시작 (T-33/34)
struct CharacterDetailView: View {
    @State private var character: Character
    @EnvironmentObject private var appState: AppState
    @Environment(\.dismiss) private var dismiss
    @State private var sessions: [ChatSession] = []
    @State private var messages: [Message] = []
    @State private var loading = false
    @State private var errorMessage: String?
    @State private var showNewChatSheet = false
    @State private var showEditSheet = false
    @State private var regenerating = false
    @State private var showDeleteConfirm = false
    @State private var deleting = false
    @State private var activeSession: ChatSession?
    @State private var showChat = false
    @State private var memories: [Memory] = []
    @State private var memorySheet: MemorySheetTarget?

    init(character: Character) {
        self.character = character
    }

    var body: some View {
        VStack(spacing: 0) {
            header

            Divider()

            ScrollView {
                VStack(alignment: .leading, spacing: 0) {
                    profileSection
                    Divider()
                        .padding(.vertical, 10)
                    sessionsSection
                    Divider()
                        .padding(.vertical, 10)
                    memoriesSection
                }
            }
        }
        .frame(minWidth: 520, minHeight: 480)
        .task { await load() }
        .sheet(isPresented: $showNewChatSheet) {
            NewChatSheet(character: character) {
                await load()
            }
        }
        .sheet(isPresented: $showEditSheet) {
            CharacterEditSheet(character: character) { updated in
                character = updated
                DebugLogger.shared.feature("캐릭터상세", "수정 반영됨: \(updated.name)")
            }
        }
        .sheet(item: $memorySheet) { target in
            MemoryEditSheet(characterID: character.id, target: target) {
                memorySheet = nil
                Task { await loadMemories() }
            }
        }
        .navigationDestination(isPresented: $showChat) {
            if let session = activeSession {
                ChatView(session: session, character: character)
            }
        }
        .onAppear {
            DebugLogger.shared.feature("캐릭터상세", "표시됨: \(character.name)")
        }
        .confirmationDialog(
            "「\(character.name)」을(를) 삭제할까요?",
            isPresented: $showDeleteConfirm,
            titleVisibility: .visible
        ) {
            Button("모든 대화·내용 삭제", role: .destructive) {
                Task { await deleteCharacter() }
            }
            Button("취소", role: .cancel) {}
        } message: {
            Text("이 캐릭터의 모든 대화방, 대화 내용, 기억이 영구 삭제됩니다. 되돌릴 수 없어요.")
        }
    }

    private func deleteCharacter() async {
        deleting = true
        do {
            try await TalkmanceAPI.shared.deleteCharacter(id: character.id)
            DebugLogger.shared.feature("캐릭터상세", "삭제 완료: \(character.name)")
            NotificationCenter.default.post(name: .init("charactersChanged"), object: nil)
            dismiss()
        } catch {
            errorMessage = (error as? APIError)?.message ?? error.localizedDescription
            DebugLogger.shared.feature("캐릭터상세", "삭제 실패: \(errorMessage ?? "?")")
        }
        deleting = false
    }

    // MARK: 헤더
    private var header: some View {
        HStack(spacing: 12) {
            Button {
                dismiss()
            } label: {
                Label("뒤로", systemImage: "chevron.left")
            }
            .buttonStyle(.borderless)
            .help("캐릭터 목록으로 돌아가기")

            CachedAvatarView(url: URL(string: character.avatarURL ?? ""), size: 48)

            VStack(alignment: .leading, spacing: 2) {
                Text(character.name).font(.title3.bold())
                if !character.title.isEmpty {
                    Text(character.title).font(.caption).foregroundStyle(.secondary)
                }
            }
            Spacer()
            Button {
                showEditSheet = true
            } label: {
                Label("수정", systemImage: "pencil")
            }
            Button {
                showDeleteConfirm = true
            } label: {
                Label("삭제", systemImage: "trash")
            }
            .foregroundStyle(.red)
            .disabled(deleting)
            Button {
                showNewChatSheet = true
            } label: {
                Label("새 대화", systemImage: "bubble.left.and.bubble.right.fill")
            }
            .buttonStyle(.borderedProminent)
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 10)
    }

    // MARK: 소개 섹션 (T-44)
    private var profileSection: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(alignment: .top, spacing: 14) {
                CachedAvatarView(url: URL(string: character.avatarURL ?? ""), size: 84, shape: .rounded(10))

                VStack(alignment: .leading, spacing: 6) {
                    Text(character.name)
                        .font(.title2.bold())
                    if !character.title.isEmpty {
                        Text(character.title).font(.subheadline).foregroundStyle(.secondary)
                    }
                    infoChips
                }
                Spacer()

                if regenerating {
                    HStack(spacing: 6) {
                        ProgressView().controlSize(.small)
                        Text("아바타 그리는 중…")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                } else {
                    Menu {
                        Button("DiceBear 아바타") {
                            Task { await regenerateAvatar(style: "dicebear") }
                        }
                        Button("AI 아바타") {
                            Task { await regenerateAvatar(style: "ai") }
                        }
                    } label: {
                        Label("아바타 다시 그리기", systemImage: "arrow.triangle.2.circlepath")
                    }
                    .menuStyle(.borderlessButton)
                    .controlSize(.small)
                    .help("아바타를 다시 생성합니다")
                }
            }

            if personaFields.isEmpty {
                Text("아직 소개가 없어요 — [수정]에서 추가해 보세요")
                    .font(.callout)
                    .foregroundStyle(.secondary)
            } else {
                ForEach(personaFields, id: \.label) { field in
                    profileRow(field)
                }
            }
        }
        .padding(.horizontal, 16)
        .padding(.top, 14)
    }

    private var infoChips: some View {
        var chips: [String] = []
        let gender = pstr("성별")
        if !gender.isEmpty { chips.append(gender) }
        if let age = character.age { chips.append("\(age)세") }
        let relation = pstr("관계")
        if !relation.isEmpty { chips.append("관계: \(relation)") }
        if chips.isEmpty { chips.append("기본 정보 없음") }
        return HStack(spacing: 6) {
            ForEach(chips, id: \.self) { chip in
                Text(chip)
                    .font(.caption)
                    .padding(.horizontal, 8)
                    .padding(.vertical, 3)
                    .background(Color.accentColor.opacity(0.12))
                    .clipShape(Capsule())
            }
        }
    }

    // MARK: 소개 항목 — persona 키 → 표시 라벨
    private var personaFields: [PersonaField] {
        var fields: [PersonaField] = []
        let tags: [(String, String)] = [
            ("성격", "성격"), ("취미", "취미"),
            ("말투", "말투"), ("스토리", "스토리"),
            ("시작전대화", "시작 전 대화"), ("배경", "배경"),
        ]
        for (key, label) in tags {
            if let arr = parr(key), !arr.isEmpty {
                fields.append(.tags(label: label, items: arr))
            } else {
                let s = pstr(key)
                if !s.isEmpty {
                    fields.append(.text(label: label, value: s))
                }
            }
        }
        return fields
    }

    @ViewBuilder
    private func profileRow(_ field: PersonaField) -> some View {
        switch field {
        case .text(let label, let value):
            VStack(alignment: .leading, spacing: 3) {
                Text(label).font(.caption).foregroundStyle(.secondary)
                Text(value).font(.body)
            }
        case .tags(let label, let items):
            VStack(alignment: .leading, spacing: 3) {
                Text(label).font(.caption).foregroundStyle(.secondary)
                HStack(spacing: 6) {
                    ForEach(items, id: \.self) { item in
                        Text(item)
                            .font(.caption)
                            .padding(.horizontal, 8)
                            .padding(.vertical, 3)
                            .background(Color.accentColor.opacity(0.12))
                            .clipShape(Capsule())
                    }
                }
            }
        }
    }

    // MARK: 대화방 섹션
    @ViewBuilder
    private var sessionsSection: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("대화방")
                .font(.headline)

            if let errorMessage {
                VStack(alignment: .leading, spacing: 6) {
                    Label("불러오기 실패", systemImage: "exclamationmark.triangle")
                        .foregroundStyle(.red)
                    Text(errorMessage).font(.caption).foregroundStyle(.secondary)
                }
                .padding(.vertical, 8)
            } else if sessions.isEmpty && !loading {
                Text("새 대화를 시작해 보세요")
                    .font(.callout)
                    .foregroundStyle(.secondary)
                    .padding(.vertical, 8)
            } else {
                VStack(spacing: 6) {
                    ForEach(sessions) { session in
                        Button {
                            activeSession = session
                            showChat = true
                            DebugLogger.shared.feature("세션", "대화방 열기: \(session.id.prefix(8))")
                        } label: {
                            HStack(alignment: .top) {
                                VStack(alignment: .leading, spacing: 2) {
                                    Text(session.lastMessage?.isEmpty == false ? session.lastMessage! : "아직 대화 없음")
                                        .font(.body)
                                        .lineLimit(1)
                                        .foregroundStyle(session.lastMessage == nil ? .secondary : .primary)
                                    Text(shortDate(session.lastMessageAt ?? session.updatedAt))
                                        .font(.caption)
                                        .foregroundStyle(.secondary)
                                }
                                Spacer()
                                Text(session.modelID)
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                                Button {
                                    confirmDeleteSession(session)
                                } label: {
                                    Image(systemName: "trash")
                                        .font(.caption)
                                        .foregroundStyle(.secondary)
                                }
                                .buttonStyle(.plain)
                                .help("대화 삭제")
                            }
                            .padding(8)
                            .background(Color.gray.opacity(0.08))
                            .clipShape(RoundedRectangle(cornerRadius: 8))
                        }
                        .buttonStyle(.plain)
                        .contextMenu {
                            Button("대화 삭제", systemImage: "trash", role: .destructive) {
                                confirmDeleteSession(session)
                            }
                        }
                    }
                }
            }
        }
        .padding(.horizontal, 16)
        .padding(.bottom, 16)
    }

    // MARK: 기억 섹션 (T-93)
    private var memoriesSection: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text("기억 카드")
                    .font(.headline)
                Spacer()
                Button {
                    memorySheet = .new
                } label: {
                    Label("기억 추가", systemImage: "plus")
                }
                .controlSize(.small)
            }

            if memories.isEmpty {
                Text("저장된 기억이 없습니다. 대화 중 기억이 자동으로 쌓이고 여기서 직접 추가할 수 있어요.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .padding(.vertical, 4)
            } else {
                VStack(spacing: 6) {
                    ForEach(memories) { memory in
                        HStack(alignment: .top, spacing: 8) {
                            Text(memory.pinned ? "📌" : "·")
                                .font(.caption)
                            VStack(alignment: .leading, spacing: 2) {
                                Text(memory.content)
                                    .font(.caption)
                                    .textSelection(.enabled)
                                Text("\(memory.memType == "long" ? "장기" : "단기") · 중요도 \(String(format: "%.1f", memory.importance))")
                                    .font(.caption2)
                                    .foregroundStyle(.secondary)
                            }
                            Spacer()
                            Button {
                                Task { await togglePin(memory) }
                            } label: {
                                Image(systemName: memory.pinned ? "pin.fill" : "pin")
                            }
                            .buttonStyle(.borderless)
                            .controlSize(.small)
                            .help(memory.pinned ? "고정 해제" : "고정")
                            Button {
                                memorySheet = .edit(memory)
                            } label: {
                                Image(systemName: "pencil")
                            }
                            .buttonStyle(.borderless)
                            .controlSize(.small)
                            Button {
                                Task { await deleteMemory(memory) }
                            } label: {
                                Image(systemName: "trash")
                            }
                            .buttonStyle(.borderless)
                            .controlSize(.small)
                            .foregroundStyle(.red)
                        }
                        .padding(8)
                        .background(Color.gray.opacity(0.08))
                        .clipShape(RoundedRectangle(cornerRadius: 8))
                    }
                }
            }
        }
        .padding(.horizontal, 16)
        .padding(.bottom, 16)
    }

    // MARK: 기억 동작
    private func loadMemories() async {
        do {
            memories = try await TalkmanceAPI.shared.listMemories(characterID: character.id)
            DebugLogger.shared.feature("캐릭터상세", "기억 로드 (\(memories.count)개)")
        } catch {
            DebugLogger.shared.error("캐릭터상세", "기억 로드 실패: \((error as? APIError)?.message ?? error.localizedDescription)")
        }
    }

    private func togglePin(_ memory: Memory) async {
        do {
            try await TalkmanceAPI.shared.updateMemory(id: memory.id, content: memory.content, memType: memory.memType, pinned: !memory.pinned)
            DebugLogger.shared.feature("기억", "고정 \(memory.pinned ? "해제" : "설정") (id=\(memory.id.prefix(8)))")
            await loadMemories()
        } catch {
            DebugLogger.shared.error("기억", "고정 변경 실패: \((error as? APIError)?.message ?? error.localizedDescription)")
        }
    }

    private func deleteMemory(_ memory: Memory) async {
        do {
            try await TalkmanceAPI.shared.deleteMemory(id: memory.id)
            DebugLogger.shared.feature("기억", "삭제됨 (id=\(memory.id.prefix(8)))")
            await loadMemories()
        } catch {
            DebugLogger.shared.error("기억", "삭제 실패: \((error as? APIError)?.message ?? error.localizedDescription)")
        }
    }

    private func confirmDeleteSession(_ session: ChatSession) {
        let alert = NSAlert()
        alert.messageText = "대화 삭제"
        alert.informativeText = "이 대화방과 모든 메시지를 삭제할까요? 되돌릴 수 없습니다."
        alert.alertStyle = .warning
        alert.addButton(withTitle: "삭제")
        alert.addButton(withTitle: "취소")
        DebugLogger.shared.feature("세션", "삭제 확인 다이얼로그 표시됨 (id=\(session.id.prefix(8)))")
        guard alert.runModal() == .alertFirstButtonReturn else { return }
        Task { await deleteSession(session) }
    }

    private func deleteSession(_ session: ChatSession) async {
        do {
            try await TalkmanceAPI.shared.deleteSession(id: session.id)
            sessions.removeAll { $0.id == session.id }
            DebugLogger.shared.feature("세션", "삭제됨 (id=\(session.id.prefix(8)))")
        } catch {
            DebugLogger.shared.error("세션", "삭제 실패: \((error as? APIError)?.message ?? error.localizedDescription)")
        }
    }

    // MARK: 동작
    private func regenerateAvatar(style: String) async {
        regenerating = true
        defer { regenerating = false }
        do {
            let url = try await TalkmanceAPI.shared.regenerateAvatar(characterID: character.id, style: style)
            var updated = character
            updated = Character(
                id: character.id, userID: character.userID, name: character.name, title: character.title,
                avatarURL: url, category: character.category, persona: character.persona,
                greeting: character.greeting, age: character.age, adult: character.adult
            )
            character = updated
            DebugLogger.shared.feature("캐릭터상세", "아바타 재생성 완료 (style=\(style))")
        } catch {
            DebugLogger.shared.error("캐릭터상세", "아바타 재생성 실패: \((error as? APIError)?.message ?? error.localizedDescription)")
        }
    }

    // MARK: persona 헬퍼
    private func pstr(_ key: String) -> String {
        (character.persona?[key]?.value as? String) ?? ""
    }

    private func parr(_ key: String) -> [String]? {
        guard let arr = character.persona?[key]?.value as? [Any] else { return nil }
        return arr.compactMap { $0 as? String }
    }

    private func load() async {
        loading = true
        errorMessage = nil
        do {
            sessions = try await TalkmanceAPI.shared.listSessions()
                .filter { $0.characterID == character.id }
            DebugLogger.shared.feature("세션", "목록 로드 (\(sessions.count)개)")
        } catch {
            errorMessage = (error as? APIError)?.message ?? error.localizedDescription
        }
        loading = false
        await loadMemories()
    }

    private func shortDate(_ iso: String) -> String {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        guard let date = formatter.date(from: iso) ?? ISO8601DateFormatter().date(from: iso) else { return "" }
        let fmt = DateFormatter()
        fmt.locale = Locale(identifier: "ko_KR")
        fmt.timeZone = TimeZone(identifier: "Asia/Seoul")
        fmt.dateFormat = "yyyy. M. d. a h:mm"
        return fmt.string(from: date)
    }
}

// MARK: - 소개 항목
private enum PersonaField {
    case text(label: String, value: String)
    case tags(label: String, items: [String])

    var label: String {
        switch self {
        case .text(let l, _): return l
        case .tags(let l, _): return l
        }
    }
}

// MARK: - 캐릭터 수정 시트 (T-44)

struct CharacterEditSheet: View {
    let character: Character
    var onSaved: (Character) -> Void
    @Environment(\.dismiss) private var dismiss

    @State private var name = ""
    @State private var title = ""
    @State private var age = ""
    @State private var gender = ""
    @State private var relationship = ""
    @State private var personality = ""
    @State private var tone = ""
    @State private var hobby = ""
    @State private var story = ""
    @State private var backstory = ""
    @State private var greeting = ""
    @State private var category = ""
    @State private var adult = false
    @State private var saving = false
    @State private var errorMessage: String?

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("캐릭터 수정")
                .font(.title3.bold())

            ScrollView {
                VStack(alignment: .leading, spacing: 10) {
                    TextField("이름 (필수)", text: $name)
                        .textFieldStyle(.roundedBorder)
                    TextField("타이틀", text: $title)
                        .textFieldStyle(.roundedBorder)

                    HStack(spacing: 10) {
                        TextField("성별 (여/남)", text: $gender)
                            .textFieldStyle(.roundedBorder)
                        TextField("나이 (19세 이상)", text: $age)
                            .textFieldStyle(.roundedBorder)
                        TextField("관계 (예: 연인)", text: $relationship)
                            .textFieldStyle(.roundedBorder)
                    }

                    TextField("성격 (쉼표 구분)", text: $personality)
                        .textFieldStyle(.roundedBorder)
                    TextField("말투", text: $tone)
                        .textFieldStyle(.roundedBorder)
                    TextField("취미 (쉼표 구분)", text: $hobby)
                        .textFieldStyle(.roundedBorder)
                    TextField("스토리 (2~3줄)", text: $story, axis: .vertical)
                        .lineLimit(2...4)
                        .textFieldStyle(.roundedBorder)
                    TextField("시작 전 대화 (과거 관계)", text: $backstory, axis: .vertical)
                        .lineLimit(2...4)
                        .textFieldStyle(.roundedBorder)
                    TextField("인사말", text: $greeting)
                        .textFieldStyle(.roundedBorder)

                    Picker("카테고리", selection: $category) {
                        Text("일반").tag("일반")
                        Text("연인").tag("연인")
                        Text("친구").tag("친구")
                        Text("가족").tag("가족")
                        Text("기타").tag("기타")
                    }
                    .pickerStyle(.menu)

                    Toggle("성인 캐릭터 (성인 대화 허용)", isOn: $adult)

                    if let errorMessage {
                        Text(errorMessage).font(.caption).foregroundStyle(.red)
                    }
                }
            }

            HStack {
                Spacer()
                Button("취소") { dismiss() }
                Button {
                    Task { await save() }
                } label: {
                    if saving {
                        ProgressView().controlSize(.small)
                    } else {
                        Text("저장")
                    }
                }
                .buttonStyle(.borderedProminent)
                .disabled(name.trimmingCharacters(in: .whitespaces).isEmpty || saving)
            }
        }
        .padding(20)
        .frame(width: 460, height: 520)
        .onAppear {
            populate()
            DebugLogger.shared.feature("캐릭터수정", "수정 시트 표시됨: \(character.name)")
        }
    }

    private func populate() {
        name = character.name
        title = character.title
        age = character.age.map(String.init) ?? ""
        gender = pstr("성별")
        relationship = pstr("관계")
        personality = parr("성격")?.joined(separator: ", ") ?? ""
        tone = pstr("말투")
        hobby = parr("취미")?.joined(separator: ", ") ?? ""
        story = pstr("스토리")
        backstory = pstr("시작전대화")
        greeting = character.greeting
        category = character.category.isEmpty ? "기타" : character.category
        adult = character.adult
    }

    private func pstr(_ key: String) -> String {
        (character.persona?[key]?.value as? String) ?? ""
    }

    private func parr(_ key: String) -> [String]? {
        guard let arr = character.persona?[key]?.value as? [Any] else { return nil }
        return arr.compactMap { $0 as? String }
    }

    private func save() async {
        saving = true
        errorMessage = nil
        do {
            var persona: [String: Any] = [:]
            if let base = character.persona {
                for (k, v) in base { persona[k] = v.value }
            }
            if !gender.isEmpty { persona["성별"] = gender }
            if !relationship.isEmpty { persona["관계"] = relationship }
            if !personality.isEmpty {
                persona["성격"] = personality.split(separator: ",").map { $0.trimmingCharacters(in: .whitespaces) }
            } else {
                persona.removeValue(forKey: "성격")
            }
            if !tone.isEmpty { persona["말투"] = tone } else { persona.removeValue(forKey: "말투") }
            if !hobby.isEmpty {
                persona["취미"] = hobby.split(separator: ",").map { $0.trimmingCharacters(in: .whitespaces) }
            } else {
                persona.removeValue(forKey: "취미")
            }
            if !story.isEmpty { persona["스토리"] = story } else { persona.removeValue(forKey: "스토리") }
            if !backstory.isEmpty { persona["시작전대화"] = backstory } else { persona.removeValue(forKey: "시작전대화") }

            let updated = try await TalkmanceAPI.shared.updateCharacter(
                id: character.id,
                name: name.trimmingCharacters(in: .whitespaces),
                title: title,
                age: Int(age),
                greeting: greeting,
                persona: persona,
                category: category,
                adult: adult,
                avatarURL: character.avatarURL
            )
            DebugLogger.shared.feature("캐릭터수정", "저장 완료: \(name)")
            dismiss()
            onSaved(updated)
        } catch {
            errorMessage = (error as? APIError)?.message ?? error.localizedDescription
            DebugLogger.shared.feature("캐릭터수정", "저장 실패: \(errorMessage ?? "?")")
        }
        saving = false
    }
}

// MARK: - 새 대화 시트 (모델 선택 — T-33)

struct NewChatSheet: View {
    let character: Character
    var onCreated: () async -> Void
    @Environment(\.dismiss) private var dismiss

    @State private var models: ModelsResponse?
    @State private var selectedModel = "gemini-3-flash-preview"
    @State private var rules: [PromptRule] = []
    @State private var selectedRuleID: String?
    @State private var quota: QuotaResponse?
    @State private var loading = false
    @State private var errorMessage: String?

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("\(character.name)과(와) 새 대화")
                .font(.title3.bold())

            if let models {
                Picker("모델", selection: $selectedModel) {
                    Section("추천") {
                        Text("gemini-2.5-flash (기본)").tag("gemini-3-flash-preview")
                        Text("nemotron-3-super-120b-a12b:free").tag("nemotron-3-super-120b-a12b:free")
                    }
                    Section("무료 모델") {
                        ForEach(models.catalog.filter(\.isFree), id: \.id) { m in
                            Text("\(m.name)").tag(m.id)
                        }
                    }
                    Section("커스텀 모델") {
                        ForEach(models.custom.filter(\.enabled), id: \.id) { m in
                            Text("\(m.name)").tag(m.modelID)
                        }
                    }
                }
                .labelsHidden()
                .frame(maxWidth: .infinity)
            } else if loading {
                ProgressView()
            }

            if !rules.isEmpty {
                Picker("대화 규칙", selection: $selectedRuleID) {
                    ForEach(rules) { rule in
                        Text(rule.isDefault ? "\(rule.name) (기본)" : rule.name).tag(Optional(rule.id))
                    }
                }
                .pickerStyle(.menu)
                .frame(maxWidth: .infinity)
            }

            if let quota {
                Text(newChatQuotaText(quota, modelID: selectedModel))
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }

            if let errorMessage {
                Text(errorMessage).font(.caption).foregroundStyle(.red)
            }

            HStack {
                Spacer()
                Button("취소") { dismiss() }
                Button("대화 시작") {
                    Task { await create() }
                }
                .buttonStyle(.borderedProminent)
            }
        }
        .padding(20)
        .frame(width: 380)
        .task {
            await loadModels()
            await loadRules()
            await loadQuota()
        }
        .onAppear {
            DebugLogger.shared.feature("새대화", "시트 표시됨 (캐릭터: \(character.name))")
        }
    }

    private func loadModels() async {
        loading = true
        errorMessage = nil
        do {
            models = try await TalkmanceAPI.shared.listModels()
            DebugLogger.shared.feature("모델", "카탈로그 로드 (free \(models?.catalog.filter(\.isFree).count ?? 0)개)")
        } catch {
            errorMessage = (error as? APIError)?.message ?? error.localizedDescription
        }
        loading = false
    }

    private func loadRules() async {
        do {
            rules = try await TalkmanceAPI.shared.listRules()
            selectedRuleID = rules.first(where: \.isDefault)?.id ?? rules.first?.id
            DebugLogger.shared.feature("새대화", "규칙 로드 (\(rules.count)개, 선택=\(selectedRuleID?.prefix(8) ?? "없음"))")
        } catch {
            DebugLogger.shared.error("새대화", "규칙 로드 실패: \(error.localizedDescription)")
        }
    }

    private func loadQuota() async {
        quota = await TalkmanceAPI.shared.fetchQuotaCached()
    }

    private func newChatQuotaText(_ q: QuotaResponse, modelID: String) -> String {
        if modelID.hasPrefix("gemini-") || modelID.hasPrefix("gemma-") {
            return "Gemini 무료 — 제한 없음"
        }
        let d = q.data
        if let limit = d.limit, limit > 0 {
            return String(format: "OpenRouter 잔여 할당량: $%.4f", max(limit - d.usage, 0))
        }
        if d.isFreeTier {
            return q.freeRemaining > 0 ? "오늘 무료 요청 \(q.freeRemaining)/\(q.freeLimitDaily)회 남음" : "오늘 무료 요청 소진 (내일 00:00 UTC 리셋)"
        }
        return "OpenRouter: \(d.label)"
    }

    private func create() async {
        do {
            let id = try await TalkmanceAPI.shared.createSession(characterID: character.id, modelID: selectedModel, ruleID: selectedRuleID)
            DebugLogger.shared.feature("새대화", "생성됨 (session=\(id.prefix(8)), rule=\(selectedRuleID?.prefix(8) ?? "기본"))")
            dismiss()
            await onCreated()
        } catch {
            errorMessage = (error as? APIError)?.message ?? error.localizedDescription
        }
    }
}

// MARK: - 채팅 뷰 (T-34) — 말풍선 + SSE 스트리밍

struct ChatView: View {
    let session: ChatSession
    let character: Character
    @EnvironmentObject private var appState: AppState
    @Environment(\.dismiss) private var dismiss

    @State private var messages: [ChatMessageItem] = []
    @State private var input = ""
    @State private var streaming = false
    @State private var errorMessage: String?
    @State private var scrollToBottomID: UUID?
    @State private var models: ModelsResponse?
    @State private var quota: QuotaResponse?
    @State private var currentModelID: String
    @State private var firstLoadDone = false

    init(session: ChatSession, character: Character) {
        self.session = session
        self.character = character
        _currentModelID = State(initialValue: session.modelID.isEmpty ? "gemini-3-flash-preview" : session.modelID)
    }

    var body: some View {
        VStack(spacing: 0) {
            header
            Divider()
            messageList
            Divider()
            inputBar
        }
        .frame(minWidth: 480, minHeight: 520)
        .task {
            await loadMessages()
            await loadModels()
            await loadQuota()
        }
        .onExitCommand { dismiss() }
        .onAppear {
            DebugLogger.shared.feature("채팅뷰", "표시됨 (session=\(session.id.prefix(8)))")
        }
    }

    private var header: some View {
        HStack(spacing: 10) {
            Button {
                dismiss()
            } label: {
                Label("뒤로", systemImage: "chevron.left")
            }
            .buttonStyle(.borderless)
            .help("캐릭터 목록으로 돌아가기")

            CachedAvatarView(url: URL(string: character.avatarURL ?? ""), size: 30)

            Text(character.name).font(.headline)

            if let models {
                Menu {
                    Section("추천") {
                        Button("gemini-2.5-flash (기본)") {
                            Task { await changeModel("gemini-3-flash-preview") }
                        }
                        Button("nemotron-3-super-120b-a12b:free") {
                            Task { await changeModel("nemotron-3-super-120b-a12b:free") }
                        }
                    }
                    Section("무료 모델") {
                        ForEach(models.catalog.filter(\.isFree), id: \.id) { m in
                            Button(m.name) {
                                Task { await changeModel(m.id) }
                            }
                        }
                    }
                    Section("커스텀 모델") {
                        ForEach(models.custom.filter(\.enabled), id: \.id) { m in
                            Button(m.name) {
                                Task { await changeModel(m.modelID) }
                            }
                        }
                    }
                } label: {
                    Text(currentModelID)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .padding(.horizontal, 8)
                        .padding(.vertical, 3)
                        .background(Color.gray.opacity(0.12))
                        .clipShape(Capsule())
                }
                .menuStyle(.borderlessButton)
            }

            Spacer()
            if let quota {
                Text(quotaBadgeText(quota, modelID: currentModelID))
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .padding(.horizontal, 8)
                    .padding(.vertical, 3)
                    .background(Color.gray.opacity(0.12))
                    .clipShape(Capsule())
                    .help("현재 모델의 남은 사용량")
            }
            if streaming {
                ProgressView().controlSize(.small)
            }
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
    }

    private var messageList: some View {
        ScrollViewReader { proxy in
            ScrollView {
                LazyVStack(spacing: 8) {
                    ForEach(messages) { item in
                        MessageBubble(item: item, avatarURL: character.avatarURL)
                            .id(item.id)
                    }
                }
                .padding(12)
            }
            .onChange(of: messages.count) {
                scrollToBottom(proxy)
            }
            .onChange(of: messages.last?.content ?? "") {
                scrollToBottom(proxy)
            }
        }
    }

    private func scrollToBottom(_ proxy: ScrollViewProxy) {
        if let last = messages.last?.id {
            withAnimation {
                proxy.scrollTo(last, anchor: .bottom)
            }
        }
    }

    private var inputBar: some View {
        HStack(spacing: 8) {
            TextField("메시지를 입력하세요… (Enter 전송, Shift+Enter 개행)", text: $input, axis: .vertical)
                .lineLimit(1...4)
                .textFieldStyle(.roundedBorder)
                .onKeyPress(phases: .down) { press in
                    if press.key == .return {
                        if press.modifiers.contains(.shift) || press.modifiers.contains(.option) {
                            if press.modifiers.contains(.command) {
                                return .ignored
                            }
                            insertNewline()
                            return .handled
                        }
                        send()
                        return .handled
                    }
                    return .ignored
                }

            Button {
                send()
            } label: {
                if streaming {
                    ProgressView().controlSize(.small)
                        .frame(width: 28, height: 28)
                } else {
                    Image(systemName: "arrow.up.circle.fill")
                        .font(.title2)
                }
            }
            .buttonStyle(.plain)
            .disabled(input.trimmingCharacters(in: .whitespaces).isEmpty || streaming)

            Button {
                Task { await retryLast() }
            } label: {
                Image(systemName: "arrow.clockwise")
                    .font(.title3)
            }
            .buttonStyle(.plain)
            .foregroundStyle(hasFailedMessage && !streaming ? Color.accentColor : Color.secondary)
            .disabled(!hasFailedMessage || streaming)
            .help("실패한 응답 다시 요청")
        }
        .padding(10)
    }

    private var hasFailedMessage: Bool {
        messages.contains { $0.failed }
    }

    private func loadMessages() async {
        do {
            let history = try await TalkmanceAPI.shared.listMessages(sessionID: session.id)
            messages = history.map { ChatMessageItem(message: $0) }
            DebugLogger.shared.feature("채팅", "히스토리 로드 (\(history.count)개)")
            await maybeAutoReply()
        } catch {
            errorMessage = (error as? APIError)?.message ?? error.localizedDescription
            DebugLogger.shared.feature("채팅", "히스토리 실패: \(errorMessage ?? "?")")
        }
    }

    /// 자동 발화: 빈 방(첫 시작) 또는 마지막 메시지가 사용자 것이면 AI가 먼저 말하기
    private func maybeAutoReply() async {
        guard !firstLoadDone else { return }
        firstLoadDone = true
        let last = messages.last
        let shouldReply = messages.isEmpty || last?.role == "user"
        guard shouldReply, !streaming else { return }
        DebugLogger.shared.feature("채팅", "자동 발화 (빈방=\(messages.isEmpty))")
        await sendAuto()
    }

    private func sendAuto() async {
        streaming = true
        let assistantItem = ChatMessageItem(role: "assistant", content: "")
        messages.append(assistantItem)
        do {
            var full = ""
            for try await chunk in TalkmanceAPI.shared.chatStream(sessionID: session.id, content: "", auto: true, polish: ServerConfig.shared.polishEnabled) {
                if let content = chunk.content {
                    full += content
                    assistantItem.updateContent(full)
                }
                if chunk.done {
                    DebugLogger.shared.feature("채팅", "자동 발화 완료 (model=\(chunk.model ?? currentModelID), tokens \(chunk.tokenIn)/\(chunk.tokenOut), cost $\(chunk.cost))")
                }
            }
            if full.isEmpty {
                assistantItem.updateContent("(응답 없음)")
            }
        } catch {
            let apiErr = error as? APIError
            errorMessage = apiErr?.message ?? error.localizedDescription
            assistantItem.markFailed("⚠️ \(errorMessage!)", detail: apiErr?.detail)
            if let apiErr {
                DebugLogger.shared.error("채팅", "자동 발화 실패 [\(apiErr.code)] \(apiErr.message) — 상세: \(apiErr.detail ?? "없음")")
            } else {
                DebugLogger.shared.error("채팅", "자동 발화 실패: \(error.localizedDescription)")
            }
        }
        streaming = false
    }

    private func loadModels() async {
        do {
            models = try await TalkmanceAPI.shared.listModels()
        } catch {
            DebugLogger.shared.feature("모델", "카탈로그 로드 실패: \((error as? APIError)?.message ?? "?")")
        }
    }

    private func loadQuota() async {
        quota = await TalkmanceAPI.shared.fetchQuotaCached()
        DebugLogger.shared.feature("채팅뷰", "할당량 표시 \(quota?.data.label ?? "없음")")
    }

    /// 현재 선택 모델 기준 남은 사용량 표시
    private func quotaBadgeText(_ q: QuotaResponse, modelID: String) -> String {
        if modelID.hasPrefix("gemini-") || modelID.hasPrefix("gemma-") {
            return "Gemini 무료 (제한 없음)"
        }
        let d = q.data
        if let limit = d.limit, limit > 0 {
            return String(format: "잔여 $%.4f", max(limit - d.usage, 0))
        }
        if d.isFreeTier && modelID.hasSuffix(":free") {
            return q.freeRemaining > 0 ? "무료 \(q.freeRemaining)/\(q.freeLimitDaily)회" : "무료 소진"
        }
        if d.isFreeTier {
            return "무료 요청 \(q.freeRemaining)/\(q.freeLimitDaily)회 남음"
        }
        return d.label
    }

    private func changeModel(_ modelID: String) async {
        guard modelID != currentModelID else { return }
        do {
            try await TalkmanceAPI.shared.updateSessionModel(id: session.id, modelID: modelID)
            currentModelID = modelID
            DebugLogger.shared.feature("채팅", "모델 변경 완료: \(modelID)")
        } catch {
            errorMessage = (error as? APIError)?.message ?? error.localizedDescription
            DebugLogger.shared.feature("채팅", "모델 변경 실패: \(errorMessage ?? "?")")
        }
    }

    /// Shift+Enter 개행 — SwiftUI TextField(axis:.vertical)는 Shift+Return을 전체 선택으로
    /// 처리하므로 키 이벤트를 가로채 커서 위치에 개행을 직접 삽입한다
    private func insertNewline() {
        guard let textView = NSApp.keyWindow?.firstResponder as? NSTextView else { return }
        textView.insertNewline(nil)
        input = textView.string
    }

    private func send() {
        let text = input.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty, !streaming else { return }
        input = ""
        streaming = true
        errorMessage = nil

        let userItem = ChatMessageItem(role: "user", content: text)
        let assistantItem = ChatMessageItem(role: "assistant", content: "")
        messages.append(userItem)
        messages.append(assistantItem)

        Task {
            do {
                var full = ""
                for try await chunk in TalkmanceAPI.shared.chatStream(sessionID: session.id, content: text, polish: ServerConfig.shared.polishEnabled) {
                    if let content = chunk.content {
                        full += content
                        assistantItem.updateContent(full)
                    }
                    if chunk.done {
                        DebugLogger.shared.feature("채팅", "완료 (model=\(chunk.model ?? currentModelID), tokens \(chunk.tokenIn)/\(chunk.tokenOut), cost $\(chunk.cost))")
                    }
                }
                if full.isEmpty {
                    assistantItem.updateContent("(응답 없음)")
                }
            } catch {
                let apiErr = error as? APIError
                errorMessage = apiErr?.message ?? error.localizedDescription
                assistantItem.markFailed("⚠️ \(errorMessage!)", detail: apiErr?.detail)
                if let apiErr {
                    DebugLogger.shared.error("채팅", "스트림 실패 [\(apiErr.code)] \(apiErr.message) — 상세: \(apiErr.detail ?? "없음")")
                } else {
                    DebugLogger.shared.error("채팅", "스트림 실패: \(error.localizedDescription)")
                }
            }
            streaming = false
        }
    }

    /// 실패한 응답 다시 요청 — 직전 user 메시지를 재사용 (서버 retry 플래그), user 메시지가 없으면 자동 발화 재시도
    private func retryLast() async {
        guard !streaming, let failedItem = messages.last(where: { $0.failed }) else { return }
        failedItem.reset()
        let hasUser = messages.contains { $0.role == "user" }
        streaming = true
        errorMessage = nil
        Task {
            do {
                var full = ""
                let stream = hasUser
                    ? TalkmanceAPI.shared.chatStream(sessionID: session.id, content: "", auto: false, retry: true, polish: ServerConfig.shared.polishEnabled)
                    : TalkmanceAPI.shared.chatStream(sessionID: session.id, content: "", auto: true, polish: ServerConfig.shared.polishEnabled)
                for try await chunk in stream {
                    if let content = chunk.content {
                        full += content
                        failedItem.updateContent(full)
                    }
                    if chunk.done {
                        DebugLogger.shared.feature("채팅", "재시도 완료 (model=\(chunk.model ?? currentModelID), tokens \(chunk.tokenIn)/\(chunk.tokenOut), cost $\(chunk.cost))")
                    }
                }
                if full.isEmpty {
                    failedItem.updateContent("(응답 없음)")
                }
            } catch {
                let apiErr = error as? APIError
                errorMessage = apiErr?.message ?? error.localizedDescription
                failedItem.markFailed("⚠️ \(errorMessage!)", detail: apiErr?.detail)
                if let apiErr {
                    DebugLogger.shared.error("채팅", "재시도 실패 [\(apiErr.code)] \(apiErr.message) — 상세: \(apiErr.detail ?? "없음")")
                } else {
                    DebugLogger.shared.error("채팅", "재시도 실패: \(error.localizedDescription)")
                }
            }
            streaming = false
        }
    }
}

// MARK: - 채팅 메시지 아이템 (참조 공유로 스트리밍 갱신)

@MainActor
final class ChatMessageItem: Identifiable, ObservableObject {
    let id = UUID()
    let role: String
    @Published private(set) var content: String
    @Published private(set) var failed = false
    @Published private(set) var detail: String?

    init(message: Message) {
        self.role = message.role
        self.content = message.content
    }

    init(role: String, content: String) {
        self.role = role
        self.content = content
    }

    func updateContent(_ newContent: String) {
        content = newContent
    }

    /// 실패 표시 (재시도 버튼 노출), detail: 서버 폴백 실패 사유
    func markFailed(_ message: String, detail: String? = nil) {
        content = message
        failed = true
        self.detail = detail
    }

    /// 재시도 준비 (내용 초기화)
    func reset() {
        content = ""
        failed = false
        detail = nil
    }
}

// MARK: - 말풍선

struct MessageBubble: View {
    @ObservedObject var item: ChatMessageItem
    var avatarURL: String?

    var isUser: Bool { item.role == "user" }

    var body: some View {
        HStack(alignment: .top, spacing: 6) {
            if isUser {
                Spacer(minLength: 60)
                bubbleText(item.content, isUser: isUser)
                    .padding(.horizontal, 12)
                    .padding(.vertical, 8)
                    .background(Color.accentColor)
                    .foregroundStyle(.white)
                    .clipShape(RoundedRectangle(cornerRadius: 14))
                    .textSelection(.enabled)
            } else {
                if let avatarURL {
                    CachedAvatarView(url: URL(string: avatarURL), size: 32)
                        .padding(.top, 8)
                }
                VStack(alignment: .leading, spacing: 4) {
                    bubbleText(item.content, isUser: isUser)
                        .padding(.horizontal, 12)
                        .padding(.vertical, 8)
                        .background(Color(nsColor: .controlBackgroundColor))
                        .foregroundStyle(.primary)
                        .clipShape(RoundedRectangle(cornerRadius: 14))
                        .textSelection(.enabled)
                    if item.failed {
                        Text("응답 실패 — 하단 다시 요청 버튼으로 재시도하세요")
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                            .padding(.horizontal, 12)
                        if let detail = item.detail {
                            Text(detail)
                                .font(.caption2)
                                .foregroundStyle(.tertiary)
                                .textSelection(.enabled)
                                .lineLimit(nil)
                                .padding(.horizontal, 12)
                        }
                    }
                }
                Spacer(minLength: 60)
            }
        }
    }

    /// [MEMORY_SAVE] → 📌, 괄호 (…) 생각 → 💭, *…* 행동 → 이탤릭 구분
    /// user 말풍선(파란 배경)에서는 흰색 계열로 표시
    private func bubbleText(_ content: String, isUser: Bool) -> Text {
        let styleColor: Color = isUser ? .white.opacity(0.85) : .secondary
        guard let regex = try? NSRegularExpression(pattern: #"\[MEMORY_SAVE\][^\n]*|\([^()]*\)|\*[^*\n]*\*"#) else {
            return Text(content)
        }
        let ns = content as NSString
        let matches = regex.matches(in: content, range: NSRange(location: 0, length: ns.length))
        guard !matches.isEmpty else { return Text(content) }

        var result = Text("")
        var last = 0
        for m in matches {
            if m.range.location > last {
                result = result + Text(ns.substring(with: NSRange(location: last, length: m.range.location - last)))
            }
            let raw = ns.substring(with: m.range)
            if raw.hasPrefix("[MEMORY_SAVE]") {
                let tagBody = raw
                    .replacingOccurrences(of: "[MEMORY_SAVE]", with: "")
                    .trimmingCharacters(in: .whitespaces)
                result = result + Text("📌 (\(tagBody))")
                    .italic()
                    .foregroundStyle(styleColor)
            } else if raw.hasPrefix("(") {
                let inner = raw.trimmingCharacters(in: CharacterSet(charactersIn: "() "))
                result = result + Text("💭 (\(inner))")
                    .italic()
                    .foregroundStyle(styleColor)
            } else {
                let inner = raw.trimmingCharacters(in: CharacterSet(charactersIn: "* "))
                result = result + Text(inner)
                    .italic()
                    .foregroundStyle(styleColor)
            }
            last = m.range.location + m.range.length
        }
        if last < ns.length {
            result = result + Text(ns.substring(from: last))
        }
        return result
    }
}
// MARK: - 기억 시트 대상

enum MemorySheetTarget: Identifiable {
    case new
    case edit(Memory)

    var id: String {
        switch self {
        case .new: return "new"
        case .edit(let m): return m.id
        }
    }
}

// MARK: - 기억 추가/편집 시트 (T-93)

struct MemoryEditSheet: View {
    let characterID: String
    let target: MemorySheetTarget
    var onDone: () -> Void
    @Environment(\.dismiss) private var dismiss

    @State private var content = ""
    @State private var memType = "long"
    @State private var saving = false
    @State private var errorMessage: String?

    private var isEdit: Bool {
        if case .edit = target { return true }
        return false
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text(isEdit ? "기억 편집" : "기억 추가")
                .font(.headline)

            TextField("기억 내용 (예: 좋아하는 음식은 초밥)", text: $content, axis: .vertical)
                .textFieldStyle(.roundedBorder)
                .lineLimit(3...6)

            Picker("유형", selection: $memType) {
                Text("장기 (중요한 기억)").tag("long")
                Text("단기 (최근 기억)").tag("short")
            }
            .pickerStyle(.segmented)

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
                .disabled(content.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty || saving)
            }
        }
        .padding(20)
        .frame(width: 420)
        .onAppear {
            DebugLogger.shared.feature("기억", "편집 시트 표시됨 (mode=\(isEdit ? "edit" : "new"))")
            if case .edit(let m) = target {
                content = m.content
                memType = m.memType
            }
        }
    }

    private func save() async {
        saving = true
        defer { saving = false }
        do {
            if case .edit(let m) = target {
                try await TalkmanceAPI.shared.updateMemory(id: m.id, content: content, memType: memType, pinned: m.pinned)
                DebugLogger.shared.feature("기억", "기억 수정됨 (\(content.prefix(20)))")
            } else {
                try await TalkmanceAPI.shared.createMemory(characterID: characterID, content: content, memType: memType)
                DebugLogger.shared.feature("기억", "기억 추가됨 (\(content.prefix(20)))")
            }
            dismiss()
            onDone()
        } catch {
            errorMessage = (error as? APIError)?.message ?? error.localizedDescription
        }
    }
}
