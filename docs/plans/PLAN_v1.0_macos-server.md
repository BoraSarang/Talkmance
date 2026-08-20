# 톡맨스 (Talkmance) — 구현 계획 v1.0 (macOS + Server)

> 버전: PLAN_v1.0 · 작성일: 2026-08-13 · 상태: 초안
> 플랫폼: [Server(Go)], [macOS]

---

## 1. 개요

- **목표**: AI 연애 채팅 앱 톡맨스 v1.0 MVP 완성 (캐릭터/대화/규칙/모델 관리/장기 기억/19금/배포)
- **원칙**: 문서 우선 → 코드 구현 → 검증(빌드+E2E) → 배포
- **비용 예산**: 서비스는 $0 (Render/Neon 무료 티어 + BYOK)

## 2. 결정 사항 (확정)

1. 플랫폼: macOS(SwiftUI) — 네이티브
2. 백엔드: Go (LLM 프록시/스트리밍에 유리 — 메모리 ~20MB, p99 안정)
3. DB: Neon PostgreSQL + pgvector (3계층 기억 RAG)
4. AI 게이트웨이: OpenRouter 기본 + 커스텀 모델 수동 추가 (BYOK 포함)
5. 배포: GitHub Releases (DMG), 스토어 미배포
6. 성인 콘텐츠: 수위 제한 없음 (19세 이상 확인 문구만)
7. 계정: 익명 기기 ID 기반 (v1.0)

## 3. 아키텍처 (요약)

```
macOS 앱 ─HTTPS/SSE─→ Go 서버(Render) ─┬→ OpenRouter API
                                      ├→ 커스텀 모델 API (Gemini 등)
                                      └→ Neon PostgreSQL + pgvector (기억 RAG)
```

- 서버가 모든 LLM 호출 프록시 + 프롬프트 조합 + RAG 검색 + 할당량 추적
- 클라이언트는 채팅 UI + 설정 전담

## 4. 구현 단계 (T-번호)

> 갱신: 2026-08-13 — 진행 순서 조정: T-24(에러코드/config)와 T-22~23(CRUD)을 앞당김 (의존성 기반)
> bd 이슈: Talkmance-server / Talkmance-macos / Talkmance-integration

### Phase 0: 저장소/문서 초기화
- [x] T-01: monorepo 스캐폴드 (apps/macos, apps/server, packages/shared, scripts)
- [x] T-02: docs/TODO.md 전체 T-번호 등록
- [x] T-03: error_message_ko.json 초기 세트 작성 (E-SRV/COM-AI/COM-CHAT/COM-QUOTA)

### Phase 1: Go 서버 코어
- [ ] T-24: 서버 .env 템플릿 + config 로딩 + 에러코드 체계 적용  ← 앞당김
- [ ] T-10: Go 모듈 초기화 + stdlib HTTP 서버 + 라우터 (/health)
- [ ] T-11: Neon 연결 (pgx) + 마이그레이션 프레임워크 (up/down SQL)
- [ ] T-12: 마이그레이션 v001 — users, characters, chat_sessions, messages, prompt_rules, user_models, character_memories(pgvector)
- [ ] T-13: 익명 기기 인증 (JWT 발급/검증, blocks)
- [ ] T-14: OpenRouter 모델 카탈로그 동기화 (`GET /models` 캐시, `:free` 감지)
- [ ] T-15: 모델 카탈로그 API + 커스텀 모델 CRUD (`user_models`)
- [ ] T-16: 할당량 조회 (OpenRouter `/auth/key`, 캐시 5분) + BYOK 키 등록/암호화 저장
- [ ] T-22: 캐릭터 CRUD + 대화 세션 CRUD + 메시지 저장 (토큰/비용 기록)  ← 앞당김
- [ ] T-23: 대화 규칙 CRUD (`prompt_rules`)  ← 앞당김
- [ ] T-17: 채팅 SSE 스트리밍 `POST /chat/stream` (OpenRouter stream 릴레이)
- [ ] T-18: 프롬프트 조합 엔진 (페르소나 + 규칙 + 기억 + 히스토리)
- [ ] T-19: RAG 파이프라인 (임베딩 생성, 벡터 저장, cosine 검색 상위 N개)
- [ ] T-20: 메모리 태그 처리 (`[MEMORY_SAVE]` 감지 → character_memories 저장, 포인트 추출)
- [ ] T-21: 중기 요약 (30턴마다 요약 LLM 호출 → chat_sessions.summary 갱신)
- [ ] T-25: /debug/* (logs, cache, quota) — DebugPanel 표준
- [ ] T-26: k6 부하 테스트 스크립트 (p95 < 300ms, error < 1%)
- [ ] T-27: Dockerfile (Go scratch) + Render 배포 설정

### Phase 2: macOS 앱 (SwiftUI)
- [ ] T-30: Xcode 프로젝트 스캐폴드 + 네비게이션 구조 (홈/채팅/설정)
- [ ] T-31: API 클라이언트 (URLSession + SSE stream 파서)
- [ ] T-32: 캐릭터 카테고리 → 목록 → 생성 화면
- [ ] T-33: 대화 시작 설정 뷰 (모델 선택 + 규칙 선택 + 할당량 표시)
- [ ] T-34: 채팅 UI (말풍선, 스트리밍 표시, 타임스탬프)
- [ ] T-35: 대화 중 모델 전환 UI (헤더 탭 → 모델 목록 → 즉시 전환)
- [ ] T-36: 설정 화면 (OpenRouter 키 BYOK, 모델 관리, 규칙 관리)
- [ ] T-37: 커스텀 모델 추가 폼 (이름/ID/BaseURL/키/설명/무료여부)
- [ ] T-38: 로컬 캐시 (채팅 미리보기/설정) + 키체인 키 저장
- [ ] T-39: 19세 이상 확인 게이트 + 히스토리 초기화 버튼
- [ ] T-40: DMG 빌드 스크립트 (ad-hoc) + GitHub Actions 워크플로

### Phase 3: 통합/검증/E2E
- [ ] T-70: 서버-클라이언트 연동 스모크 테스트 (캐릭터→대화→모델전환→기억회상)
- [ ] T-71: 장기 기억 RAG 테스트 (TC-MEM-001: "지난주 초밥" 회상 시나리오)
- [ ] T-72: 19금 대화 스모크 (TC-ADULT-001)
- [ ] T-73: 할당량 표시 검증 (Free 모델 50/일 표시, BYOK 잔액 표시)
- [ ] T-74: E2E 시나리오 작성 (docs/e2e/PLAN.md) + Playwright(서버 API 기준)
- [ ] T-75: 성능 검증 (k6 + 메모리 측정 + DebugPanel PERF 로그)
- [ ] T-76: 도큐먼트 최종 갱신 (CHANGELOG, TODO close, session 로그)
- [ ] T-77: GitHub Releases 배포 (macOS DMG)

## 5. 테스트 계획 (TC-번호)

| ID | 대상 | 시나리오 | 기대 결과 |
|---|---|---|---|
| TC-MEM-001 | 서버 RAG | 대화 10턴 → "초밥" 언급 → 새 세션에서 "초밥" 언급 → 답변에 기억 반영 | 기억 언급 포함 |
| TC-MEM-002 | 서버 중기 | 대화 40턴 이상 → summary 갱신 확인 | summary 업데이트 |
| TC-MODEL-001 | 서버 카탈로그 | free 모델 목록 조회 | :free 감지 + 설명 포함 |
| TC-MODEL-002 | 커스텀 | 커스텀 모델 추가 → 대화에서 사용 → 전환 | 동작 확인 |
| TC-QUOTA-001 | 서버 할당량 | 키 잔액/일일 한도 조회 | 숫자 반환 |
| TC-CHAT-001 | SSE | 메시지 전송 → 스트리밍 수신 → DB 저장 | 토큰/비용 기록 포함 |
| TC-ADULT-001 | 채팅 | 성인 주제 프롬프트 | 수위 제한 없이 응답 |
| TC-RULE-001 | 규칙 | 규칙 A/B 생성 → 변경 → 적용 확인 | 프롬프트 조합 변화 |

## 6. 롤백 계획

- 서버: Render 재배포 (이전 커밋) + `migrations down`
- 클라이언트: GitHub Releases 이전 버전 유지 → 재배포
- DB: 마이그레이션 up/down 필수 작성 (파괴적 변경 금지)

## 7. 성능 예산

| 지표 | 목표 |
|---|---|
| 서버 첫 토큰 | ≤3s (Free 모델 기준) |
| RAG 검색 | ≤100ms |
| 서버 메모리 | ≤256MB (Render 무료 한계) |
| macOS | 콜드스타트 ≤2s, 메모리 ≤250MB |

## 8. 에러코드 범위 (v1.0)

- E-SRV-* : 서버 공통
- E-COM-AUTH-*, E-COM-MODEL-*, E-COM-CHAT-*, E-COM-MEM-*, E-COM-QUOTA-*, E-COM-RULE-*
- 상세: error_message_ko.json

## 9. 산출물 목록

- docs/PRD.md, docs/DESIGN.md, docs/API/ENDPOINTS.md, docs/TODO.md, docs/CHANGELOG.md
- docs/AI_MODELS.json, error_message_ko.json
- apps/server, apps/macos, scripts/, .github/workflows/