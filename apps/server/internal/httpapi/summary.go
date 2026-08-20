package httpapi

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/talkmance/server/internal/orapi"
)

const (
	summaryContext  = 60 // 30턴 = 60개 메시지 (요약 대상 + 트리거 배수)
	summaryModel    = "deepseek-v4-flash-free"
	summaryTimeout  = 45 * time.Second
)

// maybeSummarize T-20: 중기 요약 — 총 메시지 수가 60(30턴)의 배수일 때 비동기 요약
// 세션에 캐릭터 관점 요약을 갱신 (비용 낮은 모델, SSE 지연 없음)
func (s *Server) maybeSummarize(sessionID, userID uuid.UUID) {
	if s.store == nil || s.client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), summaryTimeout)
	defer cancel()

	// 30턴 = user+assistant 60개. 총 메시지 수가 60의 배수일 때만 요약
	n, err := s.store.CountMessages(ctx, sessionID, userID)
	if err != nil || n < summaryContext || n%summaryContext != 0 {
		return
	}

	msgs, err := s.store.LastMessages(ctx, sessionID, userID, summaryContext)
	if err != nil || len(msgs) < summaryContext {
		return
	}

	var sb strings.Builder
	for _, m := range msgs {
		sb.WriteString(m.Role + ": " + m.Content + "\n")
	}
	msgsChat := []orapi.ChatMessage{
		{Role: "system", Content: "너는 대화 요약 AI다. 아래 대화를 캐릭터 관점에서 3~5줄 이내로 요약해라.\n" +
			"사용자에 대한 정보(이름, 취향, 약속)와 관계 상태(단계)를 반드시 포함하고, 자연스러운 한국어로 작성한다.\n" +
			"요약만 출력하고 다른 말은 하지 않는다."},
		{Role: "user", Content: sb.String()},
	}
	summary, _, _, err := s.chatOnce(ctx, summaryModel, msgsChat, 0.2)
	if err != nil {
		s.log.Errorf("요약", "요약 생성 실패: %v", err)
		return
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return
	}
	if err := s.store.UpdateSession(ctx, sessionID, userID, "", summary, nil); err != nil {
		s.log.Errorf("요약", "요약 저장 실패: %v", err)
		return
	}
	s.log.Feature("요약", "중기 요약 갱신 (session=%s, %d자)", sessionID, len(summary))
}
