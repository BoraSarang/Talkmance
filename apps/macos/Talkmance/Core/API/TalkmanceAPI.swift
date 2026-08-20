import Foundation

// MARK: - API 클라이언트 (T-31)
/// 서버 REST API + SSE 스트리밍 클라이언트
@MainActor
final class TalkmanceAPI {
    static let shared = TalkmanceAPI()

    private let config = ServerConfig.shared
    private let session: URLSession
    private let decoder = JSONDecoder()

    private init() {
        let config = URLSessionConfiguration.default
        config.timeoutIntervalForRequest = 90
        session = URLSession(configuration: config)
    }

    // MARK: - 인증

    /// 기기 등록/로그인 — JWT 발급
    @discardableResult
    func registerIfNeeded() async throws -> String {
        if let token = config.token { return token }
        DebugLogger.shared.feature("API", "→ POST /auth/register")
        let body = ["device_id": config.deviceID]
        let data = try await request(path: "/api/v1/auth/register", method: "POST", body: body, requiresAuth: false)
        do {
            let resp = try decoder.decode(AuthRegisterResponse.self, from: data)
            config.token = resp.token
            DebugLogger.shared.feature("API", "← 인증 완료 (user=\(resp.userID.prefix(8))…)")
            return resp.token
        } catch {
            DebugLogger.shared.error("API", "register 응답 디코딩 실패: \(error)")
            throw error
        }
    }

    // MARK: - 캐릭터

    /// AI 캐릭터 생성 — 기본 정보만 입력하면 AI가 스토리/성격/아바타 생성
    func generateCharacter(name: String, gender: String, age: Int?, relationship: String, category: String = "일반", adult: Bool = false) async throws -> GeneratedCharacter {
        DebugLogger.shared.feature("API", "→ POST /characters/generate (name=\(name))")
        let body: [String: Any] = [
            "name": name, "gender": gender, "category": category, "adult": adult,
            "relationship": relationship, "age": age as Any,
        ]
        let data = try await request(path: "/api/v1/characters/generate", method: "POST", body: body)
        do {
            let gen = try decoder.decode(GeneratedCharacter.self, from: data)
            DebugLogger.shared.feature("API", "← 생성 완료 (title=\(gen.title))")
            return gen
        } catch {
            DebugLogger.shared.error("API", "generate 응답 디코딩 실패: \(error)")
            throw error
        }
    }

    /// 아바타 재생성 — style: "dicebear" | "ai"
    func regenerateAvatar(characterID: String, style: String) async throws -> String {
        DebugLogger.shared.feature("API", "→ POST /characters/\(characterID.prefix(8))/avatar (style=\(style))")
        let data = try await request(path: "/api/v1/characters/\(characterID)/avatar", method: "POST", body: ["style": style])
        struct Resp: Decodable {
            let avatarURL: String
            enum CodingKeys: String, CodingKey { case avatarURL = "avatar_url" }
        }
        return try decoder.decode(Resp.self, from: data).avatarURL
    }

    func listCharacters() async throws -> [Character] {
        let data = try await request(path: "/api/v1/characters")
        struct Resp: Decodable { let characters: [Character] }
        return try decoder.decode(Resp.self, from: data).characters
    }

    @discardableResult
    func createCharacter(name: String, title: String, age: Int?, greeting: String, persona: [String: Any], category: String = "기타", adult: Bool = false, avatarURL: String? = nil) async throws -> String {
        DebugLogger.shared.feature("API", "→ POST /characters (name=\(name))")
        let body: [String: Any] = [
            "name": name, "title": title, "category": category,
            "greeting": greeting, "persona": persona, "adult": adult,
            "age": age as Any, "avatar_url": avatarURL as Any,
        ]
        let data = try await request(path: "/api/v1/characters", method: "POST", body: body)
        struct Resp: Decodable { let id: String }
        return try decoder.decode(Resp.self, from: data).id
    }

    func deleteCharacter(id: String) async throws {
        _ = try await request(path: "/api/v1/characters/\(id)", method: "DELETE")
    }

    /// 세션 모델 변경 (PUT)
    func updateSessionModel(id: String, modelID: String) async throws {
        DebugLogger.shared.feature("API", "→ PUT /sessions/\(id.prefix(8)) (model=\(modelID))")
        _ = try await request(path: "/api/v1/sessions/\(id)", method: "PUT", body: ["model_id": modelID])
    }

    /// 캐릭터 수정 (PUT)
    @discardableResult
    func updateCharacter(id: String, name: String, title: String, age: Int?, greeting: String, persona: [String: Any], category: String = "기타", adult: Bool = false, avatarURL: String? = nil) async throws -> Character {
        DebugLogger.shared.feature("API", "→ PUT /characters/\(id.prefix(8)) (name=\(name))")
        let body: [String: Any] = [
            "name": name, "title": title, "category": category,
            "greeting": greeting, "persona": persona, "adult": adult,
            "age": age as Any, "avatar_url": avatarURL as Any,
        ]
        let data = try await request(path: "/api/v1/characters/\(id)", method: "PUT", body: body)
        return try decoder.decode(Character.self, from: data)
    }

    // MARK: - 세션/메시지

    func listSessions() async throws -> [ChatSession] {
        let data = try await request(path: "/api/v1/sessions")
        struct Resp: Decodable { let sessions: [ChatSession] }
        return try decoder.decode(Resp.self, from: data).sessions
    }

    @discardableResult
    func createSession(characterID: String, modelID: String, ruleID: String? = nil) async throws -> String {
        DebugLogger.shared.feature("API", "→ POST /sessions (character=\(characterID.prefix(8))…, rule=\(ruleID?.prefix(8) ?? "기본")…)")
        var body: [String: String] = ["character_id": characterID, "model_id": modelID]
        if let ruleID {
            body["rule_id"] = ruleID
        }
        let data = try await request(path: "/api/v1/sessions", method: "POST", body: body)
        struct Resp: Decodable { let id: String }
        return try decoder.decode(Resp.self, from: data).id
    }

    func listMessages(sessionID: String) async throws -> [Message] {
        let data = try await request(path: "/api/v1/sessions/\(sessionID)/messages")
        struct Resp: Decodable { let messages: [Message] }
        return try decoder.decode(Resp.self, from: data).messages
    }

    func deleteSession(id: String) async throws {
        DebugLogger.shared.feature("API", "→ DELETE /sessions/\(id.prefix(8))")
        _ = try await request(path: "/api/v1/sessions/\(id)", method: "DELETE")
    }

    // MARK: - 모델/할당량

    func listModels() async throws -> ModelsResponse {
        let data = try await request(path: "/api/v1/models")
        return try decoder.decode(ModelsResponse.self, from: data)
    }

    /// OpenRouter 할당량 (디버그 패널)
    func fetchQuota() async throws -> QuotaResponse {
        let data = try await request(path: "/api/v1/quota")
        return try decoder.decode(QuotaResponse.self, from: data)
    }

    /// 할당량 조회 — 5분 캐시, 실패 시 nil (UI 배지용)
    private static var quotaCache: (response: QuotaResponse, fetchedAt: Date)?
    func fetchQuotaCached() async -> QuotaResponse? {
        if let cached = Self.quotaCache, Date().timeIntervalSince(cached.fetchedAt) < 300 {
            return cached.response
        }
        do {
            let quota = try await fetchQuota()
            Self.quotaCache = (quota, Date())
            return quota
        } catch {
            return nil
        }
    }

    /// SSE 채팅 스트림 — AsyncThrowingStream으로 청크 전달
    func chatStream(sessionID: String, content: String, auto: Bool = false, retry: Bool = false, polish: Bool = false) -> AsyncThrowingStream<ChatChunk, Error> {
        AsyncThrowingStream { continuation in
            let task = Task { [weak self] in
                guard let self else {
                    continuation.finish(throwing: APIError.localized("클라이언트 오류"))
                    return
                }
                await self.streamChat(sessionID: sessionID, content: content, auto: auto, retry: retry, polish: polish, continuation: continuation)
            }
            continuation.onTermination = { _ in task.cancel() }
        }
    }

    private func streamChat(sessionID: String, content: String, auto: Bool, retry: Bool, polish: Bool, continuation: AsyncThrowingStream<ChatChunk, Error>.Continuation) async {
        do {
            try await registerIfNeeded()
            guard let url = URL(string: "\(config.baseURL)/api/v1/sessions/\(sessionID)/chat") else {
                continuation.finish(throwing: APIError.localized("잘못된 서버 주소"))
                return
            }
            DebugLogger.shared.feature("API", "→ SSE /sessions/\(sessionID.prefix(8))/chat 시작 (auto=\(auto), retry=\(retry), polish=\(polish))")

            var req = URLRequest(url: url)
            req.httpMethod = "POST"
            req.setValue("application/json", forHTTPHeaderField: "Content-Type")
            req.setValue("Bearer \(config.token ?? "")", forHTTPHeaderField: "Authorization")
            var body: [String: Any] = [:]
            if auto {
                body["auto"] = true
            } else if retry {
                body["retry"] = true
            } else {
                body["content"] = content
            }
            if polish {
                body["polish"] = true
            }
            req.httpBody = try JSONSerialization.data(withJSONObject: body)
            req.timeoutInterval = 120

            let (bytes, response) = try await session.bytes(for: req)
            guard let http = response as? HTTPURLResponse, http.statusCode == 200 else {
                let code = (response as? HTTPURLResponse)?.statusCode ?? -1
                continuation.finish(throwing: APIError.localized("서버 오류 (HTTP \(code))"))
                return
            }

            for try await line in bytes.lines {
                guard line.hasPrefix("data:") else { continue }
                let payload = line.dropFirst(5).trimmingCharacters(in: .whitespaces)
                guard let data = payload.data(using: .utf8),
                      let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else { continue }

                if let error = json["error"] as? [String: Any] {
                    let message = error["message"] as? String ?? "알 수 없는 오류"
                    let code = error["code"] as? String ?? "E-UNKNOWN"
                    let detail = error["detail"] as? String
                    continuation.finish(throwing: APIError.server(code, message, detail: detail))
                    return
                }
                if let done = json["done"] as? Bool, done {
                    continuation.yield(ChatChunk(
                        content: nil, done: true,
                        model: json["model"] as? String,
                        tokenIn: json["token_in"] as? Int ?? 0,
                        tokenOut: json["token_out"] as? Int ?? 0,
                        cost: json["cost"] as? Double ?? 0,
                        error: nil
                    ))
                    continuation.finish()
                    return
                }
                if let content = json["content"] as? String {
                    continuation.yield(ChatChunk(content: content, done: false, model: nil, tokenIn: 0, tokenOut: 0, cost: 0, error: nil))
                }
            }
            continuation.finish()
        } catch is CancellationError {
            continuation.finish()
        } catch {
            continuation.finish(throwing: APIError.localized("스트림 오류: \(error.localizedDescription)"))
        }
    }

    // MARK: - 기억

    func listMemories(characterID: String) async throws -> [Memory] {
        let data = try await request(path: "/api/v1/memories/\(characterID)")
        struct Resp: Decodable { let memories: [Memory] }
        return try decoder.decode(Resp.self, from: data).memories
    }

    @discardableResult
    func createMemory(characterID: String, content: String, memType: String = "long") async throws -> String {
        let body: [String: String] = ["content": content, "mem_type": memType]
        let data = try await request(path: "/api/v1/memories/\(characterID)", method: "POST", body: body)
        struct Resp: Decodable { let id: String }
        return try decoder.decode(Resp.self, from: data).id
    }

    func updateMemory(id: String, content: String, memType: String, pinned: Bool) async throws {
        DebugLogger.shared.feature("API", "→ PUT /memories/\(id.prefix(8)) (pinned=\(pinned))")
        let body: [String: Any] = ["content": content, "mem_type": memType, "pinned": pinned]
        _ = try await request(path: "/api/v1/memories/\(id)", method: "PUT", body: body)
    }

    func deleteMemory(id: String) async throws {
        DebugLogger.shared.feature("API", "→ DELETE /memories/\(id.prefix(8))")
        _ = try await request(path: "/api/v1/memories/\(id)", method: "DELETE")
    }

    // MARK: - 규칙

    func listRules() async throws -> [PromptRule] {
        let data = try await request(path: "/api/v1/rules")
        struct Resp: Decodable { let rules: [PromptRule] }
        return try decoder.decode(Resp.self, from: data).rules
    }

    @discardableResult
    func createRule(name: String, systemPrompt: String) async throws -> String {
        DebugLogger.shared.feature("API", "→ POST /rules (name=\(name))")
        let body: [String: Any] = ["name": name, "system_prompt": systemPrompt]
        let data = try await request(path: "/api/v1/rules", method: "POST", body: body)
        struct Resp: Decodable { let id: String }
        return try decoder.decode(Resp.self, from: data).id
    }

    func updateRule(id: String, name: String, systemPrompt: String) async throws {
        DebugLogger.shared.feature("API", "→ PUT /rules/\(id.prefix(8)) (name=\(name))")
        let body: [String: Any] = ["name": name, "system_prompt": systemPrompt]
        _ = try await request(path: "/api/v1/rules/\(id)", method: "PUT", body: body)
    }

    func deleteRule(id: String) async throws {
        DebugLogger.shared.feature("API", "→ DELETE /rules/\(id.prefix(8))")
        _ = try await request(path: "/api/v1/rules/\(id)", method: "DELETE")
    }

    // MARK: - BYOK 키

    func listKeys() async throws -> [UserKey] {
        let data = try await request(path: "/api/v1/settings/keys")
        struct Resp: Decodable { let keys: [UserKey] }
        return try decoder.decode(Resp.self, from: data).keys
    }

    @discardableResult
    func createKey(label: String, apiKey: String) async throws -> String {
        DebugLogger.shared.feature("API", "→ POST /settings/keys (label=\(label))")
        let body: [String: String] = ["label": label, "api_key": apiKey]
        let data = try await request(path: "/api/v1/settings/keys", method: "POST", body: body)
        struct Resp: Decodable { let id: String }
        return try decoder.decode(Resp.self, from: data).id
    }

    func deleteKey(id: String) async throws {
        DebugLogger.shared.feature("API", "→ DELETE /settings/keys/\(id.prefix(8))")
        _ = try await request(path: "/api/v1/settings/keys/\(id)", method: "DELETE")
    }

    // MARK: - 커스텀 모델

    @discardableResult
    func createCustomModel(name: String, modelID: String, baseURL: String, apiKey: String?, isFree: Bool, description: String) async throws -> String {
        DebugLogger.shared.feature("API", "→ POST /models/custom (name=\(name))")
        var body: [String: Any] = [
            "name": name, "model_id": modelID, "base_url": baseURL,
            "is_free": isFree, "description": description,
        ]
        if let apiKey {
            body["api_key"] = apiKey
        }
        let data = try await request(path: "/api/v1/models/custom", method: "POST", body: body)
        struct Resp: Decodable { let id: String }
        return try decoder.decode(Resp.self, from: data).id
    }

    func deleteCustomModel(id: String) async throws {
        DebugLogger.shared.feature("API", "→ DELETE /models/custom/\(id.prefix(8))")
        _ = try await request(path: "/api/v1/models/custom/\(id)", method: "DELETE")
    }

    // MARK: - 내부 헬퍼

    /// 공통 요청 — 토큰 갱신 후 1회 재시도 (requiresAuth=false면 register 재귀 방지)
    private func request(path: String, method: String = "GET", body: Any? = nil, requiresAuth: Bool = true) async throws -> Data {
        if requiresAuth { try await registerIfNeeded() }
        guard let url = URL(string: config.baseURL + path) else {
            throw APIError.localized("잘못된 서버 주소: \(config.baseURL)")
        }
        var req = URLRequest(url: url)
        req.httpMethod = method
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.setValue("Bearer \(config.token ?? "")", forHTTPHeaderField: "Authorization")
        if let body {
            req.httpBody = try JSONSerialization.data(withJSONObject: body)
        }

        let (data, response) = try await session.data(for: req)
        guard let http = response as? HTTPURLResponse else {
            throw APIError.localized("서버 응답 없음")
        }
        if !(200...299).contains(http.statusCode) || http.statusCode == 200 {
            let preview = String(data: data.prefix(200), encoding: .utf8) ?? "\(data.count)바이트"
            DebugLogger.shared.feature("API", "← \(method) \(path) → \(http.statusCode) (body: \(preview))")
        }
        guard (200...299).contains(http.statusCode) else {
            let detail = try? decoder.decode(APIErrorResponse.self, from: data)
            let code = detail?.error.code ?? "E-COM-VALID-1001"
            let message = detail?.error.message ?? "요청 실패 (HTTP \(http.statusCode))"
            throw APIError.server(code, message)
        }
        return data
    }
}

/// API 에러 — 사용자 노출 메시지 포함 (detail: 서버 폴백 실패 사유 — 디버그용)
struct APIError: LocalizedError, Sendable {
    let code: String
    let message: String
    let detail: String?

    static func server(_ code: String, _ message: String, detail: String? = nil) -> APIError {
        APIError(code: code, message: message, detail: detail)
    }

    static func localized(_ message: String) -> APIError {
        APIError(code: "E-CLIENT-0000", message: message, detail: nil)
    }

    var errorDescription: String? {
        code == "E-CLIENT-0000" ? message : "[\(code)] \(message)"
    }
}