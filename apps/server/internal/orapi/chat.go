package orapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/talkmance/server/internal/errs"
)

// ChatMessage 채팅 메시지 (OpenRouter 포맷)
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest 채팅 요청
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream"`
}

// ChatChunk SSE 스트림 청크 (delta + usage)
type ChatChunk struct {
	Content  string
	Done     bool
	TokenIn  int
	TokenOut int
	Cost     float64
	Err      error
}

// ChatStream 모델 응답을 SSE 청크로 스트리밍 (ch <- 값, 끝나면 close)
// apiKey 빈 값이면 서버 키 사용. baseURL 빈 값이면 OpenRouter.
func (c *Client) ChatStream(ctx context.Context, baseURL, apiKey, model string, messages []ChatMessage, ch chan<- ChatChunk) {
	defer close(ch)

	if apiKey == "" {
		apiKey = c.apiKey
	}
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}

	body, err := json.Marshal(ChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
	})
	if err != nil {
		ch <- ChatChunk{Err: errs.Wrap(errs.ESRVNet1001, err)}
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		ch <- ChatChunk{Err: errs.Wrap(errs.ESRVNet1001, err)}
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		ch <- ChatChunk{Err: errs.Wrap(errs.ESRVNet1001, err)}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		ch <- ChatChunk{Err: fmt.Errorf("orapi: 채팅 응답 %d: %s", resp.StatusCode, truncate(string(msg), 300))}
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if !bytes.HasPrefix([]byte(line), []byte("data:")) {
			continue
		}
		data := bytes.TrimSpace(bytes.TrimPrefix([]byte(line), []byte("data:")))
		if string(data) == "[DONE]" {
			ch <- ChatChunk{Done: true}
			return
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int     `json:"prompt_tokens"`
				CompletionTokens int     `json:"completion_tokens"`
				TotalCost        float64 `json:"total_cost"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(data, &chunk); err != nil {
			continue
		}
		if chunk.Usage != nil {
			ch <- ChatChunk{TokenIn: chunk.Usage.PromptTokens, TokenOut: chunk.Usage.CompletionTokens, Cost: chunk.Usage.TotalCost}
			continue
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			ch <- ChatChunk{Content: chunk.Choices[0].Delta.Content}
		}
	}
	if err := scanner.Err(); err != nil {
		ch <- ChatChunk{Err: errs.Wrap(errs.ESRVNet1001, err)}
	}
}

// ChatOnce 비스트림 단일 응답 (요약/백그라운드 작업용)
func (c *Client) ChatOnce(ctx context.Context, baseURL, apiKey, model string, messages []ChatMessage, temperature float64) (content string, tokenIn, tokenOut int, err error) {
	if apiKey == "" {
		apiKey = c.apiKey
	}
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	body, err := json.Marshal(ChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: &temperature,
		Stream:      false,
	})
	if err != nil {
		return "", 0, 0, errs.Wrap(errs.ESRVNet1001, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", 0, 0, errs.Wrap(errs.ESRVNet1001, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", 0, 0, errs.Wrap(errs.ESRVNet1001, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", 0, 0, fmt.Errorf("orapi: 채팅 응답 %d: %s", resp.StatusCode, truncate(string(msg), 300))
	}
	var body2 struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body2); err != nil {
		return "", 0, 0, errs.Wrap(errs.ESRVNet1001, err)
	}
	if len(body2.Choices) == 0 {
		return "", 0, 0, fmt.Errorf("orapi: 응답 없음")
	}
	content = body2.Choices[0].Message.Content
	if body2.Usage != nil {
		tokenIn, tokenOut = body2.Usage.PromptTokens, body2.Usage.CompletionTokens
	}
	return content, tokenIn, tokenOut, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
