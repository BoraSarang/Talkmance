import Foundation

// MARK: - 서버 API 모델 (서버 응답과 일치 — apps/server/internal)

/// 인증 등록 응답
struct AuthRegisterResponse: Codable, Sendable {
    let token: String
    let userID: String
    let deviceID: String

    enum CodingKeys: String, CodingKey {
        case token
        case userID = "user_id"
        case deviceID = "device_id"
    }
}

/// 캐릭터
struct Character: Codable, Identifiable, Hashable, Sendable {
    let id: String
    let userID: String
    let name: String
    let title: String
    let avatarURL: String?
    let category: String
    let persona: [String: AnyCodable]?
    let greeting: String
    let age: Int?
    let adult: Bool

    enum CodingKeys: String, CodingKey {
        case id, name, title, category, persona, greeting, age, adult
        case userID = "user_id"
        case avatarURL = "avatar_url"
    }

    static func == (lhs: Character, rhs: Character) -> Bool { lhs.id == rhs.id }
    func hash(into hasher: inout Hasher) { hasher.combine(id) }
}

/// AI 캐릭터 생성 결과 (POST /characters/generate)
struct GeneratedCharacter: Codable, Sendable {
    let name: String
    let title: String
    let persona: [String: AnyCodable]?
    let greeting: String
    let avatarPrompt: String
    let avatarURL: String?

    enum CodingKeys: String, CodingKey {
        case name, title, persona, greeting
        case avatarPrompt = "avatar_prompt"
        case avatarURL = "avatar_url"
    }
}

/// 커스텀 모델
struct CustomModel: Codable, Identifiable, Sendable {
    let id: String
    let name: String
    let modelID: String
    let baseURL: String
    let isFree: Bool
    let description: String
    let enabled: Bool

    enum CodingKeys: String, CodingKey {
        case id, name, description, enabled
        case modelID = "model_id"
        case baseURL = "base_url"
        case isFree = "is_free"
    }
}

/// 카탈로그 모델 항목
struct CatalogModel: Codable, Identifiable, Sendable {
    let id: String
    let name: String
    let description: String
    let contextLength: Int
    let isFree: Bool

    enum CodingKeys: String, CodingKey {
        case id, name, description
        case contextLength = "context_length"
        case isFree = "is_free"
    }
}

/// 모델 목록 응답
struct ModelsResponse: Codable, Sendable {
    let catalog: [CatalogModel]
    let custom: [CustomModel]
}

/// OpenRouter 할당량 응답 (GET /api/v1/quota → { data: {...} })
struct QuotaResponse: Codable, Sendable {
    struct Data: Codable, Sendable {
        let label: String
        let usage: Double
        let limit: Double?
        let isFreeTier: Bool
        struct RateLimit: Codable, Sendable {
            let requests: Int
            let interval: String
        }
        let rateLimit: RateLimit?
        enum CodingKeys: String, CodingKey {
            case label, usage, limit
            case isFreeTier = "is_free_tier"
            case rateLimit = "rate_limit"
        }
    }
    let data: Data
    let freeUsedToday: Int
    let freeRemaining: Int
    let freeLimitDaily: Int
    enum CodingKeys: String, CodingKey {
        case data
        case freeUsedToday = "free_used_today"
        case freeRemaining = "free_remaining"
        case freeLimitDaily = "free_limit_daily"
    }
}

/// 대화방
struct ChatSession: Codable, Identifiable, Sendable {
    let id: String
    let characterID: String
    let modelID: String
    let status: String
    let summary: String?
    let createdAt: String
    let updatedAt: String
    let lastMessage: String?
    let lastMessageAt: String?

    enum CodingKeys: String, CodingKey {
        case id, status, summary
        case characterID = "character_id"
        case modelID = "model_id"
        case createdAt = "created_at"
        case updatedAt = "updated_at"
        case lastMessage = "last_message"
        case lastMessageAt = "last_message_at"
    }
}

/// 메시지
struct Message: Codable, Identifiable, Sendable {
    let id: String
    let sessionID: String
    let role: String
    let content: String
    let model: String?
    let tokenIn: Int
    let tokenOut: Int
    let cost: Double
    let createdAt: String

    enum CodingKeys: String, CodingKey {
        case id, role, content, model, cost
        case sessionID = "session_id"
        case tokenIn = "token_in"
        case tokenOut = "token_out"
        case createdAt = "created_at"
    }
}

/// 기억 카드
struct Memory: Codable, Identifiable, Sendable {
    let id: String
    let characterID: String
    let memType: String
    let content: String
    let importance: Float
    let pinned: Bool

    enum CodingKeys: String, CodingKey {
        case id, content, importance, pinned
        case characterID = "character_id"
        case memType = "mem_type"
    }
}

/// 대화 규칙
struct PromptRule: Codable, Identifiable, Sendable {
    let id: String
    let name: String
    let systemPrompt: String
    let isDefault: Bool

    enum CodingKeys: String, CodingKey {
        case id, name
        case systemPrompt = "system_prompt"
        case isDefault = "is_default"
    }
}

/// 사용자 BYOK API 키 (서버는 암호문만 보관, label만 노출)
struct UserKey: Codable, Identifiable, Sendable {
    let id: String
    let label: String
}

/// 서버 에러 응답
struct APIErrorResponse: Codable, Sendable {
    let error: APIErrorDetail
}

struct APIErrorDetail: Codable, Sendable {
    let code: String
    let message: String
}

/// AnyCodable — JSONB 페르소나 등 임의 JSON용
struct AnyCodable: Codable, @unchecked Sendable {
    let value: Any

    init(_ value: Any) {
        self.value = value
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        if let v = try? container.decode(String.self) { value = v }
        else if let v = try? container.decode(Bool.self) { value = v }
        else if let v = try? container.decode(Double.self) { value = v }
        else if let v = try? container.decode(Int.self) { value = v }
        else if let v = try? container.decode([AnyCodable].self) { value = v.map(\.value) }
        else if let v = try? container.decode([String: AnyCodable].self) { value = v.mapValues(\.value) }
        else { value = NSNull() }
    }

    func encode(to encoder: Encoder) throws {
        var container = encoder.singleValueContainer()
        switch value {
        case let v as String: try container.encode(v)
        case let v as Bool: try container.encode(v)
        case let v as Double: try container.encode(v)
        case let v as Int: try container.encode(v)
        case let v as [Any]: try container.encode(v.map(AnyCodable.init))
        case let v as [String: Any]: try container.encode(v.mapValues(AnyCodable.init))
        default: try container.encodeNil()
        }
    }
}

/// SSE 채팅 청크
struct ChatChunk: Sendable {
    let content: String?
    let done: Bool
    let model: String?
    let tokenIn: Int
    let tokenOut: Int
    let cost: Double
    let error: String?
}
