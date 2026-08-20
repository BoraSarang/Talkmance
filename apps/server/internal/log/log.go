// Package log — 구조화 JSON 로거 (19.1장: 서버 로거 표준)
package log

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level 로그 레벨
type Level int

const (
	Debug Level = iota
	Info
	Warn
	Error
)

// Logger 스레드세이프 구조화 로거
type Logger struct {
	mu      sync.Mutex
	out     io.Writer
	level   Level
	recent  []entry
	maxRec  int
}

// Entry 최근 로그 항목 (링 버퍼)
type Entry = entry

// New 레벨 문자열("debug|info|warn|error")로 로거 생성
func New(levelStr string) *Logger {
	return &Logger{out: os.Stdout, level: parseLevel(levelStr), maxRec: 200}
}

// Recent 최근 로그 N건 반환 (신→구)
func (l *Logger) Recent(n int) []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	if n <= 0 || n > len(l.recent) {
		n = len(l.recent)
	}
	out := make([]Entry, n)
	for i := 0; i < n; i++ {
		out[i] = l.recent[len(l.recent)-1-i]
	}
	return out
}

func parseLevel(s string) Level {
	switch s {
	case "debug":
		return Debug
	case "warn":
		return Warn
	case "error":
		return Error
	default:
		return Info
	}
}

// entry 로그 라인 구조
type entry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Tag     string `json:"tag,omitempty"`
	Message string `json:"message"`
}

func (l *Logger) write(level Level, tag, format string, args ...any) {
	if level < l.level {
		return
	}
	e := entry{
		Time:    time.Now().Format("2006-01-02 15:04:05.000"),
		Level:   levelName(level),
		Tag:     tag,
		Message: fmt.Sprintf(format, args...),
	}
	b, _ := json.Marshal(e)
	l.mu.Lock()
	defer l.mu.Unlock()
	l.recent = append(l.recent, e)
	if len(l.recent) > l.maxRec {
		l.recent = l.recent[len(l.recent)-l.maxRec:]
	}
	l.out.Write(append(b, '\n'))
}

func levelName(l Level) string {
	switch l {
	case Debug:
		return "debug"
	case Warn:
		return "warn"
	case Error:
		return "error"
	default:
		return "info"
	}
}

// Debugf 디버그 로그
func (l *Logger) Debugf(tag, format string, args ...any) { l.write(Debug, tag, format, args...) }

// Infof 정보 로그
func (l *Logger) Infof(tag, format string, args ...any) { l.write(Info, tag, format, args...) }

// Warnf 경고 로그
func (l *Logger) Warnf(tag, format string, args ...any) { l.write(Warn, tag, format, args...) }

// Errorf 에러 로그
func (l *Logger) Errorf(tag, format string, args ...any) { l.write(Error, tag, format, args...) }

// Feature 신규 기능 로그 (19.1장 의무화): [FEATURE] <기능명> 진입/완료
func (l *Logger) Feature(name, format string, args ...any) {
	l.write(Info, "FEATURE", "[%s] %s", name, fmt.Sprintf(format, args...))
}
