# 톡맨스 (Talkmance) — 작업 추적 (TODO)

> 갱신: 2026-08-20 · 플랫폼 라벨: server / macos / docs
> bd 이슈: Talkmance-server / Talkmance-macos / Talkmance-integration

## 진행 방법
- 각 작업은 PLAN_v1.5의 T-번호와 매핑 (T-97~103)
- 완료 시 `[x]` 마킹 + CHANGELOG에 기록
- bd(beads) 이슈로도 추적 가능 (선택)

## 진행중 (v1.5 — PLAN_v1.5_macos.md)

| T-번호 | 작업 | 플랫폼 | 상태 | 비고 |
|---|---|---|---|---|
| T-97 | Android 제거 (파일/문서/CI/APK) | macos+docs | 완료 | apps/android·android.yml·문서 흔적 삭제, grep 0건 |
| T-98 | NVIDIA 프로브 → 모델 확정 | server | 완료 | **google/gemma-4-31b-it** 확정 (키 교체 완료) |
| T-99 | 서버 NVIDIA 통합 | server | 완료 | config/router/models/env 반영 |
| T-100 | 한글 A 프롬프트 규칙 주입 | server | 완료 | buildChatMessages 규칙 추가 |
| T-101 | 한글 B 후처리 + macOS 토글 | macos+server | 완료 | polish.go + ChatRequest.polish + SettingsView 토글 |
| T-102 | 로컬 검증 | all | 완료 | SSE/polish on-off/회귀/테스트/빌드 전부 통과 |
| T-103 | 커밋 + GitHub 재생성 | docs | 진행중 | 백업/삭제/이력리셋/재생성/Pages/Release |

| T-번호 | 작업 | 플랫폼 | 상태 | 비고 |
|---|---|---|---|---|
| T-90 | 문서 동기화 (TODO/CHANGELOG/세션) | docs | 완료 | 1.2.0 전부 반영 |

## 완료

| T-번호 | 작업 | 플랫폼 | 상태 | 비고 |
|---|---|---|---|---|
| T-39 | 19세 확인 게이트 | macos | 완료 | 첫 실행 NSAlert 1회 + 생성 시트 문구 |
| T-91 | 대화 규칙 UI (rule_id + 선택/관리) | macos+server | 완료 | NewChatSheet Picker + SettingsView 관리 |
| T-92 | 할당량 배지 (새 대화 + 대화창) | macos | 완료 | fetchQuota 5분 캐시 |
| T-93 | 메모리 카드 UI (목록/추가/편집/고정) | macos | 완료 | memoriesSection + MemoryEditSheet |
| T-94 | BYOK 키 설정 UI | macos | 완료 | 목록/등록/삭제, AES-GCM 저장 |
| T-95 | 커스텀 모델 관리 UI | macos | 완료 | 추가/삭제, 서버 CRUD 연동 |
| T-96 | 캐릭터 카테고리 그룹핑 | macos | 완료 | 섹션 + 필터 칩 |
| T-25 | 서버 /debug/* (health/logs) | server | 완료 | DEBUG 모드만 + 링 버퍼 200 |
| T-26 | k6 부하 스크립트 | server | 완료 | p95 1.54ms (임계 300ms), 커스텀 모델 캐시로 해결 |
| T-70 | 연동 스모크 테스트 | all | 완료 | 인증/모델CRUD/채팅SSE/기억자동저장/할당량 전부 통과 |

## 완료

| T-번호 | 작업 | 플랫폼 | 상태 | 비고 |
|---|---|---|---|---|
| T-24 | .env 템플릿 + config + 에러코드 체계 | server | 완료 | config.Load + errs 23종 + 테스트 |
| T-10 | Go 모듈 초기화 + HTTP 서버 + 라우터(/health) | server | 완료 | JSON 로거 + CORS + recover + graceful shutdown |
| T-11 | Neon 연결(pgx) + 마이그레이션 프레임워크 | server | 완료 | pgx/v5 + up/down 트랜잭션 |
| T-12 | 마이그레이션 v001~v002 (8테이블 + pgvector HNSW) | server | 완료 | v002: AES-GCM 키 |
| T-13 | 익명 기기 인증 (JWT) | server | 완료 | jwt/v5 + 등록 API + 미들웨어 |
| T-14 | OpenRouter 모델 카탈로그 동기화 | server | 완료 | 410개/캐시 12h/할당량 5분 |
| T-15 | 모델 카탈로그 API + 커스텀 모델 CRUD | server | 완료 | free/q 필터 + Gemini 전체+free만 정책 (8/13) |
| T-16 | 할당량 조회 + BYOK 키 등록 | server | 완료 | AES-GCM + v002 적용 |
| T-22 | 캐릭터/세션/메시지 CRUD | server | 완료 | 소유자 검증 404 |
| T-23 | 대화 규칙 CRUD | server | 완료 | 기본규칙 삭제거부 |
| T-17 | 채팅 SSE 스트리밍 POST /chat | server | 완료 | 1.0.4, Flusher 버그 수정 |
| T-18 | 프롬프트 조합 엔진 | server | 완료 | 페르소나+규칙+맥락20+인사말+기억블록 |
| T-19 | 장기 기억 (RAG) | server | 완료(대체) | 2-gram 토큰 확장 ILIKE 검색 (pgvector 임베딩 아님) |
| T-20 | 메모리 태그 처리 ([MEMORY_SAVE]) | server | 완료 | 자동 저장 + SSE 제거 |
| T-21 | 중기 요약 (60배수) | server | 완료 | 1.0.5 + 매턴→60배수 수정 |
| T-41 | 서버 AI 캐릭터 생성 API | server | 완료 | generate + avatar 재생성, zen 폴백 |
| T-42 | 서버 구성 선택 설정 | macos | 완료 | 로컬/Render/커스텀 |
| T-43 | macOS AI 캐릭터 생성 시트 | macos | 완료 | AI 자동/직접 2모드 + 미리보기 |
| T-44 | macOS 상세 화면 + 수정 | macos | 완료 | 스토리/관계/시작전대화 + 수정 시트 |
| T-45 | macOS 아바타 재생성 UI | macos | 완료 | DiceBear/AI 토글 |
| T-30 | Xcode 스캐폴드 + 네비게이션 | macos | 완료 | 메뉴바/Dock 토글/디버그 패널 |
| T-31 | API 클라이언트 (URLSession + SSE) | macos | 완료 | 타임아웃 90s + retry |
| T-32 | 캐릭터 목록/생성 화면 | macos | 완료 | 생성 시트 + 삭제 |
| T-33 | 대화 시작 설정 뷰 (모델 선택) | macos | 완료 | 규칙 선택은 T-91 |
| T-34 | 채팅 UI (말풍선/스트리밍) | macos | 완료 | 개행/표현 구분/다시 요청/자동발화 |
| T-35 | 대화 중 모델 전환 UI | macos | 완료 | 헤더 메뉴 |
| T-38 | 로컬 캐시 + 키체인 | macos | 완료 | 이미지 NSCache+URLCache, 키는 서버 AES-GCM 저장(키체인 불필요) |

## 대기 (T-36/37 대체됨 — 전부 완료)

| T-번호 | 작업 | 플랫폼 | 상태 |
|---|---|---|---|
| T-36 | 설정 화면 (BYOK/모델/규칙) | macos | T-91/94/95로 대체 (완료) |
| T-37 | 커스텀 모델 추가 폼 | macos | T-95로 대체 (완료) |
| T-70 | 연동 스모크 테스트 | all | 완료 | 인증/모델CRUD/채팅SSE/기억자동저장/할당량 전부 통과 |
| T-71 | 장기 기억 RAG 테스트 (TC-MEM-001) | server | 완료 | 초밥 기억 주입→응답 참조 확인, 자동 저장 동작 |
| T-72 | 19금 대화 스모크 | server | 완료 | 성인 허용/비성인 차단 (금지 문구 추가 수정) |
| T-73 | 할당량 표시 검증 | macos | 완료 | 서버 22/50 = macOS 배지 일치 |
| T-74 | E2E 시나리오 문서화 | all | 완료 | docs/e2e/PLAN.md — 서버/부하/연동스모크/macOS |
| T-75 | 성능 검증 (k6 + PERF 로그) | server | T-26 후 진행 |
| T-76 | 문서 최종 갱신 (CHANGELOG/TODO close) | docs | 완료 | CHANGELOG [1.2.0] 전 섹션 기록 |
| T-77 | GitHub Releases 배포 (DMG) | all | 완료 | 태그 v* 시 자동 Release — macos.yml |