package orapi

import (
	"context"
	"os"
	"testing"

	"github.com/talkmance/server/internal/log"
)

// 실통합 테스트: 키가 있으면 실행
func testKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" {
		t.Skip("OPENROUTER_API_KEY 없음 — 스킵")
	}
	return key
}

func TestFetchModels(t *testing.T) {
	key := testKey(t)
	c := New(key, log.New("error"))
	models, err := c.FetchModels(context.Background())
	if err != nil {
		t.Fatalf("카탈로그 조회 실패: %v", err)
	}
	if len(models) < 100 {
		t.Errorf("모델 수=%d, want >=100", len(models))
	}
	// free 감지
	var free int
	for _, m := range models {
		if m.ID == "" || m.Name == "" {
			t.Errorf("필수 필드 누락: %+v", m)
		}
		if len(m.ID) > 8 && m.ID[len(m.ID)-5:] == ":free" {
			free++
		}
	}
	if free == 0 {
		t.Error("free 모델 0개 — :free 감지 실패")
	}
}

func TestCatalogCache(t *testing.T) {
	key := testKey(t)
	c := New(key, log.New("error"))
	cat := NewCatalog(c)

	m1, err := cat.List(context.Background(), false)
	if err != nil {
		t.Fatalf("최초 조회 실패: %v", err)
	}
	if len(m1) == 0 {
		t.Fatal("빈 카탈로그")
	}
	// 캐시 히트 확인 (fetched 시각이 갱신되지 않아야 함)
	m2, err := cat.List(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(m2) != len(m1) {
		t.Errorf("캐시 불일치: %d vs %d", len(m1), len(m2))
	}
}

func TestQuotaService(t *testing.T) {
	key := testKey(t)
	c := New(key, log.New("error"))
	q := NewQuotaService(c)

	quota, err := q.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("할당량 조회 실패: %v", err)
	}
	if quota.Label == "" {
		t.Errorf("키 라벨 없음: %+v", quota)
	}
	// free tier는 limit=null 가능 — limit 있으면 양수여야 함
	if quota.Limit != nil && *quota.Limit <= 0 {
		t.Errorf("Limit=%f, want >0", *quota.Limit)
	}
	// 캐시 히트
	q2, err := q.Get(context.Background(), key)
	if err != nil || q2 == nil {
		t.Errorf("캐시 히트 실패: %v", err)
	}
}
