package httpapi

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/talkmance/server/internal/store"
)

// CustomModelCache 사용자별 커스텀 모델 목록 캐시 (5초 TTL)
// 원격 DB(Neon) 왕복이 ~200ms로 /models 응답을 늦추는 병목이라 추가 (T-26 k6 실측)
type CustomModelCache struct {
	mu    sync.Mutex
	items map[uuid.UUID]customCacheEntry
}

type customCacheEntry struct {
	models  []store.CustomModel
	fetched time.Time
}

const customModelCacheTTL = 5 * time.Second

func NewCustomModelCache() *CustomModelCache {
	return &CustomModelCache{items: make(map[uuid.UUID]customCacheEntry)}
}

// Get 캐시 히트 시 반환, 미스 시 nil (호출자가 DB 조회 후 Put)
func (c *CustomModelCache) Get(userID uuid.UUID) ([]store.CustomModel, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.items[userID]
	if !ok || time.Since(e.fetched) > customModelCacheTTL {
		return nil, false
	}
	return e.models, true
}

// Put DB 조회 결과 저장 (5초 TTL)
func (c *CustomModelCache) Put(userID uuid.UUID, models []store.CustomModel) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[userID] = customCacheEntry{models: models, fetched: time.Now()}
}

// Invalidate CRUD 후 캐시 무효화
func (c *CustomModelCache) Invalidate(userID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, userID)
}