// Package auth — 익명 기기 JWT 인증 (v1.0: 기기 ID 기반)
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/talkmance/server/internal/errs"
	"github.com/talkmance/server/internal/log"
)

// 토큰 수명: access 30일 (개인 서비스 단순화 — DESIGN 9장)
const accessTTL = 30 * 24 * time.Hour

// Service 인증 서비스
type Service struct {
	pool   *pgxpool.Pool
	secret []byte
	logger *log.Logger
}

// New 인증 서비스 생성
func New(pool *pgxpool.Pool, secret string, logger *log.Logger) (*Service, error) {
	if secret == "" {
		return nil, fmt.Errorf("auth: JWT_SECRET이 설정되지 않았습니다")
	}
	return &Service{pool: pool, secret: []byte(secret), logger: logger}, nil
}

// Claims JWT 클레임
type Claims struct {
	UserID   uuid.UUID `json:"uid"`
	DeviceID string    `json:"dev"`
	jwt.RegisteredClaims
}

// RegisterOrLogin 기기 ID로 사용자 조회, 없으면 생성 후 JWT 발급
func (s *Service) RegisterOrLogin(ctx context.Context, deviceID string) (uuid.UUID, string, error) {
	var userID uuid.UUID
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (device_id) VALUES ($1)
		 ON CONFLICT (device_id) DO UPDATE SET device_id = EXCLUDED.device_id
		 RETURNING id`, deviceID).Scan(&userID)
	if err != nil {
		return uuid.Nil, "", errs.Wrap(errs.ESRVDb1001, err)
	}

	token, err := s.issueToken(userID, deviceID)
	if err != nil {
		return uuid.Nil, "", err
	}
	s.logger.Feature("인증", "기기 등록/로그인 완료 (user=%s)", userID)
	return userID, token, nil
}

// issueToken JWT 발급 (HS256)
func (s *Service) issueToken(userID uuid.UUID, deviceID string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		DeviceID: deviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "talkmance-server",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", errs.Wrap(errs.ESRVNet1001, err)
	}
	return signed, nil
}

// Parse 검증 + 클레임 추출
func (s *Service) Parse(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("auth: 예상치 못한 서명 방식 %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errs.New(errs.EComAuth1002, err)
		}
		return nil, errs.New(errs.EComAuth1001, err)
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errs.New(errs.EComAuth1001, nil)
	}
	return claims, nil
}

// ErrNoAuth 인증 헤더 없음 (핸들러에서 401 처리용)
var ErrNoAuth = errors.New("auth: Authorization 헤더 없음")

// Middleware Authorization: Bearer <token> 검증 → next(userID)
func (s *Service) Middleware(next func(w http.ResponseWriter, r *http.Request, userID uuid.UUID)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenStr := bearerToken(r.Header.Get("Authorization"))
		if tokenStr == "" {
			writeErr(w, errs.New(errs.EComAuth1001, ErrNoAuth))
			return
		}
		claims, err := s.Parse(tokenStr)
		if err != nil {
			writeErr(w, errs.New(errs.EComAuth1001, err))
			return
		}
		next(w, r, claims.UserID)
	}
}

func bearerToken(h string) string {
	const prefix = "Bearer "
	if len(h) > len(prefix) && h[:len(prefix)] == prefix {
		return h[len(prefix):]
	}
	return ""
}

func writeErr(w http.ResponseWriter, e *errs.AppError) {
	body, _ := e.ToJSON()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(e.HTTP)
	_, _ = w.Write(body)
}

// EnsureUser 존재 확인 (등록되지 않은 기기 접근 방어 — blocks 기능 예비)
func (s *Service) EnsureUser(ctx context.Context, userID uuid.UUID) error {
	var one int
	err := s.pool.QueryRow(ctx, `SELECT 1 FROM users WHERE id = $1`, userID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return errs.New(errs.EComAuth1001, err)
	}
	if err != nil {
		return errs.Wrap(errs.ESRVDb1001, err)
	}
	return nil
}
