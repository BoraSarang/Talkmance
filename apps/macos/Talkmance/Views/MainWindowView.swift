import SwiftUI

// MARK: - 이미지 캐시 (메모리 NSCache + 디스크 URLCache)

/// 아바타 이미지 캐시 로더 — 서버 캐시 헤더에 의존하지 않고 앱 메모리에 보관
final class ImageCacheManager: @unchecked Sendable {
    static let shared = ImageCacheManager()

    private let memory = NSCache<NSString, NSImage>()
    private let session: URLSession

    private init() {
        memory.countLimit = 200
        let config = URLSessionConfiguration.default
        config.urlCache = URLCache(memoryCapacity: 50_000_000, diskCapacity: 200_000_000)
        session = URLSession(configuration: config)
    }

    @MainActor
    func image(for url: URL) async -> NSImage? {
        let key = url.absoluteString as NSString
        if let hit = memory.object(forKey: key) {
            return hit
        }
        do {
            let (data, _) = try await session.data(from: url)
            guard let image = NSImage(data: data) else { return nil }
            memory.setObject(image, forKey: key)
            return image
        } catch {
            return nil
        }
    }

    /// 캐시 전체 지우기 (디버그 패널)
    func clearCache() {
        memory.removeAllObjects()
        session.configuration.urlCache?.removeAllCachedResponses()
    }
}

/// 캐시 기반 아바타 뷰 — 같은 URL은 메모리에서 즉시 표시
struct CachedAvatarView: View {
    enum AvatarShape {
        case circle
        case rounded(CGFloat)
    }

    let url: URL?
    var size: CGFloat = 44
    var shape: AvatarShape = .circle

    @State private var image: NSImage?

    var body: some View {
        Group {
            if let image {
                Image(nsImage: image)
                    .resizable()
                    .scaledToFill()
            } else {
                ZStack {
                    Rectangle()
                        .fill(Color.gray.opacity(0.2))
                        .overlay(
                            Image(systemName: "person.crop.circle.fill")
                                .resizable()
                                .scaledToFit()
                                .padding(size * 0.25)
                                .foregroundStyle(.tertiary)
                        )
                    ProgressView()
                        .controlSize(.small)
                }
            }
        }
        .frame(width: size, height: size)
        .clipShape(clip)
        .task(id: url) {
            guard let url else { return }
            image = await ImageCacheManager.shared.image(for: url)
        }
    }

    private var clip: some Shape {
        switch shape {
        case .circle: AnyShape(Circle())
        case .rounded(let radius): AnyShape(RoundedRectangle(cornerRadius: radius))
        }
    }
}

/// 메인 창 — 캐릭터 목록 + 대화 홈 (T-32)
struct MainWindowView: View {
    @EnvironmentObject private var appState: AppState
    @State private var characters: [Character] = []
    @State private var loading = false
    @State private var errorMessage: String?
    @State private var showCreateSheet = false
    @State private var selectedCharacter: Character?
    @State private var categoryFilter = "전체"

    var body: some View {
        NavigationStack {
            VStack(spacing: 0) {
                header
                Divider()
                content
            }
            .frame(minWidth: 520, minHeight: 480)
            .task { await load() }
            .onReceive(NotificationCenter.default.publisher(for: ServerConfig.serverChangedNotification)) { _ in
                Task { await load() }
            }
            .onReceive(NotificationCenter.default.publisher(for: Notification.Name("charactersChanged"))) { _ in
                Task { await load() }
            }
            .sheet(isPresented: $showCreateSheet) {
                CreateCharacterSheet { await load() }
            }
            .navigationDestination(item: $selectedCharacter) { character in
                CharacterDetailView(character: character)
            }
        }
        .onAppear {
            DebugLogger.shared.feature("홈화면", "메인 창 표시됨 (캐릭터 \(characters.count)명)")
        }
    }

    private var header: some View {
        HStack(spacing: 10) {
            Image("MenuBarIcon")
                .resizable()
                .frame(width: 28, height: 28)
            Text("톡맨스")
                .font(.title2.bold())
            Spacer()
            if loading {
                ProgressView().controlSize(.small)
            }
            Button {
                showCreateSheet = true
            } label: {
                Label("캐릭터 추가", systemImage: "plus")
            }
            .buttonStyle(.borderedProminent)
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 10)
    }

    @ViewBuilder
    private var content: some View {
        if let errorMessage {
            VStack(spacing: 0) {
                ContentUnavailableView {
                    Label("서버 연결 실패", systemImage: "wifi.exclamationmark")
                } description: {
                    Text(errorMessage)
                } actions: {
                    Button("다시 시도") { Task { await load() } }
                        .buttonStyle(.borderedProminent)
                }
                .padding(.top, 40)
                Spacer()
            }
        } else if characters.isEmpty && !loading {
            VStack(spacing: 0) {
                ContentUnavailableView {
                    Label("캐릭터가 없습니다", systemImage: "person.crop.circle.badge.plus")
                } description: {
                    Text("첫 AI 캐릭터를 만들어 대화를 시작해 보세요")
                } actions: {
                    Button("캐릭터 만들기") { showCreateSheet = true }
                        .buttonStyle(.borderedProminent)
                }
                .padding(.top, 40)
                Spacer()
            }
        } else {
            VStack(spacing: 0) {
                categoryChips
                List {
                    ForEach(groupedCategories, id: \.self) { category in
                        Section {
                            ForEach(filteredCharacters(for: category)) { character in
                                CharacterRow(character: character)
                                    .contentShape(Rectangle())
                                    .onTapGesture {
                                        selectedCharacter = character
                                        DebugLogger.shared.feature("캐릭터", "선택됨: \(character.name) (카테고리=\(category))")
                                    }
                            }
                        } header: {
                            Text("\(category) (\(filteredCharacters(for: category).count))")
                        }
                    }
                }
                .listStyle(.inset)
            }
        }
    }

    // MARK: 카테고리 필터 (T-96)
    private var categories: [String] {
        let set = Set(characters.map { $0.category.isEmpty ? "기타" : $0.category })
        var result = ["일반", "연인", "친구", "가족", "기타"].filter { set.contains($0) }
        for category in set.sorted() where !result.contains(category) {
            result.append(category)
        }
        return result
    }

    private var groupedCategories: [String] {
        let all = categories
        if categoryFilter == "전체" {
            return all
        }
        return all.filter { $0 == categoryFilter }
    }

    private func filteredCharacters(for category: String) -> [Character] {
        characters.filter {
            let cat = $0.category.isEmpty ? "기타" : $0.category
            return cat == category
        }
    }

    private var categoryChips: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: 6) {
                categoryChip("전체", selected: categoryFilter == "전체") {
                    categoryFilter = "전체"
                }
                ForEach(categories, id: \.self) { category in
                    categoryChip(category, selected: categoryFilter == category) {
                        categoryFilter = category
                    }
                }
            }
            .padding(.horizontal, 16)
            .padding(.vertical, 6)
        }
    }

    private func categoryChip(_ title: String, selected: Bool, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Text(title)
                .font(.caption)
                .padding(.horizontal, 10)
                .padding(.vertical, 4)
                .background(selected ? Color.accentColor.opacity(0.22) : Color.gray.opacity(0.1))
                .clipShape(Capsule())
        }
        .buttonStyle(.plain)
    }

    private func load() async {
        loading = true
        errorMessage = nil
        do {
            characters = try await TalkmanceAPI.shared.listCharacters()
            DebugLogger.shared.feature("캐릭터", "목록 로드 완료 (\(characters.count)명)")
        } catch {
            errorMessage = (error as? APIError)?.message ?? error.localizedDescription
            DebugLogger.shared.feature("캐릭터", "목록 로드 실패: \(errorMessage ?? "?")")
        }
        loading = false
    }
}

// MARK: - 캐릭터 행

struct CharacterRow: View {
    let character: Character

    var body: some View {
        HStack(spacing: 12) {
            CachedAvatarView(url: URL(string: character.avatarURL ?? ""), size: 44)

            VStack(alignment: .leading, spacing: 2) {
                Text(character.name)
                    .font(.headline)
                if !character.title.isEmpty {
                    Text(character.title)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                if character.adult {
                    Text("성인")
                        .font(.caption2.bold())
                        .padding(.horizontal, 6)
                        .padding(.vertical, 1)
                        .background(.red.opacity(0.15), in: Capsule())
                        .foregroundStyle(.red)
                }
            }
            Spacer()
            if let age = character.age {
                Text("\(age)세")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Image(systemName: "chevron.right")
                .font(.caption)
                .foregroundStyle(.tertiary)
        }
        .padding(.vertical, 4)
    }
}

// MARK: - 캐릭터 생성 시트 (T-32/T-43) — AI 자동 생성 + 직접 작성 2모드

struct CreateCharacterSheet: View {
    @Environment(\.dismiss) private var dismiss
    var onCreated: () async -> Void

    enum Mode: String, CaseIterable {
        case ai = "AI로 만들기"
        case manual = "직접 작성"
    }

    @State private var mode: Mode = .ai

    // 기본 정보 (AI/직접 공통)
    @State private var name = ""
    @State private var gender = "여"
    @State private var age = ""
    @State private var relationship = "연인"
    @State private var category = "일반"
    @State private var adult = false

    // AI 생성 상태
    @State private var generating = false
    @State private var saving = false
    @State private var errorMessage: String?

    // AI 생성 결과 (편집 가능)
    @State private var gen: GeneratedCharacter?
    @State private var editTitle = ""
    @State private var editGreeting = ""
    @State private var editPersonality = ""
    @State private var editTone = ""
    @State private var editStory = ""
    @State private var editBackstory = ""
    @State private var avatarURL: String?

    // 직접 작성 모드
    @State private var manualTitle = ""
    @State private var manualGreeting = ""
    @State private var manualPersonality = ""

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                Text("새 캐릭터 만들기")
                    .font(.title2.bold())
                Spacer()
                Picker("", selection: $mode) {
                    ForEach(Mode.allCases, id: \.self) { m in Text(m.rawValue).tag(m) }
                }
                .pickerStyle(.segmented)
                .frame(width: 220)
            }

            Divider()

            ScrollView {
                VStack(alignment: .leading, spacing: 12) {
                    if mode == .ai {
                        aiInputFields
                        if let gen {
                            aiResultFields
                        }
                    } else {
                        manualInputFields
                    }

                    if let errorMessage {
                        Text(errorMessage)
                            .font(.caption)
                            .foregroundStyle(.red)
                    }
                }
            }

            Divider()
            HStack {
                Spacer()
                Button("취소") { dismiss() }
                actionButton
            }
        }
        .padding(20)
        .frame(width: 480, height: 560)
        .onAppear {
            DebugLogger.shared.feature("캐릭터생성", "생성 시트 표시됨 (mode=\(mode.rawValue))")
        }
    }

    // MARK: AI 모드 — 기본 정보 입력
    private var aiInputFields: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("기본 정보만 입력하면 AI가 캐릭터를 만들어요")
                .font(.caption)
                .foregroundStyle(.secondary)

            TextField("이름 (필수)", text: $name)
                .textFieldStyle(.roundedBorder)

            HStack(spacing: 10) {
                Picker("성별", selection: $gender) {
                    Text("여").tag("여")
                    Text("남").tag("남")
                }
                .pickerStyle(.segmented)
                .frame(width: 110)

                TextField("나이 (19세 이상)", text: $age)
                    .textFieldStyle(.roundedBorder)

                TextField("관계 (예: 연인)", text: $relationship)
                    .textFieldStyle(.roundedBorder)
            }

            Picker("카테고리", selection: $category) {
                ForEach(["일반", "연인", "친구", "가족", "기타"], id: \.self) { c in
                    Text(c).tag(c)
                }
            }
            .pickerStyle(.menu)

            Toggle("성인 캐릭터 (성인 대화 허용)", isOn: $adult)
            Text("이용자는 만 19세 이상이며, 미성년자 캐릭터 설정은 금지됩니다.")
                .font(.caption2)
                .foregroundStyle(.secondary)

            if generating {
                HStack(spacing: 8) {
                    ProgressView().controlSize(.small)
                    Text("AI가 캐릭터를 만들고 있어요… (최대 30초)")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }
        }
    }

    // MARK: AI 모드 — 결과 편집
    @ViewBuilder
    private var aiResultFields: some View {
        VStack(alignment: .leading, spacing: 10) {
            Divider()
            HStack {
                Text("AI 생성 결과 — 원하는 대로 수정하세요")
                    .font(.subheadline.bold())
                Spacer()
                Button("다시 생성") {
                    Task { await generate() }
                }
                .disabled(generating)
            }

            HStack(spacing: 12) {
                CachedAvatarView(url: URL(string: avatarURL ?? ""), size: 64, shape: .rounded(8))

                VStack(alignment: .leading, spacing: 6) {
                    Button("DiceBear 아바타") {
                        avatarURL = diceBearURL(name: name)
                    }
                    Button("AI 아바타") {
                        avatarURL = aiAvatarURL(prompt: personaString("avatar_prompt"))
                    }
                }
                .buttonStyle(.bordered)
                .controlSize(.small)
            }

            TextField("타이틀", text: $editTitle)
                .textFieldStyle(.roundedBorder)

            HStack(spacing: 10) {
                TextField("성격 (쉼표 구분)", text: $editPersonality)
                    .textFieldStyle(.roundedBorder)
                TextField("말투", text: $editTone)
                    .textFieldStyle(.roundedBorder)
            }

            TextField("스토리 (2~3줄)", text: $editStory, axis: .vertical)
                .lineLimit(2...4)
                .textFieldStyle(.roundedBorder)

            TextField("시작 전 대화 (사용자와의 과거 관계)", text: $editBackstory, axis: .vertical)
                .lineLimit(2...4)
                .textFieldStyle(.roundedBorder)

            TextField("인사말", text: $editGreeting)
                .textFieldStyle(.roundedBorder)
        }
    }

    // MARK: 직접 작성 모드
    private var manualInputFields: some View {
        VStack(alignment: .leading, spacing: 10) {
            TextField("이름 (필수)", text: $name)
                .textFieldStyle(.roundedBorder)
            TextField("타이틀 (예: 내 이상형)", text: $manualTitle)
                .textFieldStyle(.roundedBorder)
            TextField("나이 (19세 이상)", text: $age)
                .textFieldStyle(.roundedBorder)
            TextField("인사말 (예: 안녕! 만나서 반가워)", text: $manualGreeting)
                .textFieldStyle(.roundedBorder)
            TextField("성격 (쉼표로 구분, 예: 밝음, 배려심)", text: $manualPersonality)
                .textFieldStyle(.roundedBorder)
            Picker("카테고리", selection: $category) {
                ForEach(["일반", "연인", "친구", "가족", "기타"], id: \.self) { c in
                    Text(c).tag(c)
                }
            }
            .pickerStyle(.menu)
            Toggle("성인 캐릭터 (성인 대화 허용)", isOn: $adult)
            Text("이용자는 만 19세 이상이며, 미성년자 캐릭터 설정은 금지됩니다.")
                .font(.caption2)
                .foregroundStyle(.secondary)
        }
    }

    // MARK: 하단 버튼
    @ViewBuilder
    private var actionButton: some View {
        switch mode {
        case .ai:
            if gen == nil {
                Button {
                    Task { await generate() }
                } label: {
                    if generating {
                        ProgressView().controlSize(.small)
                    } else {
                        Text("AI 캐릭터 생성")
                    }
                }
                .buttonStyle(.borderedProminent)
                .disabled(name.trimmingCharacters(in: .whitespaces).isEmpty || generating)
            } else {
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
                .disabled(saving)
            }
        case .manual:
            Button {
                Task { await save() }
            } label: {
                if saving {
                    ProgressView().controlSize(.small)
                } else {
                    Text("만들기")
                }
            }
            .buttonStyle(.borderedProminent)
            .disabled(name.trimmingCharacters(in: .whitespaces).isEmpty || saving)
        }
    }

    // MARK: 동작
    private func generate() async {
        generating = true
        errorMessage = nil
        do {
            let result = try await TalkmanceAPI.shared.generateCharacter(
                name: name.trimmingCharacters(in: .whitespaces),
                gender: gender,
                age: Int(age),
                relationship: relationship,
                adult: adult
            )
            gen = result
            editTitle = result.title
            editGreeting = result.greeting
            editPersonality = personaStringArray("성격").joined(separator: ", ")
            editTone = personaString("말투")
            editStory = personaString("스토리")
            editBackstory = personaString("시작전대화")
            avatarURL = result.avatarURL
            DebugLogger.shared.feature("캐릭터생성", "AI 생성 완료 (title=\(result.title))")
        } catch {
            errorMessage = (error as? APIError)?.message ?? error.localizedDescription
            DebugLogger.shared.feature("캐릭터생성", "AI 생성 실패: \(errorMessage ?? "?")")
        }
        generating = false
    }

    private func save() async {
        saving = true
        errorMessage = nil
        do {
            let persona: [String: Any]
            if let gen {
                var p: [String: Any] = [:]
                if let base = gen.persona {
                    for (k, v) in base { p[k] = v.value }
                }
                p["성별"] = gender
                p["관계"] = relationship
                p["avatar_prompt"] = personaString("avatar_prompt")
                if !editPersonality.isEmpty {
                    p["성격"] = editPersonality.split(separator: ",").map { $0.trimmingCharacters(in: .whitespaces) }
                }
                if !editTone.isEmpty { p["말투"] = editTone }
                if !editStory.isEmpty { p["스토리"] = editStory }
                if !editBackstory.isEmpty { p["시작전대화"] = editBackstory }
                persona = p
            } else {
                persona = manualPersonality.isEmpty
                    ? [:]
                    : ["성격": manualPersonality.split(separator: ",").map { $0.trimmingCharacters(in: .whitespaces) }]
            }
            _ = try await TalkmanceAPI.shared.createCharacter(
                name: name.trimmingCharacters(in: .whitespaces),
                title: gen == nil ? manualTitle : editTitle,
                age: Int(age),
                greeting: gen == nil ? manualGreeting : editGreeting,
                persona: persona,
                category: category,
                adult: adult,
                avatarURL: avatarURL
            )
            DebugLogger.shared.feature("캐릭터생성", "저장 완료 (name=\(name))")
            dismiss()
            await onCreated()
        } catch {
            errorMessage = (error as? APIError)?.message ?? error.localizedDescription
            DebugLogger.shared.feature("캐릭터생성", "저장 실패: \(errorMessage ?? "?")")
        }
        saving = false
    }

    // MARK: 헬퍼
    private func personaString(_ key: String) -> String {
        guard let p = gen?.persona?[key]?.value as? String else { return "" }
        return p
    }

    private func personaStringArray(_ key: String) -> [String] {
        guard let arr = gen?.persona?[key]?.value as? [Any] else { return [] }
        return arr.compactMap { $0 as? String }
    }

    private func diceBearURL(name: String) -> String {
        let styles = ["pixel-art", "adventurer", "lorelei", "thumbs", "notionists", "big-smile"]
        let style = styles.randomElement() ?? "pixel-art"
        let seed = "\(name)-\(Int.random(in: 0...99999))"
        return "https://api.dicebear.com/9.x/\(style)/svg?seed=\(seed)&backgroundColor=b6e3f4"
    }

    private func aiAvatarURL(prompt: String) -> String {
        let p = prompt.isEmpty ? "portrait of \(name), wholesome cartoon style" : prompt
        let encoded = p.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? p
        return "https://image.pollinations.ai/prompt/\(encoded)?width=512&height=512&seed=\(Int.random(in: 0...99999))&nologo=true"
    }
}