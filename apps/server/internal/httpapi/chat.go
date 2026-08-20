package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/talkmance/server/internal/errs"
	"github.com/talkmance/server/internal/orapi"
	"github.com/talkmance/server/internal/store"
)

// ---- POST /api/v1/sessions/{id}/chat — SSE 스트림 채팅 ----
// 요청: {"content": "안녕?"}
// 응답: data: {"content":"..."} ... data: {"done":true,"token_in":N,"token_out":N,"cost":0.001}
//
//	data: {"error":{"code":"...","message":"..."}}
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, errs.New(errs.EComValid1001, err))
		return
	}
	if s.store == nil || s.client == nil {
		writeErr(w, errs.New(errs.EComChat1001, nil))
		return
	}

	var req struct {
		Content string `json:"content"`
		Auto    bool   `json:"auto"`
		Retry   bool   `json:"retry"`
		Polish  bool   `json:"polish"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, errs.New(errs.EComValid1001, err))
		return
	}
	if strings.TrimSpace(req.Content) == "" && !req.Auto && !req.Retry {
		writeErr(w, errs.New(errs.EComValid2001, nil))
		return
	}

	// auto=true: AI가 먼저 말 걸기 (첫 시작 / 재입장 시 이어 말하기)
	userContent := req.Content
	if req.Auto {
		userContent = "지금 사용자가 대화방에 들어온 상황이고 대화가 멈춰 있다. " +
			"이전 대화 맥락과 캐릭터 설정에 맞춰 너(캐릭터)가 먼저 자연스럽게 말을 걸어라. " +
			"대화가 처음이라면 시작 전 대화(과거 관계)와 첫 만남 인사말에 맞춰 먼저 인사를 건네라. " +
			"이 지시에 답하는 형태가 아니라 너의 발언으로 응답하라."
		s.log.Feature("채팅", "자동 발화 요청 (session=%s)", id)
	}

	// 세션/캐릭터/규칙 로드
	sess, err := s.store.GetSession(r.Context(), id, userID)
	if err != nil {
		writeErr(w, toAppErr(err))
		return
	}
	character, err := s.store.GetCharacter(r.Context(), sess.CharacterID, userID)
	if err != nil {
		writeErr(w, toAppErr(err))
		return
	}

	// 규칙: 세션에 지정된 규칙 → 없으면 기본 규칙 → 없으면 빈 시스템 프롬프트
	systemPrompt := ""
	if sess.RuleID != nil {
		if rule, err := s.store.GetRule(r.Context(), *sess.RuleID, userID); err == nil {
			systemPrompt = rule.SystemPrompt
		}
	} else {
		rules, err := s.store.ListRules(r.Context(), userID)
		if err == nil {
			for _, rule := range rules {
				if rule.IsDefault {
					systemPrompt = rule.SystemPrompt
					break
				}
			}
		}
	}

	// T-19: 기억 블록 (RAG 검색 + 중기 요약) — 시스템 프롬프트에 주입
	// 사용자 메시지에서 단어 토큰을 추출해 ILIKE 검색 (임베딩 미설정 시 키워드 폴백)
	var memoryBlock strings.Builder
	if sess.Summary != "" {
		memoryBlock.WriteString("[대화 요약] " + sess.Summary + "\n")
	}
	queryTokens := tokenize(req.Content)
	if len(queryTokens) > 0 && s.store != nil {
		results, err := s.store.SearchMemoriesByTokens(r.Context(), sess.CharacterID, userID, queryTokens, 5)
		if err != nil {
			s.log.Errorf("기억", "검색 실패: %v", err)
		} else {
			for _, m := range results {
				memoryBlock.WriteString("[기억] " + m.Content + "\n")
			}
		}
	}
	s.log.Feature("기억", "검색 완료 (session=%s, 토큰=%d, 기억블록=%d자)", id, len(queryTokens), memoryBlock.Len())

	// T-18: 프롬프트 조합 (캐릭터 페르소나 + 규칙 + 기억 블록 + 최근 맥락)
	recent, err := s.store.LastMessages(r.Context(), id, userID, 20)
	if err != nil {
		writeErr(w, toAppErr(err))
		return
	}
	// retry=true: 마지막 user 메시지를 재사용 (중복 저장 방지)
	if req.Retry {
		for i := len(recent) - 1; i >= 0; i-- {
			if recent[i].Role == "user" {
				userContent = recent[i].Content
				s.log.Feature("채팅", "재시도 요청 (마지막 user 메시지 재사용)")
				break
			}
		}
	}
	messages := buildChatMessages(character, systemPrompt, memoryBlock.String(), recent, userContent)

	// 사용자 메시지 저장 (auto/retry 발화는 저장하지 않음)
	if !req.Auto && !req.Retry {
		if _, err := s.store.AppendMessage(r.Context(), id, userID, store.Message{
			SessionID: id, Role: "user", Content: req.Content, Model: sess.ModelID,
		}); err != nil {
			writeErr(w, toAppErr(err))
			return
		}
	}
	s.log.Feature("채팅", "채팅 시작 (session=%s, model=%s, persona=%v)", id, sess.ModelID, character.Name)

	// SSE 헤더
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, errs.New(errs.EComChat1001, nil))
		return
	}

	ch := make(chan orapi.ChatChunk, 16)
	model := sess.ModelID
	if model == "" {
		model = defaultFreeModel
	}

	// 폴백 체인: NVIDIA(1순위, 키 설정 시) → Gemini(키 설정 시) → 세션 모델(기본 zen) → zen 5초 후 재시도
// → gpt-oss-20b → openrouter/free
// 즉시 오류(429 등)뿐 아니라 빈 응답(tokens 0/0)도 실패로 간주해 다음 모델 재시도
	attempts := []string{}
	if s.nvidiaAPIKey != "" {
		attempts = append(attempts, nvidiaModel)
	}
	if s.geminiAPIKey != "" && geminiModel != model {
		attempts = append(attempts, geminiModel)
	}
	if model != nvidiaModel {
		attempts = append(attempts, model)
	}
	if model == zenModelFree || model == mimoModelFree {
		attempts = append(attempts, model) // zen 재시도 (5초 대기)
	}
	attempts = append(attempts, orFallback, openRouterFree)
	usedModel := model

	var full strings.Builder
	tokenIn, tokenOut := 0, 0
	var cost float64
	failReasons := []string{}

streamAttempt:
	for ai, m := range attempts {
		// OpenRouter 무료 티어(50회/일) 사용 카운트 — gpt-oss:free, openrouter/free만
		if strings.HasSuffix(m, ":free") || m == openRouterFree {
			s.freeToday.Inc()
		}
		// 연속 동일 모델(zen 재시도)이면 잠시 대기 — 분 단위 레이트리밋 해제 대비
		if ai > 0 && m == attempts[ai-1] && (m == zenModelFree || m == mimoModelFree) {
			select {
			case <-time.After(5 * time.Second):
			case <-r.Context().Done():
				return
			}
		}
		baseURL, apiKey := "", ""
		if m == nvidiaModel && s.nvidiaAPIKey != "" {
			baseURL, apiKey = nvidiaBaseURL, s.nvidiaAPIKey
		} else if m == geminiModel && s.geminiAPIKey != "" {
			baseURL, apiKey = geminiBaseURL, s.geminiAPIKey
		} else if s.openCodeKey != "" && (m == zenModelFree || m == mimoModelFree) {
			baseURL, apiKey = zenBaseURL, s.openCodeKey
		}
		ch = make(chan orapi.ChatChunk, 16)
		if m == geminiModel {
			go s.client.GeminiStream(r.Context(), s.geminiAPIKey, m, messages, ch)
		} else {
			go s.client.ChatStream(r.Context(), baseURL, apiKey, m, messages, ch)
		}

		first := <-ch
		if first.Err != nil {
			failReasons = append(failReasons, m+": "+first.Err.Error())
			if ai < len(attempts)-1 {
				s.log.Warnf("채팅", "스트림 즉시 실패(%s), %s 폴백 재시도: %v", m, attempts[ai+1], first.Err)
				continue
			}
			writeSSE(w, flusher, s.errPayload(first.Err, failReasons))
			s.log.Errorf("채팅", "모든 폴백 스트림 실패: %v (시도: %v)", first.Err, failReasons)
			return
		}

		full.Reset()
		tokenIn, tokenOut, cost = 0, 0, 0
		if first.Content != "" {
			full.WriteString(first.Content)
			if !req.Polish {
				writeSSE(w, flusher, map[string]any{"content": first.Content})
			}
		}
		if first.TokenIn > 0 || first.TokenOut > 0 {
			tokenIn, tokenOut = first.TokenIn, first.TokenOut
			cost = first.Cost
		}

		streamFailed := false
		for chunk := range ch {
			if chunk.Err != nil {
				streamFailed = true
				break
			}
			if chunk.Done {
				break
			}
			if chunk.Content != "" {
				full.WriteString(chunk.Content)
				if !req.Polish {
					writeSSE(w, flusher, map[string]any{"content": chunk.Content})
				}
			}
			if chunk.TokenIn > 0 || chunk.TokenOut > 0 {
				tokenIn, tokenOut = chunk.TokenIn, chunk.TokenOut
				cost = chunk.Cost
			}
		}
		if streamFailed || full.Len() == 0 {
			if ai < len(attempts)-1 {
				s.log.Warnf("채팅", "%s 응답 실패(빈내용=%v), %s 폴백 재시도", m, full.Len() == 0, attempts[ai+1])
				continue
			}
		}
		usedModel = m
		break streamAttempt
	}

	// T-19: [MEMORY_SAVE] 태그 추출 → 기억 자동 저장
	_, tags := s.extractMemoryTags(sess.CharacterID, full.String())
	for _, tag := range tags {
		if _, err := s.store.CreateMemory(r.Context(), sess.CharacterID, userID, store.Memory{
			MemType: "long", Content: tag, Importance: 0.8,
		}); err != nil {
			s.log.Errorf("기억", "자동 저장 실패: %v", err)
		}
	}
	if len(tags) > 0 {
		s.log.Feature("기억", "자동 저장 %d건 (session=%s)", len(tags), id)
	}

	// assistant 메시지 저장 (polish=true면 후처리 적용, [MEMORY_SAVE] 태그는 앱에서 표시 변환)
	content := full.String()
	if req.Polish {
		content = polishKoreanAssist(content)
		if content != "" {
			if content != full.String() {
				s.log.Feature("채팅", "한글 다듬기(B) 적용 (session=%s, %d자)", id, len([]rune(content)))
			}
			// polish 모드: 스트리밍 대신 최종 텍스트 1회 전송 (원문과 같아도 전송)
			writeSSE(w, flusher, map[string]any{"content": content})
		}
	}
	if content != "" {
		if _, err := s.store.AppendMessage(r.Context(), id, userID, store.Message{
			SessionID: id, Role: "assistant", Content: content, Model: usedModel,
			TokenIn: tokenIn, TokenOut: tokenOut, Cost: cost,
		}); err != nil {
			s.log.Errorf("채팅", "assistant 저장 실패: %v", err)
		}
	}
	s.log.Feature("채팅", "완료 (session=%s, model=%s, tokens=%d/%d, cost=$%.4f)", id, usedModel, tokenIn, tokenOut, cost)
	writeSSE(w, flusher, map[string]any{
		"done": true, "model": usedModel, "token_in": tokenIn, "token_out": tokenOut, "cost": cost,
	})

	// T-20: 30턴마다 중기 요약 (비동기 — SSE 응답 지연 없음)
	s.maybeSummarize(id, userID)
}

// buildChatMessages 프롬프트 조합 (T-18, DESIGN 3.2)
// [페르소나] → [대화 규칙] → [기억 블록(요약+RAG)] → [대화 히스토리]
func buildChatMessages(c *store.Character, systemPrompt, memoryBlock string, recent []store.Message, userContent string) []orapi.ChatMessage {
	var sb strings.Builder
	sb.WriteString("너는 연애 AI 채팅 앱의 캐릭터다. 항상 캐릭터의 페르소나와 말투로 답변한다.\n")
	sb.WriteString("답변은 2~3문장마다 줄바꿈해서 읽기 쉽게 작성한다.\n")
	sb.WriteString("한국어를 사람이 쓰는 것처럼 자연스럽게 쓴다. 아래 규칙을 반드시 지킨다.\n")
	sb.WriteString("- 금지: 이모지(😊✨❤ 등), 줄표(—, –), 번역체, 이중 피동(가능해질 수 있다 등), 과도한 단정(반드시/절대/당연히), AI 상투어(도움이 되었으면 좋겠어요, 그렇게 생각해요 등)\n")
	sb.WriteString("- 금지: 3의 법칙(같은 구조 3연속 나열), 접속사 남발(그리고/하지만/그래서 반복), 종결어미 반복(요요요/네네네/다다다)\n")
	sb.WriteString("- 필수: 구어체 반말, 짧은 문장, 쉼표·물결로 자연스럽게 끊기, 대화마다 리듬 변화\n")
	sb.WriteString("행동·장면 전환(이동, 도착, 시간 경과, 감정적 행동 등)은 *행동 묘사* 형식으로 표기한다.\n")
	sb.WriteString("대화 중 사용자에 대해 알게 된 중요한 사실(취향, 이름, 약속, 좋아하는 것 등)은\n")
	sb.WriteString("[MEMORY_SAVE] 태그를 붙여 한 줄로 출력한다. 예: \"그녀의 이름은 지수야 [MEMORY_SAVE] 사용자 이름은 지수\"\n")
	sb.WriteString("태그는 답변 끝에 붙이거나 별도 줄에 출력하고, 사용자에게 태그 내용을 알려줄 필요는 없다.\n")
	if systemPrompt != "" {
		sb.WriteString("\n[대화 규칙]\n" + systemPrompt + "\n")
	}
	sb.WriteString("\n[캐릭터]\n")
	sb.WriteString("이름: " + c.Name + "\n")
	if c.Title != "" {
		sb.WriteString("타이틀: " + c.Title + "\n")
	}
	if c.Age != nil {
		sb.WriteString(fmt.Sprintf("나이: %d\n", *c.Age))
	}
	if c.Adult {
		sb.WriteString("성인 캐릭터 (성인 대화 허용)\n")
	} else {
		sb.WriteString("비성인 캐릭터 (성인·선정적 대화 금지, 건전한 대화 유지)\n")
	}
	if len(c.Persona) > 0 {
		personaJSON, _ := json.Marshal(c.Persona)
		sb.WriteString("페르소나: " + string(personaJSON) + "\n")
	}
	if c.Greeting != "" {
		sb.WriteString("\n첫 만남 인사말: " + c.Greeting + "\n")
	}
	if memoryBlock != "" {
		sb.WriteString("\n[기억 블록]\n" + memoryBlock)
	}

	msgs := []orapi.ChatMessage{{Role: "system", Content: sb.String()}}
	// 최근 맥락 주입 (user/assistant 순서 유지)
	for _, m := range recent {
		msgs = append(msgs, orapi.ChatMessage{Role: m.Role, Content: m.Content})
	}
	msgs = append(msgs, orapi.ChatMessage{Role: "user", Content: userContent})
	return msgs
}

// tokenize 검색 토큰 추출 (2자 이상 한글/영문 단어)
func tokenize(s string) []string {
	fields := strings.Fields(s)
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.Trim(f, ".,!?~[]()\"'")
		if len([]rune(f)) >= 2 {
			out = append(out, f)
		}
	}
	return out
}

// ---- SSE 페이로드 헬퍼 ----

func writeSSE(w http.ResponseWriter, f http.Flusher, payload any) {
	b, _ := json.Marshal(payload)
	fmt.Fprintf(w, "data: %s\n\n", b)
	f.Flush()
}

// errPayload 에러를 SSE payload로 (detail: 폴백 체인별 실패 사유 — 디버그용)
func (s *Server) errPayload(err error, failReasons []string) map[string]any {
	code := errs.ESRVDb1001
	message := "알 수 없는 오류가 발생했어요."
	status := http.StatusInternalServerError
	var ae *errs.AppError
	if errors.As(err, &ae) {
		code = ae.Code
		message = ae.Message
		status = ae.HTTP
	} else if strings.Contains(err.Error(), "429") {
		// 모델 레이트 리밋 — 사용자 친화 메시지로 매핑
		code = errs.EComModel1001
		message = "대화 모델을 호출하지 못했어요. 잠시 후 다시 시도해 주세요."
		status = http.StatusServiceUnavailable
	} else if strings.HasPrefix(err.Error(), "orapi:") {
		code = errs.EComModel1001
		message = "대화 모델을 호출하지 못했어요. 잠시 후 다시 시도해 주세요."
		status = http.StatusBadGateway
	}
	payload := map[string]any{"code": code, "message": message, "status": status}
	if len(failReasons) > 0 {
		payload["detail"] = strings.Join(failReasons, " | ")
	}
	return payload
}
