package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/talkmance/server/internal/crypto"
	"github.com/talkmance/server/internal/errs"
)

// ---- GET /api/v1/settings/keys — BYOK 키 목록 ----
func (s *Server) handleListKeys(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	if s.store == nil {
		writeErr(w, errs.New(errs.ESRVDb1001, nil))
		return
	}
	keys, err := s.store.ListUserKeys(r.Context(), userID)
	if err != nil {
		writeErr(w, toAppErr(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": keys})
}

// ---- POST /api/v1/settings/keys — BYOK 키 등록 (AES-GCM 암호화 저장) ----
func (s *Server) handleCreateKey(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	var req struct {
		Label  string `json:"label"`
		APIKey string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, errs.New(errs.EComValid1001, err))
		return
	}
	if req.Label == "" || req.APIKey == "" {
		writeErr(w, errs.New(errs.EComModel2001, nil))
		return
	}
	if s.store == nil {
		writeErr(w, errs.New(errs.ESRVDb1001, nil))
		return
	}
	if s.cfg.EncKey == "" {
		writeErr(w, errs.New(errs.ESRVDb1001, nil))
		return
	}

	enc, err := crypto.Encrypt(req.APIKey, s.cfg.EncKey)
	if err != nil {
		s.log.Errorf("KEYS", "암호화 실패: %v", err)
		writeErr(w, errs.New(errs.ESRVDb1001, err))
		return
	}
	id, err := s.store.CreateUserKey(r.Context(), userID, req.Label, enc)
	if err != nil {
		writeErr(w, toAppErr(err))
		return
	}
	s.log.Feature("BYOK", "API 키 등록됨 (label=%s)", req.Label)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id.String()})
}

// ---- DELETE /api/v1/settings/keys/{id} ----
func (s *Server) handleDeleteKey(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, errs.New(errs.EComValid1001, err))
		return
	}
	if s.store == nil {
		writeErr(w, errs.New(errs.ESRVDb1001, nil))
		return
	}
	if err := s.store.DeleteUserKey(r.Context(), id, userID); err != nil {
		writeErr(w, toAppErr(err))
		return
	}
	s.log.Feature("BYOK", "API 키 삭제됨 (id=%s)", id)
	w.WriteHeader(http.StatusNoContent)
}
