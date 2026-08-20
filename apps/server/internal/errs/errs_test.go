package errs

import (
	"net/http"
	"strings"
	"testing"
)

func TestLoadMessages(t *testing.T) {
	if err := LoadMessages("../../../../error_message_ko.json"); err != nil {
		t.Fatalf("메시지 로딩 실패: %v", err)
	}
	if Message(ESRVNet1001) == "" || Message(ESRVNet1001) == ESRVNet1001 {
		t.Fatalf("메시지 미로딩: %q", Message(ESRVNet1001))
	}
	if !strings.Contains(Message(ESRVNet1001), "네트워크") {
		t.Fatalf("메시지 내용 불일치: %q", Message(ESRVNet1001))
	}
}

func TestHTTPStatus(t *testing.T) {
	cases := []struct {
		code string
		want int
	}{
		{ESRVNet1001, http.StatusBadGateway},
		{ESRVDb1001, http.StatusInternalServerError},
		{EComAuth1001, http.StatusUnauthorized},
		{EComAuth2001, http.StatusForbidden},
		{EComQuota1001, http.StatusTooManyRequests},
		{EComValid1001, http.StatusBadRequest},
	}
	for _, c := range cases {
		e := New(c.code, nil)
		if e.HTTP != c.want {
			t.Errorf("%s: HTTP=%d, want %d", c.code, e.HTTP, c.want)
		}
	}
}

func TestToJSON(t *testing.T) {
	e := New(EComChat1001, nil)
	b, err := e.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON 실패: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, EComChat1001) || !strings.Contains(s, e.Message) {
		t.Fatalf("JSON 형태 불일치: %s", s)
	}
}

func TestWrapKeepsCause(t *testing.T) {
	cause := &testErr{"원인"}
	e := Wrap(ESRVDb1001, cause)
	if e.Err == nil {
		t.Fatal("원인 에러가 사라짐")
	}
	if !strings.Contains(e.Error(), "원인") {
		t.Fatalf("Error()에 원인 미포함: %s", e.Error())
	}
}

type testErr struct{ msg string }

func (e *testErr) Error() string { return e.msg }
