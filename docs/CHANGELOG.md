# 톡맨스 (Talkmance) — 변경 이력 (CHANGELOG)

> 형식: `[플랫폼] 타입: 설명 (에러코드)`

## [1.5.0] — 2026-08-20

### [docs] chore: Android 플랫폼 전면 제거 (T-97)
- `apps/android/`·`.github/workflows/android.yml`·`docs/plans/PLAN_v1.4_android.md`·`docs/screenshots/android/`·`assets/icon/android/` 삭제
- README/PRD/DESIGN/e2e PLAN/TODO/CHANGELOG/AI_MODELS 에서 Android 흔적 정리 — `grep -rn "android"` 0건
- `.gitignore` Android 항목 제거 + `.agent/` 추가 (세션 로그 추적 방지)
- `PLAN_v1.0_macos-android-server.md` → `PLAN_v1.0_macos-server.md` 축약
- `install_and_run.sh`·`scripts/gen_app_icons.py` macOS 전용으로 정리 (아이콘 재생성 성공)

### [server] feat: NVIDIA NIM 무료 모델 통합 (T-98, T-99)
- 모델 확정: `google/gemma-4-31b-it` (프로브: 200 + 스트림 지원, 한국어 우수 / mistral-nemotron·nemo-12b 탈락)
- `config.go` NVIDIA_API_KEY, `router.go` nvidiaBaseURL/nvidiaModel + chatOnce·chat 폴백 1순위, `models.go` nvidiaCatalog
- `.env`/`.env.example` NVIDIA_API_KEY 반영 (키체인 교체 완료 — 기존 키 무효 403 → 신규 키 정상)
- 채팅 스트림: Gemini 전용 GeminiStream 대신 범용 ChatStream 재사용 (NVIDIA는 OpenAI 호환)

### [server] feat: 한글 대화 품질 A+B (T-100, T-101)
- A(항상): buildChatMessages에 korean-humanizer 규칙 주입 — 이모지/줄표/번역체/AI 상투어/3의 법칙/접속사·종결어미 반복 금지 + 구어체 리듬 필수
- B(토글, 기본 끔): `polish` 요청 파라미터 + `polish.go` 후처리 — 이모지/줄표/AI 상투어 제거, 반복 축약(ㅋㅋㅋ→ㅋㅋ, !!!→!!), 편집거리 30% 가드(초과 시 원문 폴백), [MEMORY_SAVE] 태그 보존
- macOS: SettingsView "한국어 다듬기(B)" 토글(기본 끔) → ServerConfig.polishEnabled(UserDefaults) → chatStream 전달
- polish on 시 스트리밍 대신 최종 텍스트 1회 전송 (원문과 같아도 전송 보장)

### [server] test: polish 단위 테스트 (T-102)
- `polish_test.go` 6케이스 — 이모지/줄표/반복축약/AI상투어 가드/공백/MEMORY_SAVE 보존 전부 통과
- Go 전체 테스트 통과 + macOS Debug 빌드 성공 + 로컬 SSE 검증 (polish on/off, 성인/비성인 회귀)

## [1.2.0] — 2026-08-13

### [server] feat: 배포 설정 (T-27)
- `apps/server/Dockerfile` — golang:1.26-alpine → distroless 2단계 (CGO=0, nonroot, PORT 8080)
- `apps/server/render.yaml` — Render Blueprint (시크릿 env 대시보드 설정)

### [macos] feat: DMG 빌드 + 릴리스 (T-40, T-77)
- `scripts/build-dmg.sh` — xcodebuild Release → hdiutil UDZO (실빌드 2.0MB 확인)
- `.github/workflows/macos.yml` — macos-14 러너, 태그 v* 시 GitHub Release 자동 등록

### [server] perf: k6 부하 테스트 + 커스텀 모델 캐시 (T-26)
- k6 실측 p95 827ms → **1.54ms** (임계 300ms), 오류 0% — 원인: 원격 DB(Neon) 왕복 ~200ms × 매 요청
- 해결: `CustomModelCache` — 사용자별 커스텀 모델 목록 5초 TTL 캐시 (CRUD 시 무효화)

### [server] fix: 비성인 캐릭터 성인 대화 차단 (T-72)
- 시스템 프롬프트에 adult=false 시 "성인·선정적 대화 금지" 명시 누락 → 모델이 성인 대화에 응답
- `buildChatMessages`: adult 여부별 명시 문구 추가, 재검증 통과 (회피 응답 확인)

### [server] test: 연동 스모크 (T-70) + RAG (T-71) + 할당량 (T-73)
- 신규 기기 등록 → 커스텀 모델 CRUD(캐시 무효화) → 채팅 SSE 자동발화 → 기억 자동 저장 → 할당량
- TC-MEM-001: 기억 3건 저장 → "초밥" 질문 시 저장된 기억 참조 응답 확인

## [1.1.1] — 2026-08-13 (버그 수정 + 사용량 정확화)

### [server] fix: Gemini OpenAI 호환 엔드포인트 v1beta 전환
- `v1main/openai`가 404로 죽어 자동 발화 포함 모든 Gemini 호출 실패 → `v1beta/openai`로 전환 (curl 실증: v1main 404, v1beta 200)
- 재검증: 자동 발화 SSE 정상 응답

### [server] fix: 캐릭터 수정 204 → 수정본 JSON 반환
- `PUT /characters/{id}`가 204(빈 본문) 반환 — 앱이 수정된 캐릭터 디코딩 시도 → "데이터 형식 오류"
- 수정 후 `GetCharacter`로 수정본 반환 (E-COM-VALID-1001 해소)

### [server] feat: 무료 티어 일일 카운터 (T-92 확장)
- OpenRouter는 무료 잔여 요청 수를 제공하지 않아 서버가 `freeDailyLimit=50` 자체 카운트
- `GET /quota` 응답에 `free_used_today` / `free_remaining` / `free_limit_daily` 추가
- DB 영속 (마이그레이션 v003 `daily_usage` 테이블) — 서버 재시작에도 유지

### [server] fix: free 카운터 DB 로드 버그
- `NewFreeDailyCounter`가 `date`를 오늘로 초기화 → `loadLocked`의 날짜 비교에 걸려 DB 로드가 한 번도 실행되지 않던 문제 → 생성자 `date=""`로 수정 (기동 직후 실제 사용량 표시)

### [server] feat: 채팅 실패 SSE 에러에 detail 추가
- 폴백 체인별 실패 사유 수집 (`gemini-3-flash-preview: ... | gpt-oss-20b:free: orapi: 채팅 응답 429 ...`)
- SSE `error` 이벤트에 `detail` 필드로 전달 (디버그용)

### [macos] feat: 채팅 실패 사유 표시
- 실패 말풍선 아래에 상세 사유(폴백별 원인) 회색 작은 글씨로 표시
- 디버그 패널에 `[E-COM-MODEL-1001] ... — 상세: ...` 로그 기록

### [macos] feat: 할당량 배지 — 선택 모델 기준 표시
- Gemini/Gemma 모델: "Gemini 무료 (제한 없음)" / `:free` 모델: "무료 N/50회" / 유료: "잔여 $X"
- 새 대화 시트 동일 적용

### [macos] fix: 카테고리 칩 중복 표시
- "기타"가 2개 표시되던 중복 버그 수정 (표준 카테고리 + 사용자 정의 합치기 로직)
- 생성 시트(AI/직접) + 수정 시트에 카테고리 Picker 추가

### [macos] fix: Shift+Enter 개행
- TextField(axis:.vertical)가 Shift+Return을 "전체 선택"으로 처리 → onKeyPress에서 가로채 NSTextView에 개행 직접 삽입

### [macos] fix: "다시 요청" 버튼 위치
- 실패 말풍선 아래 → 입력창 오른쪽 끝(전송 버튼 옆) 이동, 실패 메시지 시 활성화 + 말풍선에 안내 문구

### [macos] feat: 대화 삭제 UI
- 대화방 목록 행 휴지통 버튼 + 우클릭 "대화 삭제" (확인 다이얼로그, DELETE /sessions/{id})

### [macos] feat: 디버그 패널 자동 스크롤
- 로그 탭 "자동 스크롤" 체크박스(기본 켬), 새 로그 시 맨 아래로 스크롤 (ScrollViewReader)

### [docs] fix: Wordville 리그 뱃지 폭 (별도 프로젝트)
- 고정 44pt 캡슐 → 텍스트 글자 수 기반 동적 폭 (다이아몬드 등 글자 잘림 수정)

## [1.1.0] — 2026-08-13 (v1.3 기능 완료 — T-39, T-91~96, T-25/26)

### [server] fix: 전 API 404 버그 수정
- `New()`에서 `s.routes()` 미호출 → mux가 비어 모든 API 404. Go 링커가 미호출 함수 dead-code 제거로 바이너리에도 라우트 문자열 부재 (grep 0건)
- `s.routes()` 호출 추가로 해결, 전 엔드포인트 정상 응답 확인 (E-SRV-*)

### [server] feat: 모델 목록 정책 재정의 (Gemini + free 전용)
- `geminiCatalog` 13종 하드코딩 (gemini-3-flash-preview / 3.5-flash / 3.5-flash-lite / 3.6-flash / 3.1-flash-lite / 3.1-pro-preview / flash-latest / flash-lite-latest / pro-latest / 2.5-flash / 2.5-pro / gemma-4-26b-a4b-it / gemma-4-31b-it) — `is_free=true`로 전체 노출
- OpenRouter/OpenCode 카탈로그는 `:free` 접미사 모델만 노출 (410개 → 16개)
- `GET /api/v1/models` 총 29개, `?free=true` 시 Gemini 제외

### [server] feat: 디버그 전용 엔드포인트 (T-25)
- `GET /api/v1/debug/health` + `GET /api/v1/debug/logs?n=` — `log_level=debug`일 때만 라우트 등록
- 로거에 최근 200건 링 버퍼 추가 (`log.Recent`)

### [macos] feat: 19세 확인 게이트 (T-39)
- 첫 실행 시 NSAlert (미성년자 캐릭터 금지 문구), 거부 시 앱 종료, `adultGatePassed` 1회 저장
- 캐릭터 생성 시트 성인 토글 2곳에 안내 문구 추가

### [macos] feat: 대화 규칙 UI (T-91)
- 새 대화 시트: 규칙 Picker + 기본 규칙 프리셋 → `POST /sessions` rule_id 전달
- 설정: 규칙 목록/편집/삭제/새 규칙 (RuleEditSheet)

### [macos] feat: 할당량 배지 (T-92)
- `fetchQuotaCached()` 5분 캐시, 대화 헤더 "잔여 $X.XXXX" 배지 + 새 대화 시트 잔여 표시

### [macos] feat: 메모리 카드 UI (T-93)
- 캐릭터 상세: 기억 목록 (장기/단기 배지) + 추가/편집/📌 고정/삭제 (MemoryEditSheet)

### [macos] feat: BYOK 키 설정 UI (T-94)
- 설정: 키 목록/등록(SecureField)/삭제 — 서버 AES-GCM 암호화, label만 노출

### [macos] feat: 커스텀 모델 관리 UI (T-95)
- 설정: 커스텀 모델 목록/추가(이름·모델ID·Base URL·키·무료)/삭제

### [macos] feat: 캐릭터 카테고리 그룹핑 (T-96)
- 목록: 카테고리 필터 칩 + 섹션 그룹 (`일반/연인/친구/가족/기타` + 사용자 정의)

### [macos] feat: 디버그 패널 전면 확장
- 로그 탭: 레벨 필터/검색/한 줄 복사(우클릭)/선택한 줄 복사(List selection)/전체 복사/NSSavePanel 저장/지우기
- 상태 탭: 서버 대상 선택/기기ID/JWT/버전/Dock 토글/메인 창 열기/서버 확인
- 통계 탭: 레벨 카운트/OpenRouter 할당량 확인/최근 기능 로그/이미지 캐시 지우기
- 창 크기 명시(620x480) + miniaturizable

### [docs] feat: E2E 시나리오 (T-26)
- `docs/e2e/PLAN.md` (서버 TC-E2E-SRV-001~007 + macOS 스모크 TC-E2E-MAC-001~008)
- `scripts/load-test.js` k6 부하 (p95<300ms, 오류율<1%)

## [1.0.5] — 2026-08-13 (기억 시스템)

### [server] feat: 3계층 기억 (T-19~21)
- 기억 CRUD: `GET/POST /api/v1/memories/{characterId}`, `PUT/DELETE /api/v1/memories/{id}` (E-COM-MEM-*)
- `[MEMORY_SAVE]` 태그 자동 감지 → 기억 저장 + 사용자 노출 제거 (DESIGN 3.1)
- RAG: 2-gram 토큰 확장 ILIKE 검색 (한국어 형태소 대응) + 기억블록 프롬프트 주입
- 중기 요약: 30턴마다 비동기 요약 (ChatOnce, deepseek-v4-flash) → `chat_sessions.summary` 갱신
- 실검증: "고양이" 기억 → "고양이들" 질문 매치(57자 블록), 요약 생성 통과

## [1.0.4] — 2026-08-13 (SSE 채팅)

### [server] feat: 실시간 채팅 (T-17~18)
- `POST /api/v1/sessions/{id}/chat` — OpenRouter SSE 스트림 중계 (E-COM-CHAT-1001)
- 프롬프트 조합: 캐릭터 페르소나 + 대화 규칙 + 최근 20개 맥락 + 인사말 (T-18)
- 사용자/assistant 메시지 자동 저장 (tokens/cost 기록)
- 수정: `statusWriter` Flusher 미구현 버그 (SSE 502 → 200)

## [1.0.3] — 2026-08-13 (서버 API 2차)

### [server] feat: 모델/캐릭터/세션/규칙 API (T-15~16, T-22~23)
- `GET/POST /api/v1/models`, `GET /api/v1/quota` (E-COM-QUOTA-1002)
- 커스텀 모델 CRUD (`user_models`, E-COM-MODEL-*)
- BYOK 키 등록/삭제 (`/api/v1/settings/keys`, AES-GCM, v002 마이그레이션)
- 캐릭터 CRUD (E-COM-CHAR-1002) + DiceBear 기본 아바타 자동 생성 (DESIGN 11.3)
- 대화방/메시지 조회 (E-COM-SESS-1002) + 규칙 CRUD/기본 설정 (E-COM-RULE-2002)
- 실검증: Neon v001~v002 적용, 소유자 검증 404, 기본 규칙 삭제 거부

### [docs] feat: 디자인 에셋 정책 (DESIGN 11장)
- 무료 픽셀 에셋: itch.io / Kenney.nl / OpenGameArt
- DiceBear API 아바타 기본값 + 플랫폼별 적용 + 라이선스 체크리스트

## [1.0.0] — 2026-08-13 (초기 문서화)

### [docs] feat: 프로젝트 초기 문서 작성
- `docs/PRD.md` — 제품 요구사항 정의서 v1.0
  - 캐릭터/대화/대화 규칙/모델 관리/장기 기억/성인 콘텐츠 요구사항
- `docs/DESIGN.md` — 기술 설계 v1.0
  - 아키텍처 (macOS 네이티브, Go 서버, Neon+pgvector)
  - 3계층 기억 시스템 (단기/중기/장기 RAG)
  - 모델 관리/할당량/배포 전략
- `docs/plans/PLAN_v1.0_macos-server.md` — 구현 로드맵 (T-01 ~ T-77)
- `docs/TODO.md` — 작업 추적 (T-번호 전체 등록, Phase 0~4)
- `docs/AI_MODELS.json` — AI 모델 카탈로그
  - 기본 chain: openrouter/free → deepseek-v4-flash → mimo-v2.5
  - 커스텀 모델 수동 추가 지원
- `docs/api/ENDPOINTS.md` — Server API 명세 (인증/캐릭터/규칙/모델/세션/SSE/기억/키)
- `error_message_ko.json` — 에러 메시지 세트 (E-COM-* / E-SRV-*)

### 핵심 설계 결정
- [server] 백엔드: Go (LLM 프록시/스트리밍 워크로드에 유리)
- [server] DB: Neon PostgreSQL + pgvector (장기 기억 RAG)
- [server] 배포: Render 무료 티어 (Docker)
- [macos] SwiftUI + DMG (GitHub Releases, ad-hoc 서명)
- [all] 배포: 스토어 미배포 (19금 수위 제한 없음)
- [all] 키 관리: 서버 .env 공용 키 + 사용자 BYOK (AES-GCM 암호화 저장)
- [all] 할당량: OpenRouter /auth/key 연동 (모델 선택 + 대화창 표시)

### 미완 (다음 단계)
- [ ] T-01 스캐폴드 이후 코드 구현 (입력 대기: 코드 작성 지시)

---

## [1.0.1] — 2026-08-13 (macOS 스캐폴드 v1)

### [macos] feat: 앱 스캐폴드 + 메뉴바/Dock 토글/디버그 패널
- `apps/macos/` XcodeGen 프로젝트 생성 (`project.yml`, Swift 6, macOS 14+)
- 메뉴바 아이콘 (MenuBarExtra + 커스텀 템플릿 아이콘) — 기본 기능: 톡맨스 열기 / Dock에 표시 토글 / 설정 / 종료
- Dock 표시/숨김 토글: `NSApp.setActivationPolicy(.regular/.accessory)` + UserDefaults 영속화 (설정 창·메뉴바 동기화)
- 설정 창 (Settings scene) — Dock 토글 + 버전 정보
- 디버그 패널: `Cmd+Shift+D` 단축키, 로그 뷰어 + 필터 (DebugLogger 경유)
- `DebugLogger` (macOS): `[시간] [LEVEL] [TAG]` 형식, 최대 500건 링 버퍼, `feature()` 태그
- AppIcon/메뉴바 아이콘/AccentColor Assets 생성 (`scripts/gen_app_icons.py` 확장)
- 앱 아이콘 세트: `assets/icon/macos/Talkmance.icns`
- 빌드 검증: `xcodebuild Debug` 성공, 앱 실행 확인

### [docs] feat: 신규 기능 디버그 로그 의무화 (19.1장)
- 글로벌 AGENTS.md에 "신규 기능 추가 시 DebugLogger 로그 필수" 규칙 추가 (19.1장)
- 모든 신규 화면/동작에 `[INFO] [FEATURE] <기능명>` 로그 적용

### [docs] 갱신
- `docs/TODO.md` — T-01, T-30 완료 표시

---

## [1.0.2] — 2026-08-13 (서버 코어 T-10~T-13)

### [server] feat: Go 서버 코어
- T-24: `.env` 파서(stdlib) + config 로딩 + 에러코드 체계 (`internal/config`, `internal/errs` 23종 + error_message_ko.json 연동 + HTTP 상태 매핑)
- T-10: 구조화 JSON 로거 (`internal/log`, 19.1장 [FEATURE] 태그) + stdlib 라우터 + `/api/v1/health` + CORS/복구/요청 로깅 미들웨어 + graceful shutdown
- T-11: pgx/v5 연결 + 마이그레이션 프레임워크 (`internal/db` — NNN_이름.up/down.sql, 트랜잭션 적용, `MIGRATE=up|down` 모드)
- T-12: 마이그레이션 v001 (users/characters/character_memories+pgvector HNSW/chat_sessions/messages/prompt_rules/user_models, up/down)
- T-13: 익명 기기 JWT 인증 (`internal/auth` — jwt/v5 HS256, 30일 TTL, `POST /api/v1/auth/register`, Bearer 미들웨어, 만료 시 E-COM-AUTH-1002)
- 검증: 유닛 테스트 20+건 통과, 서버 기동 + /health + 에러코드 응답 확인 (실 DB 동작은 Neon URL 필요)

### [docs] 갱신
- `docs/TODO.md` — T-24, T-10~13 완료 표시

---

## 이전 버전

- 없음 (프로젝트 시작)