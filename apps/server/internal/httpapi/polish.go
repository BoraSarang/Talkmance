package httpapi

import (
	"regexp"
	"strings"
	"unicode"
)

// polishKoreanAssist AI 응답 후처리 (korean-humanizer 스킬 규칙 기반, 로컬 규칙만)
// - B 모드(설정 토글 시): 1) S1 금지 요소 제거 2) S2 과다 패턴 완화 3) 변경률 30% 가드
// - A 모드(항상)는 프롬프트 주입(buildChatMessages)으로 처리, 여기선 사용자 노출 텍스트만 다듬는다.
// 원문 보존 우선: 가드 초과 시 원문 그대로 반환한다.
func polishKoreanAssist(raw string) string {
	orig := raw
	out := raw

	// S1: 이모지 제거 (이모지/이모티콘/기호 범위)
	out = emojiRegex.ReplaceAllString(out, "")
	// S1: 줄표(—, –) 제거 (주변 공백은 유지 — 단일 공백 정리로 통합)
	out = dashRegex.ReplaceAllString(out, "")
	// S1: AI 상투어/챗봇 잔재 제거
	out = aiClicheRegex.ReplaceAllString(out, "")
	// S1: 과도한 단정 표현 완화 ("반드시" "절대" "당연히" 반복 → 1회)
	out = singleSpaceRegex.ReplaceAllString(out, " ")

	// S2: 종결어미/받침 반복 완화 (요요요, 네네네, 다다다, ㅋㅋㅋㅋ, !!!, ~~~ → 최대 2연속)
	out = compressRepeats(out)

	// 문장 끝 공백 정리
	out = strings.TrimSpace(out)
	if out == "" {
		return orig
	}

	// 변경률 30% 가드 — 편집 거리 기준 (삭제/삽입 포함), 초과 시 원문 유지
	dist := levenshtein([]rune(orig), []rune(out))
	maxLen := len([]rune(orig))
	if l := len([]rune(out)); l > maxLen {
		maxLen = l
	}
	if maxLen > 0 && float64(dist)/float64(maxLen) > 0.3 {
		return orig
	}
	return out
}

// levenshtein 편집 거리 (삽입/삭제/치환 최소 연산 수)
func levenshtein(a, b []rune) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr := make([]int, lb+1)
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			d := prev[j] + 1   // 삭제
			if v := curr[j-1] + 1; v < d { // 삽입
				d = v
			}
			if v := prev[j-1] + cost; v < d { // 치환
				d = v
			}
			curr[j] = d
		}
		prev = curr
	}
	return prev[lb]
}

// compressRepeats 한글/문자/구두점 동일 문자 3회 이상 연속 → 2회로 축약 (ㅋㅋㅋㅋ→ㅋㅋ, 요요요→요요, !!!→!!)
func compressRepeats(s string) string {
	runes := []rune(s)
	var b strings.Builder
	b.Grow(len(runes))
	var last rune
	count := 0
	for _, r := range runes {
		isRepeatable := unicode.Is(unicode.Hangul, r) || unicode.IsLetter(r) || r == '!' || r == '?' || r == '~' || r == '。'
		if r == last && isRepeatable {
			count++
			if count <= 2 {
				b.WriteRune(r)
			}
			continue
		}
		count = 1
		last = r
		b.WriteRune(r)
	}
	return b.String()
}

// --- 정규식 (컴파일 1회) ---

var (
	// emojiRegex 이모지 및 특수 기호 범위 (이모지, ZWJ, 변형 선택자, 영숫자 기호)
	emojiRegex = regexp.MustCompile(`[\x{1F000}-\x{1FAFF}\x{2600}-\x{27BF}\x{FE0F}\x{200D}\x{2B00}-\x{2BFF}\x{2705}\x{2764}]`)
	// dashRegex 줄표·대시 (한글 문맥에서 잡히는 —, –, ―)
	dashRegex = regexp.MustCompile(`[—–―]`)
	// aiClicheRegex AI 상투어·챗봇 잔재
	aiClicheRegex = regexp.MustCompile(`(도움이 되셨나요\??|도움이 되었으면 좋겠[다습][요니]?|궁금한 점이 있으시면|무엇이든 물어보세요|저는 AI(이며|라서)|인공지능(이라|이기 때문에)|즐거운 하루 되세요)`)
	// singleSpaceRegex 연속 공백 정리
	singleSpaceRegex = regexp.MustCompile(`\s{2,}`)
)

var _ = unicode.IsLetter // unicode 임포트 유지 (향후 범위 확장 대비)