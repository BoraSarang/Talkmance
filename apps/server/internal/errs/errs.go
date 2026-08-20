// Package errs — 에러코드 체계 (E-{PLATFORM}-{CAT}-{NUM4}) + 사용자 메시지 매핑
// 메시지 원본: 루트 error_message_ko.json (이 파일과 동기화 필수)
package errs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
)

// 에러코드 정의 (error_message_ko.json 키와 동일)
const (
	// E-SRV-* : 서버 공통
	ESRVNet1001 = "E-SRV-NET-1001"
	ESRVNet1002 = "E-SRV-NET-1002"
	ESRVDb1001  = "E-SRV-DB-1001"

	// E-COM-AUTH-*
	EComAuth1001 = "E-COM-AUTH-1001"
	EComAuth1002 = "E-COM-AUTH-1002"
	EComAuth2001 = "E-COM-AUTH-2001"

	// E-COM-MODEL-*
	EComModel1001 = "E-COM-MODEL-1001"
	EComModel1002 = "E-COM-MODEL-1002"
	EComModel2001 = "E-COM-MODEL-2001"
	EComModel2002 = "E-COM-MODEL-2002"

	// E-COM-QUOTA-*
	EComQuota1001 = "E-COM-QUOTA-1001"
	EComQuota1002 = "E-COM-QUOTA-1002"

	// E-COM-CHAT-*
	EComChat1001 = "E-COM-CHAT-1001"
	EComChat1002 = "E-COM-CHAT-1002"

	// E-COM-MEM-*
	EComMem1001 = "E-COM-MEM-1001"
	EComMem1002 = "E-COM-MEM-1002"

	// E-COM-RULE-*
	EComRule1001 = "E-COM-RULE-1001"
	EComRule2001 = "E-COM-RULE-2001"
	EComRule2002 = "E-COM-RULE-2002"

	// E-COM-VALID-*
	EComValid1001 = "E-COM-VALID-1001"
	EComValid2001 = "E-COM-VALID-2001"
	EComValid2002 = "E-COM-VALID-2002"

	// E-COM-CHAR-*
	EComChar1001 = "E-COM-CHAR-1001"
	EComChar1002 = "E-COM-CHAR-1002"
	EComChar1003 = "E-COM-CHAR-1003"
	EComChar2001 = "E-COM-CHAR-2001"

	// E-COM-IMG-*
	EComImg1001 = "E-COM-IMG-1001"

	// E-COM-SESS-*
	EComSess1001 = "E-COM-SESS-1001"
	EComSess1002 = "E-COM-SESS-1002"
	EComSess2001 = "E-COM-SESS-2001"
)

// HTTP 상태 매핑: 2xxx = 4xx, 1xxx = 5xx, 그 외 400
var statusByCode = map[string]int{
	ESRVNet1001:   http.StatusBadGateway,
	ESRVNet1002:   http.StatusGatewayTimeout,
	ESRVDb1001:    http.StatusInternalServerError,
	EComAuth1001:  http.StatusUnauthorized,
	EComAuth1002:  http.StatusUnauthorized,
	EComAuth2001:  http.StatusForbidden,
	EComModel1001: http.StatusBadGateway,
	EComModel1002: http.StatusNotFound,
	EComModel2001: http.StatusBadRequest,
	EComModel2002: http.StatusBadRequest,
	EComQuota1001: http.StatusTooManyRequests,
	EComQuota1002: http.StatusBadGateway,
	EComChat1001:  http.StatusBadGateway,
	EComChat1002:  http.StatusNotFound,
	EComMem1001:   http.StatusInternalServerError,
	EComMem1002:   http.StatusNotFound,
	EComRule1001:  http.StatusNotFound,
	EComRule2001:  http.StatusBadRequest,
	EComRule2002:  http.StatusBadRequest,
	EComValid1001: http.StatusBadRequest,
	EComValid2001: http.StatusBadRequest,
	EComValid2002: http.StatusBadRequest,
	EComChar1001:  http.StatusNotFound,
	EComChar1002:  http.StatusNotFound,
	EComChar1003:  http.StatusBadGateway,
	EComChar2001:  http.StatusBadRequest,
	EComImg1001:   http.StatusBadGateway,
	EComSess1001:  http.StatusBadRequest,
	EComSess1002:  http.StatusNotFound,
	EComSess2001:  http.StatusBadRequest,
}

// AppError API 응답용 에러
type AppError struct {
	Code    string
	Message string
	HTTP    int
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Err }

// ResponseBody 에러 응답 JSON 형태
type ResponseBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// ToJSON 응답 직렬화
func (e *AppError) ToJSON() ([]byte, error) {
	body := ResponseBody{}
	body.Error.Code = e.Code
	body.Error.Message = e.Message
	return json.Marshal(body)
}

var (
	msgOnce sync.Once
	msgMap  map[string]string
	msgErr  error
)

// LoadMessages error_message_ko.json 로딩 (1회)
func LoadMessages(path string) error {
	msgOnce.Do(func() {
		data, err := os.ReadFile(path)
		if err != nil {
			msgErr = fmt.Errorf("errs: 메시지 파일 로딩 실패(%s): %w", path, err)
			return
		}
		m := map[string]string{}
		if err := json.Unmarshal(data, &m); err != nil {
			msgErr = fmt.Errorf("errs: 메시지 JSON 파싱 실패: %w", err)
			return
		}
		msgMap = m
	})
	return msgErr
}

// Message 코드 → 사용자 메시지 (없으면 코드 자체 반환)
func Message(code string) string {
	if msgMap != nil {
		if m, ok := msgMap[code]; ok {
			return m
		}
	}
	return code
}

// New AppError 생성
func New(code string, err error) *AppError {
	status, ok := statusByCode[code]
	if !ok {
		status = http.StatusBadRequest
	}
	return &AppError{Code: code, Message: Message(code), HTTP: status, Err: err}
}

// Wrap 서버 내부 에러를 사용자 에러로 래핑 (에러 로그용 원인 유지)
func Wrap(code string, cause error) *AppError {
	e := New(code, cause)
	e.Message = Message(code)
	return e
}
