package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/talkmance/server/internal/errs"
	"github.com/talkmance/server/internal/store"
)

// ---- GET /api/v1/rules ----
func (s *Server) handleListRules(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	if s.store == nil {
		writeErr(w, errs.New(errs.ESRVDb1001, nil))
		return
	}
	rules, err := s.store.ListRules(r.Context(), userID)
	if err != nil {
		writeErr(w, toAppErr(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": rules})
}

// ---- POST /api/v1/rules ----
func (s *Server) handleCreateRule(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	var req store.PromptRule
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, errs.New(errs.EComValid1001, err))
		return
	}
	if req.Name == "" {
		writeErr(w, errs.New(errs.EComRule2001, nil))
		return
	}
	if s.store == nil {
		writeErr(w, errs.New(errs.ESRVDb1001, nil))
		return
	}
	id, isDefault, err := s.store.CreateRule(r.Context(), userID, req)
	if err != nil {
		writeErr(w, toAppErr(err))
		return
	}
	s.log.Feature("규칙", "규칙 생성됨 (id=%s, name=%s, default=%v)", id, req.Name, isDefault)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id.String(), "is_default": isDefault})
}

// ---- GET/PUT/DELETE /api/v1/rules/{id} ----
func (s *Server) handleRule(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
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
		p, err := s.store.GetRule(r.Context(), id, userID)
		if err != nil {
			writeErr(w, toAppErr(err))
			return
		}
		writeJSON(w, http.StatusOK, p)

	case http.MethodPut:
		var req store.PromptRule
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, errs.New(errs.EComValid1001, err))
			return
		}
		if err := s.store.UpdateRule(r.Context(), id, userID, req); err != nil {
			writeErr(w, toAppErr(err))
			return
		}
		s.log.Feature("규칙", "규칙 수정됨 (id=%s)", id)
		w.WriteHeader(http.StatusNoContent)

	case http.MethodDelete:
		if err := s.store.DeleteRule(r.Context(), id, userID); err != nil {
			writeErr(w, toAppErr(err))
			return
		}
		s.log.Feature("규칙", "규칙 삭제됨 (id=%s)", id)
		w.WriteHeader(http.StatusNoContent)
	}
}

// ---- POST /api/v1/rules/{id}/default ----
func (s *Server) handleSetDefaultRule(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, errs.New(errs.EComValid1001, err))
		return
	}
	if s.store == nil {
		writeErr(w, errs.New(errs.ESRVDb1001, nil))
		return
	}
	if err := s.store.SetDefaultRule(r.Context(), id, userID); err != nil {
		writeErr(w, toAppErr(err))
		return
	}
	s.log.Feature("규칙", "기본 규칙 변경됨 (id=%s)", id)
	w.WriteHeader(http.StatusNoContent)
}
