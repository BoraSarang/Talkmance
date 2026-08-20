package httpapi

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/talkmance/server/internal/errs"
	"github.com/talkmance/server/internal/store"
)

// defaultAvatar DiceBear 기본 아바타 (문서 11.3 — pixel-art 스타일, 파스텔 배경)
func defaultAvatar(name string) string {
	seed := url.QueryEscape(name)
	return "https://api.dicebear.com/9.x/pixel-art/png?seed=" + seed + "&backgroundColor=b6e3f4"
}

// ---- GET /api/v1/characters ----
func (s *Server) handleListCharacters(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	if s.store == nil {
		writeErr(w, errs.New(errs.ESRVDb1001, nil))
		return
	}
	chars, err := s.store.ListCharacters(r.Context(), userID)
	if err != nil {
		writeErr(w, toAppErr(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"characters": chars})
}

// ---- POST /api/v1/characters ----
func (s *Server) handleCreateCharacter(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	var req store.Character
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, errs.New(errs.EComValid1001, err))
		return
	}
	if req.Name == "" {
		writeErr(w, errs.New(errs.EComChar1001, nil))
		return
	}
	// 문서 11.3: avatar_url 없으면 DiceBear 기본 아바타 자동 생성
	if req.AvatarURL == "" {
		req.AvatarURL = defaultAvatar(req.Name)
	}
	if s.store == nil {
		writeErr(w, errs.New(errs.ESRVDb1001, nil))
		return
	}
	id, err := s.store.CreateCharacter(r.Context(), userID, req)
	if err != nil {
		writeErr(w, toAppErr(err))
		return
	}
	s.log.Feature("캐릭터", "캐릭터 생성됨 (id=%s, name=%s, avatar=%s)", id, req.Name, req.AvatarURL)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id.String()})
}

// ---- GET/PUT/DELETE /api/v1/characters/{id} ----
func (s *Server) handleCharacter(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
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
		c, err := s.store.GetCharacter(r.Context(), id, userID)
		if err != nil {
			writeErr(w, toAppErr(err))
			return
		}
		writeJSON(w, http.StatusOK, c)

	case http.MethodPut:
		var req store.Character
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, errs.New(errs.EComValid1001, err))
			return
		}
		if err := s.store.UpdateCharacter(r.Context(), id, userID, req); err != nil {
			writeErr(w, toAppErr(err))
			return
		}
		s.log.Feature("캐릭터", "캐릭터 수정됨 (id=%s)", id)
		updated, err := s.store.GetCharacter(r.Context(), id, userID)
		if err != nil {
			writeErr(w, toAppErr(err))
			return
		}
		writeJSON(w, http.StatusOK, updated)

	case http.MethodDelete:
		if err := s.store.DeleteCharacter(r.Context(), id, userID); err != nil {
			writeErr(w, toAppErr(err))
			return
		}
		s.log.Feature("캐릭터", "캐릭터 삭제됨 (id=%s)", id)
		w.WriteHeader(http.StatusNoContent)
	}
}
