# 톡맨스 서버 (Go)
- Go 1.26+
- Render 배포 대상

## 로컬 실행
```bash
cp .env.example .env   # OPENROUTER_API_KEY 등 설정
go run ./cmd/server
```

## 구조 (예정)
- cmd/server — 엔트리포인트
- internal/config — .env/config 로딩 (T-24)
- internal/log — 구조화 JSON 로거 (19.1장: 서버 로거)
- internal/httpapi — 라우터 + 핸들러 (/health 등)
- internal/db — pgx + 마이그레이션 (T-11~12)
- internal/auth — 익명 기기 JWT (T-13)
- internal/models — 모델 카탈로그/커스텀 CRUD (T-14~16)
- internal/chat — SSE 스트리밍 + 프롬프트 조합 (T-17~18)
- internal/memory — RAG/메모리 태그/요약 (T-19~21)