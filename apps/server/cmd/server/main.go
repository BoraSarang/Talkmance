// 톡맨스 서버 엔트리포인트
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/talkmance/server/internal/auth"
	"github.com/talkmance/server/internal/config"
	"github.com/talkmance/server/internal/db"
	"github.com/talkmance/server/internal/errs"
	"github.com/talkmance/server/internal/httpapi"
	"github.com/talkmance/server/internal/log"
	"github.com/talkmance/server/internal/orapi"
	"github.com/talkmance/server/internal/store"
)

// ctx0 마이그레이션/연결용 기본 컨텍스트
func ctx0() context.Context {
	return context.Background()
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "서버 종료:", err)
		os.Exit(1)
	}
}

func run() error {
	// T-24: config + 에러 메시지 로딩
	cfg, err := config.Load(".env")
	if err != nil {
		return fmt.Errorf("config 로딩 실패: %w", err)
	}
	if err := errs.LoadMessages(cfg.ErrorMessagesFile); err != nil {
		return err
	}

	// T-10: 구조화 JSON 로거
	logger := log.New(cfg.LogLevel)
	logger.Feature("서버시작", "톡맨스 서버 기동 (port=%d, log_level=%s)", cfg.Port, cfg.LogLevel)

	// T-11/T-13~16: DB 연결 + 인증 + store + 카탈로그/할당량 (DATABASE_URL 없으면 인증 비활성)
	var authSvc *auth.Service
	var st *store.Store
	var cat *orapi.Catalog
	var quota *orapi.QuotaService
	if cfg.DatabaseURL != "" {
		pool, err := db.Connect(ctx0(), cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer pool.Close()
		authSvc, err = auth.New(pool, cfg.JWTSecret, logger)
		if err != nil {
			return err
		}
		st = store.New(pool)
		logger.Feature("DB", "Neon 연결 완료 (인증/스토어 활성화)")
	} else {
		logger.Warnf("DB", "DATABASE_URL 없음 — 인증 비활성 상태로 기동 (마이그레이션 필요 시 MIGRATE=up)")
	}

	// T-14: OpenRouter 클라이언트 + 카탈로그/할당량
	var orc *orapi.Client
	if cfg.OpenRouterKey != "" {
		orc = orapi.New(cfg.OpenRouterKey, logger)
		cat = orapi.NewCatalog(orc)
		quota = orapi.NewQuotaService(orc)
		logger.Feature("모델", "OpenRouter 클라이언트 초기화됨")
	} else {
		logger.Warnf("모델", "OPENROUTER_API_KEY 없음 — 모델/할당량 비활성")
	}
	if cfg.OpenCodeKey != "" {
		logger.Feature("모델", "OpenCode Zen 키 설정됨 (free 모델 우선 사용)")
	}

	// T-11: 마이그레이션 모드 (-migrate up|down)
	if action := os.Getenv("MIGRATE"); action != "" {
		return runMigrate(cfg, logger, action)
	}

	// T-10: 라우터 + 서버
	api := httpapi.New(cfg, logger, authSvc, st, orc, cat, quota)
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           api.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Infof("HTTP", "리스닝 시작 :%d", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("HTTP 서버 오류: %w", err)
	case <-ctx.Done():
		logger.Infof("HTTP", "종료 신호 수신, graceful shutdown 시작")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown 실패: %w", err)
		}
		logger.Feature("서버종료", "톡맨스 서버 정상 종료")
		return nil
	}
}

// runMigrate MIGRATE=up|down 모드: 마이그레이션만 실행 후 종료
func runMigrate(cfg *config.Config, logger *log.Logger, action string) error {
	if cfg.DatabaseURL == "" {
		return fmt.Errorf("migrate: DATABASE_URL이 설정되지 않았습니다")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	migs, err := db.LoadMigrations("migrations")
	if err != nil {
		return err
	}

	switch action {
	case "up":
		done, err := db.MigrateUp(ctx, pool, migs)
		if err != nil {
			return err
		}
		if len(done) == 0 {
			logger.Infof("MIGRATE", "적용할 마이그레이션 없음 (최신 상태)")
		} else {
			logger.Feature("마이그레이션", "up 적용 완료: %v", done)
		}
	case "down":
		done, err := db.MigrateDown(ctx, pool, migs)
		if err != nil {
			return err
		}
		if len(done) == 0 {
			logger.Infof("MIGRATE", "롤백할 마이그레이션 없음")
		} else {
			logger.Feature("마이그레이션", "down 롤백 완료: %v", done)
		}
	default:
		return fmt.Errorf("migrate: 지원 액션은 up|down 입니다 (입력: %q)", action)
	}
	return nil
}
