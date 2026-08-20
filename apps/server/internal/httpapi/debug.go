// debug.go — 디버그 전용 핸들러 (log_level=debug일 때만 라우트 등록, T-25)
package httpapi

import (
	"fmt"
	"net/http"
	"time"
)

// ---- GET /api/v1/debug/health — 서버 상태 요약 ----
func (s *Server) handleDebugHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"server_time": time.Now().Format(time.RFC3339),
		"log_level":   s.cfg.LogLevel,
		"db_ok":       s.store != nil,
		"gemini_key":  s.cfg.GeminiAPIKey != "",
		"enc_key":     s.cfg.EncKey != "",
	})
}

// ---- GET /api/v1/debug/logs — 최근 로그 N건 (기본 100, 최대 200) ----
func (s *Server) handleDebugLogs(w http.ResponseWriter, r *http.Request) {
	n := 100
	if v := r.URL.Query().Get("n"); v != "" {
		if parsed, err := parseInt(v); err == nil {
			n = parsed
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": s.log.Recent(n)})
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}