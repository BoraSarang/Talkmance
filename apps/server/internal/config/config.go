// Package config — .env/환경변수 로딩 (stdlib 파싱, godotenv 불필요)
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config 서버 전역 설정
type Config struct {
	Port          int
	DatabaseURL   string
	OpenRouterKey string
	OpenCodeKey   string
	GeminiAPIKey  string
	NVIDIAAPIKey  string
	EncKey        string
	JWTSecret     string
	LogLevel      string
	CORSOrigins   []string
	// ErrorMessagesFile: error_message_ko.json 경로 (개발: ../../, 배포: env로 지정)
	ErrorMessagesFile string
}

// Load .env 파일 + 환경변수 병합 (환경변수 우선)
func Load(envFile string) (*Config, error) {
	env, err := parseDotEnv(envFile)
	if err != nil {
		return nil, fmt.Errorf("config: .env 로딩 실패: %w", err)
	}

	cfg := &Config{
		Port:              8080,
		DatabaseURL:       "",
		OpenRouterKey:     "",
		OpenCodeKey:       "",
		NVIDIAAPIKey:      "",
		EncKey:            "",
		JWTSecret:         "",
		LogLevel:          "info",
		CORSOrigins:       []string{"*"},
		ErrorMessagesFile: "../../error_message_ko.json",
	}

	// .env 값 적용
	if v, ok := env["PORT"]; ok && v != "" {
		p, perr := strconv.Atoi(v)
		if perr != nil {
			return nil, fmt.Errorf("config: PORT가 숫자가 아닙니다: %q", v)
		}
		cfg.Port = p
	}
	if v := env["DATABASE_URL"]; v != "" {
		cfg.DatabaseURL = v
	}
	if v := env["OPENROUTER_API_KEY"]; v != "" {
		cfg.OpenRouterKey = v
	}
	if v := env["OPENCODE_API_KEY"]; v != "" {
		cfg.OpenCodeKey = v
	}
	if v := env["GEMINI_API_KEY"]; v != "" {
		cfg.GeminiAPIKey = v
	}
	if v := env["NVIDIA_API_KEY"]; v != "" {
		cfg.NVIDIAAPIKey = v
	}
	if v := env["ENC_KEY"]; v != "" {
		cfg.EncKey = v
	}
	if v := env["JWT_SECRET"]; v != "" {
		cfg.JWTSecret = v
	}
	if v := env["LOG_LEVEL"]; v != "" {
		cfg.LogLevel = v
	}
	if v := env["CORS_ORIGINS"]; v != "" {
		cfg.CORSOrigins = splitCSV(v)
	}
	if v := env["ERROR_MESSAGES_FILE"]; v != "" {
		cfg.ErrorMessagesFile = v
	}

	// 환경변수 우선 (배포용: Render env)
	if v := os.Getenv("PORT"); v != "" {
		if p, perr := strconv.Atoi(v); perr == nil {
			cfg.Port = p
		}
	}
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.DatabaseURL = v
	}
	if v := os.Getenv("OPENROUTER_API_KEY"); v != "" {
		cfg.OpenRouterKey = v
	}
	if v := os.Getenv("OPENCODE_API_KEY"); v != "" {
		cfg.OpenCodeKey = v
	}
	if v := os.Getenv("GEMINI_API_KEY"); v != "" {
		cfg.GeminiAPIKey = v
	}
	if v := os.Getenv("NVIDIA_API_KEY"); v != "" {
		cfg.NVIDIAAPIKey = v
	}
	if v := os.Getenv("ENC_KEY"); v != "" {
		cfg.EncKey = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWTSecret = v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("CORS_ORIGINS"); v != "" {
		cfg.CORSOrigins = splitCSV(v)
	}
	if v := os.Getenv("ERROR_MESSAGES_FILE"); v != "" {
		cfg.ErrorMessagesFile = v
	}

	return cfg, nil
}

// parseDotEnv .env 파일 파싱 (# 주석, KEY=VALUE)
func parseDotEnv(path string) (map[string]string, error) {
	env := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return env, nil // .env 없으면 환경변수만 사용
		}
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		env[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return env, sc.Err()
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
