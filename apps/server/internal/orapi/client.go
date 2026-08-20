// Package orapi — OpenRouter API 클라이언트 (카탈로그/할당량/채팅)
package orapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/talkmance/server/internal/errs"
	"github.com/talkmance/server/internal/log"
)

const (
	baseURL       = "https://openrouter.ai/api/v1"
	catalogTTL    = 12 * time.Hour
	quotaCacheTTL = 5 * time.Minute
	httpTimeout = 90 * time.Second
)

// Model OpenRouter 카탈로그 모델 (필요 필드만)
type Model struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	ContextLength int     `json:"context_length"`
	Pricing       Pricing `json:"pricing"`
	Endpoints     []any   `json:"endpoints"`
}

// Pricing 토큰당 USD
type Pricing struct {
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
	Image      string `json:"image"`
	Request    string `json:"request"`
}

// Client OpenRouter API 클라이언트
type Client struct {
	apiKey string
	http   *http.Client
	logger *log.Logger
}

// New 클라이언트 생성
func New(apiKey string, logger *log.Logger) *Client {
	return &Client{
		apiKey: apiKey,
		http:   &http.Client{Timeout: httpTimeout},
		logger: logger,
	}
}

// FetchModels 카탈로그 원본 조회
func (c *Client) FetchModels(ctx context.Context) ([]Model, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return nil, errs.Wrap(errs.ESRVNet1001, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, errs.Wrap(errs.ESRVNet1001, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("orapi: 카탈로그 응답 %d", resp.StatusCode)
	}
	var body struct {
		Data []Model `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, errs.Wrap(errs.ESRVNet1001, err)
	}
	c.logger.Feature("카탈로그", "OpenRouter 모델 %d개 수신", len(body.Data))
	return body.Data, nil
}

// Catalog 동기화 캐시된 카탈로그 (12시간 TTL)
type Catalog struct {
	client  *Client
	mu      sync.Mutex
	models  []Model
	fetched time.Time
}

// NewCatalog 카탈로그 서비스
func NewCatalog(client *Client) *Catalog {
	return &Catalog{client: client}
}

// List 최신 카탈로그 반환 (캐시 만료 시 재조회)
func (c *Catalog) List(ctx context.Context, force bool) ([]Model, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !force && c.fetched.IsZero() == false && time.Since(c.fetched) < catalogTTL && len(c.models) > 0 {
		return c.models, nil
	}
	models, err := c.client.FetchModels(ctx)
	if err != nil {
		// 캐시가 있으면 stale 반환 (일시 장애 허용)
		if len(c.models) > 0 {
			c.client.logger.Warnf("카탈로그", "재조회 실패 — 캐시 사용 (%v)", err)
			return c.models, nil
		}
		return nil, err
	}
	c.models = models
	c.fetched = time.Now()
	c.client.logger.Feature("카탈로그", "카탈로그 동기화 완료 (%d개, TTL 12h)", len(models))
	return models, nil
}

// KeyQuota OpenRouter 키 할당량/잔액 (5분 캐시)
type KeyQuota struct {
	// /api/v1/auth/key 응답 주요 필드 (free tier는 limit이 null)
	Label      string   `json:"label"`
	Usage      float64  `json:"usage"`
	Limit      *float64 `json:"limit"`
	IsFreeTier bool     `json:"is_free_tier"`
	RateLimit  *struct {
		Requests int    `json:"requests"`
		Interval string `json:"interval"`
	} `json:"rate_limit"`
}

// QuotaService 키 할당량 조회 (캐시)
type QuotaService struct {
	client *Client
	mu     sync.Mutex
	cache  map[string]*quotaCacheEntry
}

type quotaCacheEntry struct {
	quota   *KeyQuota
	fetched time.Time
}

// NewQuotaService 할당량 서비스 생성
func NewQuotaService(client *Client) *QuotaService {
	return &QuotaService{client: client, cache: map[string]*quotaCacheEntry{}}
}

// Get 키 할당량 조회 (5분 캐시)
func (q *QuotaService) Get(ctx context.Context, apiKey string) (*KeyQuota, error) {
	q.mu.Lock()
	if e, ok := q.cache[apiKey]; ok && time.Since(e.fetched) < quotaCacheTTL {
		q.mu.Unlock()
		return e.quota, nil
	}
	q.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/auth/key", nil)
	if err != nil {
		return nil, errs.Wrap(errs.ESRVNet1001, err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := q.client.http.Do(req)
	if err != nil {
		return nil, errs.Wrap(errs.ESRVNet1001, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("orapi: 키 조회 응답 %d", resp.StatusCode)
	}
	var body struct {
		Data *KeyQuota `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, errs.Wrap(errs.ESRVNet1001, err)
	}
	if body.Data == nil {
		return nil, fmt.Errorf("orapi: 키 데이터 없음")
	}

	q.mu.Lock()
	q.cache[apiKey] = &quotaCacheEntry{quota: body.Data, fetched: time.Now()}
	q.mu.Unlock()
	q.client.logger.Feature("할당량", "키 할당량 갱신 (free=%v, usage=%.0f)", body.Data.IsFreeTier, body.Data.Usage)
	return body.Data, nil
}
