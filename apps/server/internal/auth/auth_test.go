package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/talkmance/server/internal/errs"
)

const testSecret = "test-secret-32bytes-long-for-testing!!"

func newTestService() *Service {
	return &Service{secret: []byte(testSecret), logger: nil}
}

func TestIssueAndParse(t *testing.T) {
	s := newTestService()
	uid := uuid.New()
	token, err := s.issueToken(uid, "device-1")
	if err != nil {
		t.Fatalf("발급 실패: %v", err)
	}
	claims, err := s.Parse(token)
	if err != nil {
		t.Fatalf("검증 실패: %v", err)
	}
	if claims.UserID != uid {
		t.Errorf("UserID=%s, want %s", claims.UserID, uid)
	}
	if claims.DeviceID != "device-1" {
		t.Errorf("DeviceID=%q", claims.DeviceID)
	}
	if claims.Issuer != "talkmance-server" {
		t.Errorf("Issuer=%q", claims.Issuer)
	}
}

func TestParseExpired(t *testing.T) {
	s := newTestService()
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID: uuid.New(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "talkmance-server",
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Hour)),
		},
	})
	signed, _ := token.SignedString([]byte(testSecret))
	_, err := s.Parse(signed)
	if err == nil {
		t.Fatal("만료 토큰이 통과됨")
	}
	if ae, ok := err.(*errs.AppError); !ok || ae.Code != errs.EComAuth1002 {
		t.Errorf("만료 에러코드=%v, want %s", err, errs.EComAuth1002)
	}
}

func TestParseWrongSecret(t *testing.T) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{UserID: uuid.New()})
	signed, _ := token.SignedString([]byte("other-secret"))
	s := newTestService()
	if _, err := s.Parse(signed); err == nil {
		t.Fatal("잘못된 서명이 통과됨")
	}
}

func TestParseTampered(t *testing.T) {
	s := newTestService()
	token, err := s.issueToken(uuid.New(), "d")
	if err != nil {
		t.Fatal(err)
	}
	tampered := token[:len(token)-2] + "xx"
	if _, err := s.Parse(tampered); err == nil {
		t.Fatal("변조 토큰이 통과됨")
	}
}

func TestMiddleware(t *testing.T) {
	s := newTestService()
	uid := uuid.New()
	token, _ := s.issueToken(uid, "d")

	handler := s.Middleware(func(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(userID.String()))
	})

	// 정상
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), uid.String()) {
		t.Errorf("정상 케이스 실패: code=%d body=%s", rec.Code, rec.Body.String())
	}

	// 헤더 없음 → 401
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	rec2 := httptest.NewRecorder()
	handler(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Errorf("헤더 없음: code=%d, want 401", rec2.Code)
	}

	// 잘못된 토큰 → 401
	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	req3.Header.Set("Authorization", "Bearer bad.token.here")
	rec3 := httptest.NewRecorder()
	handler(rec3, req3)
	if rec3.Code != http.StatusUnauthorized {
		t.Errorf("잘못된 토큰: code=%d, want 401", rec3.Code)
	}
}

func TestBearerToken(t *testing.T) {
	cases := []struct{ h, want string }{
		{"Bearer abc", "abc"},
		{"bearer abc", ""},
		{"", ""},
		{"abc", ""},
		{"Bearer ", ""},
	}
	for _, c := range cases {
		if got := bearerToken(c.h); got != c.want {
			t.Errorf("bearerToken(%q)=%q, want %q", c.h, got, c.want)
		}
	}
}
