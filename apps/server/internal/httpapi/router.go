// Package httpapi — HTTP 라우터 + 핸들러 (stdlib ServeMux)
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/talkmance/server/internal/auth"
	"github.com/talkmance/server/internal/config"
	"github.com/talkmance/server/internal/errs"
	"github.com/talkmance/server/internal/log"
	"github.com/talkmance/server/internal/orapi"
	"github.com/talkmance/server/internal/store"
)

// Server HTTP 서버 의존성
type Server struct {
	cfg         *config.Config
	log         *log.Logger
	mux         *http.ServeMux
	auth        *auth.Service
	store       *store.Store
	client      *orapi.Client
	catalog     *orapi.Catalog
	quota       *orapi.QuotaService
	openCodeKey  string
	geminiAPIKey string
	nvidiaAPIKey string
	freeToday    *FreeDailyCounter
	customCache  *CustomModelCache
}

// 모델 상수 (문서 AI_MODELS.json free_models와 동기화)
const (
	zenBaseURL     = "https://opencode.ai/zen/v1" // OpenCode Zen (OpenAI 호환)
	zenModelFree   = "deepseek-v4-flash-free"     // DeepSeek V4 Flash Free
	mimoModelFree  = "mimo-v2.5-free"             // MiMo V2.5 Free
	geminiBaseURL  = "https://generativelanguage.googleapis.com/v1beta/openai" // Gemini OpenAI 호환 (v1main, 스트림 미지원)
	geminiModel    = "gemini-3-flash-preview" // Google Gemini 3 Flash Preview (무료 키)
	nvidiaBaseURL  = "https://integrate.api.nvidia.com/v1" // NVIDIA NIM (OpenAI 호환, 스트림 지원)
	nvidiaModel    = "google/gemma-4-31b-it"               // NVIDIA 무료 — Gemma 4 31B (한국어 우수, 프로브 확정)
	orFallback     = "openai/gpt-oss-20b:free"    // OpenRouter GPT-OSS 20B Free (한국어 우수)
	openRouterFree = "openrouter/free"            // OpenRouter 무료 라우터 (최종 폴백)
)

// New 라우터 생성
func New(cfg *config.Config, logger *log.Logger, authSvc *auth.Service, st *store.Store, cli *orapi.Client, cat *orapi.Catalog, q *orapi.QuotaService) *Server {
	s := &Server{cfg: cfg, log: logger, mux: http.NewServeMux(), auth: authSvc, store: st, client: cli, catalog: cat, quota: q, openCodeKey: cfg.OpenCodeKey, geminiAPIKey: cfg.GeminiAPIKey, nvidiaAPIKey: cfg.NVIDIAAPIKey, freeToday: NewFreeDailyCounter(st, logger), customCache: NewCustomModelCache()}
	s.routes()
	logger.Feature("라우터", "HTTP 라우터 초기화됨")
	return s
}

// chatOnce 비스트림 호출 — Gemini 1순위 → OpenCode Zen → OpenRouter 폴백
// (zen 전용 무료 모델 + OPENCODE_API_KEY 설정 시에만 zen 시도)
func (s *Server) chatOnce(ctx context.Context, model string, msgs []orapi.ChatMessage, temperature float64) (string, int, int, error) {
	if s.client == nil {
		return "", 0, 0, errs.New(errs.EComModel1001, nil)
	}
	if model == "" {
		model = zenModelFree
	}
	// 1순위: NVIDIA (무료 키, OpenRouter 한도와 독립, 스트리밍 지원)
	if s.nvidiaAPIKey != "" {
		content, ti, to, err := s.client.ChatOnce(ctx, nvidiaBaseURL, s.nvidiaAPIKey, nvidiaModel, msgs, temperature)
		if err == nil {
			return content, ti, to, nil
		}
		s.log.Warnf("모델", "NVIDIA 호출 실패 → Gemini 폴백: %v", err)
	}
	// 2순위: Gemini (무료 키, OpenRouter 한도와 독립)
	if s.geminiAPIKey != "" {
		content, ti, to, err := s.client.ChatOnce(ctx, geminiBaseURL, s.geminiAPIKey, geminiModel, msgs, temperature)
		if err == nil {
			return content, ti, to, nil
		}
		s.log.Warnf("모델", "Gemini 호출 실패 → zen 폴백: %v", err)
	}
	// 3순위: zen
	if s.openCodeKey != "" && (model == zenModelFree || model == mimoModelFree) {
		content, ti, to, err := s.client.ChatOnce(ctx, zenBaseURL, s.openCodeKey, model, msgs, temperature)
		if err == nil {
			return content, ti, to, nil
		}
		s.log.Warnf("모델", "OpenCode Zen 호출 실패(%s) → %s 폴백: %v", model, orFallback, err)
		model = orFallback
	}
	content, ti, to, err := s.client.ChatOnce(ctx, "", "", model, msgs, temperature)
	if err != nil {
		// 무료 모델 불안정 대비 1회 재시도 (openrouter/free 최종 라우터)
		s.log.Warnf("모델", "모델 호출 실패(%s), openrouter/free 재시도: %v", model, err)
		content, ti, to, err = s.client.ChatOnce(ctx, "", "", openRouterFree, msgs, temperature)
	}
	if err != nil {
		s.log.Errorf("모델", "최종 모델 호출 실패(%s): %v", model, err)
	}
	return content, ti, to, err
}
// Handler 미들웨어가 감싼 최종 핸들러
func (s *Server) Handler() http.Handler {
	return s.recoverMW(s.corsMW(s.logMW(s.mux)))
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/v1/auth/register", s.handleRegister)

	// 디버그 전용 (T-25): log_level=debug일 때만 노출
	if s.cfg.LogLevel == "debug" {
		s.mux.HandleFunc("GET /api/v1/debug/health", s.handleDebugHealth)
		s.mux.HandleFunc("GET /api/v1/debug/logs", s.handleDebugLogs)
	}

	// 인증 필요 (T-13 미들웨어)
	guard := s.authGuard
	s.mux.HandleFunc("GET /api/v1/models", guard(s.handleListModels))
	s.mux.HandleFunc("POST /api/v1/models/custom", guard(s.handleCreateCustomModel))
	s.mux.HandleFunc("GET /api/v1/models/custom/{id}", guard(s.handleCustomModel))
	s.mux.HandleFunc("PUT /api/v1/models/custom/{id}", guard(s.handleCustomModel))
	s.mux.HandleFunc("DELETE /api/v1/models/custom/{id}", guard(s.handleCustomModel))
	s.mux.HandleFunc("GET /api/v1/quota", guard(s.handleQuota))
	s.mux.HandleFunc("GET /api/v1/settings/keys", guard(s.handleListKeys))
	s.mux.HandleFunc("POST /api/v1/settings/keys", guard(s.handleCreateKey))
	s.mux.HandleFunc("DELETE /api/v1/settings/keys/{id}", guard(s.handleDeleteKey))

	// T-22: 캐릭터/세션/메시지
	s.mux.HandleFunc("GET /api/v1/characters", guard(s.handleListCharacters))
	s.mux.HandleFunc("POST /api/v1/characters", guard(s.handleCreateCharacter))
	s.mux.HandleFunc("POST /api/v1/characters/generate", guard(s.handleGenerateCharacter))
	s.mux.HandleFunc("POST /api/v1/characters/{id}/avatar", guard(s.handleRegenerateAvatar))
	s.mux.HandleFunc("GET /api/v1/characters/{id}", guard(s.handleCharacter))
	s.mux.HandleFunc("PUT /api/v1/characters/{id}", guard(s.handleCharacter))
	s.mux.HandleFunc("DELETE /api/v1/characters/{id}", guard(s.handleCharacter))
	s.mux.HandleFunc("GET /api/v1/sessions", guard(s.handleListSessions))
	s.mux.HandleFunc("POST /api/v1/sessions", guard(s.handleCreateSession))
	s.mux.HandleFunc("GET /api/v1/sessions/{id}", guard(s.handleSession))
	s.mux.HandleFunc("PUT /api/v1/sessions/{id}", guard(s.handleSession))
	s.mux.HandleFunc("DELETE /api/v1/sessions/{id}", guard(s.handleSession))
	s.mux.HandleFunc("GET /api/v1/sessions/{id}/messages", guard(s.handleListMessages))

	// T-23: 규칙
	s.mux.HandleFunc("GET /api/v1/rules", guard(s.handleListRules))
	s.mux.HandleFunc("POST /api/v1/rules", guard(s.handleCreateRule))
	s.mux.HandleFunc("GET /api/v1/rules/{id}", guard(s.handleRule))
	s.mux.HandleFunc("PUT /api/v1/rules/{id}", guard(s.handleRule))
	s.mux.HandleFunc("DELETE /api/v1/rules/{id}", guard(s.handleRule))
	s.mux.HandleFunc("POST /api/v1/rules/{id}/default", guard(s.handleSetDefaultRule))

	// T-17/18: SSE 채팅
	s.mux.HandleFunc("POST /api/v1/sessions/{id}/chat", guard(s.handleChat))

	// T-19: 기억
	s.mux.HandleFunc("GET /api/v1/memories/{characterId}", guard(s.handleListMemories))
	s.mux.HandleFunc("POST /api/v1/memories/{characterId}", guard(s.handleCreateMemory))
	s.mux.HandleFunc("PUT /api/v1/memories/{id}", guard(s.handleMemory))
	s.mux.HandleFunc("DELETE /api/v1/memories/{id}", guard(s.handleMemory))
}

// authGuard 인증 미들웨어 래퍼 (auth 비활성 시 401)
func (s *Server) authGuard(next func(w http.ResponseWriter, r *http.Request, userID uuid.UUID)) http.HandlerFunc {
	if s.auth == nil {
		return func(w http.ResponseWriter, r *http.Request) {
			writeErr(w, errs.New(errs.EComAuth1001, nil))
		}
	}
	return s.auth.Middleware(next)
}

// ---- 핸들러 ----

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": "1.0.0",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

// handleRegister 익명 기기 등록: POST { "device_id": "..." } → { user_id, token }
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, errs.New(errs.EComValid1001, err))
		return
	}
	if req.DeviceID == "" {
		writeErr(w, errs.New(errs.EComValid1001, nil))
		return
	}
	if s.auth == nil {
		writeErr(w, errs.New(errs.ESRVDb1001, nil))
		return
	}
	userID, token, err := s.auth.RegisterOrLogin(r.Context(), req.DeviceID)
	if err != nil {
		if ae, ok := err.(*errs.AppError); ok {
			writeErr(w, ae)
		} else {
			writeErr(w, errs.New(errs.ESRVDb1001, err))
		}
		return
	}
	s.log.Feature("인증API", "기기 등록 완료 (user=%s)", userID)
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":    userID.String(),
		"device_id":  req.DeviceID,
		"token":      token,
		"expires_in": 30 * 24 * 3600,
	})
}

// ---- 헬퍼 ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErr errs.AppError 응답
func writeErr(w http.ResponseWriter, e *errs.AppError) {
	body, _ := e.ToJSON()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(e.HTTP)
	_, _ = w.Write(body)
}

// ---- 미들웨어 ----

// logMW 요청 로깅 (path, method, status, duration)
func (s *Server) logMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		s.log.Debugf("HTTP", "%s %s → %d (%s)", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Millisecond))
	})
}

// corsMW CORS 처리
func (s *Server) corsMW(next http.Handler) http.Handler {
	allowed := strings.Join(s.cfg.CORSOrigins, ", ")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowed)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// recoverMW 패닉 복구 (5xx 응답 + 에러 로그)
func (s *Server) recoverMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Errorf("PANIC", "panic: %v (path=%s)", rec, r.URL.Path)
				writeErr(w, errs.New(errs.ESRVDb1001, nil))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// statusWriter 상태 코드 캡처용 (Flusher 지원 — SSE 대응)
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Flush() {
	if f, ok := sw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
