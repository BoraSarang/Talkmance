package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/talkmance/server/internal/errs"
	"github.com/talkmance/server/internal/store"
)

// ---- GET /api/v1/memories/{characterId} — 기억 목록 ----
func (s *Server) handleListMemories(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	cid, err := uuid.Parse(r.PathValue("characterId"))
	if err != nil {
		writeErr(w, errs.New(errs.EComValid1001, err))
		return
	}
	if s.store == nil {
		writeErr(w, errs.New(errs.ESRVDb1001, nil))
		return
	}
	mems, err := s.store.ListMemories(r.Context(), cid, userID)
	if err != nil {
		writeErr(w, toAppErr(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"memories": mems})
}

// ---- POST /api/v1/memories/{characterId} — 기억 저장 ----
func (s *Server) handleCreateMemory(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	cid, err := uuid.Parse(r.PathValue("characterId"))
	if err != nil {
		writeErr(w, errs.New(errs.EComValid1001, err))
		return
	}
	var req store.Memory
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, errs.New(errs.EComValid1001, err))
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeErr(w, errs.New(errs.EComMem1001, nil))
		return
	}
	if s.store == nil {
		writeErr(w, errs.New(errs.ESRVDb1001, nil))
		return
	}
	id, err := s.store.CreateMemory(r.Context(), cid, userID, req)
	if err != nil {
		writeErr(w, toAppErr(err))
		return
	}
	s.log.Feature("기억", "기억 저장됨 (id=%s, type=%s)", id, req.MemType)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id.String()})
}

// ---- PUT/DELETE /api/v1/memories/{id} ----
func (s *Server) handleMemory(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, errs.New(errs.EComValid1001, err))
		return
	}
	if s.store == nil {
		writeErr(w, errs.New(errs.ESRVDb1001, nil))
		return
	}

	switch r.Method {
	case http.MethodPut:
		var req store.Memory
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, errs.New(errs.EComValid1001, err))
			return
		}
		if err := s.store.UpdateMemory(r.Context(), id, userID, req); err != nil {
			writeErr(w, toAppErr(err))
			return
		}
		s.log.Feature("기억", "기억 수정됨 (id=%s)", id)
		w.WriteHeader(http.StatusNoContent)

	case http.MethodDelete:
		if err := s.store.DeleteMemory(r.Context(), id, userID); err != nil {
			writeErr(w, toAppErr(err))
			return
		}
		s.log.Feature("기억", "기억 삭제됨 (id=%s)", id)
		w.WriteHeader(http.StatusNoContent)
	}
}

// ---- [MEMORY_SAVE] 태그 파서 (T-19, DESIGN 3.1) ----
// LLM 응답에 "[MEMORY_SAVE] 중요 내용" 포함 시 자동 저장
// 태그는 응답에서 제거되어 사용자에게 노출되지 않는다.
func (s *Server) extractMemoryTags(characterID uuid.UUID, content string) (clean string, memories []string) {
	clean = content
	for {
		start := strings.Index(clean, "[MEMORY_SAVE]")
		if start < 0 {
			break
		}
		rest := clean[start+len("[MEMORY_SAVE]"):]
		end := len(rest)
		if nl := strings.Index(rest, "\n"); nl >= 0 {
			end = nl
		}
		item := strings.TrimSpace(rest[:end])
		if item != "" {
			memories = append(memories, item)
		}
		clean = clean[:start] + rest[end:]
	}
	clean = strings.TrimSpace(clean)
	return clean, memories
}
