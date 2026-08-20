package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/talkmance/server/internal/errs"
	"github.com/talkmance/server/internal/store"
)

// ---- GET /api/v1/models ----
// 쿼리: free=true (free만), q=<검색>, custom=true (커스텀만)
// 정책: Gemini는 전 모델 표시(무료 키), OpenRouter/OpenCode 카탈로그는 free 모델만
var geminiCatalog = []struct{ ID, Name string }{
	{"gemini-3-flash-preview", "Gemini 3 Flash Preview"},
	{"gemini-3.5-flash", "Gemini 3.5 Flash"},
	{"gemini-3.5-flash-lite", "Gemini 3.5 Flash Lite"},
	{"gemini-3.6-flash", "Gemini 3.6 Flash"},
	{"gemini-3.1-flash-lite", "Gemini 3.1 Flash Lite"},
	{"gemini-3.1-pro-preview", "Gemini 3.1 Pro Preview"},
	{"gemini-flash-latest", "Gemini Flash Latest"},
	{"gemini-flash-lite-latest", "Gemini Flash-Lite Latest"},
	{"gemini-pro-latest", "Gemini Pro Latest"},
	{"gemini-2.5-flash", "Gemini 2.5 Flash"},
	{"gemini-2.5-pro", "Gemini 2.5 Pro"},
	{"gemma-4-26b-a4b-it", "Gemma 4 26B A4B IT"},
	{"gemma-4-31b-it", "Gemma 4 31B IT"},
}

// nvidiaCatalog NVIDIA NIM 무료 모델 (프로브로 동작 확정된 모델만, 키 설정 시 노출)
var nvidiaCatalog = []struct{ ID, Name string }{
	{"google/gemma-4-31b-it", "Gemma 4 31B IT (NVIDIA)"},
}

func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	if s.catalog == nil || s.store == nil {
		writeErr(w, errs.New(errs.ESRVDb1001, nil))
		return
	}
	models, err := s.catalog.List(r.Context(), false)
	if err != nil {
		writeErr(w, toAppErr(err))
		return
	}

	onlyFree := r.URL.Query().Get("free") == "true"
	q := strings.ToLower(r.URL.Query().Get("q"))

	// 카탈로그 필터 + 경량 변환
	type catalogItem struct {
		ID            string  `json:"id"`
		Name          string  `json:"name"`
		Description   string  `json:"description"`
		ContextLength int     `json:"context_length"`
		IsFree        bool    `json:"is_free"`
		PromptPrice   float64 `json:"prompt_price_usd"`
		Completion    float64 `json:"completion_price_usd"`
	}
	catalog := make([]catalogItem, 0, len(models)+len(geminiCatalog))
	// Gemini 전 모델 (무료 키 — is_free=true)
	for _, g := range geminiCatalog {
		if onlyFree {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(g.ID), q) && !strings.Contains(strings.ToLower(g.Name), q) {
			continue
		}
		catalog = append(catalog, catalogItem{ID: g.ID, Name: g.Name, IsFree: true})
	}
	// NVIDIA NIM (키 설정 시 — is_free=true)
	if s.nvidiaAPIKey != "" {
		for _, n := range nvidiaCatalog {
			if onlyFree {
				continue
			}
			if q != "" && !strings.Contains(strings.ToLower(n.ID), q) && !strings.Contains(strings.ToLower(n.Name), q) {
				continue
			}
			catalog = append(catalog, catalogItem{ID: n.ID, Name: n.Name, IsFree: true})
		}
	}
	// OpenRouter/OpenCode: free 모델만 노출
	for _, m := range models {
		if !strings.HasSuffix(m.ID, ":free") {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(m.ID), q) && !strings.Contains(strings.ToLower(m.Name), q) {
			continue
		}
		catalog = append(catalog, catalogItem{
			ID:            m.ID,
			Name:          m.Name,
			Description:   m.Description,
			ContextLength: m.ContextLength,
			IsFree:        true,
			PromptPrice:   parsePrice(m.Pricing.Prompt),
			Completion:    parsePrice(m.Pricing.Completion),
		})
	}

	custom, err := s.customModels(r.Context(), userID)
	if err != nil {
		writeErr(w, toAppErr(err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"catalog": catalog,
		"custom":  custom,
	})
}

// ---- POST /api/v1/models/custom ----
func (s *Server) handleCreateCustomModel(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	var req store.CustomModel
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, errs.New(errs.EComValid1001, err))
		return
	}
	if req.Name == "" || req.ModelID == "" {
		writeErr(w, errs.New(errs.EComModel2002, nil))
		return
	}
	if s.store == nil {
		writeErr(w, errs.New(errs.ESRVDb1001, nil))
		return
	}
	id, err := s.store.CreateCustomModel(r.Context(), userID, req)
	if err != nil {
		writeErr(w, toAppErr(err))
		return
	}
	s.customCache.Invalidate(userID)
	s.log.Feature("모델API", "커스텀 모델 추가됨 (id=%s, model_id=%s)", id, req.ModelID)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id.String()})
}

// ---- GET/PUT/DELETE /api/v1/models/custom/{id} ----
func (s *Server) handleCustomModel(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeErr(w, errs.New(errs.EComValid1001, err))
		return
	}
	if s.store == nil {
		writeErr(w, errs.New(errs.ESRVDb1001, nil))
		return
	}

	switch r.Method {
	case http.MethodGet:
		m, err := s.store.GetCustomModel(r.Context(), id, userID)
		if err != nil {
			writeErr(w, toAppErr(err))
			return
		}
		writeJSON(w, http.StatusOK, m)

	case http.MethodPut:
		var req store.CustomModel
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, errs.New(errs.EComValid1001, err))
			return
		}
		if err := s.store.UpdateCustomModel(r.Context(), id, userID, req); err != nil {
			writeErr(w, toAppErr(err))
			return
		}
		s.customCache.Invalidate(userID)
		s.log.Feature("모델API", "커스텀 모델 수정됨 (id=%s)", id)
		w.WriteHeader(http.StatusNoContent)

	case http.MethodDelete:
		if err := s.store.DeleteCustomModel(r.Context(), id, userID); err != nil {
			writeErr(w, toAppErr(err))
			return
		}
		s.customCache.Invalidate(userID)
		s.log.Feature("모델API", "커스텀 모델 삭제됨 (id=%s)", id)
		w.WriteHeader(http.StatusNoContent)
	}
}

// ---- GET /api/v1/quota ----
func (s *Server) handleQuota(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	if s.quota == nil {
		writeErr(w, errs.New(errs.EComQuota1002, nil))
		return
	}
	quota, err := s.quota.Get(r.Context(), s.cfg.OpenRouterKey)
	if err != nil {
		writeErr(w, toAppErr(err))
		return
	}
	// BYOK 키 할당량도 함께 (사용자 키 목록 → 각각 조회는 무거우므로 서버 키만 우선)
	freeUsed := s.freeToday.Used()
	remaining := freeDailyLimit - freeUsed
	if remaining < 0 {
		remaining = 0
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data":             quota,
		"free_used_today":  freeUsed,
		"free_remaining":   remaining,
		"free_limit_daily": freeDailyLimit,
	})
}

// ---- 헬퍼 ----

// customModels 커스텀 모델 목록 (5초 캐시 — 원격 DB 왕복 병목 완화)
func (s *Server) customModels(ctx context.Context, userID uuid.UUID) ([]store.CustomModel, error) {
	if models, ok := s.customCache.Get(userID); ok {
		return models, nil
	}
	models, err := s.store.ListCustomModels(ctx, userID)
	if err != nil {
		return nil, err
	}
	s.customCache.Put(userID, models)
	return models, nil
}

func parsePrice(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

func toAppErr(err error) *errs.AppError {
	if ae, ok := err.(*errs.AppError); ok {
		return ae
	}
	return errs.New(errs.ESRVDb1001, err)
}
