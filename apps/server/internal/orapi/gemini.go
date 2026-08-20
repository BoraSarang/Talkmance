package orapi

import (
	"context"
)

// geminiBaseURL Gemini OpenAI 호환 엔드포인트 (v1main — 스트림 미지원, 비스트림만)
// AQ... 형식 OAuth 토큰도 Bearer 인증으로 동작
const geminiBaseURL = "https://generativelanguage.googleapis.com/v1beta/openai"

// GeminiStream — Gemini는 v1main OpenAI 호환에서 비스트림만 지원하므로
// 전체 응답을 받아 글자 단위 청크로 쪼개 스트림처럼 전달한다.
func (c *Client) GeminiStream(ctx context.Context, apiKey, model string, messages []ChatMessage, ch chan<- ChatChunk) {
	defer close(ch)

	content, tokenIn, tokenOut, err := c.ChatOnce(ctx, geminiBaseURL, apiKey, model, messages, 0.7)
	if err != nil {
		ch <- ChatChunk{Err: err}
		return
	}
	// UTF-8 안전하게 rune 단위 분할
	for _, r := range content {
		ch <- ChatChunk{Content: string(r)}
	}
	ch <- ChatChunk{TokenIn: tokenIn, TokenOut: tokenOut}
	ch <- ChatChunk{Done: true}
}