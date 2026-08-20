# E2E 시나리오 정의서 (macOS + Server)

> v2.1 표준에 따른 텍스트 전용 검증 (a11y-dump 대체). Playwright는 네이티브 앱 미지원이므로 서버 E2E (curl/k6) + macOS 앱 수동 스모크 (DebugLogger)로 검증.

## Server E2E (curl 스크립트)

| ID | 시나리오 | 검증 포인트 |
|----|----------|-------------|
| TC-E2E-SRV-001 | GET /health → GET /api/v1/models (29개) → GET /quota | 200, 카탈로그 29 (Gemini 13 + free 16), quota JSON |
| TC-E2E-SRV-002 | 인증 없이 /api/v1/models | 401 + 에러코드 E-COM-AUTH-* |
| TC-E2E-SRV-003 | POST /api/v1/auth/register (중복 user_id) | 409 + E-COM-AUTH-1004 |
| TC-E2E-SRV-004 | GET /api/v1/debug/health + /debug/logs | log_level=debug일 때만 라우트 존재 |
| TC-E2E-SRV-005 | POST /sessions + rule_id / GET /messages | rule_id 저장 확인 |
| TC-E2E-SRV-006 | POST /settings/keys + DELETE | AES-GCM 저장, 목록에 label만 노출 (암호문 미노출) |
| TC-E2E-SRV-007 | POST /models/custom + DELETE | 커스텀 모델 CRUD |

## 부하 테스트 (k6) — T-26 완료 (2026-08-13)

```bash
TOKEN=$(cat /tmp/talkmance_jwt.txt) k6 run scripts/load-test.js
# 임계값: p95 < 300ms, 오류율 < 1%
```

**실측 결과 (50 VU × 30s, 6636 iterations)**
- p(95) = **1.54ms** (임계 300ms 통과) · 오류율 0% · 체크 100% (13,272/13,272)
- 개선 이력: 최초 827ms → 커스텀 모델 목록 5초 캐시 (Neon 원격 DB 왕복 ~200ms × 매 요청 병목) → 1.54ms
- 참고: k6는 /health + /models만 대상 (SSE 스트림은 별도 — TC-E2E-SRV-008에서 curl로 검증)

## Server 연동 스모크 — T-70 완료 (2026-08-13)

| ID | 시나리오 | 결과 |
|----|----------|------|
| TC-E2E-SRV-008 | 신규 기기 등록 → 캐릭터 생성 → 세션 생성 → SSE 자동발화 | 200, 185 data 청크, done (301/99 tokens), 메시지 저장 확인 |
| TC-E2E-SRV-009 | 사용자 메시지 → AI 응답 → 기억 자동 저장 | "김치찌개" 기억 자동 추출 (MEMORY_SAVE), 기억 1→2건 |
| TC-E2E-SRV-010 | 커스텀 모델 생성→목록→수정→삭제 (캐시 무효화) | 201→1건→204→204→0건 |
| TC-E2E-SRV-011 | GET /quota | label + free_used 28/50 표시 |

## macOS 앱 스모크 (수동, 텍스트 검증)

| ID | 시나리오 | 검증 |
|----|----------|------|
| TC-E2E-MAC-001 | 첫 실행 → 19세 게이트 표시 → 허용 → 메인 창 | adultGatePassed 설정, DebugLogger "게이트" 로그 |
| TC-E2E-MAC-002 | 캐릭터 목록 → 카테고리 칩 필터 → 그룹 섹션 | 필터 동작 |
| TC-E2E-MAC-003 | 새 대화 → 규칙 Picker → 규칙 선택 → 메시지 전송 | rule_id 반영 |
| TC-E2E-MAC-004 | 캐릭터 상세 → 메모리 추가/편집/고정/삭제 | 메모리 CRUD |
| TC-E2E-MAC-005 | 설정 → 키 등록 → 목록 표시 → 삭제 | BYOK CRUD |
| TC-E2E-MAC-006 | 설정 → 커스텀 모델 추가 → 목록 → 삭제 | 커스텀 모델 CRUD |
| TC-E2E-MAC-007 | 대화 헤더 할당량 배지 표시 | 잔여 $ 표시 |
| TC-E2E-MAC-008 | Cmd+Shift+D 디버그 패널 → 로그/상태/통계 탭 → 복사 3종 | 한 줄/선택/전체 복사 |
| TC-E2E-MAC-009 | 설정 → 한국어 다듬기 토글 → 채팅 요청에 polish 전달 | polish=true 전달 + 다듬은 응답 (T-101) |