// freecounter.go — OpenRouter 무료 티어 일일 사용 카운터 (50회/일, UTC 기준)
// DB(daily_usage)에 저장 — 서버 재시작에도 유지. store가 nil이면 메모리 폴백.
package httpapi

import (
	"context"
	"sync"
	"time"

	"github.com/talkmance/server/internal/log"
	"github.com/talkmance/server/internal/store"
)

const freeDailyLimit = 50

// FreeDailyCounter 일자별 무료 모델 호출 시도 횟수 (DB 영속 + 메모리 캐시)
type FreeDailyCounter struct {
	store  *store.Store
	logger *log.Logger
	mu     sync.Mutex
	date   string
	count  int
}

func NewFreeDailyCounter(st *store.Store, logger *log.Logger) *FreeDailyCounter {
	// date를 ""로 시작 — 첫 Used()/Inc()에서 DB 로드가 반드시 실행되도록
	return &FreeDailyCounter{store: st, logger: logger, date: ""}
}

func todayKey() string { return time.Now().UTC().Format("2006-01-02") }

// load 당일 기준 메모리 캐시를 DB 값과 동기화
func (c *FreeDailyCounter) loadLocked() {
	today := todayKey()
	if c.date == today {
		return
	}
	c.date = today
	c.count = 0
	if c.store != nil {
		if n, err := c.store.GetFreeUsage(context.Background(), today); err == nil {
			c.count = n
		} else if c.logger != nil {
			c.logger.Errorf("FREE", "무료 사용량 DB 조회 실패: %v", err)
		}
	}
}

// Inc 무료 모델 호출 시도 +1 (DB upsert)
func (c *FreeDailyCounter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loadLocked()
	c.count++
	if c.store != nil {
		if err := c.store.IncFreeUsage(context.Background(), c.date); err != nil && c.logger != nil {
			c.logger.Errorf("FREE", "무료 사용량 DB 저장 실패: %v", err)
		}
	}
}

// Used 오늘 사용 횟수
func (c *FreeDailyCounter) Used() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loadLocked()
	return c.count
}