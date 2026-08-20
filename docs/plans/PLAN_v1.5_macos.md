# PLAN_v1.5_macos — macOS 전용 전환 + 대화 품질 고도화

> 생성: 2026-08-20 · 플랫폼: macos + server · 버전: v1.5.0

## 개요
- **목표 1**: Android 플랫폼 제거 — 코드/문서/CI/배포 자산 전부 삭제, "처음부터 없었던 것처럼" 처리 후 GitHub 저장소 재생성
- **목표 2**: NVIDIA AI 무료 모델(NIM) 통합 — 키체인 키 사용, 프로브로 모델 1개 확정, 폴백 체인 1순위 추가
- **목표 3**: 한국어 대화 품질 — korean-humanizer 스킬 기반 A(프롬프트 규칙 주입, 항상) + B(후처리 다듬기, 설정 토글 기본 끔)

## 결정 사항
1. Android 전체 삭제 (apps/android, android.yml, 문서 22건 흔적, Release APK 자산)
2. 서버는 로컬로만 검증 (Render 배포 보류)
3. NVIDIA 키는 서버 `.env`의 `NVIDIA_API_KEY` (키체인에서 인출, 커밋 금지)
4. 한글 다듬기 B는 `polish` 요청 파라미터 + macOS SettingsView 토글 (UserDefaults)
5. git 이력 리셋 후 GitHub 저장소 재생성 (동일 이름 BoraSarang/Talkmance, public) — Pages/Release DMG 복구

## 아키텍처
- **macOS 앱**: SettingsView.swift에 "한국어 다듬기" 토글 → 채팅 요청 `polish` 전달
- **서버**: `router.go` 모델 상수/폴백 체인에 NVIDIA 추가 → `chat.go` buildChatMessages에 한글 규칙 + polish 후처리 분기 → `models.go` nvidiaCatalog
- **모델 폴백**: NVIDIA(키 시 1순위) → Gemini(키 시) → 세션 모델(zen) → zen 재시도 → gpt-oss:free → openrouter/free

## 구현 단계 (상태: 2026-08-20 완료)
- [x] T-97: Android 제거 (파일/문서/CI/APK) — 완료, `grep -rn "android"` 0건 (계획/추적 문서 제외)
- [x] T-98: NVIDIA 프로브 → **모델 확정: `google/gemma-4-31b-it`** (HTTP 200, 스트림 지원, 한국어 우수)
  - 키체인의 이전 키는 무효(403) → 사용자 신규 키 교체 완료
  - 후보 gemma-4-31b-it(32.7초/콜드스타트, 이후 10초) / mistral-nemotron(40초, 한국어 불량 탈락) / mistral-nemo-12b(404 탈락)
- [x] T-99: 서버 NVIDIA 통합 — config.go NVIDIA_API_KEY, router.go nvidiaBaseURL/nvidiaModel + chatOnce·chat 폴백 1순위, models.go nvidiaCatalog, .env/.env.example
- [x] T-100: 한글 A 프롬프트 규칙 주입 — buildChatMessages에 이모지/줄표/번역체/AI상투어 금지 + 3의 법칙·접속사·종결어미 반복 금지 + 구어체 리듬 필수
- [x] T-101: 한글 B 후처리 — polish.go (이모지/줄표/AI상투어 제거 + 반복 축약 + 편집거리 30% 가드) + ChatRequest.polish + SettingsView 토글(기본 끔) + 앱 chatStream 전달
- [x] T-102: 로컬 검증 — NVIDIA SSE(7초/5초) + polish on(전체 후처리 1회 전송, [MEMORY_SAVE] 보존) + polish off(실시간 스트리밍) + Go 전체 테스트 통과 + macOS 빌드 성공
- [ ] T-103: 커밋 + GitHub 재생성 (백업/삭제/이력리셋/재생성/Pages/Release)

## 테스트 결과
- TC-NVDA-001: PASS — gemma-4-31b-it SSE 대화 정상, model=google/gemma-4-31b-it 확인
- TC-NVDA-002: PASS — 키 미노출 (로그에 키 값 없음)
- TC-KR-001: PASS — polish off 스트리밍 유지, 행동묘사/반말 유지
- TC-KR-002: PASS — polish on 후처리 전송, [MEMORY_SAVE] 보존, 이모지 제거
- TC-KR-003: PASS — 변경률 30% 초과 시 원문 폴백 (polish_test.go)
- TC-ADULT-001: PASS — 성인/비성인 프롬프트 구조 유지
- TC-ANDROID-REMOVE: PASS — grep 0건

## 롤백 계획
- 코드: git revert (이력 리셋 전 커밋)
- 서버: 기존 바이너리 재시작 (백업 `/tmp/talkmance-server`)
- GitHub: 저장소 재생성 전 로컬 bundle 백업 (`/tmp/talkmance-backup.bundle`)

## 성능 예산
- NVIDIA 첫 토큰 ≤ 3초 (무료 티어 콜드 스타트 고려)
- polish on 추가 지연 ≤ 5초, 실패 시 원문 즉시 폴백
- 메모리: 기존 유지

## 에러코드
- E-SRV-BRIDGE-1002: NVIDIA 스트림 실패 → 폴백 로그 (기존 체계 재사용)

## 시크릿
- `NVIDIA_API_KEY`: 로컬 `.env`만, 커밋 금지, 키체인 원본 유지