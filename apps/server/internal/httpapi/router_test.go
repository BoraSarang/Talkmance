package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/talkmance/server/internal/config"
	"github.com/talkmance/server/internal/log"
)

func newTestServer() *Server {
	cfg := &config.Config{Port: 8080, LogLevel: "error", CORSOrigins: []string{"*"}}
	return New(cfg, log.New("error"), nil, nil, nil, nil, nil)
}

func TestHealth(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status 필드=%v", body["status"])
	}
	if body["version"] != "1.0.0" {
		t.Fatalf("version=%v", body["version"])
	}
}

func TestHealthCORS(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("CORS 헤더=%q", got)
	}
}

func TestPreflightOPTIONS(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("OPTIONS status=%d, want 204", rec.Code)
	}
}

func TestNotFound(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/nothing", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404", rec.Code)
	}
}

func TestRecoverMW(t *testing.T) {
	s := newTestServer()
	s.mux.HandleFunc("GET /panic", func(w http.ResponseWriter, r *http.Request) {
		panic("테스트 패닉")
	})
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status=%d, want 500", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if len(body) == 0 {
		t.Error("에러 응답 바디 없음")
	}
}
