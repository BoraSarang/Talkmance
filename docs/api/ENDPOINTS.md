# 톡맨스 (Talkmance) — Server API 명세 (ENDPOINTS)

> 버전: v1.0.0 · 작성일: 2026-08-13 · 플랫폼: Server(Go)
> 베이스 URL: `https://{render-host}.onrender.com` · 인증: `Authorization: Bearer <JWT>`

---

## 공통 규약

- 모든 요청/응답 JSON (`application/json`)
- 에러 응답: `{ "error": { "code": "E-COM-AUTH-1001", "message": "..." } }`
- 에러코드/메시지: `error_message_ko.json` 참조
- SSE 스트림: `text/event-stream`, 이벤트 형식 아래 참조

---

## 1. 인증

### POST /api/v1/auth/register — 익명 기기 등록
```json
// 요청
{ "device_id": "uuid-...", "birth_year": 1995 }
// 응답 200
{ "token": "jwt...", "user_id": "uuid" }
```
- `device_id` 클라이언트가 최초 생성해 보관 (키체인/Preferences)
- `birth_year` 필수 (19세 이상만 가입 — 만 19세 미만 거절 E-COM-AUTH-2001)

### GET /api/v1/auth/me — 내 정보
```json
// 응답 200
{
  "user_id": "uuid",
  "adult_verified": true,
  "has_byok_key": false
}
```

---

## 2. 캐릭터

### GET /api/v1/characters — 목록
```json
// 응답 200
{
  "characters": [
    {
      "id": "uuid",
      "name": "서연",
      "title": "무심한 듯 다정한 소꿉친구",
      "category": "소꿉친구",
      "avatar_url": "https://...",
      "greeting": "오늘 하루도 고생 많았어, 나 보고 싶었지?",
      "age": 24,
      "adult": true,
      "last_message_at": "2026-08-13T09:00:00Z"
    }
  ]
}
```

### POST /api/v1/characters — 생성
```json
// 요청
{
  "name": "서연",
  "title": "무심한 듯 다정한 소꿉친구",
  "category": "소꿉친구",
  "avatar_url": "",
  "greeting": "오늘 하루도 고생 많았어, 나 보고 싶었지?",
  "age": 24,
  "persona": {
    "personality": "무심한 듯 다정함, 말은 짧지만 챙겨줌",
    "speech_style": "반말, 간결, 가끔 놀리기",
    "relationship": "소꿉친구 → 연인 지향",
    "ideal_type": ["웃는 게 예쁜 사람", "솔직한 사람"]
  }
}
// 응답 201: 생성된 캐릭터
```
- `age < 19` → E-COM-VALID-2001 (미성년자 캐릭터 금지)

### GET /api/v1/characters/:id · PUT /api/v1/characters/:id · DELETE /api/v1/characters/:id
- 표준 CRUD (본인 소유 확인 필수)

---

## 3. 대화 규칙 (시스템 프롬프트)

### GET /api/v1/rules — 목록
```json
// 응답 200
{
  "rules": [
    {
      "id": "uuid",
      "name": "반말 1일차",
      "system_prompt": "당신은 사용자의 남친/여친인 'AI 캐릭터'입니다...",
      "is_default": false
    }
  ]
}
```

### POST /api/v1/rules — 생성
```json
// 요청
{
  "name": "반말 1일차",
  "system_prompt": "당신은 ... [캐릭터 설정 자동 삽입 위치: {{persona}}] ..."
}
```
- `{{persona}}` 플레이스홀더 → 서버가 캐릭터 데이터로 치환

### PUT /api/v1/rules/:id · DELETE /api/v1/rules/:id

---

## 4. 모델 카탈로그 & 할당량

### GET /api/v1/models — 모델 목록 (+ 할당량)
```json
// 응답 200
{
  "models": [
    {
      "id": "openrouter/free",
      "name": "OpenRouter Free 라우터",
      "is_free": true,
      "description": "무료 모델 자동 라우팅 (요청당 50~1,000회/일)",
      "traits": ["무료", "자동"],
      "quota": {
        "provider": "openrouter",
        "is_free": true,
        "remaining_today": 43,
        "daily_limit": 50,
        "credit_balance_usd": 0
      }
    },
    {
      "id": "deepseek/deepseek-v4-flash",
      "name": "DeepSeek V4 Flash",
      "is_free": false,
      "description": "빠르고 자연스러운 일상/연애 대화, 가성비 1위",
      "traits": ["빠른 응답", "가성비"],
      "price": { "input_per_1m_usd": 0.09, "output_per_1m_usd": 0.18 },
      "quota": {
        "provider": "openrouter",
        "is_free": false,
        "credit_balance_usd": 3.42,
        "estimated_chats_left": 180
      }
    },
    {
      "id": "custom:my-gemini",
      "name": "내 Gemini",
      "is_free": false,
      "base_url": "https://generativelanguage.googleapis.com/v1beta",
      "description": "사용자 추가 모델",
      "quota": { "provider": "custom", "note": "키 소유자 제공 정보 없음" }
    }
  ]
}
```

### POST /api/v1/models/custom — 커스텀 모델 추가
```json
// 요청
{
  "name": "내 Gemini",
  "model_id": "gemini-2.5-flash",
  "base_url": "https://generativelanguage.googleapis.com/v1beta",
  "api_key_ref": "opt:user-key-gemini",  // 서버 .env 키 또는 사용자 키 식별자
  "api_key": "",                          // 직접 입력(평문) 또는 생략
  "is_free": false,
  "description": "설명"
}
// 응답 201
```

### GET /api/v1/models/custom/:id · PUT · DELETE

---

## 5. 세션 & 대화

### POST /api/v1/sessions — 세션 생성
```json
// 요청
{
  "character_id": "uuid",
  "model_id": "openrouter/free",
  "rule_id": "uuid"
}
// 응답 201
{
  "id": "uuid",
  "character": { "...": "..." },
  "model_id": "openrouter/free",
  "rule_id": "uuid",
  "status": "active",
  "created_at": "..."
}
```

### GET /api/v1/sessions — 세션 목록 (채팅방 목록)
```json
// 응답 200
{
  "sessions": [
    { "id": "uuid", "character_name": "서연", "avatar_url": "...",
      "last_message": "응, 그때 기억나 ㅎㅎ", "last_message_at": "...", "unread": 0 }
  ]
}
```

### GET /api/v1/sessions/:id — 대화 내역
```json
// 응답 200
{
  "id": "uuid", "character_id": "uuid", "model_id": "openrouter/free", "rule_id": "uuid",
  "summary": "중기 요약 텍스트",
  "messages": [
    { "role": "user", "content": "오늘 너무 힘들었어", "created_at": "...", "model": null },
    { "role": "assistant", "content": "ㅠㅠ 얼마나 힘들었어?", "created_at": "...", "model": "openrouter/free" }
  ]
}
```

### POST /api/v1/sessions/:id/model — 모델 전환 (대화 중)
```json
// 요청
{ "model_id": "xiaomi/mimo-v2.5" }
// 응답 200
{ "id": "uuid", "model_id": "xiaomi/mimo-v2.5", "character_id": "uuid" }
```
- 히스토리/기억 유지 (컨텍스트 유지)

### POST /api/v1/sessions/:id/reset — 히스토리 초기화
```json
// 요청
{ "keep_memories": true }
// 응답 200: 메시지 삭제, 장기 기억은 유지 옵션
```

---

## 6. 대화 스트리밍 (핵심)

### POST /api/v1/chat/stream — SSE 스트리밍 대화
```json
// 요청 (Content-Type: application/json)
{
  "session_id": "uuid",
  "content": "오늘 너무 힘들었어 ㅠㅠ",
  "model_id": "openrouter/free"           // 생략 시 세션 현재 모델
}
```

**응답: SSE (text/event-stream)**
```
event: meta
data: {"session_id":"uuid","model_id":"openrouter/free"}

event: token
data: {"text":"ㅠㅠ얼마나"}

event: token
data: {"text":" 힘들었어?"}

event: memory
data: {"kind":"save","content":"사용자가 오늘 회사에서 힘든 일이 있었다"}

event: done
data: {"message_id":"uuid","token_in":320,"token_out":42,"cost_usd":0.0001,
       "quota":{"remaining_today":42,"daily_limit":50,"credit_balance_usd":3.4}}

event: error
data: {"code":"E-COM-MODEL-1001","message":"모델 호출에 실패했습니다. ..."}
```

**서버 내부 처리 순서**
1. 사용자 메시지 DB 저장
2. RAG 검색 (character_memories cosine 상위 5~8)
3. 프롬프트 조합 (페르소나 + 규칙 + 중기요약 + RAG + 최근 30턴)
4. OpenRouter/커스텀 API 호출 (stream) → SSE 릴레이
5. 완료: 메시지 저장(토큰/비용) + 할당량 갱신 + `[MEMORY_SAVE]` 태그 처리 + 30턴마다 중기요약 비동기 갱신

---

## 7. 기억 (메모리)

### GET /api/v1/memories/:characterId
```json
// 응답 200
{
  "memories": [
    { "id": "uuid", "content": "사용자와 지난주 제주도 여행 다녀옴", "type": "long",
      "importance": 0.9, "pinned": true, "created_at": "..." }
  ]
}
```

### POST /api/v1/memories — 수동 추가
```json
// 요청
{ "character_id": "uuid", "content": "생일 3월 14일", "type": "long", "pinned": true }
```

### PUT /api/v1/memories/:id — 편집/고정/중요도 · DELETE /api/v1/memories/:id

---

## 8. 설정 / 키 관리

### POST /api/v1/settings/byok-key
```json
// 요청
{ "provider": "openrouter", "api_key": "sk-or-..." }
// 응답 200: 암호화 저장(AES-GCM), 이후 해당 키로 호출/할당량 표시
```

### DELETE /api/v1/settings/byok-key — 키 제거 (서버 키 모드로 복귀)
### GET /api/v1/settings — 현재 설정 조회

---

## 9. 시스템

### GET /health
```json
// 응답 200
{ "status": "ok", "version": "1.0.0", "db": "up", "models_cache": "fresh" }
```

### GET /debug/logs · /debug/models-cache · /debug/quota-cache (서버 디버그)
- DebugPanel 연동 (운영 외)

---

## 10. 에러코드 목록 (요약)

| 코드 | 메시지 (error_message_ko.json) |
|---|---|
| E-COM-AUTH-1001 | 인증이 필요합니다. 다시 로그인해 주세요. |
| E-COM-AUTH-2001 | 만 19세 이상만 이용할 수 있어요. (미성년자 가입 거절) |
| E-COM-MODEL-1001 | 모델 호출에 실패했습니다. 잠시 후 다시 시도해 주세요. |
| E-COM-MODEL-2001 | 추가된 모델의 API 키가 잘못되었습니다. 설정을 확인해 주세요. |
| E-COM-QUOTA-1001 | 오늘 무료 대화 할당량을 모두 사용했습니다. 모델을 변경하거나 키를 추가해 주세요. |
| E-COM-CHAT-1001 | 대화를 전송하지 못했습니다. |
| E-COM-MEM-1001 | 기억을 저장하지 못했습니다. |
| E-COM-RULE-1001 | 대화 규칙을 불러오지 못했습니다. |
| E-COM-VALID-1001 | 입력값이 올바르지 않습니다. |
| E-COM-VALID-2001 | 캐릭터 나이는 만 19세 이상이어야 합니다. |
| E-SRV-NET-1001 | 서버 연결에 실패했습니다. 네트워크를 확인해 주세요. |