package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/talkmance/server/internal/errs"
	"github.com/talkmance/server/internal/store"
)

// ---- GET /api/v1/sessions ----
func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	if s.store == nil {
		writeErr(w, errs.New(errs.ESRVDb1001, nil))
		return
	}
	sessions, err := s.store.ListSessions(r.Context(), userID)
	if err != nil {
		writeErr(w, toAppErr(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
}

// ---- POST /api/v1/sessions ----
func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	var req struct {
		CharacterID uuid.UUID `json:"character_id"`
		ModelID     string    `json:"model_id"`
		RuleID      uuid.UUID `json:"rule_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, errs.New(errs.EComValid1001, err))
		return
	}
	if req.CharacterID == uuid.Nil {
		writeErr(w, errs.New(errs.EComSess1001, nil))
		return
	}
	if s.store == nil {
		writeErr(w, errs.New(errs.ESRVDb1001, nil))
		return
	}
	id, err := s.store.CreateSession(r.Context(), userID, store.ChatSession{
		CharacterID: req.CharacterID,
		ModelID:     req.ModelID,
		RuleID:      optionalUUID(req.RuleID),
	})
	if err != nil {
		writeErr(w, toAppErr(err))
		return
	}
	s.log.Feature("세션", "대화방 생성됨 (id=%s, character=%s)", id, req.CharacterID)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id.String()})
}

// ---- GET /api/v1/sessions/{id} + 메시지 ----
func (s *Server) handleSession(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
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
	case http.MethodGet:
		sess, err := s.store.GetSession(r.Context(), id, userID)
		if err != nil {
			writeErr(w, toAppErr(err))
			return
		}
		writeJSON(w, http.StatusOK, sess)

	case http.MethodPut:
		var req struct {
			ModelID string `json:"model_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, errs.New(errs.EComValid1001, err))
			return
		}
		if err := s.store.UpdateSessionModelID(r.Context(), id, userID, req.ModelID); err != nil {
			writeErr(w, toAppErr(err))
			return
		}
		s.log.Feature("세션", "모델 변경됨 (id=%s, model=%s)", id, req.ModelID)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	case http.MethodDelete:
		if err := s.store.DeleteSession(r.Context(), id, userID); err != nil {
			writeErr(w, toAppErr(err))
			return
		}
		s.log.Feature("세션", "대화방 삭제됨 (id=%s)", id)
		w.WriteHeader(http.StatusNoContent)
	}
}

// ---- GET /api/v1/sessions/{id}/messages ----
func (s *Server) handleListMessages(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, errs.New(errs.EComValid1001, err))
		return
	}
	if s.store == nil {
		writeErr(w, errs.New(errs.ESRVDb1001, nil))
		return
	}
	msgs, err := s.store.ListMessages(r.Context(), id, userID)
	if err != nil {
		writeErr(w, toAppErr(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

// ---- 헬퍼 ----

func optionalUUID(u uuid.UUID) *uuid.UUID {
	if u == uuid.Nil {
		return nil
	}
	return &u
}
