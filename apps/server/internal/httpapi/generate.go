package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/talkmance/server/internal/errs"
	"github.com/talkmance/server/internal/orapi"
	"github.com/talkmance/server/internal/store"
)

// generateModel AI 캐릭터 생성/아바타 프롬프트용 모델 (OpenRouter Free — 문서 AI_MODELS.json)
const generateModel = "deepseek-v4-flash-free"

// defaultFreeModel 기본 무료 모델 (채팅 세션 model 미지정 시)
const defaultFreeModel = "gemini-2.5-flash"

// persona 표준 키 (문서 DESIGN 6.1 — 앱과 공유)
const (
	personaGender  = "성별"
	personaRel     = "관계"
	personaStory   = "스토리"
	personaBack    = "시작전대화"
	personaPerson  = "성격"
	personaTone    = "말투"
	personaHobby   = "취미"
	personaAvatar  = "avatar_prompt"
	personaSetting = "배경"
)

// diceBearStyles 재생성용 스타일 로테이션 (문서 11.3)
var diceBearStyles = []string{"pixel-art", "adventurer", "lorelei", "thumbs", "notionists", "big-smile"}

// GenerateRequest AI 캐릭터 생성 요청
type GenerateRequest struct {
	Name         string `json:"name"`
	Gender       string `json:"gender"`
	Age          *int   `json:"age"`
	Relationship string `json:"relationship"`
	Category     string `json:"category"`
	Adult        bool   `json:"adult"`
}

// GenerateResponse AI 생성 결과 (클라이언트가 수정 후 저장)
type GenerateResponse struct {
	Name         string         `json:"name"`
	Title        string         `json:"title"`
	Persona      map[string]any `json:"persona"`
	Greeting     string         `json:"greeting"`
	AvatarPrompt string         `json:"avatar_prompt"`
	AvatarURL    string         `json:"avatar_url"`
}

// ---- POST /api/v1/characters/generate ----
func (s *Server) handleGenerateCharacter(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, errs.New(errs.EComValid1001, err))
		return
	}
	if req.Name == "" {
		writeErr(w, errs.New(errs.EComValid2002, nil))
		return
	}
	if req.Age != nil && *req.Age < 19 {
		writeErr(w, errs.New(errs.EComValid2001, nil))
		return
	}
	if s.client == nil {
		writeErr(w, errs.New(errs.EComModel1001, nil))
		return
	}

	gen, err := s.generateCharacter(r.Context(), req)
	if err != nil {
		writeErr(w, errs.New(errs.EComChar1003, err))
		return
	}
	s.log.Feature("캐릭터AI", "AI 캐릭터 생성 완료 (name=%s, adult=%v)", req.Name, req.Adult)
	writeJSON(w, http.StatusOK, gen)
}

// generateCharacter AI로 캐릭터 설정 생성 (JSON 파싱 실패 시 1회 재시도)
func (s *Server) generateCharacter(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	system := "너는 AI 연애 채팅 앱 '톡맨스'의 캐릭터 설정 디자이너다. " +
		"사용자가 입력한 기본 정보만으로 매력적인 캐릭터 설정을 창작한다. " +
		"응답은 반드시 아래 JSON 스키마만 출력한다 (마크다운 코드블록 금지, JSON 외 텍스트 금지):\n" +
		`{"title": "짧은 타이틀", "personality": ["성격 3~4개"], "tone": "말투 1줄", "story": "2~3줄 스토리", "backstory": "사용자와의 관계 설정 2~3줄", "greeting": "첫 인사말 1줄", "avatar_prompt": "영문 캐릭터 아바타 묘사 (화풍 포함)"}`

	var input strings.Builder
	input.WriteString("기본 정보:\n")
	input.WriteString("- 이름: " + req.Name + "\n")
	if req.Gender != "" {
		input.WriteString("- 성별: " + req.Gender + "\n")
	}
	if req.Age != nil {
		input.WriteString(fmt.Sprintf("- 나이: %d세\n", *req.Age))
	}
	if req.Relationship != "" {
		input.WriteString("- 사용자와의 관계: " + req.Relationship + "\n")
	}
	if req.Category != "" {
		input.WriteString("- 카테고리: " + req.Category + "\n")
	}
	if req.Adult {
		input.WriteString("- 성인 캐릭터: 예 (성숙한 분위기 허용, 단 avatar_prompt는 반드시 'wholesome cartoon style, safe for work, no nudity' 유지)\n")
	} else {
		input.WriteString("- 성인 캐릭터: 아니오 (밝고 순수한 분위기)\n")
	}
	input.WriteString("\n위 정보를 기반으로 자연스러운 한국어로 캐릭터 설정을 창작해라. story는 캐릭터의 배경 스토리, backstory는 '대화를 시작하기 전에 나눈 대화'로 사용자와의 과거 관계를 구체적으로 써라.")

	msgs := []orapi.ChatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: input.String()},
	}
	content, _, _, err := s.chatOnce(ctx, generateModel, msgs, 0.4)
	if err != nil {
		return nil, err
	}
	if g, err := parseGenerateJSON(content); err == nil {
		g.Name = req.Name
		g.AvatarURL = diceBearAvatar(req.Name)
		return g, nil
	}
	// 1회 재시도 (파싱 실패 시)
	retryCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	content2, _, _, err := s.chatOnce(retryCtx, generateModel, msgs, 0.2)
	if err != nil {
		return nil, err
	}
	g2, err := parseGenerateJSON(content2)
	if err == nil {
		g2.Name = req.Name
		g2.AvatarURL = diceBearAvatar(req.Name)
	}
	return g2, err
}

// parseGenerateJSON 마크다운 코드블록 및 JSON 추출
func parseGenerateJSON(content string) (*GenerateResponse, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("JSON 없음: %s", truncate(content, 200))
	}
	var raw struct {
		Title        string   `json:"title"`
		Personality  []string `json:"personality"`
		Tone         string   `json:"tone"`
		Story        string   `json:"story"`
		Backstory    string   `json:"backstory"`
		Greeting     string   `json:"greeting"`
		AvatarPrompt string   `json:"avatar_prompt"`
	}
	if err := json.Unmarshal([]byte(content[start:end+1]), &raw); err != nil {
		return nil, err
	}
	if raw.Title == "" && raw.Story == "" && raw.Greeting == "" {
		return nil, fmt.Errorf("필수 필드 누락")
	}
	if len(raw.Personality) == 0 {
		raw.Personality = []string{"밝고 긍정적"}
	}
	if raw.AvatarPrompt == "" {
		raw.AvatarPrompt = "portrait of a friendly person, wholesome cartoon style"
	}
	if raw.Tone == "" {
		raw.Tone = "다정하고 부드러운 말투"
	}
	return &GenerateResponse{
		Title: raw.Title,
		Persona: map[string]any{
			personaPerson: raw.Personality,
			personaTone:   raw.Tone,
			personaStory:  raw.Story,
			personaBack:   raw.Backstory,
		},
		Greeting:     raw.Greeting,
		AvatarPrompt: raw.AvatarPrompt,
	}, nil
}

// ---- POST /api/v1/characters/{id}/avatar ----
type AvatarRequest struct {
	Style string `json:"style"` // "dicebear" | "ai"
}

func (s *Server) handleRegenerateAvatar(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, errs.New(errs.EComValid1001, err))
		return
	}
	var req AvatarRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, errs.New(errs.EComValid1001, err))
		return
	}
	if s.store == nil {
		writeErr(w, errs.New(errs.ESRVDb1001, nil))
		return
	}
	c, err := s.store.GetCharacter(r.Context(), id, userID)
	if err != nil {
		writeErr(w, toAppErr(err))
		return
	}

	var avatarURL string
	switch req.Style {
	case "ai":
		avatarURL = s.aiAvatarURL(c)
	case "dicebear", "":
		avatarURL = diceBearAvatar(c.Name)
	default:
		writeErr(w, errs.New(errs.EComValid1001, fmt.Errorf("알 수 없는 스타일: %s", req.Style)))
		return
	}
	if err := s.store.UpdateAvatarURL(r.Context(), id, userID, avatarURL); err != nil {
		writeErr(w, toAppErr(err))
		return
	}
	s.log.Feature("캐릭터AI", "아바타 재생성 (id=%s, style=%s)", id, req.Style)
	writeJSON(w, http.StatusOK, map[string]any{"avatar_url": avatarURL})
}

// diceBearAvatar 랜덤 시드 + 스타일 로테이션 아바타
func diceBearAvatar(name string) string {
	style := diceBearStyles[rand.Intn(len(diceBearStyles))]
	seed := url.QueryEscape(fmt.Sprintf("%s-%d", name, time.Now().UnixNano()%100000))
	return fmt.Sprintf("https://api.dicebear.com/9.x/%s/png?seed=%s&backgroundColor=b6e3f4", style, seed)
}

// aiAvatarURL Pollinations AI 이미지 URL (persona avatar_prompt 우선, 없으면 이름 파생)
func (s *Server) aiAvatarURL(c *store.Character) string {
	prompt := ""
	if p, ok := c.Persona[personaAvatar].(string); ok && p != "" {
		prompt = p
	} else {
		prompt = fmt.Sprintf("portrait of %s, wholesome cartoon style", c.Name)
	}
	if !c.Adult {
		prompt += ", wholesome, bright and cheerful"
	}
	seed := time.Now().UnixNano() % 100000
	u := url.QueryEscape(prompt)
	return fmt.Sprintf("https://image.pollinations.ai/prompt/%s?width=512&height=512&seed=%d&nologo=true", u, seed)
}

// truncate 긴 문자열 축약 (로그용)
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}