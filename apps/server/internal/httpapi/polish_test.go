package httpapi

import "testing"

func TestPolishKoreanAssist(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"이모지 제거", "안녕하세요! 😊 반가워요 ✨", "안녕하세요! 반가워요"},
		{"줄표 제거", "기다렸어 — 오늘 어땠어?", "기다렸어 오늘 어땠어?"},
		{"반복 자모 축약", "진짜 힘들었어 ㅋㅋㅋㅋ", "진짜 힘들었어 ㅋㅋ"},
		{"반복 구두점 축약", "정말?!!!", "정말?!!"},
		{"AI 상투어 전부면 원문 유지(가드)", "도움이 되셨나요? 무엇이든 물어보세요", "도움이 되셨나요? 무엇이든 물어보세요"},
		{"공백 정리", "안녕  자기야", "안녕 자기야"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := polishKoreanAssist(c.in)
			if got != c.want {
				t.Errorf("polish(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestPolishKoreanAssistGuard(t *testing.T) {
	// 변경률 30% 가드: 거의 전부 바뀌면 원문 유지 (이모지+상투어로 가득찬 문장)
	in := "😊😊😊 도움이 되셨나요? 😊😊😊"
	got := polishKoreanAssist(in)
	if got != in {
		t.Errorf("가드 실패: 변경률 초과인데 원문을 유지하지 않음: %q", got)
	}
}

func TestPolishKoreanAssistEmptyPreserve(t *testing.T) {
	// 빈 결과는 원문 유지 (모든 문자가 제거되는 경우)
	in := "😊✨❤"
	got := polishKoreanAssist(in)
	if got != in {
		t.Errorf("빈 결과 원문 유지 실패: %q", got)
	}
}

func TestPolishKoreanAssistPreserveMemoryTag(t *testing.T) {
	in := "오늘 힘들었어.\n\n[MEMORY_SAVE] 사용자는 회사에서 보고서를 다시 썼음"
	got := polishKoreanAssist(in)
	if got == "" {
		t.Fatal("빈 결과")
	}
	if !contains(got, "[MEMORY_SAVE]") {
		t.Errorf("MEMORY_SAVE 태그가 보존되지 않음: %q", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}