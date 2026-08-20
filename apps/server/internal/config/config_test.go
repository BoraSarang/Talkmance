package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := `# 테스트
PORT=9999
DATABASE_URL=postgres://test
LOG_LEVEL=debug
CORS_ORIGINS=*, http://localhost:3000
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load 실패: %v", err)
	}
	if cfg.Port != 9999 {
		t.Errorf("Port=%d, want 9999", cfg.Port)
	}
	if cfg.DatabaseURL != "postgres://test" {
		t.Errorf("DatabaseURL=%q", cfg.DatabaseURL)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel=%q", cfg.LogLevel)
	}
	if len(cfg.CORSOrigins) != 2 {
		t.Errorf("CORSOrigins=%v", cfg.CORSOrigins)
	}
	if cfg.ErrorMessagesFile != "../../error_message_ko.json" {
		t.Errorf("ErrorMessagesFile=%q", cfg.ErrorMessagesFile)
	}
}

func TestEnvVarOverrides(t *testing.T) {
	t.Setenv("PORT", "1234")
	cfg, err := Load("없는파일.env")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 1234 {
		t.Errorf("Port=%d, want 1234 (환경변수 우선)", cfg.Port)
	}
}

func TestMissingEnvFileOK(t *testing.T) {
	cfg, err := Load("no-such-file.env")
	if err != nil {
		t.Fatalf(".env 없어도 에러 아님: %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("기본 Port=%d, want 8080", cfg.Port)
	}
}
