# PLAN v1.2 — 서버 중기 요약 (T-21)

> 갱신: 2026-08-13 · 플랫폼: server

## 개요
- 대화가 길어질수록 맥락 유지를 위해 **30턴(60개 메시지)마다** 최근 대화를 요약해 `chat_sessions.summary`에 저장
- 요약은 채팅 프롬프트의 `[대화 요약]` 블록으로 자동 주입 (기존 memoryBlock 로직 그대로 활용)

## 결정 사항
- 요약 트리거: assistant 메시지 저장 후 **총 메시지 수가 60의 배수**일 때 (COUNT 쿼리)
- 요약 생성: `chatOnce` (zen 우선 → openrouter 폴백) — 비스트림 호출 재사용
- 프롬프트: 이전 요약(있으면) + 최근 60개 메시지 → 새 요약 3~5줄 (한국어, 캐릭터 관계·상태 변화 우선)
- `UpdateSession` 재사용 (status/summary/relation_state — summary만 갱신)
- 요약 실패해도 채팅은 정상 (로그만 남기고 무시)
- [FEATURE] 로그: `[채팅] 중기 요약 갱신 (session=..., messages=N)` 필수

## 구현 단계
- [ ] store: `CountMessages(ctx, id, userID)` 추가
- [ ] httpapi: `maybeSummarize(ctx, id, userID, sess, character)` — 60의 배수면 chatOnce로 요약 → UpdateSession
- [ ] chat.go: assistant 저장 후 maybeSummarize 호출
- [ ] 빌드 + E2E: 채팅 2회(각 1턴) → 수동 COUNT 60 검증용은 생략, 요약 프롬프트 단위 curl로 요약 정상 생성 확인
- [ ] TODO/CHANGELOG 갱신

## 테스트 계획
- TC-1: assistant 저장 후 총 메시지 60 배수 확인 로직 (단위)
- TC-2: maybeSummarize 직접 호출로 요약 생성 + summary 저장 확인 (curl + GET /sessions)
- TC-3: 요약 실패(모델 오류) 시 채팅 정상 동작 확인

## 롤백 계획
- 요약 로직 제거만으로 복귀 (DB 변경 없음)