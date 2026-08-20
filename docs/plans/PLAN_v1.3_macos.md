# PLAN v1.3 — macOS 기능 완성 + 서버 보완

> 갱신: 2026-08-13 · 플랫폼: macos + server · 상태: 진행중

## 개요
PRD v1.0 P0 공백을 앱 중심으로 마감한다.
- 서버는 대부분 완비 (rules CRUD, memories CRUD+pinned, /quota, /settings/keys, models/custom CRUD)
- 앱 UI만 없거나 세션 생성에 rule_id 미전달 → 서버 소폭 보완

## 결정 사항
1. **문서 정리 우선**: TODO/CHANGELOG/세션 로그를 실제 구현과 동기화 (T-17·18·20·21·43~45 완료 반영)
2. **19세 게이트 (T-39)**: 첫 실행 시 UserDefaults `adultGatePassed`로 1회 확인 (거부 시 앱 종료 안내), 생성 시트 성인 동의 문구
3. **대화 규칙**: `POST /sessions`에 `rule_id` 선택 파라미터 추가 (미지정 = 기본 규칙). 앱: 새 대화 시트 규칙 Picker + 설정 규칙 관리 (목록/생성/편집/삭제, 기본 규칙 삭제 거부는 서버 응답 표시)
4. **할당량**: 새 대화 시트 + 채팅 헤더에 잔여 배지 (fetchQuota, 5분 캐시, 실패 시 숨김)
5. **메모리 카드**: 캐릭터 상세에 기억 섹션 — 목록(장기/단기 배지 + 📌 고정)/추가/편집/고정 토글/삭제
6. **BYOK 키**: 설정에 OpenRouter 키 등록/삭제 (서버 AES-GCM, 응답 마스킹 확인)
7. **커스텀 모델**: 설정에 추가/삭제 폼 (이름/ID/Base URL/키/무료·유료/설명)
8. **카테고리 그룹핑**: 캐릭터 목록 섹션 그룹핑 + 필터 칩
9. **서버 보완**: `/debug/health`, `/debug/logs` (DEBUG 모드만) + `scripts/load-test.js` k6 + docs/e2e 갱신

## 구현 단계 (T-번호)
- [ ] T-90: 문서 동기화 (TODO/CHANGELOG 1.0.6~1.1.0/세션 로그)
- [ ] T-39: 19세 게이트
- [ ] T-91: 서버 POST /sessions rule_id + 앱 규칙 Picker + 규칙 관리 화면
- [ ] T-92: 할당량 배지 (새 대화 시트 + 채팅 헤더)
- [ ] T-93: 메모리 카드 UI
- [ ] T-94: BYOK 키 설정 UI
- [ ] T-95: 커스텀 모델 관리 UI
- [ ] T-96: 카테고리 그룹핑
- [ ] T-25: 서버 /debug/* (health/logs, DEBUG만)
- [ ] T-26: k6 load-test.js
- [ ] T-70~75: 검증 문서 (e2e/PLAN.md, 스모크)

## 테스트 계획
- TC-1: 서버 재시작 후 curl — 세션 생성 rule_id, /debug/health, /debug/logs, 키 등록/삭제, 커스텀 모델 CRUD
- TC-2: 앱 — 첫 실행 19세 게이트 → 캐릭터 생성 → 새 대화(규칙 선택) → 채팅(할당량 배지) → 기억 추가/고정 → 설정(키/커스텀 모델) → 카테고리 필터
- TC-3: 디버그 패널 로그로 각 기능 [FEATURE] 확인

## 롤백 계획
- 게이트: UserDefaults 키 삭제로 재확인 가능
- 서버: sessions.go rule_id 추가는 선택 필드라 하위 호환 (제거 시 단순 revert)
- 앱: 화면 단위로 커밋 분리, 문제 시 해당 뷰만 revert
