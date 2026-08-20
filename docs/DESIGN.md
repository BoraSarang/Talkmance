# 톡맨스 (Talkmance) — 기술 설계 문서 (DESIGN)

> 버전: v1.5.0 · 작성일: 2026-08-13 · 플랫폼: macOS + Server(Go)

---

## 1. 시스템 아키텍처 개요

```
┌─────────────┐
│  macOS 앱     │   ← 네이티브 (SwiftUI)
│  SwiftUI      │
└──────┬──────┘
       │
       │  HTTPS REST + SSE (스트리밍)
       ▼
┌─────────────────────────────────┐
│       Go 백엔드 (Render 무료)      │
│  • /api/v1/* REST 라우터          │
│  • 모델 프록시 (OpenRouter + 커스텀) │
│  • 페르소나/프롬프트 조합 엔진       │
│  • RAG 장기 기억 파이프라인         │
│  • 할당량 추적                     │
│  • SSE 스트림 릴레이               │
└──────┬──────────────────┬───────┘
       │                  │
┌──────▼────────┐   ┌──────▼─────────┐
│  Neon/PostgreSQL│   │  pgvector      │
│  (관계형)       │   │  (장기 기억 RAG) │
└───────────────┘   └────────────────┘
```

### 1.1 기술 스택 확정

| 영역 | 기술 | 비고 |
|---|---|---|
| macOS | Swift 6 + SwiftUI, URLSession + Async/SSE | DMG 배포 (개발자 서명 없음: ad-hoc 서명) |
| Backend | Go 1.24+ (net/http, stdlib 우선), SSE 직접 구현 | Render 무료 티어 (512MB RAM) |
| DB | Neon PostgreSQL 16 + pgvector 확장 | 무료 티어 0.5GB |
| 통신 | REST (JSON) + SSE (text/event-stream) | 모델 호출은 서버 경유 |
| 배포 | GitHub Actions → GitHub Releases (DMG) | - |

### 1.2 저장소 구조 (monorepo)

```
Talkmance/
├── apps/
│   ├── macos/          # SwiftUI 앱 (Xcode 프로젝트)
│   └── server/         # Go 백엔드
├── packages/
│   └── shared/         # 공유 타입/상수 (JSON 스키마, 모델 카탈로그)
├── docs/               # 문서 우선 원칙 문서들
├── scripts/            # 빌드/배포/검증 스크립트
└── error_message_ko.json
```

---

## 2. 데이터 모델 (PostgreSQL + pgvector)

### 2.1 테이블

| 테이블 | 주요 컬럼 | 설명 |
|---|---|---|
| `users` | id UUID, device_id, birth_year, adult_verified, created_at | 익명 기기 ID 기반 |
| `characters` | id, user_id, name, title, avatar_url, category, persona(JSON), greeting, age, adult, created_at | 캐릭터 (페르소나 JSON 포함) |
| `character_memories` | id, character_id, type(short/medium/long), content, embedding vector(1536), importance, pinned, created_at | 기억 + 벡터 |
| `chat_sessions` | id, character_id, user_id, model_id, rule_id, status, summary(중기 요약), relation_state, created_at, updated_at | 대화방 |
| `messages` | id, session_id, role(user/assistant/system), content, model, token_in, token_out, cost, created_at | 대화 메시지 |
| `prompt_rules` | id, user_id, name, system_prompt(템플릿), json_schema, is_default | 대화 규칙 |
| `user_models` | id, user_id, name, model_id, base_url, api_key_ref, is_free, description, enabled | 커스텀 모델 (+OpenRouter 모델 캐시) |
| `profile_relations` | session_id, stage(1-5), favorability, milestones | 관계 단계/호감도 (v1.2) |

### 2.2 벡터 스키마 예시

```sql
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE character_memories (
  id UUID PRIMARY KEY,
  character_id UUID NOT NULL,
  mem_type TEXT NOT NULL,            -- short / medium / long
  content TEXT NOT NULL,
  embedding VECTOR(1536),            -- text-embedding-3-small
  importance REAL DEFAULT 0.5,
  pinned BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX ON character_memories USING hnsw (embedding vector_cosine_ops);
```

### 2.3 임베딩 전략

- 모델: OpenRouter `text-embedding-3-small` (또는 무료 대체) — 서버에서 배치 생성
- 저장: 메시지 5개 단위/주요 포인트 메시지 → 벡터화 (비용 절감: 전체 대화가 아닌 **포인트 메시지 + 주기적 요약**만 벡터화)
- 검색: 사용자 메시지 임베딩 → `character_memories`에서 cosine 유사도 상위 5~8개 검색 → 시스템 프롬프트에 삽입

---

## 3. 기억 시스템 (3계층 아키텍처)

| 계층 | 구현 | 동작 |
|---|---|---|
| **단기** | 최근 메시지 20~30턴 | API 요청에 그대로 포함 (원문) |
| **중기** | `chat_sessions.summary` 주기적 갱신 (30턴마다 or 15분마다) | 요약 LLM 호출 별도 실행 (비용 낮은 모델) |
| **장기** | pgvector RAG 상위 N개 검색 | 매 사용자 메시지마다 검색 → 관련 기억 삽입 |

### 3.1 메모리 카드 (포인트 자동 추출)

- 시스템 프롬프트에 "대화 중 다음 내용을 메모리 저장 대상으로 표시하라" 지시
- 서버가 LLM 응답에 `[MEMORY_SAVE] ...` 특수 태그 감지 → `character_memories`에 저장
- 사용자가 직접 추가/편집/고정(pin) 가능

### 3.2 시스템 프롬프트 조합 순서

```
[페르소나 블록]  ← 캐릭터 persona (성격/말투/관계/이상형)
[대화 규칙 블록] ← 선택된 prompt_rule (반말, 짧은 문장, 이모지 등)
[기억 블록]      ← 중기 요약 + RAG 상위 검색 결과 (관계 최신 상태 포함)
[대화 히스토리]  ← 최근 20~30턴
```

---

## 4. 모델 관리 & 할당량

### 4.1 모델 카탈로그 (서버)

- OpenRouter `GET https://openrouter.ai/api/v1/models` 12시간마다 동기화 캐시
- `:free` suffix → `is_free=true` 자동 표시
- 모델별 설명: `docs/AI_MODELS.json`에 수동 curate (한국어/감성/속도/가격 필드)
- **커스텀 모델**: `user_models` 테이블에 사용자 등록 (OpenRouter 외 Base URL 지원 — Gemini/OpenAI/Claude 직접 등)
- **NVIDIA NIM** (v1.5): `google/gemma-4-31b-it` — `NVIDIA_API_KEY` 설정 시 카탈로그 노출(`models.go` nvidiaCatalog) + 폴백 체인 1순위 (NVIDIA → Gemini → zen → gpt-oss → openrouter/free)
- **한국어 다듬기** (v1.5): A(항상) 프롬프트 규칙 주입 + B(토글) `polish` 파라미터 후처리 — `polish.go` 편집거리 30% 가드, 실패 시 원문 폴백

### 4.2 할당량 추적

- OpenRouter `GET /api/v1/auth/key` (키 잔액 + 하루 요청수) → 서버에서 주기 캐시 (5분)
- 서버 키 모드: 서버 .env `OPENROUTER_API_KEY`
- BYOK 모드: 사용자 키를 설정에서 입력 → 서버가 사용자별 키 저장 (암호화) → 해당 키로 호출 및 할당량 조회
- 응답: `{ model_id, is_free, quota: { remaining_today, daily_limit, credit_balance } }`

### 4.3 요청 라우팅 플로우

```
클라이언트 → POST /chat/stream
  → 서버: 사용자 키 or 서버 키 결정
  → RAG 검색 → 프롬프트 조합
  → OpenRouter API (stream=true) → SSE 릴레이 → 클라이언트
  → 완료 시: 메시지 저장 + 토큰/비용 기록 + 할당량 갱신 + 메모리 태그 처리
```

---

## 5. API 설계 요약 (상세: `docs/api/ENDPOINTS.md`)

| 메서드 | 경로 | 설명 |
|---|---|---|
| POST | /api/v1/auth/register | 익명 기기 등록 |
| GET | /api/v1/characters | 캐릭터 목록 |
| POST | /api/v1/characters | 캐릭터 생성 |
| GET/PUT/DELETE | /api/v1/characters/:id | 캐릭터 관리 |
| GET | /api/v1/sessions | 세션 목록 |
| POST | /api/v1/sessions | 세션 생성 (모델+규칙 선택) |
| POST | /api/v1/chat/stream `SSE` | 대화 스트리밍 |
| GET | /api/v1/models | 모델 카탈로그 (Free+커스텀+할당량) |
| POST | /api/v1/models/custom | 커스텀 모델 추가 |
| GET/PUT | /api/v1/rules | 대화 규칙 CRUD |
| GET | /api/v1/memories/:characterId | 기억 목록 |
| POST | /api/v1/memories | 메모리 수동 추가 |
| POST | /api/v1/settings/keys | BYOK 키 등록 |
| GET | /api/v1/health | 헬스 체크 |

---

## 6. 클라이언트 설계

### 6.1 macOS (SwiftUI)

- 화면: 캐릭터 카테고리 → 캐릭터 목록 → 채팅방 목록 → 채팅창 → 설정 (모델 관리, 규칙 관리, 키 관리)
- 네트워크: URLSession + SSE 파서 (URLSession data task + AsyncStream)
- 로컬 캐시: Core Data (채팅 미리보기, 설정) / 서버가 진실 공급원
- 키체인: BYOK 키 저장 (가능 시), 설정은 UserDefaults
- **레이아웃 원칙 [2026-08-13 확정]**: 모든 창·시트·패널의 콘텐츠는 **기본 위로 정렬(top-aligned)**. 빈 상태(ContentUnavailableView 등)는 `VStack(spacing:0){ 콘텐츠; Spacer() }`로 상단 배치. 중앙 정렬 금지. (`frame(maxHeight:.infinity, alignment:.top)`은 ContentUnavailableView 내부 자체 중앙 배치로 무효 — Spacer 방식 필수)

### 6.2 공통 UX 흐름

```
홈(캐릭터 카테고리) → 캐릭터 선택 → [대화 시작 설정] →
   ┌ 모델 선택 (할당량 표시) ┐
   └ 대화 규칙 선택          ┘
→ 채팅창 (말풍선) → 우측 상단 모델 전환 UI (할당량 표시)
```

---

## 7. 성능 예산

| 지표 | macOS | Server |
|---|---|---|
| 콜드 스타트 | ≤2.0s | Render 무료 (슬립 시 ~50s) |
| 첫 토큰 (Free 모델) | ≤3s | - |
| 메모리 | ≤250MB | ≤256MB |
| 프레임 | 60fps | - |
| RAG 검색 응답 | - | ≤100ms |

---

## 8. 배포 전략

### 8.1 macOS DMG (GitHub Releases)

- `xcodebuild archive` → `dmg` (ad-hoc 서명, `--deep --force`)
- 사용자: 우클릭 → 열기 (게이트키퍼 우회 안내 문서화)

### 8.2 서버 (Render)

- `dockerfile` 기반 배포 (Go 바이너리 스크래치 이미지)
- env: `DATABASE_URL`(Neon), `OPENROUTER_API_KEY`, `JWT_SECRET`
- 무료 티어: 750시간/월, 15분 유휴 시 슬립

---

## 9. 보안

- API 키: 서버 .env (서버 키) + 사용자 키(AES-GCM 암호화 저장, `ENC_KEY` .env)
- JWT: 기기 인증 (access 30일 — 개인 서비스 단순화)
- HTTPS 필수 (Render 기본)
- 민감 정보 로그 금지 (`DebugLogger` 레벨에 키 마스킹)

---

## 10. 롤백 계획

- 서버: 이전 커밋 재배포 (Render 롤백) + DB 마이그레이션 up/down
- 클라이언트: 이전 DMG 재배포 (GitHub Releases 유지)
- DB: 스키마 변경 시 `migrations/` up/down SQL 필수

---

## 11. 디자인 에셋 정책 (캐릭터 & 아바타)

> 디자인 문제가 생겼을 때(유료 에셋 금지, 라이선스 문제, 직접 제작 부담) 아래 무료 소스를 우선 사용한다.

### 11.1 원칙

- **무료·라이선스 허용 에셋만** 사용 (상업적 이용 가능 CC0/MIT 기준)
- 직접 제작은 **마지막 수단** (AI 생성 이미지 제외 — 저작권 불명확)
- 에셋 선택 시 `docs/CHANGELOG.md`에 출처 + 라이선스 기록 필수
- 2D 픽셀 스타일로 통일 (macOS 메뉴바 아이콘은 `scripts/gen_app_icons.py`의 파스텔 톤과 조화)

### 11.2 무료 픽셀 에셋 (캐릭터/배경/타일셋)

| 소스 | URL | 특징 |
|------|-----|------|
| itch.io (무료 태그) | https://itch.io/game-assets/free | 커뮤니티 무료 2D 픽셀 캐릭터/배경 타일셋, 라이선스별 확인 |
| Kenney.nl | https://kenney.nl/assets | CC0 전용, 캐릭터/타일셋/UI 팩 대량 |
| OpenGameArt | https://opengameart.org | CC0/CC-BY 혼합, 픽셀 아트 대량 |

- 사용 규칙: 다운로드한 에셋은 `assets/characters/` 하위에 출처 폴더로 보관
- 캐릭터 이미지 → 서버 `characters.avatar_url`에 연결 (정적 파일 또는 URL)
- 배경 타일셋 → macOS 채팅 배경에 사용 가능

### 11.3 DiceBear API (아바타 기본값)

- 이미지 없이 아바타가 필요할 때 서버에서 URL만 저장:
  - 기본: `https://api.dicebear.com/9.x/adventurer/svg?seed={캐릭터이름}`
  - 대체 스타일: `adventurer-neutral`(중성), `bottts`(로봇), `avataaars`, `pixel-art`, `big-smile`, `notionists`, `thumbs`
  - 커스텀 배경색: `&backgroundColor=b6e3f4` (파스텔)
  - 픽셀 느낌: `https://api.dicebear.com/9.x/pixel-art/svg?seed={이름}`
- 저장 규칙: `characters.avatar_url`에 DiceBear URL 저장, 클라이언트는 SVG 그대로 렌더링 (다운로드 불필요)
- 서버 동작: 캐릭터 생성 시 `avatar_url`이 비어 있으면 `pixel-art` 스타일 기본 URL 자동 생성 (T-22 이후 보강)

### 11.4 플랫폼별 적용

| 플랫폼 | 적용 |
|--------|------|
| macOS | 캐릭터 선택 목록 + 채팅 헤더 아바타 (SVG 렌더링, `NSImage` 변환) |
| 서버 | `avatar_url` 필드 저장/검증만 (에셋 서빙 없음 — 외부 URL) |

### 11.5 라이선스 체크리스트 (심사 대비)

- [ ] 출처 사이트 + 라이선스(CC0/CC-BY/무료商用) 명시
- [ ] DiceBear: MIT 라이선스 (스타일별 아티스트 크레딧 포함 시 표기)
- [ ] itch.io 에셋: 페이지 라이선스 태그 확인 (상업적 사용 가능 여부)
- [ ] 원본/수정본 구분 저장 (수정 시 원본 보존)

---